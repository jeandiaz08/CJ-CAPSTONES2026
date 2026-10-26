package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nexuswst/backend-core/internal/modules/auth_roles/domain"
	"github.com/nexuswst/backend-core/internal/modules/auth_roles/ports"
)

type Repository struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

const selectUser = `
SELECT u.id, COALESCE(u.keycloak_id::text, ''), u.email, u.name, u.role, u.permissions, u.all_companies,
       u.active, u.last_login_at, u.created_at, u.updated_at,
       COALESCE(array_agg(uc.company_id ORDER BY uc.company_id) FILTER (WHERE uc.company_id IS NOT NULL), '{}')
FROM users u
LEFT JOIN user_companies uc ON uc.user_id = u.id`

func scanUser(row pgx.Row) (domain.User, error) {
	var u domain.User
	err := row.Scan(&u.ID, &u.KeycloakID, &u.Email, &u.Name, &u.Role, &u.Permissions, &u.AllCompanies,
		&u.Active, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt, &u.Companies)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, domain.ErrNotFound
	}
	return u, err
}

func (r *Repository) GetByID(ctx context.Context, id int64) (domain.User, error) {
	return scanUser(r.db.QueryRow(ctx, selectUser+` WHERE u.id = $1 GROUP BY u.id`, id))
}

func (r *Repository) GetByEmail(ctx context.Context, email string) (domain.User, error) {
	return scanUser(r.db.QueryRow(ctx, selectUser+` WHERE u.email = $1 GROUP BY u.id`, email))
}

func (r *Repository) List(ctx context.Context, f ports.ListFilter) ([]domain.User, int, error) {
	where, args := []string{"TRUE"}, []any{}
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(cond, len(args)))
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		add("(u.email ILIKE '%%' || $%[1]d || '%%' OR u.name ILIKE '%%' || $%[1]d || '%%')", q)
	}
	if f.Role != "" {
		add("u.role = $%d", f.Role)
	}
	if f.Active != nil {
		add("u.active = $%d", *f.Active)
	}
	w := strings.Join(where, " AND ")
	var total int
	if err := r.db.QueryRow(ctx, "SELECT count(*) FROM users u WHERE "+w, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	args = append(args, f.Limit, f.Offset)
	rows, err := r.db.Query(ctx, fmt.Sprintf("%s WHERE %s GROUP BY u.id ORDER BY u.active DESC, u.name LIMIT $%d OFFSET $%d",
		selectUser, w, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []domain.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, u)
	}
	return out, total, rows.Err()
}

func (r *Repository) Insert(ctx context.Context, keycloakID string, d domain.Draft, actorID *int64) (domain.User, error) {
	var id int64
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		var kc any
		if keycloakID != "" {
			kc = keycloakID
		}
		if err := tx.QueryRow(ctx, `
			INSERT INTO users (keycloak_id, email, name, role, permissions, all_companies, created_by, updated_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $7) RETURNING id`,
			kc, d.Email, d.Name, d.Role, d.Permissions, d.AllCompanies, actorID).Scan(&id); err != nil {
			return err
		}
		return setCompanies(ctx, tx, id, d.Companies)
	})
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		return domain.User{}, domain.ErrEmailTaken
	}
	if err != nil {
		return domain.User{}, err
	}
	return r.GetByID(ctx, id)
}

func (r *Repository) Update(ctx context.Context, id int64, d domain.Draft, actorID int64) error {
	return pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE users SET name=$2, role=$3, permissions=$4, all_companies=$5, updated_by=$6, updated_at=now()
			WHERE id=$1`, id, d.Name, d.Role, d.Permissions, d.AllCompanies, actorID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrNotFound
		}
		return setCompanies(ctx, tx, id, d.Companies)
	})
}

func setCompanies(ctx context.Context, tx pgx.Tx, userID int64, companies []int64) error {
	if _, err := tx.Exec(ctx, `DELETE FROM user_companies WHERE user_id = $1`, userID); err != nil {
		return err
	}
	if len(companies) == 0 {
		return nil
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO user_companies (user_id, company_id)
		SELECT $1, unnest($2::bigint[]) ON CONFLICT DO NOTHING`, userID, companies)
	return err
}

func (r *Repository) SetActive(ctx context.Context, id int64, active bool, actorID int64) error {
	tag, err := r.db.Exec(ctx, `UPDATE users SET active=$2, updated_by=$3, updated_at=now() WHERE id=$1`, id, active, actorID)
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return err
}

func (r *Repository) LinkKeycloak(ctx context.Context, id int64, keycloakID string) error {
	tag, err := r.db.Exec(ctx, `UPDATE users SET keycloak_id=$2, updated_at=now() WHERE id=$1 AND keycloak_id IS NULL`, id, keycloakID)
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return err
}

func (r *Repository) RelinkKeycloak(ctx context.Context, id int64, oldID, newID string) error {
	tag, err := r.db.Exec(ctx, `UPDATE users SET keycloak_id=$3, updated_at=now() WHERE id=$1 AND keycloak_id=$2`, id, oldID, newID)
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return err
}

func (r *Repository) TouchLogin(ctx context.Context, id int64) error {
	// Como mucho una escritura por minuto por usuario.
	_, err := r.db.Exec(ctx, `UPDATE users SET last_login_at=now()
		WHERE id=$1 AND (last_login_at IS NULL OR last_login_at < now() - interval '1 minute')`, id)
	return err
}

func (r *Repository) CompaniesExist(ctx context.Context, ids []int64) (bool, error) {
	var n int
	err := r.db.QueryRow(ctx, `SELECT count(*) FROM companies WHERE active AND id = ANY($1)`, ids).Scan(&n)
	return n == len(ids), err
}

func (r *Repository) Audit(ctx context.Context, actorID *int64, targetID int64, action string, detail map[string]any) error {
	_, err := r.db.Exec(ctx, `INSERT INTO user_audit (actor_user_id, target_user_id, action, detail) VALUES ($1,$2,$3,$4)`,
		actorID, targetID, action, detail)
	return err
}

func (r *Repository) AuditOf(ctx context.Context, targetID int64, limit int) ([]domain.AuditEntry, error) {
	rows, err := r.db.Query(ctx, `
		SELECT a.id, a.actor_user_id, COALESCE(u.name, 'Sistema'), a.action, a.detail, a.created_at
		FROM user_audit a LEFT JOIN users u ON u.id = a.actor_user_id
		WHERE a.target_user_id = $1 ORDER BY a.created_at DESC, a.id DESC LIMIT $2`, targetID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.AuditEntry{}
	for rows.Next() {
		var e domain.AuditEntry
		if err := rows.Scan(&e.ID, &e.ActorID, &e.ActorName, &e.Action, &e.Detail, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *Repository) Modules(ctx context.Context, role string) ([]domain.ModuleAccess, error) {
	rows, err := r.db.Query(ctx, `SELECT module, actions FROM role_modules WHERE role=$1 ORDER BY module`, role)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.ModuleAccess{}
	for rows.Next() {
		var m domain.ModuleAccess
		if err := rows.Scan(&m.Module, &m.Actions); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
