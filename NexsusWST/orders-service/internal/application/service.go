// Package application - casos de uso de Órdenes (CU-01 a CU-08).
//
// Reglas transversales:
//   - Toda acción comprueba rol/permiso y empresa en el servidor (RNF-SEG-01).
//   - Todo cambio de estado pasa por la máquina de estados del dominio y se
//     guarda con su evento de historial en la MISMA transacción.
//   - Los avisos (pantalla y correo) se emiten DESPUÉS de persistir.
package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/nexuswst/orders-service/internal/domain"
	"github.com/nexuswst/orders-service/internal/ports"
	"github.com/nexuswst/orders-service/internal/shared/tenant"
)

const (
	JobAssignWave     = "assign_wave"
	JobProcessWebhook = "process_webhook"
)

// Deps agrupa los puertos que usa la aplicación.
type Deps struct {
	Orders        ports.OrderRepository
	Rules         ports.RuleRepository
	Fulfillment   ports.FulfillmentRepository
	Integrations  ports.IntegrationRepository
	Notifications ports.NotificationRepository
	Catalog       ports.CatalogReader
	Notifier      ports.Notifier
	Mailer        ports.Mailer
	Files         ports.FileStorage
	WMS           ports.WMSGateway
	Couriers      ports.CourierGateway
	Channels      ports.ChannelGateway
	Secrets       SecretBox
	Log           *slog.Logger
	DevMode       bool // permite simular eventos de Invas (pick_confirmed) en desarrollo
}

type Service struct {
	Deps
	notices *Notices
}

func NewService(d Deps) *Service {
	if d.Log == nil {
		d.Log = slog.Default()
	}
	return &Service{Deps: d, notices: &Notices{repo: d.Notifications, notifier: d.Notifier, mailer: d.Mailer, catalog: d.Catalog, log: d.Log}}
}

// OpError lleva un mensaje de negocio legible junto al error base.
type OpError struct {
	Base error
	Msg  string
}

func (e OpError) Error() string { return e.Msg }
func (e OpError) Unwrap() error { return e.Base }

func fail(base error, format string, a ...any) error {
	return OpError{Base: base, Msg: fmt.Sprintf(format, a...)}
}

func NewID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// holdsCommitted: la orden tiene stock comprometido mientras está Validada
// (o en Incidencia ocurrida estando Validada) y Invas aún no confirmó la ola.
func holdsCommitted(o domain.Order) bool {
	return o.Status == domain.StatusValidated || (o.Status == domain.StatusIncident && o.PrevStatus == domain.StatusValidated)
}

// move persiste una transición y emite los avisos correspondientes.
func (s *Service) move(ctx context.Context, o domain.Order, from domain.Status, ev domain.Event, actor, reason string, opts ports.TransitionOpts) (domain.Order, error) {
	out, err := s.Orders.Transition(ctx, o, from, ev, actor, reason, opts)
	if err != nil {
		return domain.Order{}, err
	}
	s.after(ctx, out, reason)
	return out, nil
}

func (s *Service) after(ctx context.Context, o domain.Order, reason string) {
	s.Notifier.OrderChanged(ctx, o)
	s.notices.OnStatus(ctx, o, reason)
}

// ---------------------------------------------------------------------------
// Ingreso y consulta (CU-01, CU-08)
// ---------------------------------------------------------------------------

func canCreate(id tenant.Identity) bool {
	return id.HasRole(tenant.RoleAdmin, tenant.RoleJefeOps, tenant.RoleCliente)
}

// Create registra la orden como Pendiente. La validación es AUTOMÁTICA: la
// toma el Worker (no requiere autorización individual, RF-15).
func (s *Service) Create(ctx context.Context, id tenant.Identity, d domain.Draft) (domain.Order, error) {
	if !canCreate(id) {
		return domain.Order{}, domain.ErrForbidden
	}
	if d.CompanyID == 0 && !id.AllCompanies && len(id.Companies) == 1 {
		d.CompanyID = id.Companies[0] // cliente de una sola empresa
	}
	if d.CompanyID != 0 && !id.CanAccess(d.CompanyID) {
		return domain.Order{}, domain.ErrForbidden
	}
	if d.Channel == "" || !id.HasRole(tenant.RoleAdmin, tenant.RoleJefeOps) {
		d.Channel = "manual" // el canal de integraciones lo fija el webhook
	}
	o, err := domain.NewOrder(d, "manual")
	if err != nil {
		return domain.Order{}, err
	}
	uid := id.UserID
	o, err = s.Orders.Create(ctx, o, id.Actor(), &uid)
	if err != nil {
		return domain.Order{}, err
	}
	s.Notifier.OrderChanged(ctx, o)
	return o, nil
}

