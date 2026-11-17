package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nexuswst/backend-core/internal/modules/clients/application"
	"github.com/nexuswst/backend-core/internal/modules/clients/domain"
	"github.com/nexuswst/backend-core/internal/modules/clients/ports"
	"github.com/nexuswst/backend-core/internal/shared/tenant"
)

// companiesFake guarda en memoria lo justo para probar rutas, permisos y validación.
type companiesFake struct {
	ports.CompanyRepository
	company domain.Company
	audits  []ports.Audit
}

func (f *companiesFake) AdminList(context.Context, ports.CompanyFilter) ([]domain.Company, int, error) {
	return []domain.Company{f.company}, 1, nil
}

func (f *companiesFake) Get(_ context.Context, id int64) (domain.Company, error) {
	if id != f.company.ID {
		return domain.Company{}, domain.ErrNotFound
	}
	return f.company, nil
}

func (f *companiesFake) Create(_ context.Context, d domain.Draft, _ int64, a ports.Audit) (domain.Company, error) {
	if d.TaxID == "11.111.111-1" {
		return domain.Company{}, domain.ErrTaxIDTaken
	}
	f.audits = append(f.audits, a)
	return domain.Company{ID: 2, Name: d.Name, TaxID: d.TaxID, WaveCode: d.WaveCode, WMSCode: d.WMSCode, Active: true}, nil
}

func (f *companiesFake) Update(_ context.Context, _ int64, d domain.Draft, _ int64, a ports.Audit) error {
	f.audits = append(f.audits, a)
	f.company.Name, f.company.WMSCode = d.Name, d.WMSCode
	return nil
}

func (f *companiesFake) SetActive(_ context.Context, _ int64, active bool, _ int64, a ports.Audit) error {
	if f.company.Active == active {
		return domain.ErrSameState
	}
	f.audits = append(f.audits, a)
	f.company.Active = active
	return nil
}

var (
	adminManage = tenant.Profile{UserID: 1, Role: tenant.RoleAdmin, AllCompanies: true, Permissions: []string{tenant.PermManageProfiles}}
	adminRead   = tenant.Profile{UserID: 8, Role: tenant.RoleAdmin, AllCompanies: true}
	jefe        = tenant.Profile{UserID: 2, Role: tenant.RoleJefeOps, AllCompanies: true, Permissions: []string{tenant.PermDespacho}}
	cliente     = tenant.Profile{UserID: 5, Role: tenant.RoleCliente, Companies: []int64{1}}
)

func serve(f *companiesFake, p tenant.Profile, method, path, body string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	NewHandler(application.NewService(f)).Register(mux, func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(tenant.With(r.Context(), tenant.Identity{Profile: p})))
		})
	})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
	return rec
}

func newFake() *companiesFake {
	return &companiesFake{company: domain.Company{ID: 1, Name: "Empresa Demo Uno SpA", TaxID: "11.111.111-1", WMSCode: "DEMO1", Active: true}}
}

func TestCompaniesAccess(t *testing.T) {
	writes := []struct{ method, path, body string }{
		{"POST", "/v1/companies", `{"name":"Nueva","taxId":"76.123.456-0"}`},
		{"PATCH", "/v1/companies/1", `{"name":"Otra","taxId":"11.111.111-1"}`},
		{"POST", "/v1/companies/1/disable", `{"reason":"Término de contrato"}`},
		{"POST", "/v1/companies/1/enable", ``},
	}
	for _, w := range writes {
		for name, p := range map[string]tenant.Profile{"admin sin permiso": adminRead, "jefe": jefe, "cliente": cliente} {
			if rec := serve(newFake(), p, w.method, w.path, w.body); rec.Code != http.StatusForbidden {
				t.Errorf("%s %s como %s: %d, quería 403", w.method, w.path, name, rec.Code)
			}
		}
	}
	for _, path := range []string{"/v1/companies", "/v1/companies/1", "/v1/companies/export"} {
		if rec := serve(newFake(), adminRead, "GET", path, ""); rec.Code != http.StatusOK {
			t.Errorf("el Administrador sin permiso consulta %s: %d", path, rec.Code)
		}
		for name, p := range map[string]tenant.Profile{"jefe": jefe, "cliente": cliente} {
			if rec := serve(newFake(), p, "GET", path, ""); rec.Code != http.StatusForbidden {
				t.Errorf("%s no ve Administración clientes (%s): %d", name, path, rec.Code)
			}
		}
	}
	// Se valida por permiso: otro perfil con «Gestionar perfiles» también entra.
	jefeManage := jefe
	jefeManage.Permissions = []string{tenant.PermManageProfiles}
	if rec := serve(newFake(), jefeManage, "GET", "/v1/companies", ""); rec.Code != http.StatusOK {
		t.Errorf("con el permiso se consulta: %d", rec.Code)
	}
}

