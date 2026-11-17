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

// Requests - solicitudes de perfiles (profile_requests, Core 000011 y 000012): de clientes (Tarea 07)
// e internas de WeStorage (Tarea 08, sin empresa).
type Requests struct{ db *pgxpool.Pool }

func NewRequests(db *pgxpool.Pool) *Requests { return &Requests{db: db} }

// En activación y desactivación, correo, teléfono y cargo se leen del perfil afectado.
const requestCols = `pr.id, pr.scope, COALESCE(pr.company_id, 0), COALESCE(c.name, ''), pr.kind, pr.target_user_id, pr.profile_name,
	COALESCE(pr.profile_email::text, tu.email::text, ''), COALESCE(pr.profile_phone, tu.phone, ''), COALESCE(pr.profile_job_title, tu.job_title, ''),
	pr.reason, pr.status, pr.requested_by, COALESCE(rb.name, ''), pr.requested_at, pr.reviewed_by, COALESCE(rv.name, ''),
	pr.reviewed_at, COALESCE(pr.rejection_reason, ''), pr.created_user_id,
	COALESCE(pr.profile_username::text, tu.username::text, ''), COALESCE(pr.profile_role, tu.role, ''), COALESCE(pr.profile_permissions, tu.permissions, '{}')`

const requestFrom = `FROM profile_requests pr
	LEFT JOIN companies c ON c.id = pr.company_id
	LEFT JOIN users tu ON tu.id = pr.target_user_id
	LEFT JOIN users rb ON rb.id = pr.requested_by
	LEFT JOIN users rv ON rv.id = pr.reviewed_by`

func scanRequest(row pgx.Row) (domain.ProfileRequest, error) {
	var r domain.ProfileRequest
	var kind, status, scope string
	err := row.Scan(&r.ID, &scope, &r.CompanyID, &r.CompanyName, &kind, &r.TargetUserID, &r.ProfileName, &r.ProfileEmail, &r.ProfilePhone,
		&r.ProfileJobTitle, &r.Reason, &status, &r.RequestedBy, &r.RequestedByName, &r.RequestedAt, &r.ReviewedBy, &r.ReviewedByName,
		&r.ReviewedAt, &r.RejectionReason, &r.CreatedUserID, &r.ProfileUsername, &r.ProfileRole, &r.ProfilePermissions)
	if errors.Is(err, pgx.ErrNoRows) {
		return r, domain.ErrNotFound
	}
	r.Kind, r.Status, r.Scope = domain.RequestKind(kind), domain.RequestStatus(status), domain.ProfileScope(scope)
	if r.Scope == domain.ProfileScopeClient {
		r.ProfileUsername, r.ProfileRole, r.ProfilePermissions = "", "", nil // el Cliente no tiene rol ni permisos internos
	}
	return r, err
}

// Create guarda la solicitud. companyID 0 = interna (sin empresa); en las internas de perfil nuevo
// se guardan además el usuario, el rol y los permisos propuestos.
func (r *Requests) Create(ctx context.Context, companyID, requestedBy int64, d domain.RequestDraft) (int64, error) {
	var target *int64
	if d.Kind != domain.RequestNewProfile {
		target = &d.TargetUserID
	}
	scope := domain.ProfileScopeClient
	var role *string
	var perms []string
	if companyID == 0 {
		scope = domain.ProfileScopeInternal
		if d.Kind == domain.RequestNewProfile {
			role, perms = &d.Role, d.Permissions
			if perms == nil {
				perms = []string{}
			}
		}
	}
	var id int64
	err := r.db.QueryRow(ctx, `
		INSERT INTO profile_requests (scope, company_id, kind, target_user_id, profile_name, profile_email, profile_phone, profile_job_title,
			reason, requested_by, profile_username, profile_role, profile_permissions)
		VALUES ($1, NULLIF($2, 0), $3, $4, $5, NULLIF($6, ''), NULLIF($7, ''), NULLIF($8, ''), $9, $10, NULLIF($11, ''), $12, $13) RETURNING id`,
		string(scope), companyID, string(d.Kind), target, d.Name, d.Email, d.Phone, d.JobTitle, d.Reason, requestedBy, d.Username, role, perms).Scan(&id)
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" && pg.ConstraintName == "uq_profile_requests_pending" {
		return 0, domain.ErrRequestPending
	}
	return id, err
}

func (r *Requests) Get(ctx context.Context, id int64) (domain.ProfileRequest, error) {
	return scanRequest(r.db.QueryRow(ctx, "SELECT "+requestCols+" "+requestFrom+" WHERE pr.id = $1", id))
}