type Detail struct {
	Order     domain.Order
	Events    []ports.EventRecord
	Packages  []domain.Package
	Documents []ports.Document
	Tracking  []ports.TrackingEvent
}

func (s *Service) Get(ctx context.Context, scope []int64, orderID int64) (Detail, error) {
	o, err := s.Orders.Get(ctx, scope, orderID)
	if err != nil {
		return Detail{}, err
	}
	d := Detail{Order: o}
	if d.Events, err = s.Orders.Events(ctx, o.ID); err != nil {
		return d, err
	}
	if d.Packages, err = s.Fulfillment.Packages(ctx, o.ID); err != nil {
		return d, err
	}
	if d.Documents, err = s.Fulfillment.Documents(ctx, o.ID); err != nil {
		return d, err
	}
	d.Tracking, err = s.Integrations.Tracking(ctx, o.ID)
	return d, err
}

func (s *Service) List(ctx context.Context, f ports.ListFilter) ([]domain.Order, int, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	for _, st := range f.Statuses {
		if !domain.ValidStatus(st) {
			return nil, 0, fail(domain.ErrInvalid, "estado desconocido: %s", st)
		}
	}
	return s.Orders.List(ctx, f)
}

func (s *Service) Summary(ctx context.Context, scope []int64) (map[domain.Status]int, error) {
	return s.Orders.Summary(ctx, scope)
}

// ---------------------------------------------------------------------------
// Edición, anulación y cancelación (CU-06, H-09)
// ---------------------------------------------------------------------------

// Edit modifica datos mientras el estado lo permite. Si cambian productos,
// destino o courier, la orden vuelve a validarse (y libera el comprometido).
func (s *Service) Edit(ctx context.Context, id tenant.Identity, scope []int64, orderID int64, c domain.Changes, reason string) (domain.Order, error) {
	if !id.HasRole(tenant.RoleAdmin, tenant.RoleJefeOps, tenant.RoleCliente) {
		return domain.Order{}, domain.ErrForbidden
	}
	o, err := s.Orders.Get(ctx, scope, orderID)
	if err != nil {
		return domain.Order{}, err
	}
	after, diff, err := o.ApplyChanges(c)
	if errors.Is(err, domain.ErrNotEditable) {
		return domain.Order{}, fail(domain.ErrNotEditable, "La orden está «%s»: ya fue enviada a preparación y no se modifica desde Nexus (Invas no cambia por esto).", domain.Labels[o.Status])
	}
	if err != nil {
		return domain.Order{}, err
	}
	if len(diff) == 0 {
		return o, nil
	}
	var ev domain.Event
	opts := ports.TransitionOpts{}
	if o.NeedsRevalidation(after) {
		switch o.Status {
		case domain.StatusValidated, domain.StatusValidating:
			ev = domain.EvEditRevalidate
			opts.ReleaseCommitted = o.Status == domain.StatusValidated
			if _, err := after.Apply(ev); err != nil {
				return domain.Order{}, err
			}
			after.ValidationAttempts = 0
		case domain.StatusIncident:
			// Se corrige y, al resolver, vuelve a validarse desde Pendiente.
			if o.PrevStatus == domain.StatusValidated || o.PrevStatus == domain.StatusValidating {
				opts.ReleaseCommitted = o.PrevStatus == domain.StatusValidated
				after.PrevStatus = domain.StatusPending
				after.ValidationAttempts = 0
			}
		}
	}
	if r := strings.TrimSpace(reason); r != "" {
		diff = append([]string{"Motivo: " + r}, diff...)
	}
	out, err := s.Orders.SaveEdit(ctx, o, after, diff, id.Actor(), ev, opts)
	if err != nil {
		return domain.Order{}, err
	}
	// Courier cambiado con la orden en un manifiesto abierto: la línea queda «Movida a».
	if o.Courier != out.Courier {
		if mid, err := s.Fulfillment.OpenManifestOf(ctx, o.ID); err == nil && mid > 0 {
			_ = s.Fulfillment.SetLineStatus(ctx, mid, o.ID, "moved", out.Courier)
		}
	}
	s.Notifier.OrderChanged(ctx, out)
	return out, nil
}

