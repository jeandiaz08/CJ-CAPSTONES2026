package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nexuswst/backend-core/internal/modules/catalogs/domain"
)

type Source struct{ db *pgxpool.Pool }

func NewSource(db *pgxpool.Pool) *Source { return &Source{db: db} }

// Cada catálogo se arma como JSON directamente en PostgreSQL.
var queries = map[domain.Name]string{
	domain.Regions: `SELECT COALESCE(json_agg(json_build_object('code', code, 'name', name) ORDER BY sort_order, name), '[]')
	                 FROM regions`,
	domain.Couriers: `SELECT COALESCE(json_agg(json_build_object('code', code, 'name', name, 'active', active) ORDER BY name), '[]')
	                  FROM couriers WHERE active`,
	domain.Communes: `SELECT COALESCE(json_agg(json_build_object('name', name, 'regionCode', region_code) ORDER BY region_code, name), '[]')
	                  FROM communes WHERE active`,
	// Fijo por ahora (Órdenes valida contra la misma lista).
	domain.DispatchTypes: `SELECT '[{"code":"domicilio","name":"Despacho a domicilio"},{"code":"retiro_oficina","name":"Retiro en oficina del courier"},{"code":"express","name":"Express (mismo día)"}]'::json`,
}

func (s *Source) Load(ctx context.Context, name domain.Name) ([]byte, error) {
	q, ok := queries[name]
	if !ok {
		return nil, domain.ErrUnknownCatalog
	}
	var raw []byte
	err := s.db.QueryRow(ctx, q).Scan(&raw)
	return raw, err
}
