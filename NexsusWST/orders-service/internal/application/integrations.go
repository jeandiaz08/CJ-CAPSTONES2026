package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/nexuswst/orders-service/internal/domain"
	"github.com/nexuswst/orders-service/internal/ports"
	"github.com/nexuswst/orders-service/internal/shared/tenant"
)

// Proveedores con adaptador. «generic» = formato propio de Nexus (la orden
// completa en JSON, firmada); «mercadolibre» avisa solo la referencia y el
// detalle se consulta por API (solo lectura, DAS §5.4).
var ChannelProviders = []string{"generic", "mercadolibre"}
var CourierProviders = []string{"generic", "chilexpress", "blueexpress", "starken"}

var reProvider = regexp.MustCompile(`^[a-z0-9_-]{2,30}$`)

// ---------------------------------------------------------------------------
// Configuración (RF-05, H-06 T04): solo Administrador
// ---------------------------------------------------------------------------

type IntegrationWithSecret struct {
	ports.Integration
	Secret     string `json:"secret,omitempty"` // se muestra UNA vez, al crear o rotar
	WebhookURL string `json:"webhookPath"`
}

func webhookPath(i ports.Integration) string {
	return fmt.Sprintf("/webhooks/%s/%d", i.Provider, i.ID)
}

func (s *Service) ListIntegrations(ctx context.Context, id tenant.Identity, scope []int64) ([]IntegrationWithSecret, error) {
	if !id.HasRole(tenant.RoleAdmin) {
		return nil, domain.ErrForbidden
	}
	list, err := s.Integrations.List(ctx, scope)
	out := make([]IntegrationWithSecret, 0, len(list))
	for _, i := range list {
		out = append(out, IntegrationWithSecret{Integration: i, WebhookURL: webhookPath(i)})
	}
	return out, err
}

func (s *Service) CreateIntegration(ctx context.Context, id tenant.Identity, in ports.Integration) (IntegrationWithSecret, error) {
	if !id.HasRole(tenant.RoleAdmin) {
		return IntegrationWithSecret{}, domain.ErrForbidden
	}
	in.Provider = strings.ToLower(strings.TrimSpace(in.Provider))
	in.Kind = strings.ToLower(strings.TrimSpace(in.Kind))
	if in.Kind == "" {
		in.Kind = "channel"
	}
	in.Channel = strings.ToLower(strings.TrimSpace(in.Channel))
	in.DefaultCourier = strings.ToLower(strings.TrimSpace(in.DefaultCourier))
	if in.CompanyID <= 0 || !id.CanAccess(in.CompanyID) {
		return IntegrationWithSecret{}, fail(domain.ErrInvalid, "Indica una empresa válida.")
	}
	allowed := ChannelProviders
	if in.Kind == "courier" {
		allowed = CourierProviders
	} else if in.Kind != "channel" {
		return IntegrationWithSecret{}, fail(domain.ErrInvalid, "Tipo de integración: channel o courier.")
	}
	ok := false
	for _, p := range allowed {
		ok = ok || p == in.Provider
	}
	if !ok || !reProvider.MatchString(in.Provider) {
		return IntegrationWithSecret{}, fail(domain.ErrInvalid, "Proveedor no soportado: %s", strings.Join(allowed, ", "))
	}
	if in.Kind == "channel" {
		if in.Channel == "" {
			in.Channel = in.Provider
		}
		if in.DefaultCourier != "" {
			if active, err := s.Catalog.CourierActive(ctx, in.DefaultCourier); err != nil {
				return IntegrationWithSecret{}, err
			} else if !active {
				return IntegrationWithSecret{}, fail(domain.ErrInvalid, "El courier %s no existe o está inactivo.", in.DefaultCourier)
			}
		}
	} else {
		in.Channel = in.Provider
	}
	secret := NewSecret()
	enc, err := s.Secrets.Seal(secret)
	if err != nil {
		return IntegrationWithSecret{}, err
	}
	in.SecretEnc, in.Active, in.CreatedBy = enc, true, id.Actor()
	saved, err := s.Integrations.Save(ctx, in)
	if err != nil {
		return IntegrationWithSecret{}, err
	}
	return IntegrationWithSecret{Integration: saved, Secret: secret, WebhookURL: webhookPath(saved)}, nil
}

