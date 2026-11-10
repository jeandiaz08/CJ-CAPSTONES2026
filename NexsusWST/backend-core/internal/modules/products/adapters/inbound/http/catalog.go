package http

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/nexuswst/backend-core/internal/modules/products/application"
	"github.com/nexuswst/backend-core/internal/modules/products/domain"
	"github.com/nexuswst/backend-core/internal/modules/products/ports"
	"github.com/nexuswst/backend-core/internal/shared/httpx"
	"github.com/nexuswst/backend-core/internal/shared/tenant"
)

// CatalogHandler - página Productos (Tarea 09, H-04). Matriz de perfiles v1.2:
//   - Consultar y exportar: Administrador, Jefe de Operaciones y Analista (lectura) y el Cliente,
//     solo su empresa (lo fija el servidor con tenant.Scope).
//   - Crear, editar, activar y desactivar (e importar, Fase 2): solo el Administrador.
//
// TODO(matriz): la escritura del Cliente está «Por validar» en la matriz v1.2; se confirma con
// Ismael. Mientras tanto, el Cliente solo consulta y exporta lo de su empresa.
type CatalogHandler struct{ svc *application.Catalog }

func NewCatalogHandler(svc *application.Catalog) *CatalogHandler { return &CatalogHandler{svc: svc} }

func (h *CatalogHandler) Register(mux *http.ServeMux, auth func(http.Handler) http.Handler) {
	view := func(f http.HandlerFunc) http.Handler {
		return auth(tenant.RequireRole(tenant.RoleAdmin, tenant.RoleJefeOps, tenant.RoleAnalista, tenant.RoleCliente)(f))
	}
	write := func(f http.HandlerFunc) http.Handler { return auth(tenant.RequireRole(tenant.RoleAdmin)(f)) }
	mux.Handle("GET /v1/products", view(h.list))
	mux.Handle("GET /v1/products/export", view(h.export)) // Fase 2: exportar lo autorizado
	mux.Handle("GET /v1/products/template", write(h.template))
	mux.Handle("POST /v1/products/import", write(h.importFile))
	mux.Handle("GET /v1/products/{id}", view(h.get))
	mux.Handle("POST /v1/products", write(h.create))
	mux.Handle("PATCH /v1/products/{id}", write(h.update))
	mux.Handle("POST /v1/products/{id}/disable", write(h.disable))
	mux.Handle("POST /v1/products/{id}/enable", write(h.enable))
}

// scope: empresas que el usuario puede ver (nil = todas). ?company_id acota a una autorizada.
func scope(w http.ResponseWriter, r *http.Request) ([]int64, bool) {
	id, _ := tenant.From(r.Context())
	ids, _, err := id.Scope(r)
	if err != nil {
		httpx.Error(w, http.StatusForbidden, "forbidden", err.Error())
		return nil, false
	}
	return ids, true
}

func pathProductID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "id inválido")
		return 0, false
	}
	return id, true
}

// catalogFilter lee los filtros de la página: q (o search, el nombre antiguo que usa el formulario
// de órdenes), type, status, wave, ids, limit y offset. company_id lo resuelve tenant.Scope.
func catalogFilter(r *http.Request, companyIDs []int64) (ports.CatalogFilter, error) {
	q := r.URL.Query()
	f := ports.CatalogFilter{CompanyIDs: companyIDs, Query: q.Get("q"), Wave: strings.ToUpper(strings.TrimSpace(q.Get("wave")))}
	if f.Query == "" {
		f.Query = q.Get("search")
	}
	if f.Wave == "GENERAL" {
		f.Wave = "general"
	}
	switch t := domain.ProductType(q.Get("type")); t {
	case "", domain.TypeSimple, domain.TypeKit:
		f.Type = t
	default:
		return f, errors.New("tipo inválido: usa simple o kit")
	}
	switch q.Get("status") {
	case "":
	case "active", "inactive":
		v := q.Get("status") == "active"
		f.Active = &v
	default:
		return f, errors.New("estado inválido: usa active o inactive")
	}
	if raw := q.Get("ids"); raw != "" {
		for _, part := range strings.Split(raw, ",") {
			id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
			if err != nil || id <= 0 {
				return f, errors.New("ids inválidos")
			}
			f.IDs = append(f.IDs, id)
		}
	}
	f.Limit, _ = strconv.Atoi(q.Get("limit"))
	f.Offset, _ = strconv.Atoi(q.Get("offset"))
	return f, nil
}

