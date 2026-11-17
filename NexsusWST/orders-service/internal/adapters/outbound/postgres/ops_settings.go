package postgres

import (
	"context"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/nexuswst/orders-service/internal/domain"
)

// OperationSchedule: sin la clave, el horario no restringe nada.
func (r *RuleRepository) OperationSchedule(ctx context.Context) (domain.OperationSchedule, error) {
	s := domain.OperationSchedule{OperationStart: "00:00", OperationDays: domain.WeekDays}
	rows, err := r.db.Query(ctx, `SELECT key, value FROM ops_settings WHERE key IN ('assignment_deadline','operation_start','operation_days','priority_overrides_deadline')`)
	if err != nil {
		return s, err
	}
	defer rows.Close()
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return s, err
		}
		switch key {
		case "assignment_deadline":
			s.AssignmentDeadline = value
		case "operation_start":
			s.OperationStart = value
		case "operation_days":
			s.OperationDays = strings.Split(value, ",")
		case "priority_overrides_deadline":
			s.PriorityOverridesDeadline = value == "true"
		}
	}
	return s, rows.Err()
}

func (r *RuleRepository) SaveOperationSchedule(ctx context.Context, s domain.OperationSchedule, actor string) error {
	return pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		for key, value := range map[string]string{
			"operation_start":             s.OperationStart,
			"operation_days":              strings.Join(s.OperationDays, ","),
			"priority_overrides_deadline": strconv.FormatBool(s.PriorityOverridesDeadline),
		} {
			if _, err := tx.Exec(ctx, `INSERT INTO ops_settings (key,value,updated_by) VALUES ($1,$2,$3)
 ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value, updated_by=EXCLUDED.updated_by, updated_at=now()`, key, value, actor); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *RuleRepository) AssignmentDeadline(ctx context.Context) (string, error) {
	var value string
	err := r.db.QueryRow(ctx, `SELECT value FROM ops_settings WHERE key='assignment_deadline'`).Scan(&value)
	return value, err
}

func (r *RuleRepository) SaveAssignmentDeadline(ctx context.Context, value, actor string) error {
	_, err := r.db.Exec(ctx, `INSERT INTO ops_settings (key,value,updated_by) VALUES ('assignment_deadline',$1,$2)
 ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value, updated_by=EXCLUDED.updated_by,updated_at=now()`, value, actor)
	return err
}
