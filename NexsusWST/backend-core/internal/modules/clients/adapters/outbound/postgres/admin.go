package postgres

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/nexuswst/backend-core/internal/modules/clients/domain"
	"github.com/nexuswst/backend-core/internal/modules/clients/ports"
)

// Empresas para Clientes › Administración clientes (Tarea 07). Cada cambio deja su fila en
// user_audit (target_company_id) en la misma transacción, para el Registro de acciones.

const adminCols = `c.id, c.name, c.tax_id, COALESCE(c.wms_code, ''), COALESCE(c.email::text, ''), COALESCE(c.phone, ''),
	c.active, c.wave_code, c.created_at, c.updated_at,
	(SELECT count(*) FROM user_companies uc JOIN users u ON u.id = uc.user_id
	 WHERE uc.company_id = c.id AND u.role = 'cliente')`

// Fecha de creación por día de Chile.
const createdDay = `(c.created_at AT TIME ZONE 'America/Santiago')::date`

var notRUTChars = regexp.MustCompile(`[^0-9kK]`)

func scanCompany(row pgx.Row) (domain.Company, error) {
	var c domain.Company
	err := row.Scan(&c.ID, &c.Name, &c.TaxID, &c.WMSCode, &c.Email, &c.Phone, &c.Active, &c.WaveCode,
		&c.CreatedAt, &c.UpdatedAt, &c.ProfilesCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, domain.ErrNotFound
	}
	return c, err
}

func (r *Repository) AdminList(ctx context.Context, f ports.CompanyFilter) ([]domain.Company, int, error) {
	where, args := []string{"TRUE"}, []any{}
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(cond, len(args)))
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		args = append(args, q)
		n := len(args)
		cond := fmt.Sprintf("(c.name ILIKE '%%' || $%d || '%%' OR c.tax_id ILIKE '%%' || $%d || '%%'", n, n)
		// «76123456» encuentra «76.123.456-0».
		if digits := notRUTChars.ReplaceAllString(q, ""); len(digits) >= 3 {
			args = append(args, strings.ToUpper(digits))
			cond += fmt.Sprintf(" OR upper(regexp_replace(c.tax_id, '[^0-9kK]', '', 'g')) LIKE '%%' || $%d || '%%'", len(args))
		}
		where = append(where, cond+")")
	}
	if f.Active != nil {
		add("c.active = $%d", *f.Active)
	}
	switch f.Wave {
	case "":
	case "general":
		where = append(where, "c.wave_code IS NULL")
	default:
		add("c.wave_code = $%d", f.Wave)
	}
	if f.From != nil {
		add(createdDay+" >= $%d::date", f.From.Format("2006-01-02"))
	}
	if f.To != nil {
		add(createdDay+" <= $%d::date", f.To.Format("2006-01-02"))
	}
	if len(f.IDs) > 0 {
		add("c.id = ANY($%d)", f.IDs)
	}
	w := strings.Join(where, " AND ")
	var total int
	if err := r.db.QueryRow(ctx, "SELECT count(*) FROM companies c WHERE "+w, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, f.Limit, f.Offset)
	rows, err := r.db.Query(ctx, fmt.Sprintf("SELECT %s FROM companies c WHERE %s ORDER BY c.active DESC, c.name, c.id LIMIT $%d OFFSET $%d",
		adminCols, w, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]domain.Company, 0)
	for rows.Next() {
		c, err := scanCompany(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, c)
	}
	return out, total, rows.Err()
}

func (r *Repository) Get(ctx context.Context, id int64) (domain.Company, error) {
	return scanCompany(r.db.QueryRow(ctx, "SELECT "+adminCols+" FROM companies c WHERE c.id = $1", id))
}

func (r *Repository) Create(ctx context.Context, d domain.Draft, actorID int64, a ports.Audit) (domain.Company, error) {
	var id int64
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			INSERT INTO companies (name, tax_id, email, phone, wave_code, wms_code, created_by, updated_by)
			VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), $5, NULLIF($6, ''), $7, $7)
			RETURNING id`, d.Name, d.TaxID, d.Email, d.Phone, d.WaveCode, d.WMSCode, actorID).Scan(&id)
		if err != nil {
			return taxIDTaken(err)
		}
		return audit(ctx, tx, actorID, id, a)
	})
	if err != nil {
		return domain.Company{}, err
	}
	return r.Get(ctx, id)
}

func (r *Repository) Update(ctx context.Context, id int64, d domain.Draft, actorID int64, a ports.Audit) error {
	return pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE companies SET name = $2, tax_id = $3, email = NULLIF($4, ''), phone = NULLIF($5, ''),
			       wave_code = $6, wms_code = NULLIF($7, ''), updated_by = $8, updated_at = now()
			WHERE id = $1`, id, d.Name, d.TaxID, d.Email, d.Phone, d.WaveCode, d.WMSCode, actorID)
		if err != nil {
			return taxIDTaken(err)
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrNotFound
		}
		return audit(ctx, tx, actorID, id, a)
	})
}

func (r *Repository) SetActive(ctx context.Context, id int64, active bool, actorID int64, a ports.Audit) error {
	return pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE companies SET active = $2, updated_by = $3, updated_at = now() WHERE id = $1 AND active <> $2`,
			id, active, actorID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			var exists bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM companies WHERE id = $1)`, id).Scan(&exists); err != nil {
				return err
			}
			if !exists {
				return domain.ErrNotFound
			}
			return domain.ErrSameState
		}
		return audit(ctx, tx, actorID, id, a)
	})
}

func audit(ctx context.Context, tx pgx.Tx, actorID, companyID int64, a ports.Audit) error {
	if a.Detail == nil {
		a.Detail = map[string]any{}
	}
	_, err := tx.Exec(ctx, `INSERT INTO user_audit (actor_user_id, target_company_id, action, detail) VALUES ($1, $2, $3, $4)`,
		actorID, companyID, a.Action, a.Detail)
	return err
}

func taxIDTaken(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" && (pg.ConstraintName == "companies_tax_id_key" || pg.ConstraintName == "uq_companies_tax_id_norm") {
		return domain.ErrTaxIDTaken
	}
	return err
}
