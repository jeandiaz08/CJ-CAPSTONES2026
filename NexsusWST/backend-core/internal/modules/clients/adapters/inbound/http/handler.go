package http

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/nexuswst/backend-core/internal/modules/clients/application"
	"github.com/nexuswst/backend-core/internal/modules/clients/domain"
	"github.com/nexuswst/backend-core/internal/shared/httpx"
	"github.com/nexuswst/backend-core/internal/shared/tenant"
	"github.com/nexuswst/backend-core/internal/shared/wave"
)

type Handler struct{ svc *application.Service }

func NewHandler(svc *application.Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(mux *http.ServeMux, auth func(http.Handler) http.Handler) {
	mux.Handle("GET /v1/clients", auth(http.HandlerFunc(h.list)))
	mux.Handle("PATCH /v1/clients/{id}/wave", auth(tenant.RequireRole(tenant.RoleAdmin, tenant.RoleJefeOps)(http.HandlerFunc(h.setWave))))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	id, _ := tenant.From(r.Context())
	scope, _, err := id.Scope(r)
	if err != nil {
		httpx.Error(w, http.StatusForbidden, "forbidden", err.Error())
		return
	}
	items, err := h.svc.List(r.Context(), scope, r.URL.Query().Get("search"))
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "no se pudo listar")
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, c := range items {
		out = append(out, map[string]any{"id": c.ID, "name": c.Name, "taxId": c.TaxID, "wmsCode": c.WMSCode, "active": c.Active, "waveCode": c.WaveCode})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": out})
}

// setWave: PATCH /v1/clients/{id}/wave  {"waveCode": "AM" | null} (PV-19).
func (h *Handler) setWave(w http.ResponseWriter, r *http.Request) {
	id, _ := tenant.From(r.Context())
	cid, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "id inválido")
		return
	}
	if !id.CanAccess(cid) {
		httpx.Error(w, http.StatusNotFound, "not_found", domain.ErrNotFound.Error())
		return
	}
	var body struct {
		WaveCode *string `json:"waveCode"`
	}
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "JSON inválido")
		return
	}
	code, err := h.svc.SetWave(r.Context(), cid, body.WaveCode)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		httpx.Error(w, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, wave.ErrInvalidCode):
		httpx.Error(w, http.StatusUnprocessableEntity, "invalid", err.Error())
	case err != nil:
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "no se pudo guardar la ola")
	default:
		httpx.JSON(w, http.StatusOK, map[string]any{"id": cid, "waveCode": code})
	}
}
