// Package postgres implementa ports.ProductRepository con PostgreSQL (pgx).
package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nexuswst/backend-core/internal/modules/products/domain"
	"github.com/nexuswst/backend-core/internal/modules/products/ports"
)

type Repository struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

var _ ports.ProductRepository = (*Repository)(nil)

const cols = `id, company_id, sku, description, type, active, wave_code, created_at, updated_at`

func scan(row pgx.Row) (domain.Product, error) {
	var p domain.Product
	var t string
	err := row.Scan(&p.ID, &p.CompanyID, &p.SKU, &p.Description, &t, &p.Active, &p.WaveCode, &p.CreatedAt, &p.UpdatedAt)
	p.Type = domain.ProductType(t)
	return p, err
}

func (r *Repository) List(ctx context.Context, f ports.ListFilter) ([]domain.Product, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+cols+` FROM products
		WHERE ($1::bigint[] IS NULL OR company_id = ANY($1))
		  AND ($2 = '' OR sku ILIKE '%' || $2 || '%' OR description ILIKE '%' || $2 || '%')
		ORDER BY id DESC
		LIMIT $3 OFFSET $4`, f.CompanyIDs, f.Search, f.Limit, f.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.Product, 0)
	for rows.Next() {
		p, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *Repository) GetByID(ctx context.Context, companyIDs []int64, id int64) (domain.Product, error) {
	p, err := scan(r.db.QueryRow(ctx, `SELECT `+cols+` FROM products WHERE ($1::bigint[] IS NULL OR company_id = ANY($1)) AND id=$2`, companyIDs, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Product{}, domain.ErrNotFound
	}
	return p, err
}

func (r *Repository) Create(ctx context.Context, p domain.Product) (domain.Product, error) {
	out, err := scan(r.db.QueryRow(ctx, `
		INSERT INTO products (company_id, sku, description, type, active, wave_code)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING `+cols, p.CompanyID, p.SKU, p.Description, string(p.Type), p.Active, p.WaveCode))
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
		return domain.Product{}, domain.ErrDuplicateSKU
	}
	return out, err
}

// --- Composición de kits (H-04-T01) ---------------------------------------

func (r *Repository) BySKUs(ctx context.Context, companyID int64, skus []string) ([]domain.Product, error) {
	rows, err := r.db.Query(ctx, `SELECT `+cols+` FROM products WHERE company_id=$1 AND sku = ANY($2)`, companyID, skus)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.Product, 0, len(skus))
	for rows.Next() {
		p, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *Repository) Components(ctx context.Context, kitID int64) ([]domain.KitComponent, error) {
	rows, err := r.db.Query(ctx, `
		SELECT p.id, p.sku, p.description, k.quantity
		FROM kit_components k JOIN products p ON p.id = k.component_id
		WHERE k.kit_id = $1
		ORDER BY p.sku`, kitID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.KitComponent, 0)
	for rows.Next() {
		var c domain.KitComponent
		if err := rows.Scan(&c.ComponentID, &c.SKU, &c.Description, &c.Quantity); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *Repository) ReplaceComponents(ctx context.Context, kitID int64, comps []domain.KitComponent) error {
	return pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		// Bloquea el kit: dos ediciones simultáneas no mezclan sus líneas.
		if _, err := tx.Exec(ctx, `SELECT 1 FROM products WHERE id=$1 FOR UPDATE`, kitID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM kit_components WHERE kit_id=$1`, kitID); err != nil {
			return err
		}
		for _, c := range comps {
			if _, err := tx.Exec(ctx, `INSERT INTO kit_components (kit_id, component_id, quantity) VALUES ($1,$2,$3)`,
				kitID, c.ComponentID, c.Quantity); err != nil {
				return err
			}
		}
		_, err := tx.Exec(ctx, `UPDATE products SET updated_at=now() WHERE id=$1`, kitID)
		return err
	})
}

// SetWave guarda la ola propia del producto (PV-19); nil la quita.
func (r *Repository) SetWave(ctx context.Context, id int64, code *string) error {
	tag, err := r.db.Exec(ctx, `UPDATE products SET wave_code=$2, updated_at=now() WHERE id=$1`, id, code)
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return err
}
