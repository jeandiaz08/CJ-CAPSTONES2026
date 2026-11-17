package application

import (
	"context"
	"errors"
	"io"
	"slices"
	"testing"
	"time"

	"github.com/nexuswst/orders-service/internal/domain"
	"github.com/nexuswst/orders-service/internal/ports"
	"github.com/nexuswst/orders-service/internal/shared/tenant"
)

// labelWorld guarda el estado de una orden y su etiqueta como lo haría la base.
type labelWorld struct {
	ports.OrderRepository
	ports.FulfillmentRepository
	order    domain.Order
	attempts int
	due      bool
	stored   []string
	events   []string
	changed  int
	notices  []string
	calls    []int
	answers  []error
}

func (w *labelWorld) Get(context.Context, []int64, int64) (domain.Order, error) { return w.order, nil }
func (w *labelWorld) AddEvent(_ context.Context, _ int64, _, ev, _, _ string) error {
	w.events = append(w.events, ev)
	return nil
}
func (w *labelWorld) Transition(_ context.Context, o domain.Order, _ domain.Status, _ domain.Event, _, _ string, _ ports.TransitionOpts) (domain.Order, error) {
	w.order = o
	return o, nil
}
func (w *labelWorld) Packages(context.Context, int64) ([]domain.Package, error) {
	return []domain.Package{{Code: "NX-1-1"}}, nil
}
func (w *labelWorld) Document(_ context.Context, _, id int64) (ports.Document, error) {
	return ports.Document{ID: id}, nil
}
func (w *labelWorld) RequestLabel(context.Context, domain.Order, string) error {
	w.order.LabelStatus, w.attempts, w.due = domain.LabelPending, 0, true
	return nil
}
func (w *labelWorld) ClaimLabel(context.Context, int64) (int, bool, error) {
	if !w.due {
		return 0, false, nil
	}
	w.due = false
	w.attempts++
	return w.attempts, true, nil
}
func (w *labelWorld) ClaimDueLabels(context.Context, int) ([]ports.LabelClaim, error) {
	attempt, ok, _ := w.ClaimLabel(context.Background(), 1)
	if !ok {
		return nil, nil
	}
	return []ports.LabelClaim{{OrderID: 1, Attempt: attempt}}, nil
}
func (w *labelWorld) RetryLabel(context.Context, int64, int, time.Duration, string) error {
	w.due = true
	return nil
}
func (w *labelWorld) FailLabel(context.Context, int64, int, string, string) error {
	w.order.LabelStatus, w.due = domain.LabelFailed, false
	return nil
}
func (w *labelWorld) ClearLabel(context.Context, int64) error {
	w.order.LabelStatus, w.due = domain.LabelNone, false
	return nil
}
func (w *labelWorld) CompleteLabel(_ context.Context, d ports.Document, _ string, _ int, _ string) (bool, error) {
	id := int64(77)
	w.order.LabelStatus, w.order.LabelDocumentID, w.due = domain.LabelReady, &id, false
	w.stored = append(w.stored, d.StorageKey)
	return true, nil
}

// Puertos de salida
func (w *labelWorld) Label(_ context.Context, _ domain.Order, _ []domain.Package, attempt int) ([]byte, string, error) {
	w.calls = append(w.calls, attempt)
	if len(w.answers) >= attempt {
		if err := w.answers[attempt-1]; err != nil {
			return nil, "", err
		}
	}
	return []byte("%PDF"), "SIM1", nil
}
func (w *labelWorld) Put(context.Context, string, io.Reader) (int64, error) { return 4, nil }
func (w *labelWorld) Open(context.Context, string) (io.ReadCloser, error)   { return nil, nil }
func (w *labelWorld) OrderChanged(context.Context, domain.Order)            { w.changed++ }
func (w *labelWorld) Notify(context.Context, ports.Notification)            {}

type labelNotifications struct {
	ports.NotificationRepository
	world *labelWorld
}

func (n labelNotifications) Add(_ context.Context, x ports.Notification) (ports.Notification, error) {
	n.world.notices = append(n.world.notices, x.Kind+"/"+x.Audience+"/"+x.Warehouse)
	return x, nil
}

type labelRules struct{ ports.RuleRepository }

func (labelRules) CourierSettings(context.Context, string) (ports.CourierSettings, error) {
	return ports.CourierSettings{}, nil
}

func labelService(strategy, status string) (*Service, *labelWorld) {
	w := &labelWorld{order: domain.Order{ID: 1, CompanyID: 7, Warehouse: "central", Courier: "starken", Status: domain.StatusPacking,
		LabelStrategy: strategy, LabelStatus: status, Items: []domain.Item{{SKU: "A", Quantity: 1}}}}
	return NewService(Deps{Orders: w, Fulfillment: w, Couriers: w, Files: w, Notifier: w, Rules: labelRules{},
		Notifications: labelNotifications{world: w}}), w
}

var labelPacker = tenant.Identity{Profile: tenant.Profile{UserID: 1, Role: tenant.RoleEmpaque, AllCompanies: true}}

func TestOfficeLabelNeverCallsTheCourier(t *testing.T) {
	s, w := labelService(domain.LabelStrategyOffice, domain.LabelOffice)
	_, err := s.PrintLabel(context.Background(), labelPacker, nil, 1)
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("esperaba conflicto, llegó %v", err)
	}
	if len(w.calls) != 0 || w.order.LabelStatus != domain.LabelOffice {
		t.Fatalf("llamó al courier %v o cambió el estado a %s", w.calls, w.order.LabelStatus)
	}
}

