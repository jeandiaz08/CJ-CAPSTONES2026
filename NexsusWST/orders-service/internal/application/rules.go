package application

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/nexuswst/orders-service/internal/domain"
	"github.com/nexuswst/orders-service/internal/ports"
	"github.com/nexuswst/orders-service/internal/shared/tenant"
)

const ActorRules = "system:reglas"

// Matches indica si la regla aplica a la orden (condición vacía = cualquiera).
func RuleMatches(r ports.WaveRule, o domain.Order) bool {
	eq := func(cond, val string) bool { return cond == "" || strings.EqualFold(cond, val) }
	return r.Active &&
		eq(r.Courier, o.Courier) && eq(r.Channel, o.Channel) &&
		eq(r.DispatchType, o.DispatchType) && eq(r.Warehouse, o.Warehouse) &&
		(r.CompanyID == nil || *r.CompanyID == o.CompanyID)
}

// FirstMatch - gana la primera regla (por posición) que coincide (RF-17).
func FirstMatch(rules []ports.WaveRule, o domain.Order) (ports.WaveRule, bool) {
	for _, r := range rules {
		if RuleMatches(r, o) {
			return r, true
		}
	}
	return ports.WaveRule{}, false
}

// RulesEngine asigna automáticamente las órdenes Validadas sin ola (CU-02,
// CU-07). Sin regla aplicable, la orden queda Validada sin ola para que el
// Jefe decida. Si Invas no entrega las olas, se reintenta en la próxima pasada.
type RulesEngine struct{ svc *Service }

func NewRulesEngine(svc *Service) *RulesEngine { return &RulesEngine{svc: svc} }

// Candidatas que se evalúan en cada pasada y cuántas se asignan. Las retenidas no se marcan
// como revisadas: vuelven solas cuando se abre la operación.
const (
	maxRuleCandidates = 5000
	maxRuleAssigned   = 100
)

type queuedOrder struct {
	order domain.Order
	rank  string
}

// queue deja solo lo que hoy se puede asignar, en orden de cola.
func queue(schedule domain.OperationSchedule, now time.Time, orders []domain.Order, cutoffs map[string][]string) []queuedOrder {
	out := make([]queuedOrder, 0, len(orders))
	for _, o := range orders {
		if d := schedule.Decide(now, cutoffs[o.Courier], o.Priority); d.Assign {
			out = append(out, queuedOrder{order: o, rank: d.Rank})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		return domain.QueueLess(a.rank, a.order.Priority, a.order.CreatedAt, a.order.ID, b.rank, b.order.Priority, b.order.CreatedAt, b.order.ID)
	})
	return out
}

func courierCutoffs(cutoffs []ports.Cutoff) map[string][]string {
	out := map[string][]string{}
	for _, c := range cutoffs {
		for _, courier := range c.Couriers {
			out[courier.Code] = append(out[courier.Code], c.CutoffTime)
		}
	}
	return out
}

