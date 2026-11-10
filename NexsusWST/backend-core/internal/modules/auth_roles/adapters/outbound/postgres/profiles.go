package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nexuswst/backend-core/internal/modules/auth_roles/domain"
	"github.com/nexuswst/backend-core/internal/modules/auth_roles/ports"
)

// Profiles - perfiles de Clientes › Administración clientes (Tarea 07) y de Usuarios WeStorage (Tarea 08). Lee users con su teléfono,
// cargo y empresa; las altas y cambios de identidad siguen en Repository y Keycloak.
type Profiles struct{ db *pgxpool.Pool }

func NewProfiles(db *pgxpool.Pool) *Profiles { return &Profiles{db: db} }

const profileCols = `u.id, COALESCE(u.keycloak_id::text, ''), COALESCE(u.email::text, ''), COALESCE(u.username::text, ''), u.name, u.role,
	u.permissions, u.all_companies, u.active, u.last_login_at, u.created_at, u.updated_at,
	COALESCE((SELECT array_agg(x.company_id ORDER BY x.company_id) FROM user_companies x WHERE x.user_id = u.id), '{}'),
	COALESCE(u.phone, ''), COALESCE(u.job_title, ''), COALESCE(c.id, 0), COALESCE(c.name, '')`

// La empresa del perfil: la primera (un perfil Cliente tiene una sola).
const profileFrom = `FROM users u
	LEFT JOIN LATERAL (SELECT uc.company_id FROM user_companies uc WHERE uc.user_id = u.id ORDER BY uc.company_id LIMIT 1) first ON true
	LEFT JOIN companies c ON c.id = first.company_id`

func scanProfile(row pgx.Row) (ports.Profile, error) {
	var p ports.Profile
	err := row.Scan(&p.ID, &p.KeycloakID, &p.Email, &p.Username, &p.Name, &p.Role, &p.Permissions, &p.AllCompanies,
		&p.Active, &p.LastLoginAt, &p.CreatedAt, &p.UpdatedAt, &p.Companies, &p.Phone, &p.JobTitle, &p.CompanyID, &p.CompanyName)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, domain.ErrNotFound
	}
	return p, err
}

func (r *Profiles) ListProfiles(ctx context.Context, f ports.ProfileFilter) ([]ports.Profile, int, error) {
	where, args := []string{}, []any{}
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(cond, len(args)))
	}
	switch f.Scope {
	case domain.ProfileScopeClient:
		where = append(where, "u.role = 'cliente'")
	case domain.ProfileScopeInternal:
		// Analista, KAM y SAC son perfiles futuros (regla 11): no se listan en Usuarios WeStorage.
		add("u.role = ANY($%d)", domain.ProfileRoles)
	default:
		return nil, 0, fmt.Errorf("alcance de perfiles desconocido: %q", f.Scope)
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		args = append(args, q)
		n := len(args)
		where = append(where, fmt.Sprintf(`(u.name ILIKE '%%' || $%[1]d || '%%' OR u.email ILIKE '%%' || $%[1]d || '%%'
			OR u.username ILIKE '%%' || $%[1]d || '%%' OR c.name ILIKE '%%' || $%[1]d || '%%')`, n))
	}
	if f.CompanyID != 0 { // -1: Cliente sin empresa activa (no ve nada)
		add("EXISTS (SELECT 1 FROM user_companies uc WHERE uc.user_id = u.id AND uc.company_id = $%d)", f.CompanyID)
	}
	if f.Active != nil {
		add("u.active = $%d", *f.Active)
	}
	if f.From != nil {
		add("(u.created_at AT TIME ZONE 'America/Santiago')::date >= $%d::date", f.From.Format("2006-01-02"))
	}
	if f.To != nil {
		add("(u.created_at AT TIME ZONE 'America/Santiago')::date <= $%d::date", f.To.Format("2006-01-02"))
	}
	if len(f.IDs) > 0 {
		add("u.id = ANY($%d)", f.IDs)
	}
	w := strings.Join(where, " AND ")
	var total int
	if err := r.db.QueryRow(ctx, "SELECT count(*) "+profileFrom+" WHERE "+w, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, f.Limit, f.Offset)
	rows, err := r.db.Query(ctx, fmt.Sprintf("SELECT %s %s WHERE %s ORDER BY u.active DESC, u.name, u.id LIMIT $%d OFFSET $%d",
		profileCols, profileFrom, w, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]ports.Profile, 0)
	for rows.Next() {
		p, err := scanProfile(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, p)
	}
	return out, total, rows.Err()
}

