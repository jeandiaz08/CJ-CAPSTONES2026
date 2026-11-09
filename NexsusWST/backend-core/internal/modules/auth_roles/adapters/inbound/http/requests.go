package http

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/nexuswst/backend-core/internal/modules/auth_roles/application"
	"github.com/nexuswst/backend-core/internal/modules/auth_roles/domain"
	"github.com/nexuswst/backend-core/internal/modules/auth_roles/ports"
	"github.com/nexuswst/backend-core/internal/shared/httpx"
	"github.com/nexuswst/backend-core/internal/shared/tenant"
)

// RequestHandler - solicitudes de perfiles (Tarea 07, Fase 4).
//   - Crear: el Cliente, para su empresa.
//   - Consultar: el Cliente (las de su empresa, filtradas en el servidor) y Administración clientes
//     (Administrador o quien tenga «Gestionar perfiles»).
//   - Aprobar y rechazar: solo con «Gestionar perfiles» (403 sin él).
type RequestHandler struct{ svc *application.Requests }

func NewRequestHandler(svc *application.Requests) *RequestHandler { return &RequestHandler{svc: svc} }

func (h *RequestHandler) Register(mux *http.ServeMux, auth func(http.Handler) http.Handler) {
	view := func(f http.HandlerFunc) http.Handler {
		return auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, _ := tenant.From(r.Context())
			if !id.HasRole(tenant.RoleCliente) && !id.CanViewClientAdmin() {
				httpx.Error(w, http.StatusForbidden, "forbidden", "Las solicitudes de perfiles son del Cliente y de Administración clientes.")
				return
			}
			f(w, r)
		}))
	}
	client := func(f http.HandlerFunc) http.Handler { return auth(tenant.RequireRole(tenant.RoleCliente)(f)) }
	manage := func(f http.HandlerFunc) http.Handler {
		return auth(tenant.RequireManageProfiles(tenant.ProfileScopeClient)(f))
	}
	mux.Handle("GET /v1/profile-requests", view(h.list))
	mux.Handle("GET /v1/profile-requests/{id}", view(h.get))
	mux.Handle("POST /v1/profile-requests", client(h.create))
	mux.Handle("POST /v1/profile-requests/{id}/approve", manage(h.approve))
	mux.Handle("POST /v1/profile-requests/{id}/reject", manage(h.reject))
}

func requestFilter(r *http.Request) (ports.RequestFilter, error) {
	q := r.URL.Query()
	f := ports.RequestFilter{Query: q.Get("q")}
	switch s := domain.RequestStatus(q.Get("status")); s {
	case "", domain.RequestPending, domain.RequestApproved, domain.RequestRejected:
		f.Status = s
	default:
		return f, errors.New("estado inválido: usa pending, approved o rejected")
	}
	switch k := domain.RequestKind(q.Get("kind")); k {
	case "", domain.RequestNewProfile, domain.RequestActivation, domain.RequestDeactivation:
		f.Kind = k
	default:
		return f, errors.New("tipo inválido: usa new_profile, activation o deactivation")
	}
	if v := q.Get("company_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 {
			return f, errors.New("empresa inválida")
		}
		f.CompanyIDs = []int64{id}
	}
	for _, p := range []struct {
		key string
		dst **time.Time
	}{{"from", &f.From}, {"to", &f.To}} {
		if v := q.Get(p.key); v != "" {
			t, err := time.Parse("2006-01-02", v)
			if err != nil {
				return f, fmt.Errorf("fecha inválida en %s: usa AAAA-MM-DD", p.key)
			}
			*p.dst = &t
		}
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

func (h *RequestHandler) list(w http.ResponseWriter, r *http.Request) {
	f, err := requestFilter(r)
	if err != nil {
		httpx.Error(w, http.StatusUnprocessableEntity, "validation_error", err.Error())
		return
	}
	actor, _ := tenant.From(r.Context())
	items, total, counts, err := h.svc.List(r.Context(), actor, f)
	if err != nil {
		writeRequestErr(w, err)
		return
	}
	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": items, "total": total, "counts": counts, "limit": limit, "offset": max(f.Offset, 0)})
}

func (h *RequestHandler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	actor, _ := tenant.From(r.Context())
	v, err := h.svc.Get(r.Context(), actor, id)
	if err != nil {
		writeRequestErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, v)
}

func (h *RequestHandler) create(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Kind         string `json:"kind"`
		TargetUserID int64  `json:"targetUserId"`
		Name         string `json:"name"`
		Email        string `json:"email"`
		Phone        string `json:"phone"`
		JobTitle     string `json:"jobTitle"`
		Reason       string `json:"reason"`
	}
	if err := httpx.Decode(r, &b); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "JSON inválido")
		return
	}
	actor, _ := tenant.From(r.Context())
	v, err := h.svc.Create(r.Context(), actor, domain.RequestDraft{Kind: domain.RequestKind(b.Kind), TargetUserID: b.TargetUserID, Name: b.Name,
		Email: b.Email, Phone: b.Phone, JobTitle: b.JobTitle, Reason: b.Reason})
	if err != nil {
		writeRequestErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, v)
}

// approve: sin cuerpo, acepta tal cual; con {name, email, phone, jobTitle} es «Editar y aceptar».
func (h *RequestHandler) approve(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var b *struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Phone    string `json:"phone"`
		JobTitle string `json:"jobTitle"`
	}
	if err := httpx.Decode(r, &b); err != nil && !errors.Is(err, io.EOF) {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "JSON inválido")
		return
	}
	var edits *domain.ProfileDraft
	if b != nil {
		edits = &domain.ProfileDraft{Name: b.Name, Email: b.Email, Phone: b.Phone, JobTitle: b.JobTitle}
	}
	actor, _ := tenant.From(r.Context())
	v, err := h.svc.Approve(r.Context(), actor, id, edits)
	if err != nil {
		writeRequestErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, v)
}

func (h *RequestHandler) reject(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
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
	v, err := h.svc.Reject(r.Context(), actor, id, b.Reason)
	if err != nil {
		writeRequestErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, v)
}

func writeRequestErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrRequestPending):
		httpx.Error(w, http.StatusConflict, "request_pending", "Ya hay una solicitud pendiente para ese perfil y tipo.")
	case errors.Is(err, domain.ErrRequestReviewed):
		httpx.Error(w, http.StatusConflict, "request_reviewed", "La solicitud ya fue respondida; recarga la página.")
	case errors.Is(err, application.ErrAlreadyInState):
		httpx.Error(w, http.StatusConflict, "same_state", "El perfil ya está en el estado que se pide: rechaza la solicitud.")
	case errors.Is(err, application.ErrNoCompany):
		httpx.Error(w, http.StatusForbidden, "forbidden", "Tu usuario no tiene una empresa activa.")
	case errors.Is(err, application.ErrEditOnlyNew):
		httpx.Error(w, http.StatusUnprocessableEntity, "validation_error", err.Error())
	case errors.Is(err, application.ErrRejectReason):
		httpx.JSON(w, http.StatusUnprocessableEntity, map[string]any{"error": "validation_error", "message": "Revisa los datos marcados",
			"fields": domain.ValidationErrors{{Field: "reason", Message: "Indica el motivo del rechazo (entre 5 y 500 caracteres)."}}})
	default:
		writeProfileErr(w, err)
	}
}
