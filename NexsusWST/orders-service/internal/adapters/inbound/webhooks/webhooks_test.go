package webhooks

// Pruebas de SCRUM-123 (H-06): el webhook a nivel de endpoint y de flujo.
// Usan el Service y el SecretBox reales con dobles en memoria de los puertos
// (mismo enfoque que los tests de application): no necesitan PostgreSQL.
// Los dobles imitan el contrato de los repositorios reales: Receive registra
// cada aviso (también los reenvíos), Orders.Create devuelve ErrDuplicate si la
// referencia ya existe para la empresa y AddTracking ignora un eventId repetido.

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nexuswst/orders-service/internal/application"
	"github.com/nexuswst/orders-service/internal/domain"
	"github.com/nexuswst/orders-service/internal/ports"
)

const testKey = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"

type inboxRow struct {
	ports.InboxItem
	externalID, status, errMsg string
	orderID                    *int64
}

type memIntegrations struct {
	ports.IntegrationRepository
	orders       *memOrders
	integrations map[int64]ports.Integration
	inbox        []*inboxRow
	tracking     []ports.TrackingEvent
}

func (m *memIntegrations) Get(_ context.Context, id int64) (ports.Integration, error) {
	i, ok := m.integrations[id]
	if !ok {
		return i, domain.ErrNotFound
	}
	return i, nil
}

func (m *memIntegrations) Receive(_ context.Context, provider string, integrationID int64, externalID, payload string) (int64, bool, error) {
	dup := false
	for _, r := range m.inbox {
		dup = dup || (externalID != "" && r.IntegrationID == integrationID && r.externalID == externalID)
	}
	id := int64(len(m.inbox) + 1)
	m.inbox = append(m.inbox, &inboxRow{InboxItem: ports.InboxItem{ID: id, Provider: provider, IntegrationID: integrationID, Payload: payload},
		externalID: externalID, status: "received"})
	return id, dup, nil
}

func (m *memIntegrations) InboxItem(_ context.Context, id int64) (ports.InboxItem, error) {
	if id < 1 || int(id) > len(m.inbox) {
		return ports.InboxItem{}, domain.ErrNotFound
	}
	return m.inbox[id-1].InboxItem, nil
}

func (m *memIntegrations) FinishInbox(_ context.Context, id int64, status, errMsg string, orderID *int64) error {
	r := m.inbox[id-1]
	r.status, r.errMsg, r.orderID = status, errMsg, orderID
	r.Attempts++
	return nil
}

func (m *memIntegrations) AddTracking(_ context.Context, e ports.TrackingEvent) (bool, error) {
	for _, t := range m.tracking {
		if t.Provider == e.Provider && t.EventID == e.EventID {
			return false, nil
		}
	}
	m.tracking = append(m.tracking, e)
	return true, nil
}

func (m *memIntegrations) FindOrderByRef(_ context.Context, companyID int64, ref string) (int64, error) {
	for _, o := range m.orders.list {
		if o.CompanyID == companyID && o.ExternalRef == ref {
			return o.ID, nil
		}
	}
	return 0, domain.ErrNotFound
}

func (m *memIntegrations) FindOrderByCourierCode(context.Context, string, string) (int64, error) {
	return 0, domain.ErrNotFound
}

type memOrders struct {
	ports.OrderRepository
	list   []domain.Order
	events []string // "orderID:tipo:evento"
}

func (m *memOrders) Create(_ context.Context, o domain.Order, _ string, _ *int64) (domain.Order, error) {
	for _, e := range m.list {
		if e.CompanyID == o.CompanyID && e.ExternalRef == o.ExternalRef {
			return domain.Order{}, domain.ErrDuplicate
		}
	}
	o.ID = int64(len(m.list) + 1)
	m.list = append(m.list, o)
	return o, nil
}

func (m *memOrders) Get(_ context.Context, _ []int64, id int64) (domain.Order, error) {
	for _, o := range m.list {
		if o.ID == id {
			return o, nil
		}
	}
	return domain.Order{}, domain.ErrNotFound
}

func (m *memOrders) AddEvent(_ context.Context, orderID int64, typ, ev, _, _ string) error {
	m.events = append(m.events, strings.Join([]string{string(rune('0' + orderID)), typ, ev}, ":"))
	return nil
}

type silentNotifier struct{ ports.Notifier }

func (silentNotifier) OrderChanged(context.Context, domain.Order) {}

type fixture struct {
	svc    *application.Service
	repo   *memIntegrations
	orders *memOrders
	srv    *httptest.Server
}