func (h *CatalogHandler) list(w http.ResponseWriter, r *http.Request) {
	ids, ok := scope(w, r)
	if !ok {
		return
	}
	f, err := catalogFilter(r, ids)
	if err != nil {
		httpx.Error(w, http.StatusUnprocessableEntity, "validation_error", err.Error())
		return
	}
	items, total, err := h.svc.List(r.Context(), f)
	if err != nil {
		writeCatalogErr(w, err)
		return
	}
	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": items, "total": total, "limit": limit, "offset": max(f.Offset, 0)})
}

func (h *CatalogHandler) get(w http.ResponseWriter, r *http.Request) {
	ids, ok := scope(w, r)
	if !ok {
		return
	}
	id, ok := pathProductID(w, r)
	if !ok {
		return
	}
	v, err := h.svc.Get(r.Context(), ids, id)
	if err != nil {
		writeCatalogErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, v)
}

type componentBody struct {
	SKU      string `json:"sku"`
	Quantity int    `json:"quantity"`
}

func componentInputs(in []componentBody) []domain.ComponentInput {
	if in == nil {
		return nil
	}
	out := make([]domain.ComponentInput, 0, len(in))
	for _, c := range in {
		out = append(out, domain.ComponentInput{SKU: c.SKU, Quantity: c.Quantity})
	}
	return out
}

func (h *CatalogHandler) create(w http.ResponseWriter, r *http.Request) {
	ids, ok := scope(w, r)
	if !ok {
		return
	}
	var b struct {
		CompanyID   int64           `json:"companyId"`
		SKU         string          `json:"sku"`
		Description string          `json:"description"`
		Type        string          `json:"type"`
		WaveCode    *string         `json:"waveCode"`
		Components  []componentBody `json:"components"`
	}
	if err := httpx.Decode(r, &b); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "JSON inválido")
		return
	}
	actor, _ := tenant.From(r.Context())
	v, err := h.svc.Create(r.Context(), actor.UserID, ids, domain.ProductDraft{CompanyID: b.CompanyID, SKU: b.SKU, Description: b.Description,
		Type: domain.ProductType(b.Type), WaveCode: b.WaveCode}, componentInputs(b.Components))
	if err != nil {
		writeCatalogErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, v)
}

// update: {description, waveCode, components?}. Cliente, SKU y tipo no se editan (400).
func (h *CatalogHandler) update(w http.ResponseWriter, r *http.Request) {
	ids, ok := scope(w, r)
	if !ok {
		return
	}
	id, ok := pathProductID(w, r)
	if !ok {
		return
	}
	var b struct {
		Description string          `json:"description"`
		WaveCode    *string         `json:"waveCode"`
		Components  []componentBody `json:"components"`
	}
	if err := httpx.Decode(r, &b); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "JSON inválido (el cliente, el SKU y el tipo no se editan)")
		return
	}
	actor, _ := tenant.From(r.Context())
	v, err := h.svc.Update(r.Context(), actor.UserID, ids, id, domain.ProductDraft{Description: b.Description, WaveCode: b.WaveCode}, componentInputs(b.Components))
	if err != nil {
		writeCatalogErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, v)
}

func (h *CatalogHandler) disable(w http.ResponseWriter, r *http.Request) { h.setActive(w, r, false) }
func (h *CatalogHandler) enable(w http.ResponseWriter, r *http.Request)  { h.setActive(w, r, true) }

func (h *CatalogHandler) setActive(w http.ResponseWriter, r *http.Request, active bool) {
	ids, ok := scope(w, r)
	if !ok {
		return
	}
	id, ok := pathProductID(w, r)
	if !ok {
		return
	}
	var b struct {
		Reason string `json:"reason"`
	}
	if err := httpx.Decode(r, &b); err != nil && !errors.Is(err, io.EOF) {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "JSON inválido")
		return
	}
	actor, _ := tenant.From(r.Context())
	v, err := h.svc.SetActive(r.Context(), actor.UserID, ids, id, active, b.Reason)
	if err != nil {
		writeCatalogErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, v)
}