func TestLabelReadyAtOnceIsPrintable(t *testing.T) {
	s, w := labelService(domain.LabelStrategyAPI, domain.LabelNone)
	res, err := s.PrintLabel(context.Background(), labelPacker, nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	if res.Document == nil || res.Document.ID != 77 || len(w.stored) != 1 {
		t.Fatalf("sin documento: %+v, guardados %v", res.Document, w.stored)
	}
	if len(w.notices) != 0 {
		t.Fatalf("quien la pidió ya tiene la respuesta, no necesita aviso: %v", w.notices)
	}
	// Reimprimir no vuelve a pedirla al courier.
	if _, err = s.PrintLabel(context.Background(), labelPacker, nil, 1); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(w.calls, []int{1}) || !slices.Equal(w.events, []string{"label", "label"}) {
		t.Fatalf("llamadas %v, eventos %v", w.calls, w.events)
	}
}

func TestLabelPendingArrivesLaterAndNotifies(t *testing.T) {
	s, w := labelService(domain.LabelStrategyAPI, domain.LabelNone)
	w.answers = []error{domain.ErrLabelNotReady, domain.ErrLabelNotReady}
	res, err := s.PrintLabel(context.Background(), labelPacker, nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	if res.Document != nil || res.Order.LabelStatus != domain.LabelPending {
		t.Fatalf("debía quedar pendiente: %+v", res)
	}
	// Mientras está pendiente, otro clic no vuelve a llamar al courier.
	if _, err = s.PrintLabel(context.Background(), labelPacker, nil, 1); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(w.calls, []int{1}) {
		t.Fatalf("llamadas %v", w.calls)
	}
	worker := NewLabelRetry(s)
	worker.RunOnce(context.Background())
	if w.order.LabelStatus != domain.LabelPending || len(w.notices) != 0 {
		t.Fatalf("segundo intento: %s, avisos %v", w.order.LabelStatus, w.notices)
	}
	before := w.changed
	worker.RunOnce(context.Background())
	if w.order.LabelStatus != domain.LabelReady || !slices.Equal(w.calls, []int{1, 2, 3}) {
		t.Fatalf("tercer intento: %s, llamadas %v", w.order.LabelStatus, w.calls)
	}
	if w.changed != before+1 || !slices.Equal(w.notices, []string{"label_ready/internal/central"}) {
		t.Fatalf("avisos en pantalla %d, notificaciones %v", w.changed-before, w.notices)
	}
	// Ya llegó: un reinicio o una vuelta más no la pide de nuevo.
	worker.RunOnce(context.Background())
	if len(w.calls) != 3 {
		t.Fatalf("la pidió otra vez: %v", w.calls)
	}
}

func TestLabelFailsAfterLastAttemptAndCanRetry(t *testing.T) {
	s, w := labelService(domain.LabelStrategyAPI, domain.LabelNone)
	for range domain.LabelMaxAttempts {
		w.answers = append(w.answers, errors.New("courier caído"))
	}
	if _, err := s.PrintLabel(context.Background(), labelPacker, nil, 1); err != nil {
		t.Fatal(err)
	}
	worker := NewLabelRetry(s)
	for range domain.LabelMaxAttempts { // la última vuelta ya no encuentra nada
		worker.RunOnce(context.Background())
	}
	if w.order.LabelStatus != domain.LabelFailed || len(w.calls) != domain.LabelMaxAttempts {
		t.Fatalf("estado %s, %d llamadas", w.order.LabelStatus, len(w.calls))
	}
	if !slices.Equal(w.notices, []string{"label_failed/internal/central"}) {
		t.Fatalf("avisos %v", w.notices)
	}
	// Reintentar a mano parte de nuevo.
	w.answers = nil
	res, err := s.PrintLabel(context.Background(), labelPacker, nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	if res.Document == nil || w.attempts != 1 {
		t.Fatalf("reintento manual: %+v, intentos %d", res.Document, w.attempts)
	}
}

func TestLabelWorkerStopsWhenOrderLeftPacking(t *testing.T) {
	s, w := labelService(domain.LabelStrategyAPI, domain.LabelPending)
	w.due, w.order.Status = true, domain.StatusIncident
	NewLabelRetry(s).RunOnce(context.Background())
	if len(w.calls) != 0 || w.order.LabelStatus != domain.LabelNone {
		t.Fatalf("llamadas %v, estado %s", w.calls, w.order.LabelStatus)
	}
}

func TestConfirmPackingRequestsLabelOnlyWhenNoneWasRequested(t *testing.T) {
	cases := []struct{ strategy, status, want string }{
		{domain.LabelStrategyAPI, domain.LabelNone, domain.LabelPending},
		{domain.LabelStrategyAPI, domain.LabelFailed, domain.LabelFailed},
		{domain.LabelStrategyAPI, domain.LabelReady, domain.LabelReady},
		{domain.LabelStrategyOffice, domain.LabelOffice, domain.LabelOffice},
	}
	for _, c := range cases {
		s, w := labelService(c.strategy, c.status)
		uid := int64(1)
		w.order.ClaimedBy = &uid
		res, err := s.ConfirmPacking(context.Background(), labelPacker, nil, 1, []domain.Item{{SKU: "A", Quantity: 1}}, 0)
		if err != nil {
			t.Fatal(err)
		}
		if res.Order.Status != domain.StatusReadyPickup {
			t.Fatalf("%s/%s: una etiqueta sin resolver no debe bloquear: %s", c.strategy, c.status, res.Order.Status)
		}
		if w.order.LabelStatus != c.want || len(w.calls) != 0 {
			t.Fatalf("%s/%s: quedó %s, llamadas %v", c.strategy, c.status, w.order.LabelStatus, w.calls)
		}
	}
}
