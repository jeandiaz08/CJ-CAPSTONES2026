package postgres

import (
	"context"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nexuswst/orders-service/internal/ports"
)

// Registro histórico (Tarea 06): Canceladas de inmediato; cualquier otra orden con
// más de 1 año desde su último cambio de estado. La bandeja excluye lo mismo.
func TestRegistryRuleAndTrayAreComplementary(t *testing.T) {
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
 CREATE TEMP TABLE order_events (LIKE public.order_events INCLUDING DEFAULTS);
 CREATE TEMP TABLE users (LIKE public.users INCLUDING DEFAULTS);
 CREATE TEMP TABLE companies (LIKE public.companies INCLUDING DEFAULTS);
 CREATE TEMP TABLE couriers (LIKE public.couriers INCLUDING DEFAULTS);
 CREATE TEMP TABLE ops_settings (LIKE public.ops_settings INCLUDING DEFAULTS);
 CREATE TEMP TABLE job_outbox (LIKE public.job_outbox INCLUDING DEFAULTS);
 CREATE TEMP TABLE cutoffs (LIKE public.cutoffs INCLUDING DEFAULTS);
 CREATE TEMP TABLE cutoff_couriers (LIKE public.cutoff_couriers INCLUDING DEFAULTS);
 INSERT INTO ops_settings(key,value) VALUES('assignment_deadline','12:00');
 INSERT INTO users(id,name,role,email) VALUES(77,'Jefe Uno','jefe_operaciones','jefe-registro@test.local');
 INSERT INTO couriers(code,name) VALUES('starken','Starken');`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO companies(id,name,tax_id) VALUES(1,'Tienda Uno','REG-1'),(2,'Tienda Dos','REG-2')`); err != nil {
		t.Fatal(err)
	}
	const (
		cancelledRecent int64 = 9100001 + iota
		voidedOld
		voidedRecent
		deliveredOld
		deliveredRecent
		pendingOldNoEvents
		incidentOldTouched
		otherCompanyCancelled
	)
	order := func(id int64, company int, status string, createdDaysAgo int, deliveredDaysAgo *int) {
		t.Helper()
		if _, err := db.Exec(ctx, `INSERT INTO orders(id,company_id,external_ref,channel,courier,status,source,created_at,handed_over_at)
 VALUES($1::bigint,$2::bigint,'R'||$1::text,'manual','starken',$3,'manual',now()-make_interval(days=>$4::int),now()-make_interval(days=>$5::int))`,
			id, company, status, createdDaysAgo, deliveredDaysAgo); err != nil {
			t.Fatal(err)
		}
	}
	event := func(id int64, from, to, actor, reason string, daysAgo int) {
		t.Helper()
		if _, err := db.Exec(ctx, `INSERT INTO order_events(order_id,event_type,event,from_status,to_status,actor,reason,occurred_at)
 VALUES($1,'status_change','x',NULLIF($2,''),$3,$4,NULLIF($5,''),now()-make_interval(days=>$6::int))`, id, from, to, actor, reason, daysAgo); err != nil {
			t.Fatal(err)
		}
	}
	order(cancelledRecent, 1, "cancelled", 3, nil)
	event(cancelledRecent, "pending", "cancelled", "user:77 Jefe Uno", "Compra desistida", 1)
	order(voidedOld, 1, "voided", 500, nil)
	event(voidedOld, "validating", "voided", "Sistema", "Sin stock", 400)
	order(voidedRecent, 1, "voided", 20, nil)
	event(voidedRecent, "validating", "voided", "Sistema", "Sin stock", 10)
	order(deliveredOld, 1, "handed_over", 400, days(366))
	event(deliveredOld, "ready_pickup", "handed_over", "user:77 Jefe Uno", "Retiro confirmado", 366)
	order(deliveredRecent, 1, "handed_over", 30, days(20))
	order(pendingOldNoEvents, 1, "pending", 800, nil)
	order(incidentOldTouched, 1, "incident", 500, nil)
	event(incidentOldTouched, "pending", "incident", "Marco Rojas", "Dirección sin número", 400)
	event(incidentOldTouched, "incident", "incident", "Marco Rojas", "nota sin cambio de estado", 2) // no cuenta
	order(otherCompanyCancelled, 2, "cancelled", 5, nil)
	event(otherCompanyCancelled, "pending", "cancelled", "Sistema", "", 4)

	reg := NewRegistry(db)
	ids := func(rows []ports.RegistryEntry) []int64 {
		out := []int64{}
		for _, r := range rows {
			out = append(out, r.OrderID)
		}
		return out
	}
	all, total, err := reg.Registry(ctx, ports.RegistryFilter{Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	want := []int64{cancelledRecent, otherCompanyCancelled, deliveredOld, incidentOldTouched, voidedOld, pendingOldNoEvents}
	if !slices.Equal(ids(all), want) || total != len(want) {
		t.Fatalf("registro %v (total %d), quería %v", ids(all), total, want)
	}
	byID := map[int64]ports.RegistryEntry{}
	for _, r := range all {
		byID[r.OrderID] = r
	}
	c := byID[cancelledRecent]
	if c.Stale || c.Actor != "user:77 Jefe Uno" || c.ActorRole != "jefe_operaciones" || c.PreviousStatus != "pending" ||
		c.Reason != "Compra desistida" || c.CompanyName != "Tienda Uno" || c.CourierName != "Starken" {
		t.Fatalf("cancelada %+v", c)
	}
	if v := byID[voidedOld]; !v.Stale || v.Reason != "Sin stock" || v.PreviousStatus != "validating" {
		t.Fatalf("anulada antigua %+v", v)
	}
	if i := byID[incidentOldTouched]; i.Reason != "Dirección sin número" || i.ActorRole != "" {
		t.Fatalf("el evento sin cambio de estado no cuenta: %+v", i)
	}
	if d := byID[deliveredOld]; time.Since(d.ClosedAt) < 365*24*time.Hour {
		t.Fatalf("entregada: la fecha es handed_over_at, llegó %v", d.ClosedAt)
	}

	// La bandeja tiene exactamente las demás: ninguna queda en los dos lados ni en ninguno.
	tray, _, err := NewRepository(db).List(ctx, ports.ListFilter{Limit: 200})
	if err != nil {
		t.Fatal(err)
	}
	trayIDs := []int64{}
	for _, o := range tray {
		trayIDs = append(trayIDs, o.ID)
	}
	slices.Sort(trayIDs)
	if !slices.Equal(trayIDs, []int64{voidedRecent, deliveredRecent}) {
		t.Fatalf("bandeja %v", trayIDs)
	}
	registry, _, err := NewRepository(db).List(ctx, ports.ListFilter{View: ports.ViewRegistry, Limit: 200})
	if err != nil || len(registry) != len(want) {
		t.Fatalf("view=registro de GET /v1/orders: %d órdenes (%v)", len(registry), err)
	}

	from := time.Now().AddDate(0, 0, -2)
	for _, tc := range []struct {
		name string
		f    ports.RegistryFilter
		want []int64
	}{
		{"alcance del cliente", ports.RegistryFilter{CompanyIDs: []int64{2}}, []int64{otherCompanyCancelled}},
		{"estado", ports.RegistryFilter{Statuses: []string{"voided"}}, []int64{voidedOld}},
		{"búsqueda por n° de orden", ports.RegistryFilter{Search: "R9100004"}, []int64{deliveredOld}},
		{"fechas del cierre", ports.RegistryFilter{From: &from}, []int64{cancelledRecent}},
		{"seleccionadas", ports.RegistryFilter{IDs: []int64{voidedOld, voidedRecent}}, []int64{voidedOld}},
		{"paginado: el total no depende de la página", ports.RegistryFilter{Limit: 2, Offset: 2}, []int64{deliveredOld, incidentOldTouched}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.f.Limit == 0 {
				tc.f.Limit = 50
			}
			rows, total, err := reg.Registry(ctx, tc.f)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(ids(rows), tc.want) {
				t.Fatalf("got %v, want %v", ids(rows), tc.want)
			}
			if tc.f.Offset == 0 && total != len(tc.want) {
				t.Fatalf("total %d", total)
			}
			if tc.f.Offset > 0 && total != len(want) {
				t.Fatalf("total paginado %d", total)
			}
		})
	}
}