const maxImportBytes = 5 << 20

func sendXLSX(w http.ResponseWriter, name string, data []byte) {
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
	_, _ = w.Write(data)
}

// export: mismos filtros de la tabla (o ?ids= los seleccionados).
func (h *CatalogHandler) export(w http.ResponseWriter, r *http.Request) {
	ids, ok := scope(w, r)
	if !ok {
		return
	}
	f, err := catalogFilter(r, ids)
	if err != nil {
		httpx.Error(w, http.StatusUnprocessableEntity, "validation_error", err.Error())
		return
	}
	var buf bytes.Buffer
	if err := h.svc.Export(r.Context(), f, &buf); err != nil {
		writeCatalogErr(w, err)
		return
	}
	sendXLSX(w, fmt.Sprintf("productos-%s.xlsx", time.Now().Format("20060102-1504")), buf.Bytes())
}

func (h *CatalogHandler) template(w http.ResponseWriter, r *http.Request) {
	var buf bytes.Buffer
	if err := h.svc.Template(r.Context(), &buf); err != nil {
		writeCatalogErr(w, err)
		return
	}
	sendXLSX(w, "plantilla-productos.xlsx", buf.Bytes())
}

func (h *CatalogHandler) importFile(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxImportBytes)
	file, _, err := r.FormFile("file")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "Adjunta el archivo .xlsx en el campo «file» (máximo 5 MB).")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "No se pudo leer el archivo.")
		return
	}
	actor, _ := tenant.From(r.Context())
	res, err := h.svc.Import(r.Context(), actor.UserID, data)
	if err != nil {
		writeCatalogErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

func writeCatalogErr(w http.ResponseWriter, err error) {
	var fields domain.ValidationErrors
	var importErr application.ImportError
	if errors.As(err, &importErr) || errors.Is(err, application.ErrExportTooBig) {
		httpx.Error(w, http.StatusUnprocessableEntity, "validation_error", err.Error())
		return
	}
	var inUse domain.InUseError
	var inactive domain.InactiveComponentsError
	switch {
	case errors.Is(err, domain.ErrDuplicateSKU):
		httpx.JSON(w, http.StatusConflict, map[string]any{"error": "duplicate", "message": "Ese SKU ya existe para el cliente.",
			"fields": domain.ValidationErrors{{Field: "sku", Message: "Ese SKU ya existe para el cliente."}}})
	case errors.As(err, &fields):
		httpx.JSON(w, http.StatusUnprocessableEntity, map[string]any{"error": "validation_error", "message": "Revisa los datos marcados", "fields": fields})
	case errors.Is(err, domain.ErrReasonRequired):
		httpx.JSON(w, http.StatusUnprocessableEntity, map[string]any{"error": "validation_error", "message": "Revisa los datos marcados",
			"fields": domain.ValidationErrors{{Field: "reason", Message: "Indica el motivo (entre 5 y 500 caracteres)."}}})
	case errors.Is(err, domain.ErrNotFound):
		httpx.Error(w, http.StatusNotFound, "not_found", "Producto no encontrado.")
	case errors.Is(err, domain.ErrSameState):
		httpx.Error(w, http.StatusConflict, "same_state", "El producto ya está en ese estado; recarga la página.")
	case errors.As(err, &inUse):
		httpx.Error(w, http.StatusConflict, "in_use", "No se puede desactivar: es componente de kits activos ("+strings.Join(inUse.Kits, ", ")+"). Desactiva o edita esos kits primero.")
	case errors.As(err, &inactive):
		httpx.Error(w, http.StatusConflict, "inactive_components", "No se puede activar: el kit tiene componentes inactivos ("+strings.Join(inactive.SKUs, ", ")+"). Actívalos primero.")
	default:
		httpx.Internal(w, err, "Error inesperado en productos")
	}
}