func (s *Service) RotateIntegrationSecret(ctx context.Context, id tenant.Identity, scope []int64, integrationID int64) (IntegrationWithSecret, error) {
	if !id.HasRole(tenant.RoleAdmin) {
		return IntegrationWithSecret{}, domain.ErrForbidden
	}
	in, err := s.integrationInScope(ctx, scope, integrationID)
	if err != nil {
		return IntegrationWithSecret{}, err
	}
	secret := NewSecret()
	if in.SecretEnc, err = s.Secrets.Seal(secret); err != nil {
		return IntegrationWithSecret{}, err
	}
	saved, err := s.Integrations.Save(ctx, in)
	return IntegrationWithSecret{Integration: saved, Secret: secret, WebhookURL: webhookPath(saved)}, err
}

func (s *Service) SetIntegrationActive(ctx context.Context, id tenant.Identity, scope []int64, integrationID int64, active bool) error {
	if !id.HasRole(tenant.RoleAdmin) {
		return domain.ErrForbidden
	}
	if _, err := s.integrationInScope(ctx, scope, integrationID); err != nil {
		return err
	}
	return s.Integrations.SetActive(ctx, integrationID, active)
}

func (s *Service) integrationInScope(ctx context.Context, scope []int64, integrationID int64) (ports.Integration, error) {
	in, err := s.Integrations.Get(ctx, integrationID)
	if err != nil {
		return in, err
	}
	if scope != nil {
		for _, c := range scope {
			if c == in.CompanyID {
				return in, nil
			}
		}
		return ports.Integration{}, domain.ErrNotFound
	}
	return in, nil
}

// ---------------------------------------------------------------------------
// Recepción de webhooks (DA-14): sin JWT, secreto o firma por integración
// ---------------------------------------------------------------------------

var ErrWebhookAuth = errors.New("solicitud de integración no autenticada")

// ReceiveWebhook autentica, registra y encola. Responde de inmediato (Mercado
// Libre exige HTTP 200 rápido); el procesamiento lo hace el Worker.
func (s *Service) ReceiveWebhook(ctx context.Context, provider string, integrationID int64, body []byte, signature, token string) (int64, error) {
	in, err := s.Integrations.Get(ctx, integrationID)
	if err != nil || !in.Active || in.Provider != provider {
		return 0, ErrWebhookAuth // no se revela si la integración existe
	}
	secret, err := s.Secrets.Open(in.SecretEnc)
	if err != nil {
		return 0, err
	}
	if !VerifySignature(secret, body, signature, token) {
		return 0, ErrWebhookAuth
	}
	extID := externalID(provider, in.Kind, body)
	inboxID, _, err := s.Integrations.Receive(ctx, provider, in.ID, extID, string(body))
	return inboxID, err
}

// externalID extrae la referencia del aviso (para registrar reintentos).
func externalID(provider, kind string, body []byte) string {
	var m map[string]any
	_ = json.Unmarshal(body, &m)
	for _, k := range []string{"externalRef", "eventId", "resource", "_id", "id"} {
		if v, ok := m[k]; ok {
			return fmt.Sprint(v)
		}
	}
	return ""
}

// ProcessInbox lo ejecuta el Worker (trabajo process_webhook).
func (s *Service) ProcessInbox(ctx context.Context, inboxID int64) error {
	item, err := s.Integrations.InboxItem(ctx, inboxID)
	if err != nil {
		return err
	}
	in, err := s.Integrations.Get(ctx, item.IntegrationID)
	if err != nil {
		return s.Integrations.FinishInbox(ctx, inboxID, "error", "integración inexistente", nil)
	}
	if in.Kind == "courier" {
		return s.processTracking(ctx, in, item)
	}
	return s.processChannelOrder(ctx, in, item)
}

type genericOrder struct {
	ExternalRef string `json:"externalRef"`
	Courier     string `json:"courier"`
	domain.Destination
	DispatchType  string        `json:"dispatchType"`
	CustomerNotes string        `json:"customerNotes"`
	Items         []domain.Item `json:"items"`
	// Mercado Libre: {"topic":"orders_v2","resource":"/orders/123"}
	Topic    string `json:"topic"`
	Resource string `json:"resource"`
}

