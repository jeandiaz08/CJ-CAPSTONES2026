package postgres

import (
	"context"
	"os"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PV-19: lee la ola de los productos de la orden y la del cliente desde las
// vistas del Core. Requiere TEST_DATABASE_URL con las migraciones aplicadas.
func TestWavePreferenceReadsCoreViews(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL required")
	}
	ctx := context.Background()
	db, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // la prueba no deja datos
	var companyID, orderID int64
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(tx.QueryRow(ctx, `INSERT INTO companies (name, tax_id, wave_code) VALUES ('Prueba PV-19', 'PV19-TEST', 'AM') RETURNING id`).Scan(&companyID))
	_, err = tx.Exec(ctx, `INSERT INTO products (company_id, sku, description, wave_code) VALUES
		($1,'PV19-A','A','FRIO'), ($1,'PV19-B','B','FRIO'), ($1,'PV19-C','C',NULL), ($1,'PV19-D','D','NOCHE')`, companyID)
	must(err)
	must(tx.QueryRow(ctx, `INSERT INTO orders (company_id, external_ref, channel, courier, status)
		VALUES ($1, 'PV19-1', 'manual', 'chilexpress', 'validated') RETURNING id`, companyID).Scan(&orderID))
	_, err = tx.Exec(ctx, `INSERT INTO order_items (order_id, sku, quantity) VALUES ($1,'PV19-A',1), ($1,'PV19-B',2), ($1,'PV19-C',1)`, orderID)
	must(err)

	// Dentro de la transacción se consulta con la misma SQL que usa el lector.
	var waves []string
	var company *string
	must(tx.QueryRow(ctx, wavePreferenceSQL, orderID, companyID).Scan(&waves, &company))
	if !slices.Equal(waves, []string{"FRIO"}) || company == nil || *company != "AM" {
		t.Fatalf("olas %v, cliente %v", waves, company)
	}
	_, err = tx.Exec(ctx, `INSERT INTO order_items (order_id, sku, quantity) VALUES ($1,'PV19-D',1)`, orderID)
	must(err)
	must(tx.QueryRow(ctx, wavePreferenceSQL, orderID, companyID).Scan(&waves, &company))
	if !slices.Equal(waves, []string{"FRIO", "NOCHE"}) {
		t.Fatalf("olas distintas ordenadas: %v", waves)
	}
}
