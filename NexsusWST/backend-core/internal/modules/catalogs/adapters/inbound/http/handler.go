package http

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"

	"github.com/nexuswst/backend-core/internal/modules/catalogs/application"
	"github.com/nexuswst/backend-core/internal/modules/catalogs/domain"
	"github.com/nexuswst/backend-core/internal/shared/httpx"
	"github.com/nexuswst/backend-core/internal/shared/tenant"
)

type Handler struct{ svc *application.Service }

func NewHandler(svc *application.Service) *Handler { return &Handler{svc: svc} }

// Catálogos globales y comunes: requieren sesión, no dependen de la empresa.
func (h *Handler) Register(mux *http.ServeMux, auth func(http.Handler) http.Handler) {
	mux.Handle("GET /v1/catalogs/{name}", auth(http.HandlerFunc(h.get)))
	mux.Handle("POST /v1/catalogs/{name}/refresh", auth(tenant.RequireRole(tenant.RoleAdmin)(http.HandlerFunc(h.refresh))))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	data, hit, err := h.svc.Get(r.Context(), domain.Name(r.PathValue("name")))
	if errors.Is(err, domain.ErrUnknownCatalog) {
		httpx.Error(w, http.StatusNotFound, "not_found", err.Error())
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusServiceUnavailable, "unavailable", "catálogo no disponible")
		return
	}
	sum := sha256.Sum256(data)
	etag := `"` + hex.EncodeToString(sum[:8]) + `"`
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "private, max-age=300") // el navegador reutiliza (RF-30)
	if hit {
		w.Header().Set("X-Cache", "HIT")
	} else {
		w.Header().Set("X-Cache", "MISS")
	}
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write([]byte(`{"data":`))
	_, _ = w.Write(data)
	_, _ = w.Write([]byte(`}`))
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Refresh(r.Context(), domain.Name(r.PathValue("name"))); err != nil {
		httpx.Error(w, http.StatusBadRequest, "refresh_failed", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