// Cancel deja la orden Cancelada (terminal, oculta, no se elimina). Si la
// orden ya está en una ola de Invas, queda en Incidencia hasta que Operaciones
// confirme la intervención en Invas (CU-06).
func (s *Service) Cancel(ctx context.Context, id tenant.Identity, scope []int64, orderID int64, reason string) (domain.Order, error) {
	if !id.HasRole(tenant.RoleAdmin, tenant.RoleJefeOps, tenant.RoleCliente) {
		return domain.Order{}, domain.ErrForbidden
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return domain.Order{}, fail(domain.ErrInvalid, "Indica el motivo de la cancelación.")
	}
	o, err := s.Orders.Get(ctx, scope, orderID)
	if err != nil {
		return domain.Order{}, err
	}
	switch o.Status {
	case domain.StatusHandedOver:
		return domain.Order{}, fail(domain.ErrInvalidTransition, "La orden ya fue entregada al courier: no se puede cancelar.")
	case domain.StatusCancelled:
		return o, nil
	}
	internal := id.HasRole(tenant.RoleAdmin, tenant.RoleJefeOps)
	msg := fmt.Sprintf("%s (%s): %s", id.Name, id.Role, reason)

	if o.InInvas() && !(o.Status == domain.StatusIncident && internal) {
		if o.Status == domain.StatusIncident {
			return domain.Order{}, fail(domain.ErrInvalidTransition, "La orden ya está en Incidencia esperando que Operaciones confirme la cancelación en Invas.")
		}
		from, err := o.Incident(domain.IncidentCancelRequest)
		if err != nil {
			return domain.Order{}, err
		}
		return s.move(ctx, o, from, domain.EvIncident, id.Actor(), "Cancelación solicitada por "+msg+". Requiere intervención en Invas.", ports.TransitionOpts{})
	}
	release := holdsCommitted(o)
	from, err := o.Apply(domain.EvCancel)
	if err != nil {
		return domain.Order{}, fail(domain.ErrInvalidTransition, "La orden «%s» no se puede cancelar.", domain.Labels[from])
	}
	out, err := s.move(ctx, o, from, domain.EvCancel, id.Actor(), "Cancelada por "+msg, ports.TransitionOpts{ReleaseCommitted: release, ClearWave: true})
	if err == nil {
		if mid, e := s.Fulfillment.OpenManifestOf(ctx, o.ID); e == nil && mid > 0 {
			_ = s.Fulfillment.SetLineStatus(ctx, mid, o.ID, "cancelled", "")
		}
	}
	return out, err
}

// RecoverVoided - «Resolver anulación»: cuando Invas tiene stock de nuevo, la
// orden Anulada vuelve a Pendiente y se valida otra vez (CU-06 alternativo).
func (s *Service) RecoverVoided(ctx context.Context, id tenant.Identity, scope []int64, orderID int64, note string) (domain.Order, error) {
	if !id.HasRole(tenant.RoleAdmin, tenant.RoleJefeOps) {
		return domain.Order{}, domain.ErrForbidden
	}
	o, err := s.Orders.Get(ctx, scope, orderID)
	if err != nil {
		return domain.Order{}, err
	}
	from, err := o.Apply(domain.EvRevalidate)
	if err != nil {
		return domain.Order{}, fail(domain.ErrInvalidTransition, "Solo una orden Anulada se puede reintentar.")
	}
	o.ValidationAttempts = 0
	return s.move(ctx, o, from, domain.EvRevalidate, id.Actor(), strings.TrimSpace("Reintento de validación. "+note), ports.TransitionOpts{})
}

// ---------------------------------------------------------------------------
// Incidencias (CU-03, H-11)
// ---------------------------------------------------------------------------

