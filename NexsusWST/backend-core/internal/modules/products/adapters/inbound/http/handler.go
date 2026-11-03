// Package http es el adaptador de ENTRADA: traduce HTTP <-> casos de uso.
package http

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/nexuswst/backend-core/internal/modules/products/application"
	"github.com/nexuswst/backend-core/internal/modules/products/domain"
	"github.com/nexuswst/backend-core/internal/shared/httpx"
	"github.com/nexuswst/backend-core/internal/shared/tenant"
	"github.com/nexuswst/backend-core/internal/shared/wave"
)

type Handler struct{ svc *application.Service }

func NewHandler(svc *application.Service) *Handler { return &Handler{svc: svc} }

// Register monta las rutas del módulo. "auth" verifica el JWT.
func (h *Handler) Register(mux *http.ServeMux, auth func(http.Handler) http.Handler) {
	write := tenant.RequireRole(tenant.RoleAdmin, tenant.RoleJefeOps, tenant.RoleCliente)
	mux.Handle("GET /v1/products", auth(http.HandlerFunc(h.list)))
	mux.Handle("GET /v1/products/{id}", auth(http.HandlerFunc(h.get)))
	mux.Handle("POST /v1/products", auth(write(http.HandlerFunc(h.create))))
	mux.Handle("PATCH /v1/products/{id}/wave", auth(tenant.RequireRole(tenant.RoleAdmin, tenant.RoleJefeOps)(http.HandlerFunc(h.setWave))))
	mux.Handle("GET /v1/products/{id}/components", auth(http.HandlerFunc(h.components)))
	mux.Handle("PUT /v1/products/{id}/components", auth(write(http.HandlerFunc(h.setComponents))))
}

type productDTO struct {
	ID          int64     `json:"id"`
	CompanyID   int64     `json:"companyId"`
	SKU         string    `json:"sku"`
	Description string    `json:"description"`
	Type        string    `json:"type"`
	Active      bool      `json:"active"`
	WaveCode    *string   `json:"waveCode"`
	CreatedAt   time.Time `json:"createdAt"`
}

func toDTO(p domain.Product) productDTO {
	return productDTO{ID: p.ID, CompanyID: p.CompanyID, SKU: p.SKU, Description: p.Description, Type: string(p.Type), Active: p.Active, WaveCode: p.WaveCode, CreatedAt: p.CreatedAt}
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	id, _ := tenant.From(r.Context())
	scope, _, err := id.Scope(r)
	if err != nil {
		httpx.Error(w, http.StatusForbidden, "forbidden", err.Error())
		return
	}
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	items, err := h.svc.List(r.Context(), scope, q.Get("search"), limit, offset)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "no se pudo listar")
		return
	}
	out := make([]productDTO, 0, len(items))
	for _, p := range items {
		out = append(out, toDTO(p))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": out})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, _ := tenant.From(r.Context())
	scope, _, err := id.Scope(r)
	if err != nil {
		httpx.Error(w, http.StatusForbidden, "forbidden", err.Error())
		return
	}
	pid, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "id inválido")
		return
	}
	p, err := h.svc.Get(r.Context(), scope, pid)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		httpx.Error(w, http.StatusNotFound, "not_found", err.Error())
	case err != nil:
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "error al obtener")
	default:
		httpx.JSON(w, http.StatusOK, toDTO(p))
	}
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	id, _ := tenant.From(r.Context())
	var body struct {
		CompanyID   int64   `json:"companyId"`
		SKU         string  `json:"sku"`
		Description string  `json:"description"`
		Type        string  `json:"type"`
		WaveCode    *string `json:"waveCode"`
	}
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "JSON inválido")
		return
	}
	if !id.CanAccess(body.CompanyID) {
		httpx.Error(w, http.StatusForbidden, "forbidden", "empresa no autorizada")
		return
	}
	// La ola del producto la definen Administrador o Jefe de Operaciones (PV-19).
	if body.WaveCode != nil && *body.WaveCode != "" && !id.HasRole(tenant.RoleAdmin, tenant.RoleJefeOps) {
		httpx.Error(w, http.StatusForbidden, "forbidden", "solo Administrador o Jefe de Operaciones definen la ola del producto")
		return
	}
	p, err := h.svc.Create(r.Context(), body.CompanyID, application.CreateInput{SKU: body.SKU, Description: body.Description, Type: body.Type, WaveCode: body.WaveCode})
	switch {
	case errors.Is(err, domain.ErrInvalid), errors.Is(err, wave.ErrInvalidCode):
		httpx.Error(w, http.StatusUnprocessableEntity, "invalid", err.Error())
	case errors.Is(err, domain.ErrDuplicateSKU):
		httpx.Error(w, http.StatusConflict, "duplicate", err.Error())
	case err != nil:
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "error al crear")
	default:
		httpx.JSON(w, http.StatusCreated, toDTO(p))
	}
}

