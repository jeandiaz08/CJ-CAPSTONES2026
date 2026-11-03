package postgres

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nexuswst/orders-service/internal/domain"
	"github.com/nexuswst/orders-service/internal/ports"
)

func labelDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
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
	t.Cleanup(db.Close)
	if _, err = db.Exec(ctx, queueTables+` CREATE TEMP TABLE order_items (LIKE public.order_items INCLUDING DEFAULTS);
 CREATE TEMP TABLE order_events (LIKE public.order_events INCLUDING DEFAULTS);
 CREATE TEMP TABLE order_packages (LIKE public.order_packages INCLUDING DEFAULTS);
 CREATE TEMP TABLE order_documents (LIKE public.order_documents INCLUDING DEFAULTS);
 CREATE TEMP TABLE order_labels (LIKE public.order_labels INCLUDING ALL);
 CREATE TEMP TABLE courier_settings (LIKE public.courier_settings INCLUDING ALL);
 CREATE TEMP TABLE notifications (LIKE public.notifications INCLUDING DEFAULTS);
 INSERT INTO courier_settings(courier,max_packages,label_strategy) VALUES ('oficina',1,'office');
 INSERT INTO orders(id,company_id,external_ref,channel,courier,status,warehouse) VALUES
 (9400001,7,'a','manual','api','packing','central'),
 (9400002,7,'b','manual','oficina','packing','central');
 INSERT INTO order_packages(order_id,code) VALUES (9400001,'NX-1');`); err != nil {
		t.Fatal(err)
	}
	return db
}

func labelDoc() ports.Document {
	return ports.Document{OrderID: 9400001, CompanyID: 7, Kind: "etiqueta", Filename: "e.pdf", ContentType: "application/pdf",
		SizeBytes: 4, SHA256: "x", StorageKey: "k", UploadedBy: "t"}
}

func TestLabelClaimLeaseAndRetry(t *testing.T) {
	db := labelDB(t)
	ctx := context.Background()
	r := NewFulfillmentRepository(db)
	o := domain.Order{ID: 9400001, CompanyID: 7, Warehouse: "central"}
	if err := r.RequestLabel(ctx, o, "t"); err != nil {
		t.Fatal(err)
	}
	attempt, ok, err := r.ClaimLabel(ctx, o.ID)
	if err != nil || !ok || attempt != 1 {
		t.Fatalf("primer reclamo: %d %v %v", attempt, ok, err)
	}
	// Reclamada: nadie más la toma mientras dura el intento.
	if _, ok, _ = r.ClaimLabel(ctx, o.ID); ok {
		t.Fatal("se reclamó dos veces")
	}
	if due, _ := r.ClaimDueLabels(ctx, 10); len(due) != 0 {
		t.Fatalf("no debía estar vencida: %v", due)
	}
	if err := r.RetryLabel(ctx, o.ID, attempt, 0, "aún no"); err != nil {
		t.Fatal(err)
	}
	due, err := r.ClaimDueLabels(ctx, 10)
	if err != nil || len(due) != 1 || due[0].Attempt != 2 {
		t.Fatalf("reintento: %v %v", due, err)
	}
}

