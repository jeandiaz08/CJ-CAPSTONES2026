package application

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/nexuswst/orders-service/internal/domain"
	"github.com/nexuswst/orders-service/internal/ports"
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
	schedule, err := s.Rules.OperationSchedule(ctx)
	if err != nil {
		return err
	}
	if value <= schedule.OperationStart {
		return fail(domain.ErrInvalid, "La hora límite debe ser posterior al inicio de operación (%s).", schedule.OperationStart)
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

// OperationState: cómo está la operación ahora, para avisar en pantalla.
type OperationState struct {
	domain.OperationSchedule
	OperationDay  bool      `json:"operationDay"`
	BeforeStart   bool      `json:"beforeStart"`
	AfterDeadline bool      `json:"afterDeadline"`
	NextRelease   time.Time `json:"nextRelease"`
}

func (s *Service) OperationSchedule(ctx context.Context, id tenant.Identity) (OperationState, error) {
	if !canManageRules(id) {
		return OperationState{}, domain.ErrForbidden
	}
	schedule, err := s.Rules.OperationSchedule(ctx)
	if err != nil {
		return OperationState{}, err
	}
	now := s.now()
	return OperationState{OperationSchedule: schedule, OperationDay: schedule.IsOperationDay(now), BeforeStart: schedule.BeforeStart(now),
		AfterDeadline: schedule.AfterDeadline(now), NextRelease: schedule.NextRelease(now)}, nil
}

// SaveOperationSchedule: inicio y días de operación. Sin priorityOverrides, el parámetro no cambia.
func (s *Service) SaveOperationSchedule(ctx context.Context, id tenant.Identity, start string, days []string, priorityOverrides *bool) error {
	if !canManageRules(id) {
		return domain.ErrForbidden
	}
	if !reHHMM.MatchString(start) {
		return fail(domain.ErrInvalid, "El inicio de operación debe tener formato HH:MM.")
	}
	clean := make([]string, 0, len(days))
	for _, day := range domain.WeekDays[1:] {
		if slices.Contains(days, day) {
			clean = append(clean, day)
		}
	}
	if slices.Contains(days, domain.WeekDays[0]) {
		clean = append(clean, domain.WeekDays[0])
	}
	if len(clean) != len(days) || len(clean) == 0 {
		return fail(domain.ErrInvalid, "Elige al menos un día de operación válido (%s).", strings.Join(domain.WeekDays, ", "))
	}
	schedule, err := s.Rules.OperationSchedule(ctx)
	if err != nil {
		return err
	}
	cutoffs, err := s.Rules.Cutoffs(ctx)
	if err != nil {
		return err
	}
	for _, c := range cutoffs {
		if c.CutoffTime <= start {
			return fail(domain.ErrInvalid, "El inicio de operación debe ser anterior al primer corte (%s).", firstCutoff(cutoffs))
		}
	}
	if schedule.AssignmentDeadline != "" && start >= schedule.AssignmentDeadline {
		return fail(domain.ErrInvalid, "El inicio de operación debe ser anterior a la hora límite (%s).", schedule.AssignmentDeadline)
	}
	schedule.OperationStart, schedule.OperationDays = start, clean
	if priorityOverrides != nil {
		schedule.PriorityOverridesDeadline = *priorityOverrides
	}
	return s.Rules.SaveOperationSchedule(ctx, schedule, id.Actor())
}

func firstCutoff(cutoffs []ports.Cutoff) string {
	first := cutoffs[0].CutoffTime
	for _, c := range cutoffs {
		if c.CutoffTime < first {
			first = c.CutoffTime
		}
	}
	return first
}
