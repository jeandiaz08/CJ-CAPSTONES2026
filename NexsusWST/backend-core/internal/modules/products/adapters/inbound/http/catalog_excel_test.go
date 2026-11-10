package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nexuswst/backend-core/internal/modules/products/application"
	"github.com/nexuswst/backend-core/internal/modules/products/domain"
	"github.com/nexuswst/backend-core/internal/platform/xlsx"
	"github.com/nexuswst/backend-core/internal/shared/tenant"
)

// Tarea 09 · Fase 2: importar, plantilla y exportar.

func (m *memCatalog) CompanyByTaxID(_ context.Context, rut string) (int64, string, bool, error) {
	switch strings.NewReplacer(".", "", "-", "").Replace(rut) {
	case "111111111":
		return 1, "Empresa 1", true, nil
	case "222222222":
		return 2, "Empresa 2", true, nil
	case "333333333":
		return 3, "Empresa 3", false, nil
	}
	return 0, "", false, domain.ErrNotFound
}

func (m *memCatalog) ActiveCompanies(context.Context) ([][2]string, error) {
	return [][2]string{{"Empresa 1", "11.111.111-1"}}, nil
}

func importRequest(t *testing.T, rows [][]any) *http.Request {
	t.Helper()
	var file bytes.Buffer
	if err := xlsx.Write(&file, "Productos", []string{"RUT cliente", "SKU", "Descripción", "Ola"}, rows); err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, _ := mw.CreateFormFile("file", "productos.xlsx")
	_, _ = part.Write(file.Bytes())
	_ = mw.Close()
	r := httptest.NewRequest("POST", "/v1/products/import", &body)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	return r
}

func serveCatalogReq(m *memCatalog, p tenant.Profile, r *http.Request) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	NewCatalogHandler(application.NewCatalog(m, m)).Register(mux, func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(tenant.With(req.Context(), tenant.Identity{Profile: p})))
		})
	})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, r)
	return rec
}

func TestCatalogImport(t *testing.T) {
	m := newMemCatalog()
	rows := [][]any{
		{"11.111.111-1", "EJEMPLO-1", "No se importa", ""},      // fila 2: ejemplo
		{"11.111.111-1", "ao-50", "Mochila", "am"},              // fila 3: válida (SKU y ola en mayúsculas)
		{"", "AO-51", "Sin cliente", ""},                        // fila 4: sin RUT
		{"99.999.999-9", "AO-52", "Cliente inexistente", ""},    // fila 5
		{"33.333.333-3", "AO-53", "Cliente desactivado", ""},    // fila 6
		{"11.111.111-1", "", "Sin SKU", ""},                     // fila 7
		{"11.111.111-1", "AO-50", "Repetida en el archivo", ""}, // fila 8
		{"11.111.111-1", "AO-1", "Ya existe en el cliente", ""}, // fila 9
		{"11.111.111-1", "AO-54", "Ola mala", "ola mala"},       // fila 10
		{"11.111.111-1", "AO-55", "", ""},                       // fila 11: sin descripción
		{"222222222", "AO-50", "Mismo SKU, otro cliente", ""},   // fila 12: válida
	}
	// Solo el Administrador importa.
	for name, p := range map[string]tenant.Profile{"jefe": jefe, "cliente": cliente} {
		if rec := serveCatalogReq(m, p, importRequest(t, rows)); rec.Code != http.StatusForbidden {
			t.Errorf("%s importa: %d", name, rec.Code)
		}
	}
	rec := serveCatalogReq(m, admin, importRequest(t, rows))
	var res application.ImportResult
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	got := []string{}
	for _, e := range res.Errors {
		got = append(got, fmt.Sprintf("%d:%s", e.Row, e.Field))
	}
	if rec.Code != http.StatusOK || res.Rows != 10 || len(res.Created) != 2 ||
		fmt.Sprint(got) != "[4:companyId 5:companyId 6:companyId 7:sku 8:sku 9:sku 10:waveCode 11:description]" {
		t.Fatalf("importación: %d %v %s", rec.Code, got, rec.Body.String())
	}
	created := m.products[res.Created[0].ID]
	if created.SKU != "AO-50" || created.Type != domain.TypeSimple || created.WaveCode == nil || *created.WaveCode != "AM" {
		t.Fatalf("producto importado: %+v", created)
	}
	// Archivo sin la columna obligatoria: 422 con el motivo.
	var bad bytes.Buffer
	_ = xlsx.Write(&bad, "Productos", []string{"SKU"}, [][]any{{"X"}})
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, _ := mw.CreateFormFile("file", "x.xlsx")
	_, _ = part.Write(bad.Bytes())
	_ = mw.Close()
	r := httptest.NewRequest("POST", "/v1/products/import", &body)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	if rec := serveCatalogReq(m, admin, r); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "RUT cliente") {
		t.Fatalf("columna faltante: %d %s", rec.Code, rec.Body.String())
	}
}

func TestCatalogExportAndTemplate(t *testing.T) {
	m := newMemCatalog()
	// Exportar: Administrador, Jefe y Cliente (su empresa); el kit con su composición.
	for name, p := range map[string]tenant.Profile{"admin": admin, "jefe": jefe, "cliente": cliente} {
		rec := serveCatalog(m, p, "GET", "/v1/products/export", "")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get("Content-Disposition"), "productos-") {
			t.Fatalf("%s exporta: %d", name, rec.Code)
		}
		table, err := xlsx.ReadFirstSheet(rec.Body.Bytes())
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range table[1:] {
			if p.Role == tenant.RoleCliente && row[3] != "Empresa 1" {
				t.Fatalf("el Cliente exporta otra empresa: %v", row)
			}
			if row[0] == "AO-KIT" && (row[2] != "Kit" || row[6] != "AO-1 ×2") {
				t.Fatalf("kit exportado: %v", row)
			}
		}
	}
	// Plantilla: solo el Administrador.
	if rec := serveCatalog(m, jefe, "GET", "/v1/products/template", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("plantilla como jefe: %d", rec.Code)
	}
	rec := serveCatalog(m, admin, "GET", "/v1/products/template", "")
	table, err := xlsx.ReadFirstSheet(rec.Body.Bytes())
	if rec.Code != http.StatusOK || err != nil || strings.Join(table[0], "|") != "RUT cliente|SKU|Descripción|Ola" {
		t.Fatalf("plantilla: %d %v %v", rec.Code, table, err)
	}
}