func TestLabelCompleteSavesAllAndIgnoresStaleResult(t *testing.T) {
	db := labelDB(t)
	ctx := context.Background()
	r := NewFulfillmentRepository(db)
	repo := NewRepository(db)
	o := domain.Order{ID: 9400001, CompanyID: 7, Warehouse: "central"}
	if err := r.RequestLabel(ctx, o, "t"); err != nil {
		t.Fatal(err)
	}
	// Se pidió con otra cantidad de bultos: el resultado ya no sirve.
	done, err := r.CompleteLabel(ctx, labelDoc(), "SIM1", 2, "t")
	if err != nil || done {
		t.Fatalf("resultado viejo: %v %v", done, err)
	}
	if done, err = r.CompleteLabel(ctx, labelDoc(), "SIM1", 1, "t"); err != nil || !done {
		t.Fatalf("resultado válido: %v %v", done, err)
	}
	got, err := repo.Get(ctx, nil, o.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.LabelStatus != domain.LabelReady || got.LabelDocumentID == nil || got.CourierCode != "SIM1" || got.Version != 1 {
		t.Fatalf("orden: %s %v %s versión %d", got.LabelStatus, got.LabelDocumentID, got.CourierCode, got.Version)
	}
	// Una etiqueta lista no se vuelve a pedir.
	if err := r.RequestLabel(ctx, got, "t"); err != nil {
		t.Fatal(err)
	}
	if got, _ = repo.Get(ctx, nil, o.ID); got.LabelStatus != domain.LabelReady {
		t.Fatalf("volvió a %s", got.LabelStatus)
	}
	// El evento guarda la empresa y la bodega de la orden.
	var company int64
	var warehouse string
	if err := db.QueryRow(ctx, `SELECT company_id, warehouse FROM order_events WHERE order_id=$1 AND event='ready'`, o.ID).Scan(&company, &warehouse); err != nil {
		t.Fatal(err)
	}
	if company != 7 || warehouse != "central" {
		t.Fatalf("evento: %d %s", company, warehouse)
	}
}

func TestLabelPackagesChangeInvalidatesIt(t *testing.T) {
	db := labelDB(t)
	ctx := context.Background()
	r := NewFulfillmentRepository(db)
	repo := NewRepository(db)
	o := domain.Order{ID: 9400001, CompanyID: 7, Warehouse: "central"}
	if err := r.RequestLabel(ctx, o, "t"); err != nil {
		t.Fatal(err)
	}
	if done, err := r.CompleteLabel(ctx, labelDoc(), "SIM1", 1, "t"); err != nil || !done {
		t.Fatalf("%v %v", done, err)
	}
	weight := 2.5
	if _, err := r.ReplacePackages(ctx, o.ID, []domain.Package{{WeightKg: &weight}}, "t", 1); err != nil {
		t.Fatal(err)
	}
	if got, _ := repo.Get(ctx, nil, o.ID); got.LabelStatus != domain.LabelReady {
		t.Fatalf("misma cantidad: %s", got.LabelStatus)
	}
	if _, err := r.ReplacePackages(ctx, o.ID, []domain.Package{{}, {}}, "t", 1); err != nil {
		t.Fatal(err)
	}
	if got, _ := repo.Get(ctx, nil, o.ID); got.LabelStatus != domain.LabelNone {
		t.Fatalf("otra cantidad: %s", got.LabelStatus)
	}
}

func TestLabelStrategyAndLimitInOrder(t *testing.T) {
	db := labelDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	office, err := repo.Get(ctx, nil, 9400002)
	if err != nil {
		t.Fatal(err)
	}
	if office.LabelStrategy != domain.LabelStrategyOffice || office.LabelStatus != domain.LabelOffice || office.CourierMaxPackages == nil || *office.CourierMaxPackages != 1 {
		t.Fatalf("oficina: %s %s %v", office.LabelStrategy, office.LabelStatus, office.CourierMaxPackages)
	}
	api, err := repo.Get(ctx, nil, 9400001)
	if err != nil {
		t.Fatal(err)
	}
	if api.LabelStrategy != domain.LabelStrategyAPI || api.LabelStatus != domain.LabelNone || api.CourierMaxPackages != nil {
		t.Fatalf("api: %s %s %v", api.LabelStrategy, api.LabelStatus, api.CourierMaxPackages)
	}
	id := int64(9400001)
	n, err := NewNotificationRepository(db).Add(ctx, ports.Notification{CompanyID: 7, Warehouse: "central", Audience: "internal", OrderID: &id, Kind: "label_ready", Title: "t", Body: "b"})
	if err != nil {
		t.Fatal(err)
	}
	var warehouse string
	if err := db.QueryRow(ctx, `SELECT warehouse FROM notifications WHERE id=$1`, n.ID).Scan(&warehouse); err != nil || warehouse != "central" {
		t.Fatalf("aviso: %q %v", warehouse, err)
	}
}
