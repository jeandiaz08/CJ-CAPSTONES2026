package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nexuswst/backend-core/internal/modules/products/application"
	"github.com/nexuswst/backend-core/internal/modules/products/domain"
	"github.com/nexuswst/backend-core/internal/modules/products/ports"
	"github.com/nexuswst/backend-core/internal/shared/tenant"
)

// memCatalog: productos, composición y registro en memoria (implementa los dos puertos).
type memCatalog struct {
	ports.ProductRepository
	products  map[int64]*domain.Product
	comps     map[int64][]domain.KitComponent
	companies map[int64]bool // id → activa
	audits    []string
	nextID    int64
}

func newMemCatalog() *memCatalog {
	m := &memCatalog{products: map[int64]*domain.Product{}, comps: map[int64][]domain.KitComponent{}, companies: map[int64]bool{1: true, 2: true, 3: false}, nextID: 100}
	add := func(id, company int64, sku string, t domain.ProductType, active bool) {
		m.products[id] = &domain.Product{ID: id, CompanyID: company, SKU: sku, Description: "Producto " + sku, Type: t, Active: active, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	}
	add(1, 1, "AO-1", domain.TypeSimple, true)
	add(2, 1, "AO-2", domain.TypeSimple, true)
	add(3, 1, "AO-OFF", domain.TypeSimple, false)
	add(4, 2, "CB-1", domain.TypeSimple, true)
	add(5, 1, "AO-KIT", domain.TypeKit, true)
	m.comps[5] = []domain.KitComponent{{ComponentID: 1, SKU: "AO-1", Quantity: 2}}
	return m
}

func (m *memCatalog) visible(ids []int64, p *domain.Product) bool {
	return ids == nil || slices.Contains(ids, p.CompanyID)
}

func (m *memCatalog) item(p *domain.Product) ports.CatalogItem {
	return ports.CatalogItem{Product: *p, CompanyName: fmt.Sprintf("Empresa %d", p.CompanyID)}
}

func (m *memCatalog) ListCatalog(_ context.Context, f ports.CatalogFilter) ([]ports.CatalogItem, int, error) {
	out := []ports.CatalogItem{}
	for _, p := range m.products {
		if m.visible(f.CompanyIDs, p) && (f.Type == "" || p.Type == f.Type) {
			out = append(out, m.item(p))
		}
	}
	return out, len(out), nil
}

func (m *memCatalog) GetCatalog(_ context.Context, ids []int64, id int64) (ports.CatalogItem, error) {
	p, ok := m.products[id]
	if !ok || !m.visible(ids, p) {
		return ports.CatalogItem{}, domain.ErrNotFound
	}
	return m.item(p), nil
}

func (m *memCatalog) GetByID(_ context.Context, ids []int64, id int64) (domain.Product, error) {
	p, ok := m.products[id]
	if !ok || !m.visible(ids, p) {
		return domain.Product{}, domain.ErrNotFound
	}
	return *p, nil
}

func (m *memCatalog) BySKUs(_ context.Context, company int64, skus []string) ([]domain.Product, error) {
	out := []domain.Product{}
	for _, p := range m.products {
		if slices.Contains(skus, p.SKU) {
			out = append(out, *p) // a propósito sin filtrar la empresa: la regla la aplica la aplicación
		}
	}
	return out, nil
}

func (m *memCatalog) Components(_ context.Context, kitID int64) ([]domain.KitComponent, error) {
	return m.comps[kitID], nil
}

func (m *memCatalog) CreateProduct(_ context.Context, p domain.Product, comps []domain.KitComponent, a ports.Audit) (domain.Product, error) {
	for _, x := range m.products {
		if x.CompanyID == p.CompanyID && x.SKU == p.SKU {
			return domain.Product{}, domain.ErrDuplicateSKU
		}
	}
	m.nextID++
	p.ID, p.CreatedAt, p.UpdatedAt = m.nextID, time.Now(), time.Now()
	m.products[p.ID] = &p
	m.comps[p.ID] = comps
	m.audits = append(m.audits, a.Action)
	return p, nil
}

func (m *memCatalog) UpdateProduct(_ context.Context, id int64, desc string, wave *string, comps []domain.KitComponent, a ports.Audit) error {
	m.products[id].Description, m.products[id].WaveCode = desc, wave
	if comps != nil {
		m.comps[id] = comps
	}
	m.audits = append(m.audits, a.Action)
	return nil
}

func (m *memCatalog) SetActive(_ context.Context, id int64, active bool, a ports.Audit) error {
	m.products[id].Active = active
	m.audits = append(m.audits, a.Action)
	return nil
}

func (m *memCatalog) ActiveKitsUsing(_ context.Context, componentID int64) ([]string, error) {
	out := []string{}
	for kitID, comps := range m.comps {
		for _, c := range comps {
			if c.ComponentID == componentID && m.products[kitID].Active {
				out = append(out, m.products[kitID].SKU)
			}
		}
	}
	return out, nil
}

func (m *memCatalog) Company(_ context.Context, id int64) (string, bool, error) {
	active, ok := m.companies[id]
	if !ok {
		return "", false, domain.ErrNotFound
	}
	return fmt.Sprintf("Empresa %d", id), active, nil
}

var (
	admin   = tenant.Profile{UserID: 1, Role: tenant.RoleAdmin, AllCompanies: true}
	jefe    = tenant.Profile{UserID: 2, Role: tenant.RoleJefeOps, AllCompanies: true}
	empaque = tenant.Profile{UserID: 3, Role: tenant.RoleEmpaque, AllCompanies: true}
	cliente = tenant.Profile{UserID: 5, Role: tenant.RoleCliente, Companies: []int64{1}}
)

func serveCatalog(m *memCatalog, p tenant.Profile, method, path, body string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	NewCatalogHandler(application.NewCatalog(m, m)).Register(mux, func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(tenant.With(r.Context(), tenant.Identity{Profile: p})))
		})
	})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
	return rec
}

