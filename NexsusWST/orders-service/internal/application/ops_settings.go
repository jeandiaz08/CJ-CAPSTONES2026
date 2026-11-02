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
	return s.Rules.SaveAssignmentDeadline(ctx, value, id.Actor())
}
