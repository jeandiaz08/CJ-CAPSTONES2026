package application

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/nexuswst/orders-service/internal/domain"
	"github.com/nexuswst/orders-service/internal/ports"
	"github.com/nexuswst/orders-service/internal/shared/tenant"
)

type queueOrders struct {
	ports.OrderRepository
	orders   []domain.Order
	assigned []int64
	ranks    map[int64]string
	checked  []int64
}

func (f *queueOrders) RuleCandidates(context.Context, int) ([]domain.Order, error) {
	return f.orders, nil
}

func (f *queueOrders) List(_ context.Context, q ports.ListFilter) ([]domain.Order, int, error) {
	var out []domain.Order
	for _, o := range f.orders {
		if slices.Contains(q.IDs, o.ID) {
			out = append(out, o)
		}
	}
	return out, len(out), nil
}

func (f *queueOrders) AssignWave(_ context.Context, _, ids []int64, _, _, _ string, ranks map[int64]string) ([]int64, error) {
	f.assigned = append(f.assigned, ids...)
	if f.ranks == nil {
		f.ranks = map[int64]string{}
	}
	for id, rank := range ranks {
		f.ranks[id] = rank
	}
	return ids, nil
}

func (f *queueOrders) Get(context.Context, []int64, int64) (domain.Order, error) {
	return domain.Order{}, nil
}

func (f *queueOrders) MarkRulesChecked(_ context.Context, ids []int64) error {
	f.checked = append(f.checked, ids...)
	return nil
}

type queueRules struct {
	ports.RuleRepository
	schedule domain.OperationSchedule
	noRules  bool
}

func (f queueRules) OperationSchedule(context.Context) (domain.OperationSchedule, error) {
	return f.schedule, nil
}

func (queueRules) Cutoffs(context.Context) ([]ports.Cutoff, error) {
	return []ports.Cutoff{
		{CutoffTime: "08:00", Couriers: []ports.CutoffCourier{{Code: "temprano"}}},
		{CutoffTime: "11:00", Couriers: []ports.CutoffCourier{{Code: "tarde"}}},
	}, nil
}

func (f queueRules) List(context.Context, bool) ([]ports.WaveRule, error) {
	if f.noRules {
		return nil, nil
	}
	return []ports.WaveRule{{Name: "Todas", Active: true, WaveCode: "GEN"}}, nil
}

type queueWMS struct{ ports.WMSGateway }

func (queueWMS) ListWaves(context.Context) ([]ports.Wave, error) {
	return []ports.Wave{{ID: "OLA-GEN", Code: "GEN", Name: "General"}}, nil
}

var santiago = func() *time.Location {
	loc, err := time.LoadLocation("America/Santiago")
	if err != nil {
		panic(err)
	}
	return loc
}()

// Semana de noviembre de 2026: lunes 2 a domingo 8; lunes siguiente, 9.
func when(day, hour, minute int) time.Time {
	return time.Date(2026, time.November, day, hour, minute, 0, 0, santiago)
}

func schedule() domain.OperationSchedule {
	return domain.OperationSchedule{AssignmentDeadline: "12:00", OperationStart: "06:30", OperationDays: []string{"mon", "tue", "wed", "thu", "fri"}}
}

func queueOrder(id int64, courier string, priority bool, created time.Time) domain.Order {
	return domain.Order{ID: id, Courier: courier, Priority: priority, CreatedAt: created, Status: domain.StatusValidated}
}

func engineAt(now time.Time, sched domain.OperationSchedule, orders ...domain.Order) (*RulesEngine, *queueOrders) {
	repo := &queueOrders{orders: orders}
	svc := NewService(Deps{Orders: repo, Rules: queueRules{schedule: sched}, WMS: queueWMS{}, Notifier: silentNotifier{}, Now: func() time.Time { return now }})
	return NewRulesEngine(svc), repo
}

func run(t *testing.T, e *RulesEngine) {
	t.Helper()
	if _, err := e.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
}