func canReportIncident(id tenant.Identity) bool {
	return id.HasRole(tenant.RoleAdmin, tenant.RoleJefeOps, tenant.RoleEmpaque) || id.HasPermission(tenant.PermDespacho)
}

// ReportIncident detiene la orden con un motivo desde cualquier etapa activa.
func (s *Service) ReportIncident(ctx context.Context, id tenant.Identity, scope []int64, orderID int64, kind, reason string) (domain.Order, error) {
	if !canReportIncident(id) {
		return domain.Order{}, domain.ErrForbidden
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return domain.Order{}, fail(domain.ErrInvalid, "Describe el motivo de la incidencia.")
	}
	if kind == "" {
		kind = domain.IncidentOther
	}
	valid := false
	for _, k := range domain.IncidentKinds {
		valid = valid || k == kind
	}
	if !valid {
		return domain.Order{}, fail(domain.ErrInvalid, "Tipo de incidencia desconocido.")
	}
	o, err := s.Orders.Get(ctx, scope, orderID)
	if err != nil {
		return domain.Order{}, err
	}
	from, err := o.Incident(kind)
	if err != nil {
		return domain.Order{}, fail(domain.ErrInvalidTransition, "No se puede reportar una incidencia en una orden «%s».", domain.Labels[o.Status])
	}
	o.ClaimedBy = nil // el trabajo tomado vuelve a quedar libre
	out, err := s.move(ctx, o, from, domain.EvIncident, id.Actor(), fmt.Sprintf("%s: %s", domain.IncidentLabels[kind], reason), ports.TransitionOpts{})
	if err == nil {
		// Detectada al retirar: la orden sale del manifiesto (CU-05).
		if mid, e := s.Fulfillment.OpenManifestOf(ctx, o.ID); e == nil && mid > 0 {
			_ = s.Fulfillment.SetLineStatus(ctx, mid, o.ID, "removed", "")
		}
	}
	return out, err
}

// Acciones al resolver una incidencia.
const (
	ResolveResume = "resume" // corregido: vuelve al estado previo y continúa
	ResolveRetry  = "retry"  // vuelve al estado previo y reejecuta el trabajo fallido
	ResolveCancel = "cancel" // no se puede resolver: se cancela
)

