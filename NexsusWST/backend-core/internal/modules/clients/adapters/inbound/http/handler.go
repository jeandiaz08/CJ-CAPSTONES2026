package http

import (
	"net/http"

	"github.com/nexuswst/backend-core/internal/modules/clients/application"
	"github.com/nexuswst/backend-core/internal/shared/httpx"
	"github.com/nexuswst/backend-core/internal/shared/tenant"
)

type Handler struct{ svc *application.Service }

func NewHandler(svc *application.Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(mux *http.ServeMux, auth func(http.Handler) http.Handler) {
	mux.Handle("GET /v1/clients", auth(http.HandlerFunc(h.list)))
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
		out = append(out, map[string]any{"id": c.ID, "name": c.Name, "taxId": c.TaxID, "wmsCode": c.WMSCode, "active": c.Active})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": out})
}