func TestCompaniesWrites(t *testing.T) {
	f := newFake()
	rec := serve(f, adminManage, "POST", "/v1/companies", `{"name":"Andes Outdoor SpA","taxId":"76086428-5","waveCode":"am","wmsCode":"W9"}`)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"taxId":"76.086.428-5"`) || !strings.Contains(rec.Body.String(), `"waveCode":"AM"`) {
		t.Fatalf("crear: %d %s", rec.Code, rec.Body.String())
	}
	if f.audits[0].Action != "company_created" {
		t.Fatalf("la creación debe quedar en el registro: %+v", f.audits)
	}

	// Ola «General (según reglas)» = null.
	rec = serve(newFake(), adminManage, "POST", "/v1/companies", `{"name":"Sin ola","taxId":"76.123.456-0","waveCode":null}`)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"waveCode":null`) {
		t.Fatalf("ola general: %d %s", rec.Code, rec.Body.String())
	}

	// RUT repetido: 409 con el error en el campo.
	rec = serve(newFake(), adminManage, "POST", "/v1/companies", `{"name":"Repetida","taxId":"11.111.111-1"}`)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), `"field":"taxId"`) {
		t.Fatalf("RUT repetido: %d %s", rec.Code, rec.Body.String())
	}

	// Errores por campo (no aplanados).
	rec = serve(newFake(), adminManage, "POST", "/v1/companies", `{"name":"","taxId":"12.345.678-9","email":"x"}`)
	var body struct {
		Fields []domain.FieldError `json:"fields"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if rec.Code != http.StatusUnprocessableEntity || len(body.Fields) != 3 {
		t.Fatalf("validación: %d %s", rec.Code, rec.Body.String())
	}

	// Desactivar exige motivo; queda el antes, el después y el motivo.
	f = newFake()
	if rec := serve(f, adminManage, "POST", "/v1/companies/1/disable", `{"reason":"no"}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("sin motivo: %d", rec.Code)
	}
	if rec := serve(f, adminManage, "POST", "/v1/companies/1/disable", `{"reason":"Término de contrato"}`); rec.Code != http.StatusOK {
		t.Fatalf("desactivar: %d %s", rec.Code, rec.Body.String())
	}
	a := f.audits[len(f.audits)-1]
	if a.Action != "company_disabled" || a.Detail["reason"] != "Término de contrato" || a.Detail["active"] == nil {
		t.Fatalf("auditoría de la desactivación: %+v", a)
	}
	if rec := serve(f, adminManage, "POST", "/v1/companies/1/disable", `{"reason":"Otra vez igual"}`); rec.Code != http.StatusConflict {
		t.Fatalf("ya inactiva: %d", rec.Code)
	}
	if rec := serve(f, adminManage, "POST", "/v1/companies/1/enable", ``); rec.Code != http.StatusOK {
		t.Fatalf("activar sin motivo: %d %s", rec.Code, rec.Body.String())
	}
}

func TestCompaniesWMSOnlyAdmin(t *testing.T) {
	// El código Invas solo lo ve y lo cambia el Administrador.
	jefeManage := jefe
	jefeManage.Permissions = []string{tenant.PermManageProfiles}
	f := newFake()
	rec := serve(f, jefeManage, "PATCH", "/v1/companies/1", `{"name":"Cambio","taxId":"11.111.111-1","wmsCode":"OTRO"}`)
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "wmsCode") || f.company.WMSCode != "DEMO1" {
		t.Fatalf("sin perfil Administrador no se ve ni se cambia el código Invas: %d %s %q", rec.Code, rec.Body.String(), f.company.WMSCode)
	}
	if rec := serve(newFake(), adminRead, "GET", "/v1/companies/1", ""); !strings.Contains(rec.Body.String(), `"wmsCode":"DEMO1"`) {
		t.Fatalf("el Administrador ve el código Invas: %s", rec.Body.String())
	}
}
