package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nexuswst/backend-core/internal/modules/clients/domain"
)

type Repository struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

func (r *Repository) List(ctx context.Context, ids []int64, search string) ([]domain.Company, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, name, COALESCE(tax_id,''), COALESCE(wms_code,''), active, created_at
		FROM companies
		WHERE ($1::bigint[] IS NULL OR id = ANY($1))
		  AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR tax_id ILIKE '%' || $2 || '%')
		ORDER BY name`, ids, search)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.Company, 0)
	for rows.Next() {
		var c domain.Company
		if err := rows.Scan(&c.ID, &c.Name, &c.TaxID, &c.WMSCode, &c.Active, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