func TestCatalogAccess(t *testing.T) {
	m := newMemCatalog()
	// Consultan Administrador, Jefe y Cliente; Empaque no.
	for name, p := range map[string]tenant.Profile{"admin": admin, "jefe": jefe, "cliente": cliente} {
		if rec := serveCatalog(m, p, "GET", "/v1/products", ""); rec.Code != http.StatusOK {
			t.Errorf("%s consulta: %d", name, rec.Code)
		}
	}
	if rec := serveCatalog(m, empaque, "GET", "/v1/products", ""); rec.Code != http.StatusForbidden {
		t.Errorf("empaque consulta: %d", rec.Code)
	}
	// Solo el Administrador escribe.
	writes := [][3]string{
		{"POST", "/v1/products", `{"companyId":1,"sku":"X-1","description":"X","type":"simple"}`},
		{"PATCH", "/v1/products/1", `{"description":"Otra"}`},
		{"POST", "/v1/products/1/disable", `{"reason":"Descontinuado"}`},
		{"POST", "/v1/products/3/enable", ``},
	}
	for _, w := range writes {
		for name, p := range map[string]tenant.Profile{"jefe": jefe, "cliente": cliente} {
			if rec := serveCatalog(m, p, w[0], w[1], w[2]); rec.Code != http.StatusForbidden {
				t.Errorf("%s %s como %s: %d, quería 403", w[0], w[1], name, rec.Code)
			}
		}
	}
	// Aislamiento del Cliente: solo su empresa; un producto de otra empresa no existe.
	var list struct {
		Data []application.ProductView `json:"data"`
	}
	rec := serveCatalog(m, cliente, "GET", "/v1/products", "")
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	for _, p := range list.Data {
		if p.CompanyID != 1 {
			t.Fatalf("el Cliente ve un producto de otra empresa: %+v", p)
		}
	}
	if rec := serveCatalog(m, cliente, "GET", "/v1/products/4", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("producto de otra empresa: %d", rec.Code)
	}
	if rec := serveCatalog(m, cliente, "GET", "/v1/products?company_id=2", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("company_id ajeno: %d", rec.Code)
	}
}

func TestCatalogCreate(t *testing.T) {
	m := newMemCatalog()
	post := func(body string) *httptest.ResponseRecorder {
		return serveCatalog(m, admin, "POST", "/v1/products", body)
	}
	// Obligatorios por campo.
	rec := post(`{"companyId":0,"sku":"","description":"","type":"otro"}`)
	if rec.Code != http.StatusUnprocessableEntity || strings.Count(rec.Body.String(), `"field"`) != 4 {
		t.Fatalf("obligatorios: %d %s", rec.Code, rec.Body.String())
	}
	// Cliente desactivado y ola inválida.
	if rec := post(`{"companyId":3,"sku":"X-1","description":"X","type":"simple","waveCode":"ola mala"}`); rec.Code != http.StatusUnprocessableEntity ||
		!strings.Contains(rec.Body.String(), "desactivado") || !strings.Contains(rec.Body.String(), `"field":"waveCode"`) {
		t.Fatalf("cliente desactivado y ola: %d %s", rec.Code, rec.Body.String())
	}
	// Artículo con ola «General» (null); el SKU se guarda en mayúsculas.
	rec = post(`{"companyId":1,"sku":" ao-9 ","description":"Mochila","type":"simple","waveCode":null}`)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"sku":"AO-9"`) || !strings.Contains(rec.Body.String(), `"waveCode":null`) {
		t.Fatalf("crear artículo: %d %s", rec.Code, rec.Body.String())
	}
	// SKU repetido en la empresa: 409 en el campo; en otra empresa, sí se puede.
	if rec := post(`{"companyId":1,"sku":"AO-9","description":"Otra","type":"simple"}`); rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), `"field":"sku"`) {
		t.Fatalf("SKU repetido: %d %s", rec.Code, rec.Body.String())
	}
	if rec := post(`{"companyId":2,"sku":"AO-9","description":"Otra","type":"simple"}`); rec.Code != http.StatusCreated {
		t.Fatalf("mismo SKU en otra empresa: %d %s", rec.Code, rec.Body.String())
	}
	// Kit: al menos un componente, de la misma empresa, activo y simple.
	for body, want := range map[string]string{
		`{"companyId":1,"sku":"K-1","description":"Kit","type":"kit"}`:                                               "al menos un componente",
		`{"companyId":1,"sku":"K-1","description":"Kit","type":"kit","components":[{"sku":"CB-1","quantity":1}]}`:    "otra empresa",
		`{"companyId":1,"sku":"K-1","description":"Kit","type":"kit","components":[{"sku":"AO-OFF","quantity":1}]}`:  "inactivo",
		`{"companyId":1,"sku":"K-1","description":"Kit","type":"kit","components":[{"sku":"AO-KIT","quantity":1}]}`:  "kits dentro de kits",
		`{"companyId":1,"sku":"K-1","description":"Kit","type":"kit","components":[{"sku":"AO-1","quantity":0}]}`:    "mayor que 0",
		`{"companyId":1,"sku":"S-1","description":"Art","type":"simple","components":[{"sku":"AO-1","quantity":1}]}`: "Solo un kit",
	} {
		if rec := post(body); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `"field":"components"`) || !strings.Contains(rec.Body.String(), want) {
			t.Errorf("%s: %d %s", body, rec.Code, rec.Body.String())
		}
	}
	rec = post(`{"companyId":1,"sku":"K-1","description":"Kit trekking","type":"kit","components":[{"sku":"ao-1","quantity":2},{"sku":"AO-2","quantity":1}]}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("crear kit: %d %s", rec.Code, rec.Body.String())
	}
	// Un kit se muestra como composición, nunca como stock.
	var kit application.ProductView
	_ = json.Unmarshal(rec.Body.Bytes(), &kit)
	if kit.Type != "kit" || len(kit.Components) != 2 || strings.Contains(strings.ToLower(rec.Body.String()), "stock") {
		t.Fatalf("kit: %s", rec.Body.String())
	}
	if fmt.Sprint(m.audits) != "[product_created product_created product_created]" {
		t.Fatalf("registro: %v", m.audits)
	}
}

