package postgres

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nexuswst/orders-service/internal/domain"
	"github.com/nexuswst/orders-service/internal/ports"
)

// Cambiar el courier saca la orden del manifiesto abierto en la misma transacción;
// con el manifiesto cerrado, el cambio se rechaza y no queda nada a medias.
func TestResolutionLeavesOpenManifest(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL habilita PostgreSQL")
	}
	ctx := context.Background()
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.MaxConns = 1 // las tablas temporales viven en una sola conexión
	db, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(ctx, `CREATE TEMP TABLE orders (LIKE public.orders INCLUDING DEFAULTS);
 CREATE TEMP TABLE users (LIKE public.users INCLUDING DEFAULTS);
 CREATE TEMP TABLE ops_settings (LIKE public.ops_settings INCLUDING DEFAULTS);
 CREATE TEMP TABLE job_outbox (LIKE public.job_outbox INCLUDING DEFAULTS);
 CREATE TEMP TABLE cutoffs (LIKE public.cutoffs INCLUDING DEFAULTS);
 CREATE TEMP TABLE cutoff_couriers (LIKE public.cutoff_couriers INCLUDING DEFAULTS);
 CREATE TEMP TABLE order_events (LIKE public.order_events INCLUDING DEFAULTS);
 CREATE TEMP TABLE order_items (LIKE public.order_items INCLUDING DEFAULTS);
 CREATE TEMP TABLE order_packages (LIKE public.order_packages INCLUDING DEFAULTS);
 CREATE TEMP TABLE manifests (LIKE public.manifests INCLUDING DEFAULTS);
 CREATE TEMP TABLE manifest_lines (LIKE public.manifest_lines INCLUDING DEFAULTS);`); err != nil {
		t.Fatal(err)
	}
	const (
		openOrder   int64 = 9100001
		closedOrder int64 = 9100002
		noManifest  int64 = 9100003
	)
	for _, id := range []int64{openOrder, closedOrder, noManifest} {
		if _, err = db.Exec(ctx, `INSERT INTO orders(id,company_id,external_ref,channel,courier,courier_code,status,prev_status,incident_kind,note,version,recipient_name,address_line,commune)
 VALUES($1::bigint,1,($1::bigint)::text,'manual','starken','OLD-1','incident','ready_pickup','courier_cobertura','Sin cobertura',3,'Ana','Calle 123','Ñuñoa')`, id); err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(ctx, `INSERT INTO order_items(order_id,sku,quantity) VALUES($1,'AO-1',1)`, id); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = db.Exec(ctx, `INSERT INTO manifests(id,courier,status,created_by) VALUES(1,'starken','open','t'),(2,'starken','closed','t');
 INSERT INTO manifest_lines(manifest_id,order_id,status) VALUES(1,9100001,'included'),(2,9100002,'pending_packages');`); err != nil {
		t.Fatal(err)
	}

	repo := NewRepository(db)
	resolve := func(id int64) (domain.Order, error) {
		before, err := repo.Get(ctx, nil, id)
		if err != nil {
			t.Fatal(err)
		}
		courier := "recibelo"
		after, diff, err := before.ApplyCorrections(domain.Corrections{Courier: &courier})
		if err != nil {
			t.Fatal(err)
		}
		from, err := after.Apply(domain.EvResolve)
		if err != nil {
			t.Fatal(err)
		}
		return repo.SaveResolution(ctx, before, after, from, diff, "Jefe", "Resuelta", ports.TransitionOpts{})
	}

	out, err := resolve(openOrder)
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != domain.StatusReadyPickup || out.Courier != "recibelo" || out.CourierCode != "" || out.PrevStatus != "" || out.Note != "" {
		t.Fatalf("orden resuelta: %+v", out)
	}
	var lineStatus, movedTo string
	if err = db.QueryRow(ctx, `SELECT status, moved_to FROM manifest_lines WHERE order_id=$1`, openOrder).Scan(&lineStatus, &movedTo); err != nil {
		t.Fatal(err)
	}
	if lineStatus != "moved" || movedTo != "recibelo" {
		t.Fatalf("línea del manifiesto: %s → %s", lineStatus, movedTo)
	}
	events, err := repo.Events(ctx, openOrder)
	if err != nil {
		t.Fatal(err)
	}
	var history []string
	for _, e := range events {
		history = append(history, e.Reason)
	}
	joined := strings.Join(history, " | ")
	for _, want := range []string{"Sale del manifiesto #1", "Courier: «starken» → «recibelo»", "Resuelta"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("historial sin %q: %s", want, joined)
		}
	}

	if _, err = resolve(closedOrder); !errors.Is(err, domain.ErrManifestClosed) || !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("manifiesto cerrado: %v", err)
	}
	still, err := repo.Get(ctx, nil, closedOrder)
	if err != nil {
		t.Fatal(err)
	}
	if still.Status != domain.StatusIncident || still.Courier != "starken" || still.Version != 3 {
		t.Fatalf("no debe cambiar nada: %+v", still)
	}
	if err = db.QueryRow(ctx, `SELECT status FROM manifest_lines WHERE order_id=$1`, closedOrder).Scan(&lineStatus); err != nil || lineStatus != "pending_packages" {
		t.Fatalf("la línea del manifiesto cerrado no cambia: %s %v", lineStatus, err)
	}

	if out, err = resolve(noManifest); err != nil || out.Courier != "recibelo" {
		t.Fatalf("sin manifiesto: %+v %v", out, err)
	}
}
