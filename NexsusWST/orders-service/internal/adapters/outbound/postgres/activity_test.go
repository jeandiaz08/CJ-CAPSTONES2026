package postgres

import (
	"context"
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/nexuswst/orders-service/internal/application"
	"github.com/nexuswst/orders-service/internal/ports"
	"github.com/nexuswst/orders-service/internal/shared/tenant"
)

// Registro de acciones sobre la vista activity_log (migración 000009). Todo ocurre en una
// transacción que se deshace al final: no deja datos en la base de desarrollo.
func TestActivityLogView(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL habilita PostgreSQL")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var company, other, jefe, target int64
	one := func(sql string, dst *int64, args ...any) {
		t.Helper()
		if err := tx.QueryRow(ctx, sql, args...).Scan(dst); err != nil {
			t.Fatal(err)
		}
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := tx.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	one(`INSERT INTO companies(name, tax_id) VALUES('Actividad Uno','ACT-1') RETURNING id`, &company)
	one(`INSERT INTO companies(name, tax_id) VALUES('Actividad Dos','ACT-2') RETURNING id`, &other)
	one(`INSERT INTO users(name, role, email) VALUES('Jefa Actividad','jefe_operaciones','jefa-actividad@test.local') RETURNING id`, &jefe)
	one(`INSERT INTO users(name, role, email) VALUES('Operario Actividad','empaque','op-actividad@test.local') RETURNING id`, &target)
	actor := "user:" + strings.TrimSpace(jsonNum(jefe)) + " Jefa Actividad"

	var order, otherOrder, manifest int64
	one(`INSERT INTO orders(company_id, external_ref, channel, courier, status, source, courier_code)
		VALUES($1,'ACT-ORD-1','manual','starken','packing','manual','ST-999') RETURNING id`, &order, company)
	one(`INSERT INTO orders(company_id, external_ref, channel, courier, status, source)
		VALUES($1,'ACT-ORD-2','manual','starken','pending','manual') RETURNING id`, &otherOrder, other)
	exec(`INSERT INTO order_packages(order_id, code) VALUES($1,'ACT-B1'),($1,'ACT-B2')`, order)
	ev := func(id int64, typ, event, from, to, who, reason string, ago time.Duration, details string) {
		t.Helper()
		exec(`INSERT INTO order_events(order_id, company_id, event_type, event, from_status, to_status, actor, reason, occurred_at, details)
			VALUES($1,(SELECT company_id FROM orders WHERE id=$1),$2,$3,NULLIF($4,''),NULLIF($5,''),$6,NULLIF($7,''),now()-$8::interval,NULLIF($9,'')::jsonb)`,
			id, typ, event, from, to, who, reason, ago.String(), details)
	}
	ev(order, "status_change", "validate", "validating", "validated", "system:invas", "Stock disponible", time.Hour, "")
	ev(order, "status_change", "start_packing", "picked", "packing", actor, "", 30*time.Minute, `{"password":"no-debe-salir","token":"tampoco"}`)
	ev(order, "print", "label", "", "", actor, "", 20*time.Minute, "")
	ev(order, "status_change", "cancel", "pending", "cancelled", actor, "Muy antiguo", 400*24*time.Hour, "")
	ev(otherOrder, "created", "create", "", "pending", "Sistema", "Ingreso manual", 10*time.Minute, "")
	one(`INSERT INTO manifests(courier, created_by, created_at, closed_by, closed_at, status, picker_name, picker_id, vehicle_plate, notes)
		VALUES('starken',$1,now()-interval '15 minutes',$1,now()-interval '5 minutes','closed','Juan Retira','11.111.111-1','AB12CD',
		'Empresa del transporte: Transportes Sur') RETURNING id`, &manifest, actor)
	exec(`INSERT INTO manifest_lines(manifest_id, order_id, status) VALUES($1,$2,'delivered')`, manifest, order)
	exec(`INSERT INTO user_audit(actor_user_id, target_user_id, action, detail) VALUES
		($1,$2,'updated','{"role":{"from":"empaque","to":"jefe_operaciones"},"password":"x"}'),
		(NULL,$2,'linked','{"keycloakId":"00000000-0000-0000-0000-000000000000"}')`, jefe, target)

	svc := application.NewActivity(NewActivity(tx))
	admin := tenant.Identity{Profile: tenant.Profile{UserID: 1, Role: tenant.RoleAdmin, AllCompanies: true}}
	list := func(f ports.ActivityFilter) []application.ActivityItem {
		t.Helper()
		f.Limit = 200
		items, _, err := svc.List(ctx, admin, f)
		if err != nil {
			t.Fatal(err)
		}
		mine := []application.ActivityItem{} // la base de desarrollo tiene más datos: solo cuentan los de esta prueba
		for _, it := range items {
			if strings.HasPrefix(it.Entity.Label, "ACT-ORD-") || it.Entity.ID == jsonNum(manifest) || it.Entity.ID == jsonNum(target) {
				mine = append(mine, it)
			}
		}
		return mine
	}
	actions := func(items []application.ActivityItem) []string {
		out := []string{}
		for _, it := range items {
			out = append(out, it.Module+"/"+it.Action)
		}
		return out
	}

	all := list(ports.ActivityFilter{})
	// user_audit usa now(), que en la transacción es el mismo instante: las dos de usuarios van primero, en cualquier orden.
	got := actions(all)
	if len(got) != 8 || !slices.Equal(got[2:], []string{"dispatch/manifest_closed", "orders/create", "dispatch/manifest_created", "packing/print_label", "packing/start_packing", "orders/validate"}) ||
		!slices.Contains(got[:2], "users/updated") || !slices.Contains(got[:2], "users/linked") {
		t.Fatalf("acciones (sin la de hace más de 1 año), lo más reciente primero: %v", got)
	}
	for _, it := range all {
		if it.Reason == "Muy antiguo" {
			t.Fatal("salió una acción de hace más de 1 año")
		}
		raw, _ := json.Marshal(it)
		for _, secret := range []string{"password", "token", "keycloakId", "no-debe-salir", "00000000-0000"} {
			if strings.Contains(string(raw), secret) {
				t.Fatalf("se filtró %q: %s", secret, raw)
			}
		}
	}

	closed := list(ports.ActivityFilter{Module: "dispatch"})[0]
	if closed.Action != "manifest_closed" || closed.Actor.Name != "Jefa Actividad" || closed.Actor.Role != "jefe_operaciones" ||
		closed.Details["pickerName"] != "Juan Retira" || closed.Details["pickerId"] != "11.111.111-1" || closed.Details["vehiclePlate"] != "AB12CD" ||
		closed.Details["orders"] != float64(1) || closed.Details["packages"] != float64(2) || closed.Entity.Label != "#"+jsonNum(manifest) {
		t.Fatalf("cierre de manifiesto %+v", closed)
	}
	packing := list(ports.ActivityFilter{Module: "packing", Actor: "Jefa"})
	if len(packing) != 2 || packing[1].Details["packages"] != float64(2) || packing[1].Details["labelCode"] != "ST-999" || packing[1].CompanyName != "Actividad Uno" {
		t.Fatalf("empaque %+v", packing)
	}
	updated := list(ports.ActivityFilter{Module: "users", Kind: "human"})
	if len(updated) != 1 || updated[0].Entity.Label != "Operario Actividad" || updated[0].Details["targetRole"] != "empaque" ||
		updated[0].Details["role"].(map[string]any)["to"] != "jefe_operaciones" {
		t.Fatalf("usuarios %+v", updated)
	}
	for name, tc := range map[string]struct {
		f    ports.ActivityFilter
		want []string
	}{
		"sistema":           {ports.ActivityFilter{Kind: "system"}, []string{"users/linked", "orders/create", "orders/validate"}},
		"perfil":            {ports.ActivityFilter{Role: "jefe_operaciones", Module: "packing"}, []string{"packing/print_label", "packing/start_packing"}},
		"actor por id":      {ports.ActivityFilter{Actor: jsonNum(jefe), Module: "orders"}, []string{}},
		"n° de orden":       {ports.ActivityFilter{Search: "ACT-ORD-2"}, []string{"orders/create"}},
		"empresa explícita": {ports.ActivityFilter{CompanyID: &other}, []string{"orders/create"}},
		"alcance del jefe":  {ports.ActivityFilter{CompanyIDs: []int64{other}, Module: "orders"}, []string{"orders/create"}},
	} {
		if got := actions(list(tc.f)); !slices.Equal(got, tc.want) {
			t.Fatalf("%s: %v, quería %v", name, got, tc.want)
		}
	}
	from := time.Now().Add(-12 * time.Minute)
	if got := actions(list(ports.ActivityFilter{From: &from})); len(got) != 4 || !slices.Equal(got[2:], []string{"dispatch/manifest_closed", "orders/create"}) {
		t.Fatalf("desde: %v", got)
	}
}

func jsonNum(v int64) string { b, _ := json.Marshal(v); return string(b) }
