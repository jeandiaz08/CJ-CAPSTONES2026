package application

import (
	"context"

	"github.com/nexuswst/orders-service/internal/domain"
	"github.com/nexuswst/orders-service/internal/shared/tenant"
)

func (s *Service) AssignmentDeadline(ctx context.Context, id tenant.Identity) (string, error) {
	if !canManageRules(id) {
		return "", domain.ErrForbidden
	}
	return s.Rules.AssignmentDeadline(ctx)
}

func (s *Service) SaveAssignmentDeadline(ctx context.Context, id tenant.Identity, value string) error {
	if !canManageRules(id) {
		return domain.ErrForbidden
	}
	if !reHHMM.MatchString(value) {
		return fail(domain.ErrInvalid, "La hora límite debe tener formato HH:MM.")
	}
	cutoffs, err := s.Rules.Cutoffs(ctx)
	if err != nil {
		return err
	}
	latest := ""
	for _, c := range cutoffs {
		if c.CutoffTime > latest {
			latest = c.CutoffTime
		}
	}
	if latest > value {
		return fail(domain.ErrInvalid, "La hora límite no puede ser anterior al último corte (%s). Muévelo o desactívalo primero.", latest)
	}
	return s.Rules.SaveAssignmentDeadline(ctx, value, id.Actor())
}
