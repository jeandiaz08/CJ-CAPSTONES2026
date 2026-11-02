package application

import (
	"context"
	"github.com/nexuswst/orders-service/internal/domain"
	"github.com/nexuswst/orders-service/internal/ports"
	"github.com/nexuswst/orders-service/internal/shared/tenant"
	"strings"
)

func (s *Service) Cutoffs(ctx context.Context, id tenant.Identity) ([]ports.Cutoff, error) {
	if !canManageRules(id) {
		return nil, domain.ErrForbidden
	}
	return s.Rules.Cutoffs(ctx)
}
func (s *Service) SaveCutoff(ctx context.Context, id tenant.Identity, c ports.Cutoff) (ports.Cutoff, error) {
	if !canManageRules(id) {
		return c, domain.ErrForbidden
	}
	if !reHHMM.MatchString(c.CutoffTime) {
		return c, fail(domain.ErrInvalid, "Hora de corte inválida (HH:MM).")
	}
	schedule, err := s.Rules.OperationSchedule(ctx)
	if err != nil {
		return c, err
	}
	if schedule.AssignmentDeadline != "" && c.CutoffTime > schedule.AssignmentDeadline {
		return c, fail(domain.ErrInvalid, "El corte no puede ser posterior a la hora límite (%s).", schedule.AssignmentDeadline)
	}
	if c.CutoffTime <= schedule.OperationStart {
		return c, fail(domain.ErrInvalid, "El corte debe ser posterior al inicio de operación (%s).", schedule.OperationStart)
	}
	if c.Name != nil {
		v := strings.TrimSpace(*c.Name)
		if v == "" {
			c.Name = nil
		} else {
			c.Name = &v
		}
	}
	if len(c.Couriers) == 0 {
		return c, fail(domain.ErrInvalid, "Agrega al menos un courier.")
	}
	seen := map[string]bool{}
	for i := range c.Couriers {
		v := &c.Couriers[i]
		v.Code = strings.ToLower(strings.TrimSpace(v.Code))
		if seen[v.Code] {
			return c, fail(domain.ErrInvalid, "Courier repetido.")
		}
		seen[v.Code] = true
		if !reHHMM.MatchString(v.ArrivalTime) || v.ArrivalTime < c.CutoffTime {
			return c, fail(domain.ErrInvalid, "La llegada debe ser HH:MM y no anterior al corte.")
		}
		ok, err := s.Catalog.CourierActive(ctx, v.Code)
		if err != nil {
			return c, err
		}
		if !ok {
			return c, fail(domain.ErrInvalid, "Courier inexistente o inactivo.")
		}
	}
	return s.Rules.SaveCutoff(ctx, c, id.Actor())
}
func (s *Service) DeleteCutoff(ctx context.Context, id tenant.Identity, n int64) error {
	if !canManageRules(id) {
		return domain.ErrForbidden
	}
	return s.Rules.DeleteCutoff(ctx, n, id.Actor())
}
