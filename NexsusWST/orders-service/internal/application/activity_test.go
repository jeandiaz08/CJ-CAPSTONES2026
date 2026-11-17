package application

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/nexuswst/orders-service/internal/domain"
	"github.com/nexuswst/orders-service/internal/ports"
	"github.com/nexuswst/orders-service/internal/shared/tenant"
)

type fakeActivity struct{ got ports.ActivityFilter }

func (f *fakeActivity) Activity(_ context.Context, af ports.ActivityFilter) ([]ports.ActivityEntry, int, error) {
	f.got = af
	return []ports.ActivityEntry{{ID: "ua-1", ActorName: "Sistema", ActorKind: "system", Module: "users", Action: "linked",
		Details: map[string]any{"keycloakId": "abc", "password": "x", "token": "y", "role": "empaque",
			"permissions": map[string]any{"from": []any{}, "to": []any{"despacho"}, "secret": "z"}}}}, 1, nil
}

func TestActivityAccessAndValidation(t *testing.T) {
	ctx := context.Background()
	for _, role := range []string{tenant.RoleCliente, tenant.RoleEmpaque, tenant.RoleAnalista} {
		_, _, err := NewActivity(&fakeActivity{}).List(ctx, who(role, tenant.PermDespacho), ports.ActivityFilter{})
		if !errors.Is(err, domain.ErrForbidden) {
			t.Fatalf("%s: quería 403, llegó %v", role, err)
		}
	}
	for _, f := range []ports.ActivityFilter{{Module: "config"}, {Kind: "robot"}} {
		if _, _, err := NewActivity(&fakeActivity{}).List(ctx, who(tenant.RoleJefeOps), f); !errors.Is(err, domain.ErrInvalid) {
			t.Fatalf("%+v: quería 422, llegó %v", f, err)
		}
	}
}

func TestActivityRetentionLimitAndSafeDetails(t *testing.T) {
	now := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	repo := &fakeActivity{}
	a := NewActivity(repo)
	a.now = func() time.Time { return now }
	for _, role := range []string{tenant.RoleAdmin, tenant.RoleJefeOps} {
		items, total, err := a.List(context.Background(), who(role), ports.ActivityFilter{Module: "users", Limit: 5000})
		if err != nil || total != 1 {
			t.Fatalf("%s: %v", role, err)
		}
		if !repo.got.Since.Equal(now.Add(-ActivityRetention)) || repo.got.Limit != 50 {
			t.Fatalf("retención %v, límite %d", repo.got.Since, repo.got.Limit)
		}
		d := items[0].Details
		for _, k := range []string{"keycloakId", "password", "token"} {
			if _, ok := d[k]; ok {
				t.Fatalf("%s no debería salir: %v", k, d)
			}
		}
		perm := d["permissions"].(map[string]any)
		if d["role"] != "empaque" || perm["secret"] != nil || perm["to"] == nil {
			t.Fatalf("detalle %v", d)
		}
	}
}