// Lo que queda del viernes, más lo del sábado y el domingo, sale el lunes 06:30:
// corte → prioridad → antigüedad.
func TestEngineReleasesOnNextOperationDay(t *testing.T) {
	orders := []domain.Order{
		queueOrder(1, "tarde", true, when(6, 13, 0)),     // viernes, prioritaria, corte 11:00
		queueOrder(2, "temprano", false, when(8, 10, 0)), // domingo, corte 08:00
		queueOrder(3, "temprano", false, when(6, 14, 0)), // viernes, más antigua, corte 08:00
		queueOrder(4, "temprano", true, when(7, 9, 0)),   // sábado, prioritaria, corte 08:00
		queueOrder(5, "sin_cortes", true, when(6, 13, 0)),
	}
	for name, now := range map[string]time.Time{
		"viernes después de la hora límite": when(6, 12, 1),
		"sábado":                            when(7, 9, 0),
		"domingo":                           when(8, 23, 59),
		"lunes un minuto antes del inicio":  when(9, 6, 29),
	} {
		t.Run(name, func(t *testing.T) {
			e, repo := engineAt(now, schedule(), orders...)
			run(t, e)
			if len(repo.assigned) != 0 || len(repo.checked) != 0 {
				t.Fatalf("asignó %v y marcó %v", repo.assigned, repo.checked)
			}
		})
	}
	e, repo := engineAt(when(9, 6, 30), schedule(), orders...)
	run(t, e)
	if want := []int64{4, 3, 2, 1, 5}; !slices.Equal(repo.assigned, want) {
		t.Fatalf("orden: %v, esperado %v", repo.assigned, want)
	}
	if repo.ranks[4] != "08:00" || repo.ranks[1] != "11:00" || repo.ranks[5] != "" {
		t.Fatalf("lugar en la cola: %v", repo.ranks)
	}
}

func TestEngineCutoffRules(t *testing.T) {
	created := when(2, 10, 55)
	t.Run("creada 10:55 y validada 11:05 con corte 11:00: retenida", func(t *testing.T) {
		e, repo := engineAt(when(2, 11, 5), schedule(), queueOrder(1, "tarde", false, created))
		run(t, e)
		if len(repo.assigned) != 0 || len(repo.checked) != 0 {
			t.Fatalf("debe quedar retenida: %v %v", repo.assigned, repo.checked)
		}
	})
	t.Run("la retenida sale al día siguiente", func(t *testing.T) {
		e, repo := engineAt(when(3, 6, 30), schedule(), queueOrder(1, "tarde", false, created))
		run(t, e)
		if !slices.Equal(repo.assigned, []int64{1}) || repo.ranks[1] != "11:00" {
			t.Fatalf("asignadas: %v %v", repo.assigned, repo.ranks)
		}
	})
	t.Run("prioritaria fuera de corte sale el mismo día, antes que las demás", func(t *testing.T) {
		e, repo := engineAt(when(2, 11, 5), schedule(), queueOrder(1, "sin_cortes", false, created), queueOrder(2, "tarde", true, created))
		run(t, e)
		if !slices.Equal(repo.assigned, []int64{2, 1}) || repo.ranks[2] != domain.PriorityRank {
			t.Fatalf("asignadas: %v %v", repo.assigned, repo.ranks)
		}
	})
	t.Run("la prioridad no salta la hora límite", func(t *testing.T) {
		e, repo := engineAt(when(2, 12, 1), schedule(), queueOrder(1, "tarde", true, created))
		run(t, e)
		if len(repo.assigned) != 0 {
			t.Fatalf("asignó %v", repo.assigned)
		}
	})
	t.Run("con priority_overrides_deadline, sí la salta", func(t *testing.T) {
		sched := schedule()
		sched.PriorityOverridesDeadline = true
		e, repo := engineAt(when(2, 12, 1), sched, queueOrder(1, "tarde", true, created), queueOrder(2, "tarde", false, created))
		run(t, e)
		if !slices.Equal(repo.assigned, []int64{1}) {
			t.Fatalf("asignó %v", repo.assigned)
		}
	})
}

func TestEngineLimitKeepsEarliestCutoffs(t *testing.T) {
	orders := make([]domain.Order, 0, 151)
	for i := int64(1); i <= 150; i++ {
		orders = append(orders, queueOrder(i, "tarde", false, when(2, 6, 30)))
	}
	orders = append(orders, queueOrder(999, "temprano", false, when(2, 7, 0)))
	e, repo := engineAt(when(2, 7, 30), schedule(), orders...)
	run(t, e)
	if len(repo.assigned) != maxRuleAssigned || repo.assigned[0] != 999 {
		t.Fatalf("asignadas %d, primera %d", len(repo.assigned), repo.assigned[0])
	}
}