func (e *RulesEngine) RunOnce(ctx context.Context) (assigned int, err error) {
	s := e.svc
	schedule, err := s.Rules.OperationSchedule(ctx)
	if err != nil {
		return 0, err
	}
	now := s.now()
	if !schedule.IsOperationDay(now) || schedule.BeforeStart(now) {
		return 0, nil
	}
	candidates, err := s.Orders.RuleCandidates(ctx, maxRuleCandidates)
	if err != nil || len(candidates) == 0 {
		return 0, err
	}
	cutoffs, err := s.Rules.Cutoffs(ctx)
	if err != nil {
		return 0, err
	}
	ready := queue(schedule, now, candidates, courierCutoffs(cutoffs))
	if len(ready) == 0 {
		return 0, nil
	}
	if len(ready) > maxRuleAssigned {
		ready = ready[:maxRuleAssigned]
	}
	rules, err := s.Rules.List(ctx, true)
	if err != nil {
		return 0, err
	}
	// Las olas de Invas se piden una sola vez por pasada y solo si hacen falta.
	var byCode map[string]ports.Wave
	waveByCode := func(code string) (ports.Wave, bool, error) {
		if byCode == nil {
			waves, err := s.WMS.ListWaves(ctx)
			if err != nil {
				return ports.Wave{}, false, fmt.Errorf("olas de Invas: %w", err) // se reintenta después
			}
			byCode = map[string]ports.Wave{}
			for _, w := range waves {
				byCode[strings.ToUpper(w.Code)] = w
			}
		}
		w, ok := byCode[strings.ToUpper(code)]
		return w, ok, nil
	}
	var checked []int64
	for _, item := range ready {
		o := item.order
		// PV-19: producto → cliente → reglas.
		var code, reason, missing string
		productWaves, companyWave, err := s.Catalog.WavePreference(ctx, o.ID, o.CompanyID)
		if err != nil {
			return assigned, err
		}
		if pref, ok := domain.ResolveWavePreference(productWaves, companyWave); ok {
			code, reason = pref.Code, pref.Reason
			missing = fmt.Sprintf("La %s pide la ola %s, que hoy no existe en Invas", map[domain.WaveSource]string{
				domain.WaveFromProduct: "ficha del producto", domain.WaveFromCompany: "ficha del cliente"}[pref.Source], pref.Code)
		} else if r, ok := FirstMatch(rules, o); ok {
			code, reason = r.WaveCode, fmt.Sprintf("Regla «%s»", r.Name)
			missing = fmt.Sprintf("La regla «%s» pide la ola %s, que hoy no existe en Invas", r.Name, r.WaveCode)
		} else {
			checked = append(checked, o.ID) // sin regla que calce: decide el Jefe
			continue
		}
		w, ok, err := waveByCode(code)
		if err != nil {
			return assigned, err
		}
		if !ok {
			checked = append(checked, o.ID)
			_ = s.Orders.AddEvent(ctx, o.ID, "rule", "no_wave", ActorRules, missing)
			continue
		}
		ids, err := s.Orders.AssignWave(ctx, nil, []int64{o.ID}, w.ID, ActorRules, reason+" → "+w.Name, map[int64]string{o.ID: item.rank})
		if err != nil {
			return assigned, err
		}
		if len(ids) == 1 {
			assigned++
			if o2, e := s.Orders.Get(ctx, nil, o.ID); e == nil {
				s.Notifier.OrderChanged(ctx, o2)
			}
		}
	}
	return assigned, s.Orders.MarkRulesChecked(ctx, checked)
}

// ---------------------------------------------------------------------------
// Administración de reglas y couriers (Jefe de Operaciones / Administrador)
// ---------------------------------------------------------------------------

func canManageRules(id tenant.Identity) bool {
	return id.HasRole(tenant.RoleAdmin, tenant.RoleJefeOps)
}

func (s *Service) ListRules(ctx context.Context, id tenant.Identity) ([]ports.WaveRule, error) {
	if !canManageRules(id) {
		return nil, domain.ErrForbidden
	}
	return s.Rules.List(ctx, false)
}

var reWaveCode = regexp.MustCompile(`^[A-Za-z0-9_-]{1,20}$`)

func (s *Service) SaveRule(ctx context.Context, id tenant.Identity, r ports.WaveRule) (ports.WaveRule, error) {
	if !canManageRules(id) {
		return ports.WaveRule{}, domain.ErrForbidden
	}
	r.Name = strings.TrimSpace(r.Name)
	r.Courier = strings.ToLower(strings.TrimSpace(r.Courier))
	r.Channel = strings.ToLower(strings.TrimSpace(r.Channel))
	r.DispatchType = strings.ToLower(strings.TrimSpace(r.DispatchType))
	r.Warehouse = strings.ToLower(strings.TrimSpace(r.Warehouse))
	r.WaveCode = strings.ToUpper(strings.TrimSpace(r.WaveCode))
	if r.Name == "" || len(r.Name) > 80 {
		return ports.WaveRule{}, fail(domain.ErrInvalid, "La regla necesita un nombre (máx. 80 caracteres).")
	}
	if !reWaveCode.MatchString(r.WaveCode) {
		return ports.WaveRule{}, fail(domain.ErrInvalid, "Indica el código de ola de Invas (p. ej. GEN, AM, PM).")
	}
	if r.DispatchType != "" && !slices.Contains(domain.DispatchTypes, r.DispatchType) {
		return ports.WaveRule{}, fail(domain.ErrInvalid, "Tipo de despacho desconocido.")
	}
	if r.Courier != "" {
		if ok, err := s.Catalog.CourierActive(ctx, r.Courier); err != nil {
			return ports.WaveRule{}, err
		} else if !ok {
			return ports.WaveRule{}, fail(domain.ErrInvalid, "El courier %s no existe o está inactivo.", r.Courier)
		}
	}
	saved, err := s.Rules.Save(ctx, r, id.Actor())
	if err == nil {
		_ = s.Rules.ResetChecks(ctx) // las órdenes sin ola se evalúan con la regla nueva
	}
	return saved, err
}

