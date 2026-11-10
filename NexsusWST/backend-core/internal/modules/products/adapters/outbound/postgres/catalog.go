package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/nexuswst/backend-core/internal/modules/products/domain"
	"github.com/nexuswst/backend-core/internal/modules/products/ports"
)

// Catálogo de productos (Tarea 09). Reutiliza Repository (misma conexión) y agrega el listado con
// filtros, el nombre del cliente, «Modificado por» (la última acción de user_audit, Core 000013) y
// las escrituras con su acción en el registro, en una transacción.

var _ ports.CatalogRepository = (*Repository)(nil)

const catalogCols = `p.id, p.company_id, p.sku, p.description, p.type, p.active, p.wave_code, p.created_at, p.updated_at,
	COALESCE(c.name, ''), COALESCE(last.name, ''), last.created_at`

// last: la última acción sobre el producto y quién la hizo («Sistema» si no hubo actor).
const catalogFrom = `FROM products p
	JOIN companies c ON c.id = p.company_id
	LEFT JOIN LATERAL (
		SELECT COALESCE(u.name, 'Sistema') AS name, a.created_at
		FROM user_audit a LEFT JOIN users u ON u.id = a.actor_user_id
		WHERE a.target_product_id = p.id ORDER BY a.created_at DESC, a.id DESC LIMIT 1
	) last ON true`

func scanCatalog(row pgx.Row) (ports.CatalogItem, error) {
	var it ports.CatalogItem
	var t string
	err := row.Scan(&it.ID, &it.CompanyID, &it.SKU, &it.Description, &t, &it.Active, &it.WaveCode, &it.CreatedAt, &it.UpdatedAt,
		&it.CompanyName, &it.UpdatedByName, &it.UpdatedByAt)
	it.Type = domain.ProductType(t)
	if errors.Is(err, pgx.ErrNoRows) {
		return it, domain.ErrNotFound
	}
	return it, err
}

func (r *Repository) ListCatalog(ctx context.Context, f ports.CatalogFilter) ([]ports.CatalogItem, int, error) {
	where, args := []string{"TRUE"}, []any{}
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(cond, len(args)))
	}
	if f.CompanyIDs != nil {
		add("p.company_id = ANY($%d)", f.CompanyIDs)
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		args = append(args, q)
		where = append(where, fmt.Sprintf("(p.sku ILIKE '%%' || $%[1]d || '%%' OR p.description ILIKE '%%' || $%[1]d || '%%')", len(args)))
	}
	if f.Type != "" {
		add("p.type = $%d", string(f.Type))
	}
	if f.Active != nil {
		add("p.active = $%d", *f.Active)
	}
	switch f.Wave {
	case "":
	case "general":
		where = append(where, "p.wave_code IS NULL")
	default:
		add("p.wave_code = $%d", f.Wave)
	}
	if len(f.IDs) > 0 {
		add("p.id = ANY($%d)", f.IDs)
	}
	w := strings.Join(where, " AND ")
	var total int
	if err := r.db.QueryRow(ctx, "SELECT count(*) FROM products p WHERE "+w, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, f.Limit, f.Offset)
	rows, err := r.db.Query(ctx, fmt.Sprintf("SELECT %s %s WHERE %s ORDER BY p.active DESC, p.updated_at DESC, p.id DESC LIMIT $%d OFFSET $%d",
		catalogCols, catalogFrom, w, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]ports.CatalogItem, 0)
	for rows.Next() {
		it, err := scanCatalog(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, it)
	}
	return out, total, rows.Err()
}

func (r *Repository) GetCatalog(ctx context.Context, companyIDs []int64, id int64) (ports.CatalogItem, error) {
	return scanCatalog(r.db.QueryRow(ctx, "SELECT "+catalogCols+" "+catalogFrom+
		" WHERE ($1::bigint[] IS NULL OR p.company_id = ANY($1)) AND p.id = $2", companyIDs, id))
}

func audit(ctx context.Context, tx pgx.Tx, productID int64, a ports.Audit) error {
	if a.Detail == nil {
		a.Detail = map[string]any{} // user_audit.detail es NOT NULL
	}
	_, err := tx.Exec(ctx, `INSERT INTO user_audit (actor_user_id, target_product_id, action, detail) VALUES ($1, $2, $3, $4)`,
		a.ActorID, productID, a.Action, a.Detail)
	return err
}

func insertComponents(ctx context.Context, tx pgx.Tx, kitID int64, comps []domain.KitComponent) error {
	for _, c := range comps {
		if _, err := tx.Exec(ctx, `INSERT INTO kit_components (kit_id, component_id, quantity) VALUES ($1, $2, $3)`, kitID, c.ComponentID, c.Quantity); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) CreateProduct(ctx context.Context, p domain.Product, comps []domain.KitComponent, a ports.Audit) (domain.Product, error) {
	var out domain.Product
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		var err error
		out, err = scan(tx.QueryRow(ctx, `INSERT INTO products (company_id, sku, description, type, active, wave_code)
			VALUES ($1, $2, $3, $4, TRUE, $5) RETURNING `+cols, p.CompanyID, p.SKU, p.Description, string(p.Type), p.WaveCode))
		if err != nil {
			return err
		}
		if err := insertComponents(ctx, tx, out.ID, comps); err != nil {
			return err
		}
		return audit(ctx, tx, out.ID, a)
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && strings.Contains(pgErr.ConstraintName, "products") {
		return domain.Product{}, domain.ErrDuplicateSKU
	}
	return out, err
}

func (r *Repository) UpdateProduct(ctx context.Context, id int64, description string, wave *string, comps []domain.KitComponent, a ports.Audit) error {
	return pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE products SET description = $2, wave_code = $3, updated_at = now() WHERE id = $1`, id, description, wave)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrNotFound
		}
		if comps != nil {
			if _, err := tx.Exec(ctx, `DELETE FROM kit_components WHERE kit_id = $1`, id); err != nil {
				return err
			}
			if err := insertComponents(ctx, tx, id, comps); err != nil {
				return err
			}
		}
		return audit(ctx, tx, id, a)
	})
}

func (r *Repository) SetActive(ctx context.Context, id int64, active bool, a ports.Audit) error {
	return pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE products SET active = $2, updated_at = now() WHERE id = $1 AND active <> $2`, id, active)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrSameState
		}
		return audit(ctx, tx, id, a)
	})
}

func (r *Repository) ActiveKitsUsing(ctx context.Context, componentID int64) ([]string, error) {
	rows, err := r.db.Query(ctx, `SELECT p.sku FROM kit_components k JOIN products p ON p.id = k.kit_id
		WHERE k.component_id = $1 AND p.active ORDER BY p.sku`, componentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *Repository) Company(ctx context.Context, id int64) (string, bool, error) {
	var name string
	var active bool
	err := r.db.QueryRow(ctx, `SELECT name, active FROM companies WHERE id = $1`, id).Scan(&name, &active)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, domain.ErrNotFound
	}
	return name, active, err
}