func (r *Profiles) GetProfile(ctx context.Context, id int64) (ports.Profile, error) {
	return scanProfile(r.db.QueryRow(ctx, "SELECT "+profileCols+" "+profileFrom+" WHERE u.id = $1", id))
}

func (r *Profiles) SetContact(ctx context.Context, id int64, phone, jobTitle string) error {
	tag, err := r.db.Exec(ctx, `UPDATE users SET phone = NULLIF($2, ''), job_title = NULLIF($3, ''), updated_at = now() WHERE id = $1`,
		id, phone, jobTitle)
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return err
}

func (r *Profiles) CompanyByTaxID(ctx context.Context, taxID string) (int64, string, bool, error) {
	var id int64
	var name string
	var active bool
	err := r.db.QueryRow(ctx, `SELECT id, name, active FROM companies
		WHERE upper(regexp_replace(tax_id, '[^0-9kK]', '', 'g')) = upper(regexp_replace($1, '[^0-9kK]', '', 'g'))`, taxID).Scan(&id, &name, &active)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, "", false, domain.ErrNotFound
	}
	return id, name, active, err
}

func (r *Profiles) Company(ctx context.Context, id int64) (string, bool, error) {
	var name string
	var active bool
	err := r.db.QueryRow(ctx, `SELECT name, active FROM companies WHERE id = $1`, id).Scan(&name, &active)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, domain.ErrNotFound
	}
	return name, active, err
}

func (r *Profiles) AuditCompany(ctx context.Context, actorID, companyID int64, action string, detail map[string]any) error {
	_, err := r.db.Exec(ctx, `INSERT INTO user_audit (actor_user_id, target_company_id, action, detail) VALUES ($1, $2, $3, $4)`,
		actorID, companyID, action, detail)
	return err
}

// managerLockKey: candado de PostgreSQL para los cambios que pueden quitar un gestor de perfiles.
const managerLockKey = "nexuswst:profile_managers"

// WithManagerLock ejecuta fn con el candado tomado dentro de una transacción
// (pg_advisory_xact_lock): dos cambios simultáneos sobre gestores se hacen uno tras otro, y el
// segundo cuenta los gestores con lo que el primero ya dejó escrito. El candado se suelta al
// terminar la transacción, después de fn. Si fn falla, el error se devuelve igual.
func (r *Profiles) WithManagerLock(ctx context.Context, fn func(context.Context) error) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.WithoutCancel(ctx)) //nolint:errcheck // sin efecto tras el Commit
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtext($1))", managerLockKey); err != nil {
		return err
	}
	if err := fn(ctx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// CountManagers cuenta los Administradores activos con «Gestionar perfiles», sin contar a exceptID.
func (r *Profiles) CountManagers(ctx context.Context, exceptID int64) (int, error) {
	var n int
	err := r.db.QueryRow(ctx, `SELECT count(*) FROM users
		WHERE role = 'admin' AND active AND 'gestionar_perfiles' = ANY(permissions) AND id <> $1`, exceptID).Scan(&n)
	return n, err
}

func (r *Profiles) ActiveCompanies(ctx context.Context) ([][2]string, error) {
	rows, err := r.db.Query(ctx, `SELECT name, tax_id FROM companies WHERE active ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := [][2]string{}
	for rows.Next() {
		var c [2]string
		if err := rows.Scan(&c[0], &c[1]); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