func TestCatalogUpdateAndActive(t *testing.T) {
	m := newMemCatalog()
	// Cliente, SKU y tipo no se editan.
	if rec := serveCatalog(m, admin, "PATCH", "/v1/products/1", `{"description":"X","sku":"OTRO"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("SKU no se edita: %d", rec.Code)
	}
	// Editar descripción y ola: una acción.
	m.audits = nil
	rec := serveCatalog(m, admin, "PATCH", "/v1/products/1", `{"description":"Mochila verde","waveCode":"am"}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"waveCode":"AM"`) || fmt.Sprint(m.audits) != "[product_updated]" {
		t.Fatalf("editar: %d %s %v", rec.Code, rec.Body.String(), m.audits)
	}
	// Composición del kit; en un artículo no se acepta.
	if rec := serveCatalog(m, admin, "PATCH", "/v1/products/5", `{"description":"Kit","components":[{"sku":"AO-2","quantity":3}]}`); rec.Code != http.StatusOK ||
		m.comps[5][0].SKU != "AO-2" || m.comps[5][0].Quantity != 3 {
		t.Fatalf("editar composición: %d %s", rec.Code, rec.Body.String())
	}
	if rec := serveCatalog(m, admin, "PATCH", "/v1/products/1", `{"description":"X","components":[{"sku":"AO-2","quantity":1}]}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("composición en un artículo: %d", rec.Code)
	}
	// Desactivar exige motivo; un componente de un kit activo no se desactiva.
	if rec := serveCatalog(m, admin, "POST", "/v1/products/2/disable", `{"reason":"no"}`); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `"field":"reason"`) {
		t.Fatalf("motivo: %d %s", rec.Code, rec.Body.String())
	}
	if rec := serveCatalog(m, admin, "POST", "/v1/products/2/disable", `{"reason":"Descontinuado"}`); rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "AO-KIT") {
		t.Fatalf("componente de un kit activo: %d %s", rec.Code, rec.Body.String())
	}
	// Desactivar el kit libera el componente; no hay borrado físico.
	if rec := serveCatalog(m, admin, "POST", "/v1/products/5/disable", `{"reason":"Fin de temporada"}`); rec.Code != http.StatusOK {
		t.Fatalf("desactivar kit: %d %s", rec.Code, rec.Body.String())
	}
	if rec := serveCatalog(m, admin, "POST", "/v1/products/2/disable", `{"reason":"Descontinuado"}`); rec.Code != http.StatusOK || m.products[2] == nil {
		t.Fatalf("desactivar componente libre: %d %s", rec.Code, rec.Body.String())
	}
	if rec := serveCatalog(m, admin, "POST", "/v1/products/2/disable", `{"reason":"Otra vez"}`); rec.Code != http.StatusConflict {
		t.Fatalf("ya inactivo: %d", rec.Code)
	}
	// Un kit con componentes inactivos no se activa.
	if rec := serveCatalog(m, admin, "POST", "/v1/products/5/enable", ``); rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "AO-2") {
		t.Fatalf("activar kit con componente inactivo: %d %s", rec.Code, rec.Body.String())
	}
}