// Integraciones de la empresa 7: 1 = canal generic activo, 2 = canal generic inactivo,
// 3 = courier generic activo. Todas con el secreto "s3cr3t" cifrado con el SecretBox real.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	box, err := application.NewSecretBox(testKey)
	if err != nil {
		t.Fatal(err)
	}
	enc, err := box.Seal("s3cr3t")
	if err != nil {
		t.Fatal(err)
	}
	orders := &memOrders{}
	repo := &memIntegrations{orders: orders, integrations: map[int64]ports.Integration{
		1: {ID: 1, CompanyID: 7, Provider: "generic", Kind: "channel", Channel: "generic", DefaultCourier: "starken", Active: true, SecretEnc: enc},
		2: {ID: 2, CompanyID: 7, Provider: "generic", Kind: "channel", Channel: "generic", Active: false, SecretEnc: enc},
		3: {ID: 3, CompanyID: 7, Provider: "generic", Kind: "courier", Channel: "generic", Active: true, SecretEnc: enc},
	}}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := application.NewService(application.Deps{Orders: orders, Integrations: repo, Notifier: silentNotifier{}, Secrets: box, Log: log})
	mux := http.NewServeMux()
	NewHandler(svc, log).Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return &fixture{svc: svc, repo: repo, orders: orders, srv: srv}
}

func sign(secret, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return hex.EncodeToString(mac.Sum(nil))
}

func (f *fixture) post(t *testing.T, path, body string, headers map[string]string) (int, map[string]any) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, f.srv.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	return res.StatusCode, out
}

const orderBody = `{"externalRef":"WEB-1001","recipientName":"Ana Pérez","addressLine":"Av. Providencia 1208","commune":"Providencia","items":[{"sku":"AO-1001","quantity":2}]}`

// Caso 1: firma HMAC válida → 200 y el aviso queda en la bandeja (webhook_inbox).
func TestWebhookValidSignatureIsAcceptedAndStored(t *testing.T) {
	f := newFixture(t)
	status, out := f.post(t, "/webhooks/generic/1", orderBody, map[string]string{"X-Nexus-Signature": sign("s3cr3t", orderBody)})
	if status != http.StatusOK {
		t.Fatalf("firma válida: status %d, want 200", status)
	}
	if len(f.repo.inbox) != 1 {
		t.Fatalf("avisos en la bandeja: %d, want 1", len(f.repo.inbox))
	}
	row := f.repo.inbox[0]
	if out["received"] != float64(row.ID) {
		t.Fatalf("respuesta %v, want received=%d", out, row.ID)
	}
	if row.IntegrationID != 1 || row.Provider != "generic" || row.Payload != orderBody || row.externalID != "WEB-1001" || row.status != "received" {
		t.Fatalf("aviso guardado distinto de lo enviado: %+v", *row)
	}
	if len(f.orders.list) != 0 {
		t.Fatal("el endpoint solo registra: la orden la crea el Worker (ProcessInbox)")
	}

	// También se aceptan el prefijo sha256= y el secreto como token (cabecera o ?token=).
	for name, send := range map[string]func() int{
		"sha256=": func() int {
			s, _ := f.post(t, "/webhooks/generic/1", orderBody, map[string]string{"X-Nexus-Signature": "sha256=" + sign("s3cr3t", orderBody)})
			return s
		},
		"X-Nexus-Token": func() int {
			s, _ := f.post(t, "/webhooks/generic/1", orderBody, map[string]string{"X-Nexus-Token": "s3cr3t"})
			return s
		},
		"?token=": func() int { s, _ := f.post(t, "/webhooks/generic/1?token=s3cr3t", orderBody, nil); return s },
	} {
		if s := send(); s != http.StatusOK {
			t.Errorf("%s: status %d, want 200", name, s)
		}
	}
	if len(f.repo.inbox) != 4 {
		t.Fatalf("avisos en la bandeja: %d, want 4", len(f.repo.inbox))
	}
}

// Caso 2: sin firma, firma inválida, cuerpo alterado, integración inactiva o proveedor
// que no coincide → 401 y no se guarda nada.
func TestWebhookRejectedStoresNothing(t *testing.T) {
	valid := sign("s3cr3t", orderBody)
	for _, tc := range []struct {
		name, path, body string
		headers          map[string]string
		want             int
	}{
		{"sin firma ni token", "/webhooks/generic/1", orderBody, nil, 401},
		{"firma de otro secreto", "/webhooks/generic/1", orderBody, map[string]string{"X-Nexus-Signature": sign("otro", orderBody)}, 401},
		{"firma con formato inválido", "/webhooks/generic/1", orderBody, map[string]string{"X-Nexus-Signature": "no-es-hex"}, 401},
		{"cuerpo alterado", "/webhooks/generic/1", strings.Replace(orderBody, "WEB-1001", "WEB-9999", 1), map[string]string{"X-Nexus-Signature": valid}, 401},
		{"token incorrecto", "/webhooks/generic/1", orderBody, map[string]string{"X-Nexus-Token": "otro"}, 401},
		{"firma inválida aunque el token sea correcto", "/webhooks/generic/1", orderBody, map[string]string{"X-Nexus-Signature": sign("otro", orderBody), "X-Nexus-Token": "s3cr3t"}, 401},
		{"integración inactiva con firma válida", "/webhooks/generic/2", orderBody, map[string]string{"X-Nexus-Signature": valid}, 401},
		{"proveedor que no coincide con la integración", "/webhooks/mercadolibre/1", orderBody, map[string]string{"X-Nexus-Signature": valid}, 401},
		{"integración inexistente", "/webhooks/generic/99", orderBody, map[string]string{"X-Nexus-Signature": valid}, 401},
		{"id no numérico", "/webhooks/generic/abc", orderBody, map[string]string{"X-Nexus-Signature": valid}, 404},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			status, out := f.post(t, tc.path, tc.body, tc.headers)
			if status != tc.want {
				t.Fatalf("status %d, want %d", status, tc.want)
			}
			if _, ok := out["received"]; ok {
				t.Fatalf("una solicitud rechazada no debe devolver received: %v", out)
			}
			if len(f.repo.inbox) != 0 || len(f.orders.list) != 0 {
				t.Fatalf("no debe guardarse nada: bandeja=%d órdenes=%d", len(f.repo.inbox), len(f.orders.list))
			}
		})
	}
}