// --- Composición de kits (H-04-T01) ---------------------------------------

type componentDTO struct {
	ProductID   int64  `json:"productId"`
	SKU         string `json:"sku"`
	Description string `json:"description"`
	Quantity    int    `json:"quantity"`
}

func kitResponse(kit domain.Product, comps []domain.KitComponent) map[string]any {
	out := make([]componentDTO, 0, len(comps))
	for _, c := range comps {
		out = append(out, componentDTO{ProductID: c.ComponentID, SKU: c.SKU, Description: c.Description, Quantity: c.Quantity})
	}
	return map[string]any{"kit": toDTO(kit), "components": out}
}

func (h *Handler) components(w http.ResponseWriter, r *http.Request) {
	id, _ := tenant.From(r.Context())
	scope, _, err := id.Scope(r)
	if err != nil {
		httpx.Error(w, http.StatusForbidden, "forbidden", err.Error())
		return
	}
	pid, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "id inválido")
		return
	}
	kit, comps, err := h.svc.Components(r.Context(), scope, pid)
	if writeKitErr(w, err) {
		return
	}
	httpx.JSON(w, http.StatusOK, kitResponse(kit, comps))
}

func (h *Handler) setComponents(w http.ResponseWriter, r *http.Request) {
	id, _ := tenant.From(r.Context())
	scope, _, err := id.Scope(r)
	if err != nil {
		httpx.Error(w, http.StatusForbidden, "forbidden", err.Error())
		return
	}
	pid, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "id inválido")
		return
	}
	var body struct {
		Components []struct {
			SKU      string `json:"sku"`
			Quantity int    `json:"quantity"`
		} `json:"components"`
	}
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "JSON inválido")
		return
	}
	in := make([]domain.ComponentInput, 0, len(body.Components))
	for _, c := range body.Components {
		in = append(in, domain.ComponentInput{SKU: c.SKU, Quantity: c.Quantity})
	}
	kit, comps, err := h.svc.SetComponents(r.Context(), scope, pid, in)
	if writeKitErr(w, err) {
		return
	}
	httpx.JSON(w, http.StatusOK, kitResponse(kit, comps))
}

// writeKitErr traduce los errores de composición; devuelve true si respondió.
func writeKitErr(w http.ResponseWriter, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, domain.ErrNotFound):
		httpx.Error(w, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, domain.ErrNotKit):
		httpx.Error(w, http.StatusConflict, "not_kit", err.Error())
	case errors.Is(err, domain.ErrInvalidComponent):
		httpx.Error(w, http.StatusUnprocessableEntity, "invalid_component", err.Error())
	default:
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "error en la composición del kit")
	}
	return true
}

// setWave: PATCH /v1/products/{id}/wave  {"waveCode": "AM" | null} (PV-19).
func (h *Handler) setWave(w http.ResponseWriter, r *http.Request) {
	id, _ := tenant.From(r.Context())
	scope, _, err := id.Scope(r)
	if err != nil {
		httpx.Error(w, http.StatusForbidden, "forbidden", err.Error())
		return
	}
	pid, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "id inválido")
		return
	}
	var body struct {
		WaveCode *string `json:"waveCode"`
	}
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "JSON inválido")
		return
	}
	p, err := h.svc.SetWave(r.Context(), scope, pid, body.WaveCode)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		httpx.Error(w, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, wave.ErrInvalidCode):
		httpx.Error(w, http.StatusUnprocessableEntity, "invalid", err.Error())
	case err != nil:
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "no se pudo guardar la ola")
	default:
		httpx.JSON(w, http.StatusOK, toDTO(p))
	}
}