// Resolve - el Jefe corrige la causa y la orden vuelve al estado donde
// ocurrió, registrando quién y qué se hizo (CU-03).
func (s *Service) Resolve(ctx context.Context, id tenant.Identity, scope []int64, orderID int64, action, note string) (domain.Order, error) {
	if !id.HasRole(tenant.RoleAdmin, tenant.RoleJefeOps) {
		return domain.Order{}, domain.ErrForbidden
	}
	note = strings.TrimSpace(note)
	if note == "" {
		return domain.Order{}, fail(domain.ErrInvalid, "Deja escrito qué se hizo para resolver la incidencia.")
	}
	if action == ResolveCancel {
		return s.Cancel(ctx, id, scope, orderID, note)
	}
	if action != ResolveResume && action != ResolveRetry {
		return domain.Order{}, fail(domain.ErrInvalid, "Acción desconocida: usa resume, retry o cancel.")
	}
	o, err := s.Orders.Get(ctx, scope, orderID)
	if err != nil {
		return domain.Order{}, err
	}
	if o.Status != domain.StatusIncident {
		return domain.Order{}, fail(domain.ErrInvalidTransition, "La orden no está en Incidencia.")
	}
	kind := o.IncidentKind
	from, err := o.Apply(domain.EvResolve)
	if err != nil {
		return domain.Order{}, err
	}
	if o.Status == domain.StatusPending || o.Status == domain.StatusValidating {
		o.ValidationAttempts = 0 // vuelve a la cola del validador de inmediato
	}
	label := domain.IncidentLabels[kind]
	if label == "" {
		label = "Incidencia"
	}
	out, err := s.move(ctx, o, from, domain.EvResolve, id.Actor(), fmt.Sprintf("Resuelta (%s): %s", label, note), ports.TransitionOpts{})
	if err != nil {
		return domain.Order{}, err
	}
	if action == ResolveRetry {
		if n, err := s.Orders.RetryFailedJobs(ctx, out.ID); err == nil && n > 0 {
			_ = s.Orders.AddEvent(ctx, out.ID, "job_retry", "retry", id.Actor(), fmt.Sprintf("%d trabajo(s) reencolado(s)", n))
		}
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Prioridad y olas manuales (RF-15, RF-17)
// ---------------------------------------------------------------------------

// SetPriority - «Enviar primero a la cola» (RF-15, DAS §4.3).
func (s *Service) SetPriority(ctx context.Context, id tenant.Identity, scope []int64, orderID int64, priority bool, reason string) (domain.Order, error) {
	if !id.HasRole(tenant.RoleAdmin, tenant.RoleJefeOps) {
		return domain.Order{}, domain.ErrForbidden
	}
	o, err := s.Orders.Get(ctx, scope, orderID)
	if err != nil {
		return domain.Order{}, err
	}
	switch o.Status {
	case domain.StatusHandedOver, domain.StatusCancelled:
		return domain.Order{}, fail(domain.ErrInvalidTransition, "La orden ya terminó su recorrido.")
	}
	o, err = s.Orders.SetPriority(ctx, o, priority, id.Actor(), reason)
	if err == nil {
		s.Notifier.OrderChanged(ctx, o)
	}
	return o, err
}

func (s *Service) Waves(ctx context.Context) ([]ports.Wave, error) { return s.WMS.ListWaves(ctx) }

// AssignWave - asignación MANUAL a ola (RF-17). Asignar es encolar: el Worker
// envía a Invas en lotes pequeños, primero las órdenes prioritarias.
func (s *Service) AssignWave(ctx context.Context, id tenant.Identity, scope []int64, orderIDs []int64, waveID string) ([]int64, error) {
	if !id.HasRole(tenant.RoleAdmin, tenant.RoleJefeOps) {
		return nil, domain.ErrForbidden
	}
	if waveID == "" || len(orderIDs) == 0 {
		return nil, fail(domain.ErrInvalid, "Selecciona órdenes Validadas y una ola.")
	}
	waves, err := s.WMS.ListWaves(ctx)
	if err != nil {
		return nil, fail(err, "No se pudieron obtener las olas de Invas.")
	}
	found := false
	for _, w := range waves {
		found = found || w.ID == waveID
	}
	if !found {
		return nil, fail(domain.ErrInvalid, "La ola %s no existe en Invas.", waveID)
	}
	ids, err := s.Orders.AssignWave(ctx, scope, orderIDs, waveID, id.Actor(), "Asignación manual a "+waveID)
	if err == nil {
		for _, oid := range ids {
			if o, e := s.Orders.Get(ctx, nil, oid); e == nil {
				s.Notifier.OrderChanged(ctx, o)
			}
		}
	}
	return ids, err
}

// SimulatePicked - SOLO desarrollo: simula que Invas confirmó la preparación.
func (s *Service) SimulatePicked(ctx context.Context, id tenant.Identity, scope []int64, orderID int64) (domain.Order, error) {
	if !s.DevMode || !id.HasRole(tenant.RoleAdmin, tenant.RoleJefeOps) {
		return domain.Order{}, domain.ErrForbidden
	}
	o, err := s.Orders.Get(ctx, scope, orderID)
	if err != nil {
		return domain.Order{}, err
	}
	from, err := o.Apply(domain.EvPickConfirmed)
	if err != nil {
		return domain.Order{}, err
	}
	return s.move(ctx, o, from, domain.EvPickConfirmed, "system:invas-simulado", "[SIMULADO] preparación confirmada", ports.TransitionOpts{})
}

// SystemIncident - un proceso (Worker, Invas) detiene la orden con su motivo.
func (s *Service) SystemIncident(ctx context.Context, orderID int64, kind, reason, actor string) {
	o, err := s.Orders.Get(ctx, nil, orderID)
	if err != nil {
		return
	}
	from, err := o.Incident(kind)
	if err != nil {
		return
	}
	if _, err := s.move(ctx, o, from, domain.EvIncident, actor, reason, ports.TransitionOpts{}); err != nil {
		s.Log.Warn("no se pudo registrar la incidencia", "order_id", orderID, "err", err)
	}
}