func (s *Service) processChannelOrder(ctx context.Context, in ports.Integration, item ports.InboxItem) error {
	var msg genericOrder
	if err := json.Unmarshal([]byte(item.Payload), &msg); err != nil {
		return s.Integrations.FinishInbox(ctx, item.ID, "error", "JSON inválido: "+err.Error(), nil)
	}
	var d domain.Draft
	switch in.Provider {
	case "mercadolibre":
		if msg.Resource == "" {
			return s.Integrations.FinishInbox(ctx, item.ID, "error", "aviso sin recurso", nil)
		}
		fetched, err := s.Channels.FetchOrder(ctx, in, msg.Resource)
		if err != nil {
			return err // temporal: el trabajo se reintenta
		}
		d = fetched
	default:
		d = domain.Draft{ExternalRef: msg.ExternalRef, Courier: msg.Courier, Destination: msg.Destination,
			DispatchType: msg.DispatchType, CustomerNotes: msg.CustomerNotes, Items: msg.Items}
	}
	d.CompanyID, d.Channel = in.CompanyID, in.Channel
	if d.Courier == "" {
		d.Courier = in.DefaultCourier
	}
	o, err := domain.NewOrder(d, "webhook")
	if err != nil {
		return s.Integrations.FinishInbox(ctx, item.ID, "error", err.Error(), nil)
	}
	actor := "integration:" + in.Provider
	o, err = s.Orders.Create(ctx, o, actor, nil)
	if errors.Is(err, domain.ErrDuplicate) {
		// No se crea una segunda orden; el reintento queda registrado (H-06 esc. 2).
		existing, ferr := s.Integrations.FindOrderByRef(ctx, in.CompanyID, d.ExternalRef)
		if ferr == nil {
			_ = s.Orders.AddEvent(ctx, existing, "webhook_retry", "duplicate", actor, "El canal reenvió la orden; no se duplicó")
			return s.Integrations.FinishInbox(ctx, item.ID, "duplicate", "", &existing)
		}
		return s.Integrations.FinishInbox(ctx, item.ID, "duplicate", "", nil)
	}
	if err != nil {
		return err
	}
	s.Notifier.OrderChanged(ctx, o)
	return s.Integrations.FinishInbox(ctx, item.ID, "processed", "", &o.ID)
}

type trackingMsg struct {
	EventID     string `json:"eventId"`
	CourierCode string `json:"courierCode"`
	OrderRef    string `json:"orderRef"`
	Status      string `json:"status"`
	Description string `json:"description"`
	OccurredAt  string `json:"occurredAt"`
}

// processTracking registra el evento del courier TAL COMO LLEGA; no cambia
// por sí solo el estado interno de la orden (RF-25, H-14 escenario 3).
func (s *Service) processTracking(ctx context.Context, in ports.Integration, item ports.InboxItem) error {
	var m trackingMsg
	if err := json.Unmarshal([]byte(item.Payload), &m); err != nil || m.EventID == "" || m.Status == "" {
		return s.Integrations.FinishInbox(ctx, item.ID, "error", "evento sin eventId o status", nil)
	}
	var orderID int64
	var err error
	if m.CourierCode != "" {
		orderID, err = s.Integrations.FindOrderByCourierCode(ctx, in.Provider, m.CourierCode)
	}
	if orderID == 0 && m.OrderRef != "" {
		orderID, err = s.Integrations.FindOrderByRef(ctx, in.CompanyID, m.OrderRef)
	}
	if orderID == 0 {
		msg := "orden no encontrada para el evento"
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return err
		}
		return s.Integrations.FinishInbox(ctx, item.ID, "error", msg, nil)
	}
	at, perr := time.Parse(time.RFC3339, m.OccurredAt)
	if perr != nil {
		at = time.Now()
	}
	inserted, err := s.Integrations.AddTracking(ctx, ports.TrackingEvent{OrderID: orderID, Provider: in.Provider,
		EventID: m.EventID, StatusRaw: m.Status, Description: m.Description, OccurredAt: at})
	if err != nil {
		return err
	}
	status := "processed"
	if !inserted {
		status = "duplicate"
	} else if o, e := s.Orders.Get(ctx, nil, orderID); e == nil {
		s.Notifier.OrderChanged(ctx, o) // la pantalla del cliente muestra el seguimiento nuevo
	}
	return s.Integrations.FinishInbox(ctx, item.ID, status, "", &orderID)
}

// Tracking - seguimiento de la orden (vacío = el courier no informa; no se inventan estados).
func (s *Service) Tracking(ctx context.Context, scope []int64, orderID int64) ([]ports.TrackingEvent, error) {
	if _, err := s.Orders.Get(ctx, scope, orderID); err != nil {
		return nil, err
	}
	return s.Integrations.Tracking(ctx, orderID)
}