func (r *Requests) List(ctx context.Context, f ports.RequestFilter) ([]domain.ProfileRequest, int, ports.RequestCounts, error) {
	var counts ports.RequestCounts
	where, args := []string{"TRUE"}, []any{}
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(cond, len(args)))
	}
	scope := f.Scope
	if scope == "" {
		scope = domain.ProfileScopeClient
	}
	add("pr.scope = $%d", string(scope))
	if f.CompanyIDs != nil {
		add("pr.company_id = ANY($%d)", f.CompanyIDs)
	}
	if f.Kind != "" {
		add("pr.kind = $%d", string(f.Kind))
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		args = append(args, q)
		where = append(where, fmt.Sprintf("(c.name ILIKE '%%' || $%[1]d || '%%' OR pr.profile_name ILIKE '%%' || $%[1]d || '%%')", len(args)))
	}
	if f.From != nil {
		add("(pr.requested_at AT TIME ZONE 'America/Santiago')::date >= $%d::date", f.From.Format("2006-01-02"))
	}
	if f.To != nil {
		add("(pr.requested_at AT TIME ZONE 'America/Santiago')::date <= $%d::date", f.To.Format("2006-01-02"))
	}
	if len(f.IDs) > 0 {
		add("pr.id = ANY($%d)", f.IDs)
	}
	// Los KPIs cuentan con los mismos filtros, sin el estado.
	base := strings.Join(where, " AND ")
	if err := r.db.QueryRow(ctx, `SELECT count(*) FILTER (WHERE pr.status = 'pending'), count(*) FILTER (WHERE pr.status = 'approved'),
		count(*) FILTER (WHERE pr.status = 'rejected') `+requestFrom+` WHERE `+base, args...).Scan(&counts.Pending, &counts.Approved, &counts.Rejected); err != nil {
		return nil, 0, counts, err
	}
	if f.Status != "" {
		add("pr.status = $%d", string(f.Status))
	}
	w := strings.Join(where, " AND ")
	var total int
	if err := r.db.QueryRow(ctx, "SELECT count(*) "+requestFrom+" WHERE "+w, args...).Scan(&total); err != nil {
		return nil, 0, counts, err
	}
	args = append(args, f.Limit, f.Offset)
	rows, err := r.db.Query(ctx, fmt.Sprintf(`SELECT %s %s WHERE %s
		ORDER BY (pr.status = 'pending') DESC, pr.requested_at DESC, pr.id DESC LIMIT $%d OFFSET $%d`,
		requestCols, requestFrom, w, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, counts, err
	}
	defer rows.Close()
	out := make([]domain.ProfileRequest, 0)
	for rows.Next() {
		req, err := scanRequest(rows)
		if err != nil {
			return nil, 0, counts, err
		}
		out = append(out, req)
	}
	return out, total, counts, rows.Err()
}

func (r *Requests) Approve(ctx context.Context, id, reviewerID int64, createdUserID *int64) error {
	tag, err := r.db.Exec(ctx, `UPDATE profile_requests SET status = 'approved', reviewed_by = $2, reviewed_at = now(), created_user_id = $3
		WHERE id = $1 AND status = 'pending'`, id, reviewerID, createdUserID)
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrRequestReviewed
	}
	return err
}

func (r *Requests) Reject(ctx context.Context, id, reviewerID int64, reason string) error {
	tag, err := r.db.Exec(ctx, `UPDATE profile_requests SET status = 'rejected', reviewed_by = $2, reviewed_at = now(), rejection_reason = $3
		WHERE id = $1 AND status = 'pending'`, id, reviewerID, reason)
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrRequestReviewed
	}
	return err
}

func (r *Requests) PendingFor(ctx context.Context, userIDs []int64) (map[int64][]domain.RequestKind, error) {
	out := map[int64][]domain.RequestKind{}
	if len(userIDs) == 0 {
		return out, nil
	}
	rows, err := r.db.Query(ctx, `SELECT target_user_id, kind FROM profile_requests WHERE status = 'pending' AND target_user_id = ANY($1)`, userIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var kind string
		if err := rows.Scan(&id, &kind); err != nil {
			return nil, err
		}
		out[id] = append(out[id], domain.RequestKind(kind))
	}
	return out, rows.Err()
}

func (r *Requests) AuditRequest(ctx context.Context, actorID, requestID int64, targetUserID *int64, action string, detail map[string]any) error {
	_, err := r.db.Exec(ctx, `INSERT INTO user_audit (actor_user_id, target_user_id, request_id, action, detail) VALUES ($1, $2, $3, $4, $5)`,
		actorID, targetUserID, requestID, action, detail)
	return err
}

func (r *Requests) ManagerEmails(ctx context.Context) ([]string, error) {
	rows, err := r.db.Query(ctx, `SELECT email::text FROM users
		WHERE active AND email IS NOT NULL AND 'gestionar_perfiles' = ANY(permissions) ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var e string
		if err := rows.Scan(&e); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
