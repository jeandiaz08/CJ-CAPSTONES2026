// Package http - rutas de sesión (perfil) y administración de usuarios.
//
// No hay login/refresh/logout aquí: los atiende Keycloak (OIDC con PKCE
// desde la interfaz). El Core solo expone el perfil y la gestión de usuarios.
package http

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/nexuswst/backend-core/internal/modules/auth_roles/application"
	"github.com/nexuswst/backend-core/internal/modules/auth_roles/domain"
	"github.com/nexuswst/backend-core/internal/modules/auth_roles/ports"
	"github.com/nexuswst/backend-core/internal/shared/httpx"
	"github.com/nexuswst/backend-core/internal/shared/tenant"
)

type Handler struct{ svc *application.Service }

func NewHandler(svc *application.Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(mux *http.ServeMux, authMW func(http.Handler) http.Handler) {
	mux.Handle("GET /v1/auth/me", authMW(http.HandlerFunc(h.me)))

	// Facultad administrativa: solo el perfil Administrador (RF-02).
	admin := func(f http.HandlerFunc) http.Handler {
		return authMW(tenant.RequireRole(tenant.RoleAdmin)(f))
	}
	mux.Handle("GET /v1/users", admin(h.list))
	mux.Handle("POST /v1/users", admin(h.create))
	mux.Handle("GET /v1/users/{id}", admin(h.get))
	mux.Handle("PATCH /v1/users/{id}", admin(h.update))
	mux.Handle("POST /v1/users/{id}/disable", admin(h.disable))
	mux.Handle("POST /v1/users/{id}/enable", admin(h.enable))
	mux.Handle("POST /v1/users/{id}/password-email", admin(h.passwordEmail))
	mux.Handle("GET /v1/users/{id}/history", admin(h.history))
	mux.Handle("GET /v1/roles", authMW(http.HandlerFunc(h.roles)))
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	id, _ := tenant.From(r.Context())
	me, err := h.svc.Me(r.Context(), id)
	if err != nil {
		writeErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, me)
}

func (h *Handler) roles(w http.ResponseWriter, _ *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]any{"roles": domain.Roles, "permissions": domain.Permissions})
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := ports.ListFilter{Query: q.Get("q"), Role: q.Get("role")}
	if v := q.Get("active"); v != "" {
		b := v == "true"
		f.Active = &b
	}
	f.Limit, _ = strconv.Atoi(q.Get("limit"))
	f.Offset, _ = strconv.Atoi(q.Get("offset"))
	items, total, err := h.svc.List(r.Context(), f)
	if err != nil {
		writeErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	u, err := h.svc.Get(r.Context(), id)
	if err != nil {
		writeErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, u)
}

type userBody struct {
	Email        string   `json:"email"`
	Name         string   `json:"name"`
	Role         string   `json:"role"`
	Permissions  []string `json:"permissions"`
	AllCompanies bool     `json:"allCompanies"`
	Companies    []int64  `json:"companies"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var b userBody
	if err := httpx.Decode(r, &b); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "JSON inválido")
		return
	}
	actor, _ := tenant.From(r.Context())
	u, err := h.svc.Create(r.Context(), actor, domain.Draft{Email: b.Email, Name: b.Name, Role: b.Role,
		Permissions: b.Permissions, AllCompanies: b.AllCompanies, Companies: b.Companies})
	if err != nil {
		writeErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, u)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var b struct {
		Name         *string   `json:"name"`
		Role         *string   `json:"role"`
		Permissions  *[]string `json:"permissions"`
		AllCompanies *bool     `json:"allCompanies"`
		Companies    *[]int64  `json:"companies"`
	}
	if err := httpx.Decode(r, &b); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "JSON inválido")
		return
	}
	actor, _ := tenant.From(r.Context())
	u, err := h.svc.Update(r.Context(), actor, id, domain.Changes{Name: b.Name, Role: b.Role,
		Permissions: b.Permissions, AllCompanies: b.AllCompanies, Companies: b.Companies})
	if err != nil {
		writeErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, u)
}

func (h *Handler) disable(w http.ResponseWriter, r *http.Request) { h.setActive(w, r, false) }
func (h *Handler) enable(w http.ResponseWriter, r *http.Request)  { h.setActive(w, r, true) }

func (h *Handler) setActive(w http.ResponseWriter, r *http.Request, active bool) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var b struct {
		Reason string `json:"reason"`
	}
	_ = httpx.Decode(r, &b) // el motivo es opcional
	actor, _ := tenant.From(r.Context())
	var u application.UserView
	var err error
	if active {
		u, err = h.svc.Enable(r.Context(), actor, id, b.Reason)
	} else {
		u, err = h.svc.Disable(r.Context(), actor, id, b.Reason)
	}
	if err != nil {
		writeErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, u)
}

func (h *Handler) passwordEmail(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	actor, _ := tenant.From(r.Context())
	if err := h.svc.ResendSetPassword(r.Context(), actor, id); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) history(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	items, err := h.svc.History(r.Context(), id)
	if err != nil {
		writeErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "id inválido")
		return 0, false
	}
	return id, true
}

func writeErr(w http.ResponseWriter, err error) {
	var ve application.ValidationError
	switch {
	case errors.As(err, &ve):
		httpx.Error(w, http.StatusUnprocessableEntity, "validation_error", ve.Msg)
	case errors.Is(err, domain.ErrInvalid):
		httpx.Error(w, http.StatusUnprocessableEntity, "validation_error", err.Error())
	case errors.Is(err, domain.ErrNotFound):
		httpx.Error(w, http.StatusNotFound, "not_found", domain.ErrNotFound.Error())
	case errors.Is(err, domain.ErrEmailTaken):
		httpx.Error(w, http.StatusConflict, "email_taken", err.Error())
	case errors.Is(err, domain.ErrSelfChange):
		httpx.Error(w, http.StatusForbidden, "forbidden", err.Error())
	case errors.Is(err, domain.ErrIdPUnavailable):
		httpx.Error(w, http.StatusServiceUnavailable, "idp_unavailable", domain.ErrIdPUnavailable.Error())
	default:
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "error inesperado")
	}
}
