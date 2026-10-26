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
)

type Handler struct{ svc *application.Service }

func NewHandler(svc *application.Service) *Handler { return &Handler{svc: svc} }

// Register monta las rutas del módulo. "auth" verifica el JWT.
func (h *Handler) Register(mux *http.ServeMux, auth func(http.Handler) http.Handler) {
	write := tenant.RequireRole(tenant.RoleAdmin, tenant.RoleJefeOps, tenant.RoleCliente)
	mux.Handle("GET /v1/products", auth(http.HandlerFunc(h.list)))
	mux.Handle("GET /v1/products/{id}", auth(http.HandlerFunc(h.get)))
	mux.Handle("POST /v1/products", auth(write(http.HandlerFunc(h.create))))
}

type productDTO struct {
	ID          int64     `json:"id"`
	CompanyID   int64     `json:"companyId"`
	SKU         string    `json:"sku"`
	Description string    `json:"description"`
	Type        string    `json:"type"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"createdAt"`
}

func toDTO(p domain.Product) productDTO {
	return productDTO{ID: p.ID, CompanyID: p.CompanyID, SKU: p.SKU, Description: p.Description, Type: string(p.Type), Active: p.Active, CreatedAt: p.CreatedAt}
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
		CompanyID   int64  `json:"companyId"`
		SKU         string `json:"sku"`
		Description string `json:"description"`
		Type        string `json:"type"`
	}
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "JSON inválido")
		return
	}
	if !id.CanAccess(body.CompanyID) {
		httpx.Error(w, http.StatusForbidden, "forbidden", "empresa no autorizada")
		return
	}
	p, err := h.svc.Create(r.Context(), body.CompanyID, application.CreateInput{SKU: body.SKU, Description: body.Description, Type: body.Type})
	switch {
	case errors.Is(err, domain.ErrInvalid):
		httpx.Error(w, http.StatusUnprocessableEntity, "invalid", err.Error())
	case errors.Is(err, domain.ErrDuplicateSKU):
		httpx.Error(w, http.StatusConflict, "duplicate", err.Error())
	case err != nil:
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "error al crear")
	default:
		httpx.JSON(w, http.StatusCreated, toDTO(p))
	}
}
