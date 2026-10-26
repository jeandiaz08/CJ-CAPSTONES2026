package application

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"

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

func (e *RulesEngine) RunOnce(ctx context.Context) (assigned int, err error) {
	s := e.svc
	orders, err := s.Orders.RuleCandidates(ctx, 100)
	if err != nil || len(orders) == 0 {
		return 0, err
	}
	rules, err := s.Rules.List(ctx, true)
	if err != nil {
		return 0, err
	}
	var checked []int64
	if len(rules) > 0 {
		waves, err := s.WMS.ListWaves(ctx)
		if err != nil {
			return 0, fmt.Errorf("olas de Invas: %w", err) // se reintenta después
		}
		byCode := map[string]ports.Wave{}
		for _, w := range waves {
			byCode[strings.ToUpper(w.Code)] = w
		}
		for _, o := range orders {
			r, ok := FirstMatch(rules, o)
			if !ok {
				checked = append(checked, o.ID)
				continue
			}
			w, ok := byCode[strings.ToUpper(r.WaveCode)]
			if !ok {
				checked = append(checked, o.ID)
				_ = s.Orders.AddEvent(ctx, o.ID, "rule", "no_wave", ActorRules,
					fmt.Sprintf("La regla «%s» pide la ola %s, que hoy no existe en Invas", r.Name, r.WaveCode))
				continue
			}
			ids, err := s.Orders.AssignWave(ctx, nil, []int64{o.ID}, w.ID, ActorRules, fmt.Sprintf("Regla «%s» → %s", r.Name, w.Name))
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
	} else {
		for _, o := range orders {
			checked = append(checked, o.ID)
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

// SaveCourierSettings - hora de llegada (corte), máximo de bultos y cobertura.
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
	if cs.ArrivalTime != "" && !reHHMM.MatchString(cs.ArrivalTime) {
		return fail(domain.ErrInvalid, "La hora de llegada debe tener formato HH:MM.")
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