func TestActivityTexts(t *testing.T) {
	it := ActivityItem{Module: "dispatch", Action: "manifest_closed", Details: map[string]any{"courier": "starken",
		"notes": "Empresa del transporte: Transportes Sur\nObservación · #1 · faltó un cable", "pickerName": "Juan"}}
	presentActivity(&it, "Starken")
	if it.ActionLabel != "Cerró manifiesto (entrega)" || it.CourierName != "Starken" || it.TransportCompany != "Transportes Sur" ||
		it.Observation != "Observación · #1 · faltó un cable" {
		t.Fatalf("retiro %+v", it)
	}
	u := ActivityItem{Module: "users", Action: "updated", Details: map[string]any{
		"role": map[string]any{"from": "empaque", "to": "jefe_operaciones"}, "permissions": map[string]any{"from": []any{}, "to": []any{"despacho"}}}}
	presentActivity(&u, "")
	if want := []string{"Permisos: — → despacho", "Perfil: Empaque → Jefe de Operaciones"}; len(u.DetailLines) != 2 || u.DetailLines[0] != want[0] || u.DetailLines[1] != want[1] {
		t.Fatalf("usuarios %v", u.DetailLines)
	}
	// Tarea 07: empresas cliente en Registro de acciones › Usuarios (antes, después y motivo).
	c := ActivityItem{Module: "users", Action: "company_disabled", Details: safeDetails(map[string]any{
		"active": map[string]any{"from": true, "to": false}, "reason": "Término de contrato", "secret": "x"})}
	presentActivity(&c, "")
	if want := []string{"Estado: Activa → Inactiva", "Motivo: Término de contrato"}; c.ActionLabel != "Empresa desactivada" ||
		len(c.DetailLines) != 2 || c.DetailLines[0] != want[0] || c.DetailLines[1] != want[1] {
		t.Fatalf("empresa %q %v", c.ActionLabel, c.DetailLines)
	}
	e := ActivityItem{Module: "users", Action: "company_updated", Details: safeDetails(map[string]any{
		"waveCode": map[string]any{"from": "General (según reglas)", "to": "AM"}, "taxId": map[string]any{"from": "1", "to": "2"}})}
	presentActivity(&e, "")
	if want := []string{"RUT: 1 → 2", "Ola: General (según reglas) → AM"}; e.ActionLabel != "Empresa editada" || len(e.DetailLines) != 2 ||
		e.DetailLines[0] != want[0] || e.DetailLines[1] != want[1] {
		t.Fatalf("empresa editada %v", e.DetailLines)
	}
	sol := ActivityItem{Module: "users", Action: "request_rejected", Details: safeDetails(map[string]any{"requestKind": "deactivation", "reason": "Debe seguir activo", "name": "Pablo"})}
	presentActivity(&sol, "")
	if want := "Nombre: Pablo|Motivo: Debe seguir activo|Tipo de solicitud: Desactivación"; sol.ActionLabel != "Solicitud de perfil rechazada" || strings.Join(sol.DetailLines, "|") != want {
		t.Fatalf("solicitud %q %v", sol.ActionLabel, sol.DetailLines)
	}
	imp := ActivityItem{Module: "users", Action: "profiles_imported", Details: safeDetails(map[string]any{"created": float64(2), "rows": float64(5), "failed": float64(3)})}
	presentActivity(&imp, "")
	if want := "Perfiles creados: 2|Filas con error: 3|Filas leídas: 5"; imp.ActionLabel != "Carga masiva de perfiles" || strings.Join(imp.DetailLines, "|") != want {
		t.Fatalf("carga masiva %q %v", imp.ActionLabel, imp.DetailLines)
	}
	vacio := ActivityItem{Module: "users", Action: "company_updated", Details: safeDetails(map[string]any{"phone": map[string]any{"from": "", "to": "+56 2 2222 3333"}})}
	presentActivity(&vacio, "")
	if want := "Teléfono: — → +56 2 2222 3333"; strings.Join(vacio.DetailLines, "|") != want {
		t.Fatalf("dato vacío %v", vacio.DetailLines)
	}
	o := ActivityItem{Module: "orders", Action: "validate", Details: map[string]any{"fromStatus": "validating", "toStatus": "validated"}}
	presentActivity(&o, "")
	if o.StatusChange != "En validación → Validada" || o.ActionLabel != "Validada" {
		t.Fatalf("órdenes %+v", o)
	}
	if err := NewActivity(&fakeActivity{}).Export(context.Background(), who(tenant.RoleAdmin), ports.ActivityFilter{}, io.Discard); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("exportar sin apartado: %v", err)
	}
	if err := NewActivity(&fakeActivity{}).Export(context.Background(), who(tenant.RoleEmpaque), ports.ActivityFilter{Module: "users"}, io.Discard); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("exportar como Empaque: %v", err)
	}
	if err := NewActivity(&fakeActivity{}).Export(context.Background(), who(tenant.RoleJefeOps), ports.ActivityFilter{Module: "users"}, io.Discard); err != nil {
		t.Fatal(err)
	}
}
