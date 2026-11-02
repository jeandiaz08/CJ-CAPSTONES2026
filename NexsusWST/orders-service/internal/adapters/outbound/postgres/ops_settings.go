package postgres

import "context"

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