func TestEngineWithoutRulesOnlyChecksWhatItCouldAssign(t *testing.T) {
	repo := &queueOrders{orders: []domain.Order{queueOrder(1, "temprano", false, when(2, 7, 0)), queueOrder(2, "tarde", false, when(2, 7, 0))}}
	svc := NewService(Deps{Orders: repo, Rules: queueRules{schedule: schedule(), noRules: true}, Now: func() time.Time { return when(2, 9, 0) }})
	if _, err := NewRulesEngine(svc).RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(repo.checked, []int64{2}) {
		t.Fatalf("marcadas como revisadas: %v", repo.checked)
	}
}

func TestManualAssignment(t *testing.T) {
	assign := func(now time.Time, sched domain.OperationSchedule, ids []int64, orders ...domain.Order) (*queueOrders, []int64, error) {
		repo := &queueOrders{orders: orders}
		svc := NewService(Deps{Orders: repo, Rules: queueRules{schedule: sched}, WMS: queueWMS{}, Notifier: silentNotifier{}, Now: func() time.Time { return now }})
		got, err := svc.AssignWave(context.Background(), admin(), nil, ids, "OLA-GEN")
		return repo, got, err
	}
	normal := queueOrder(1, "tarde", false, when(2, 7, 0))
	priority := queueOrder(2, "tarde", true, when(2, 7, 0))

	t.Run("fuera de corte se permite y va al final", func(t *testing.T) {
		repo, got, err := assign(when(2, 11, 30), schedule(), []int64{1}, normal)
		if err != nil || !slices.Equal(got, []int64{1}) || repo.ranks[1] != "" {
			t.Fatalf("%v %v %v", got, repo.ranks, err)
		}
	})
	t.Run("en un día sin operación se permite", func(t *testing.T) {
		_, got, err := assign(when(7, 15, 0), schedule(), []int64{1}, normal)
		if err != nil || !slices.Equal(got, []int64{1}) {
			t.Fatalf("%v %v", got, err)
		}
	})
	t.Run("antes del inicio se permite", func(t *testing.T) {
		if _, got, err := assign(when(2, 6, 0), schedule(), []int64{1}, normal); err != nil || len(got) != 1 {
			t.Fatalf("%v %v", got, err)
		}
	})
	t.Run("después de la hora límite se bloquea, aun siendo prioritaria", func(t *testing.T) {
		repo, _, err := assign(when(2, 12, 1), schedule(), []int64{1, 2}, normal, priority)
		var op OpError
		if !errors.As(err, &op) || !errors.Is(err, domain.ErrInvalid) || len(repo.assigned) != 0 {
			t.Fatalf("debe responder 422: %v %v", err, repo.assigned)
		}
	})
	t.Run("con priority_overrides_deadline pasan solo las prioritarias", func(t *testing.T) {
		sched := schedule()
		sched.PriorityOverridesDeadline = true
		_, got, err := assign(when(2, 12, 1), sched, []int64{1, 2}, normal, priority)
		if err != nil || !slices.Equal(got, []int64{2}) {
			t.Fatalf("%v %v", got, err)
		}
	})
}

type packingFulfillment struct {
	ports.FulfillmentRepository
	includeDisabled bool
}

func (f *packingFulfillment) PackingQueue(_ context.Context, _ []int64, _ int64, includeDisabled bool) ([]domain.Order, error) {
	f.includeDisabled = includeDisabled
	return nil, nil
}

func TestPackingQueueDisabledOnlyForManagers(t *testing.T) {
	for role, want := range map[string]bool{tenant.RoleAdmin: true, tenant.RoleJefeOps: true, tenant.RoleEmpaque: false} {
		repo := &packingFulfillment{includeDisabled: !want}
		s := NewService(Deps{Fulfillment: repo})
		id := tenant.Identity{Profile: tenant.Profile{UserID: 1, Role: role, AllCompanies: true}}
		if _, err := s.PackingQueue(context.Background(), id, nil); err != nil {
			t.Fatal(err)
		}
		if repo.includeDisabled != want {
			t.Fatalf("%s: incluye deshabilitados = %v", role, repo.includeDisabled)
		}
	}
}