// Caso 3 (canal): la misma orden enviada dos veces se registra dos veces en la bandeja
// (el reenvío queda trazado) pero ProcessInbox crea UNA sola orden.
func TestWebhookDuplicateOrderIsCreatedOnce(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	headers := map[string]string{"X-Nexus-Signature": sign("s3cr3t", orderBody)}
	var ids []int64
	for i := 0; i < 2; i++ {
		status, out := f.post(t, "/webhooks/generic/1", orderBody, headers)
		if status != http.StatusOK {
			t.Fatalf("envío %d: status %d, want 200", i+1, status)
		}
		ids = append(ids, int64(out["received"].(float64)))
	}
	if len(f.repo.inbox) != 2 || ids[0] == ids[1] {
		t.Fatalf("los dos avisos deben quedar registrados: bandeja=%d ids=%v", len(f.repo.inbox), ids)
	}
	for _, id := range ids {
		if err := f.svc.ProcessInbox(ctx, id); err != nil {
			t.Fatalf("ProcessInbox(%d): %v", id, err)
		}
	}
	if len(f.orders.list) != 1 {
		t.Fatalf("órdenes creadas: %d, want 1", len(f.orders.list))
	}
	o := f.orders.list[0]
	if o.CompanyID != 7 || o.ExternalRef != "WEB-1001" || o.Channel != "generic" || o.Courier != "starken" || o.Source != "webhook" {
		t.Fatalf("orden creada con datos inesperados: %+v", o)
	}
	first, second := f.repo.inbox[0], f.repo.inbox[1]
	if first.status != "processed" || first.orderID == nil || *first.orderID != o.ID {
		t.Fatalf("primer aviso: status=%q orderID=%v, want processed con la orden %d", first.status, first.orderID, o.ID)
	}
	if second.status != "duplicate" || second.orderID == nil || *second.orderID != o.ID {
		t.Fatalf("reenvío: status=%q orderID=%v, want duplicate apuntando a la orden %d", second.status, second.orderID, o.ID)
	}
	if len(f.orders.events) != 1 || f.orders.events[0] != "1:webhook_retry:duplicate" {
		t.Fatalf("el reenvío debe quedar en el historial de la orden (H-06 esc. 2): %v", f.orders.events)
	}

	// Reprocesar el mismo aviso (reintento del Worker) tampoco crea otra orden.
	if err := f.svc.ProcessInbox(ctx, ids[0]); err != nil {
		t.Fatal(err)
	}
	if len(f.orders.list) != 1 {
		t.Fatalf("un reintento del Worker creó otra orden: %d", len(f.orders.list))
	}
}

// Caso 3 (courier): el mismo evento de seguimiento enviado dos veces se guarda una vez.
func TestWebhookDuplicateTrackingEventIsStoredOnce(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.orders.list = []domain.Order{{ID: 1, CompanyID: 7, ExternalRef: "WEB-1001", Courier: "starken", Status: domain.StatusHandedOver}}
	body := `{"eventId":"EVT-1","orderRef":"WEB-1001","status":"EN_REPARTO","occurredAt":"2026-11-10T12:00:00Z"}`
	headers := map[string]string{"X-Nexus-Signature": sign("s3cr3t", body)}
	for i := 0; i < 2; i++ {
		status, out := f.post(t, "/webhooks/generic/3", body, headers)
		if status != http.StatusOK {
			t.Fatalf("envío %d: status %d, want 200", i+1, status)
		}
		if err := f.svc.ProcessInbox(ctx, int64(out["received"].(float64))); err != nil {
			t.Fatal(err)
		}
	}
	if len(f.repo.tracking) != 1 {
		t.Fatalf("eventos de seguimiento guardados: %d, want 1", len(f.repo.tracking))
	}
	if e := f.repo.tracking[0]; e.OrderID != 1 || e.EventID != "EVT-1" || e.StatusRaw != "EN_REPARTO" {
		t.Fatalf("evento guardado con datos inesperados: %+v", e)
	}
	if got := []string{f.repo.inbox[0].status, f.repo.inbox[1].status}; got[0] != "processed" || got[1] != "duplicate" {
		t.Fatalf("estados de la bandeja %v, want [processed duplicate]", got)
	}
	// El seguimiento se registra tal como llega: no cambia el estado interno de la orden (RF-25).
	if f.orders.list[0].Status != domain.StatusHandedOver {
		t.Fatalf("el evento del courier cambió el estado de la orden: %s", f.orders.list[0].Status)
	}
}
