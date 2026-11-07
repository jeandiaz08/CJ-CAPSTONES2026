package postgres

import (
	"context"

	"github.com/nexuswst/orders-service/internal/domain"
)

// Kits (RF-06, SCRUM-100). La composición la mantiene el Core en
// kit_components; Órdenes solo la lee (DA-12).

// expandSQL convierte las líneas de la orden en líneas de stock: un kit se
// reemplaza por sus componentes (cantidad del kit × cantidad del componente)
// y los SKU repetidos se suman. Un producto simple queda igual.
const expandSQL = `
	SELECT sku, SUM(qty)::int FROM (
		SELECT COALESCE(c.sku, i.sku) AS sku, i.qty * COALESCE(k.quantity, 1) AS qty
		FROM unnest($2::text[], $3::int[]) AS i(sku, qty)
		LEFT JOIN products p       ON p.company_id = $1 AND p.sku = i.sku AND p.type = 'kit'
		LEFT JOIN kit_components k ON k.kit_id = p.id
		LEFT JOIN products c       ON c.id = k.component_id
	) x
	GROUP BY sku
	ORDER BY sku`

// stockLines devuelve las líneas de stock ordenadas por SKU (orden fijo para
// tomar los bloqueos del comprometido sin deadlocks).
func stockLines(ctx context.Context, q querier, companyID int64, items []domain.Item) ([]domain.Item, error) {
	skus := make([]string, len(items))
	qtys := make([]int32, len(items))
	for i, it := range items {
		skus[i], qtys[i] = it.SKU, int32(it.Quantity)
	}
	rows, err := q.Query(ctx, expandSQL, companyID, skus, qtys)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.Item, 0, len(items))
	for rows.Next() {
		var it domain.Item
		if err := rows.Scan(&it.SKU, &it.Quantity); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// StockLines: lo que hay que consultar a Invas para validar la orden.
func (p *CatalogReader) StockLines(ctx context.Context, companyID int64, items []domain.Item) ([]domain.Item, error) {
	return stockLines(ctx, p.db, companyID, items)
}

// EmptyKits: kits de la orden que todavía no tienen composición definida.
func (p *CatalogReader) EmptyKits(ctx context.Context, companyID int64, skus []string) ([]string, error) {
	rows, err := p.db.Query(ctx, `
		SELECT p.sku FROM products p
		WHERE p.company_id = $1 AND p.sku = ANY($2) AND p.type = 'kit'
		  AND NOT EXISTS (SELECT 1 FROM kit_components k WHERE k.kit_id = p.id)
		ORDER BY p.sku`, companyID, skus)
	if err != nil {
		return nil, err
	}
	return collect[string](rows)
}

// WavePreference lee la ola de los productos de la orden y la ola por defecto
// del cliente desde las vistas del Core (PV-19), en solo lectura (DA-12).
// Devuelve las olas DISTINTAS de producto, ordenadas.
func (p *CatalogReader) WavePreference(ctx context.Context, orderID, companyID int64) ([]string, string, error) {
	var productWaves []string
	var companyWave *string
	err := p.db.QueryRow(ctx, wavePreferenceSQL, orderID, companyID).
		Scan(&productWaves, &companyWave)
	if err != nil {
		return nil, "", err
	}
	if companyWave == nil {
		return productWaves, "", nil
	}
	return productWaves, *companyWave, nil
}

const wavePreferenceSQL = `
	SELECT ARRAY(SELECT DISTINCT pw.wave_code
	             FROM order_items i
	             JOIN v_product_waves pw ON pw.company_id = $2 AND pw.sku = i.sku
	             WHERE i.order_id = $1
	             ORDER BY 1),
	       (SELECT wave_code FROM v_company_waves WHERE company_id = $2)`
