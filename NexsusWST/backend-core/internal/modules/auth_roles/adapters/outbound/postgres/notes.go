package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/nexuswst/backend-core/internal/modules/auth_roles/domain"
)

const noteCols = `id, body, to_char(due_date,'YYYY-MM-DD'), done, done_at, created_at, updated_at`

func scanNote(row pgx.Row) (domain.Note, error) {
	var n domain.Note
	err := row.Scan(&n.ID, &n.Body, &n.DueDate, &n.Done, &n.DoneAt, &n.CreatedAt, &n.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return n, domain.ErrNotFound
	}
	return n, err
}

func (r *Repository) ListNotes(ctx context.Context, userID int64) ([]domain.Note, error) {
	rows, err := r.db.Query(ctx, `SELECT `+noteCols+` FROM user_notes WHERE user_id=$1 ORDER BY done, due_date NULLS LAST, id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Note{}
	for rows.Next() {
		n, err := scanNote(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (r *Repository) CreateNote(ctx context.Context, userID int64, c domain.NoteChanges) (domain.Note, error) {
	done := false
	if c.Done != nil {
		done = *c.Done
	}
	return scanNote(r.db.QueryRow(ctx, `INSERT INTO user_notes(user_id,body,due_date,done,done_at)
 VALUES($1,$2,$3::date,$4,CASE WHEN $4 THEN now() END) RETURNING `+noteCols, userID, c.Body, c.DueDate, done))
}

func (r *Repository) UpdateNote(ctx context.Context, userID, noteID int64, c domain.NoteChanges) (domain.Note, error) {
	return scanNote(r.db.QueryRow(ctx, `UPDATE user_notes SET body=COALESCE($3,body),
 due_date=CASE WHEN $4 THEN $5::date ELSE due_date END,
 done=COALESCE($6,done),done_at=CASE WHEN $6::boolean IS NULL THEN done_at
 WHEN $6 THEN COALESCE(done_at,now()) ELSE NULL END,updated_at=now()
 WHERE user_id=$1 AND id=$2 RETURNING `+noteCols, userID, noteID, c.Body, c.DueDateSet, c.DueDate, c.Done))
}

func (r *Repository) DeleteNote(ctx context.Context, userID, noteID int64) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM user_notes WHERE user_id=$1 AND id=$2`, userID, noteID)
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return err
}
