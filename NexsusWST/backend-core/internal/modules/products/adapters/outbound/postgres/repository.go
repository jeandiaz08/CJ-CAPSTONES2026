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

const cols = `id, company_id, sku, description, type, active, created_at, updated_at`

func scan(row pgx.Row) (domain.Product, error) {
	var p domain.Product
	var t string
	err := row.Scan(&p.ID, &p.CompanyID, &p.SKU, &p.Description, &t, &p.Active, &p.CreatedAt, &p.UpdatedAt)
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
		INSERT INTO products (company_id, sku, description, type, active)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING `+cols, p.CompanyID, p.SKU, p.Description, string(p.Type), p.Active))
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
		return domain.Product{}, domain.ErrDuplicateSKU
	}
	return out, err
}