func (s *Service) DeleteRule(ctx context.Context, id tenant.Identity, ruleID int64) error {
	if !canManageRules(id) {
		return domain.ErrForbidden
	}
	return s.Rules.Delete(ctx, ruleID)
}

func (s *Service) ReorderRules(ctx context.Context, id tenant.Identity, ids []int64) error {
	if !canManageRules(id) {
		return domain.ErrForbidden
	}
	if len(ids) == 0 {
		return fail(domain.ErrInvalid, "Envía el orden completo de las reglas.")
	}
	if err := s.Rules.Reorder(ctx, ids, id.Actor()); err != nil {
		return err
	}
	return s.Rules.ResetChecks(ctx)
}

// Warehouses - sugerencias para el campo Bodega de las reglas.
func (s *Service) Warehouses(ctx context.Context, id tenant.Identity, scope []int64) ([]string, error) {
	if !canManageRules(id) {
		return nil, domain.ErrForbidden
	}
	return s.Orders.Warehouses(ctx, scope)
}

// RunRulesNow - el Jefe puede pedir que se apliquen las reglas de inmediato.
func (s *Service) RunRulesNow(ctx context.Context, id tenant.Identity) (int, error) {
	if !canManageRules(id) {
		return 0, domain.ErrForbidden
	}
	if err := s.Rules.ResetChecks(ctx); err != nil {
		return 0, err
	}
	return NewRulesEngine(s).RunOnce(ctx)
}

var reHHMM = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)

func (s *Service) ListCourierSettings(ctx context.Context, id tenant.Identity) ([]ports.CourierSettings, error) {
	if !canManageRules(id) && !id.HasPermission(tenant.PermDespacho) {
		return nil, domain.ErrForbidden
	}
	return s.Rules.ListCourierSettings(ctx)
}

// SaveCourierSettings - máximo de bultos y cobertura. La llegada vive en los cortes.
func (s *Service) SaveCourierSettings(ctx context.Context, id tenant.Identity, cs ports.CourierSettings) error {
	if !canManageRules(id) {
		return domain.ErrForbidden
	}
	cs.Courier = strings.ToLower(strings.TrimSpace(cs.Courier))
	if ok, err := s.Catalog.CourierActive(ctx, cs.Courier); err != nil {
		return err
	} else if !ok {
		return fail(domain.ErrInvalid, "El courier %s no existe o está inactivo.", cs.Courier)
	}
	if cs.MaxPackages != nil && *cs.MaxPackages <= 0 {
		return fail(domain.ErrInvalid, "El máximo de bultos debe ser mayor que 0.")
	}
	if !cs.CoversAll {
		communes, err := s.Catalog.Communes(ctx)
		if err != nil {
			return err
		}
		clean := make([]string, 0, len(cs.Communes))
		for _, c := range cs.Communes {
			off, sug := domain.MatchCommune(c, communes)
			if off == "" {
				if sug != "" {
					return fail(domain.ErrInvalid, "Comuna «%s» no reconocida. ¿Quisiste decir «%s»?", c, sug)
				}
				return fail(domain.ErrInvalid, "Comuna «%s» no reconocida.", c)
			}
			if !slices.Contains(clean, off) {
				clean = append(clean, off)
			}
		}
		if len(clean) == 0 {
			return fail(domain.ErrInvalid, "Indica al menos una comuna de cobertura o marca «cubre todo el país».")
		}
		cs.Communes = clean
	} else {
		cs.Communes = nil
	}
	return s.Rules.SaveCourierSettings(ctx, cs, id.Actor())
}
