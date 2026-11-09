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

	"github.com/nexuswst/backend-core/internal/modules/clients/application"
	"github.com/nexuswst/backend-core/internal/modules/clients/domain"
	"github.com/nexuswst/backend-core/internal/modules/clients/ports"
	"github.com/nexuswst/backend-core/internal/shared/httpx"
	"github.com/nexuswst/backend-core/internal/shared/tenant"
)

// Clientes › Administración clientes › Empresas (Tarea 07). Consultan el Administrador y quien
// gestione perfiles; crear, editar, activar y desactivar exige «Gestionar perfiles» (403 sin él).
func (h *Handler) registerAdmin(mux *http.ServeMux, auth func(http.Handler) http.Handler) {
	view := func(f http.HandlerFunc) http.Handler { return auth(requireClientAdminView(f)) }
	manage := func(f http.HandlerFunc) http.Handler {
		return auth(tenant.RequireManageProfiles(tenant.ProfileScopeClient)(f))
	}
	mux.Handle("GET /v1/companies", view(h.adminList))
	mux.Handle("GET /v1/companies/export", view(h.adminExport))
	mux.Handle("GET /v1/companies/{id}", view(h.adminGet))
	mux.Handle("POST /v1/companies", manage(h.adminCreate))
	mux.Handle("PATCH /v1/companies/{id}", manage(h.adminUpdate))
	mux.Handle("POST /v1/companies/{id}/disable", manage(h.adminDisable))
	mux.Handle("POST /v1/companies/{id}/enable", manage(h.adminEnable))
}

func requireClientAdminView(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := tenant.From(r.Context())
		if !ok || !id.CanViewClientAdmin() {
			httpx.Error(w, http.StatusForbidden, "forbidden", "Administración clientes es solo para el Administrador o quien tenga el permiso Gestionar perfiles.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// El código Invas solo lo ve y lo cambia el Administrador.
func isAdmin(r *http.Request) bool {
	id, _ := tenant.From(r.Context())
	return id.HasRole(tenant.RoleAdmin)
}

func companyJSON(c domain.Company, withWMS bool) map[string]any {
	out := map[string]any{
		"id": c.ID, "name": c.Name, "taxId": c.TaxID, "email": c.Email, "phone": c.Phone, "waveCode": c.WaveCode,
		"profilesCount": c.ProfilesCount, "active": c.Active, "createdAt": c.CreatedAt, "updatedAt": c.UpdatedAt,
	}
	if withWMS {
		out["wmsCode"] = c.WMSCode
	}
	return out
}

func parseFilter(r *http.Request) (ports.CompanyFilter, error) {
	q := r.URL.Query()
	f := ports.CompanyFilter{Query: q.Get("q"), Wave: strings.TrimSpace(q.Get("wave"))}
	switch q.Get("status") {
	case "":
	case "active", "inactive":
		v := q.Get("status") == "active"
		f.Active = &v
	default:
		return f, errors.New("estado inválido: usa active o inactive")
	}
	if f.Wave != "" && f.Wave != "general" {
		f.Wave = strings.ToUpper(f.Wave)
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

func (h *Handler) adminList(w http.ResponseWriter, r *http.Request) {
	f, err := parseFilter(r)
	if err != nil {
		httpx.Error(w, http.StatusUnprocessableEntity, "validation_error", err.Error())
		return
	}
	items, total, err := h.svc.AdminList(r.Context(), f)
	if err != nil {
		httpx.Internal(w, err, "No se pudieron listar las empresas")
		return
	}
	withWMS := isAdmin(r)
	out := make([]map[string]any, 0, len(items))
	for _, c := range items {
		out = append(out, companyJSON(c, withWMS))
	}
	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": out, "total": total, "limit": limit, "offset": max(f.Offset, 0)})
}

func (h *Handler) adminExport(w http.ResponseWriter, r *http.Request) {
	f, err := parseFilter(r)
	if err != nil {
		httpx.Error(w, http.StatusUnprocessableEntity, "validation_error", err.Error())
		return
	}
	var buf bytes.Buffer
	if err := h.svc.Export(r.Context(), f, isAdmin(r), &buf); err != nil {
		if errors.Is(err, application.ErrExportTooBig) {
			httpx.Error(w, http.StatusUnprocessableEntity, "too_many_rows", err.Error())
			return
		}
		httpx.Internal(w, err, "No se pudo generar el Excel")
		return
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="empresas-%s.xlsx"`, time.Now().Format("20060102-1504")))
	_, _ = w.Write(buf.Bytes())
}

func (h *Handler) adminGet(w http.ResponseWriter, r *http.Request) {
	id, ok := companyID(w, r)
	if !ok {
		return
	}
	c, err := h.svc.Get(r.Context(), id)
	if err != nil {
		writeAdminErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, companyJSON(c, isAdmin(r)))
}

type companyBody struct {
	Name     string  `json:"name"`
	TaxID    string  `json:"taxId"`
	Email    string  `json:"email"`
	Phone    string  `json:"phone"`
	WaveCode *string `json:"waveCode"` // null = «General (según reglas)»
	WMSCode  string  `json:"wmsCode"`
}

func (b companyBody) draft() domain.Draft {
	return domain.Draft{Name: b.Name, TaxID: b.TaxID, Email: b.Email, Phone: b.Phone, WaveCode: b.WaveCode, WMSCode: b.WMSCode}
}

func (h *Handler) adminCreate(w http.ResponseWriter, r *http.Request) {
	var b companyBody
	if err := httpx.Decode(r, &b); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "JSON inválido")
		return
	}
	actor, _ := tenant.From(r.Context())
	c, err := h.svc.Create(r.Context(), actor, b.draft(), isAdmin(r))
	if err != nil {
		writeAdminErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, companyJSON(c, isAdmin(r)))
}

func (h *Handler) adminUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := companyID(w, r)
	if !ok {
		return
	}
	var b companyBody
	if err := httpx.Decode(r, &b); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "JSON inválido")
		return
	}
	actor, _ := tenant.From(r.Context())
	c, err := h.svc.Update(r.Context(), actor, id, b.draft(), isAdmin(r))
	if err != nil {
		writeAdminErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, companyJSON(c, isAdmin(r)))
}

func (h *Handler) adminDisable(w http.ResponseWriter, r *http.Request) { h.adminSetActive(w, r, false) }
func (h *Handler) adminEnable(w http.ResponseWriter, r *http.Request)  { h.adminSetActive(w, r, true) }

func (h *Handler) adminSetActive(w http.ResponseWriter, r *http.Request, active bool) {
	id, ok := companyID(w, r)
	if !ok {
		return
	}
	var b struct {
		Reason string `json:"reason"`
	}
	if err := httpx.Decode(r, &b); err != nil && !errors.Is(err, io.EOF) { // el cuerpo es opcional al activar
		httpx.Error(w, http.StatusBadRequest, "bad_request", "JSON inválido")
		return
	}
	actor, _ := tenant.From(r.Context())
	var c domain.Company
	var err error
	if active {
		c, err = h.svc.Enable(r.Context(), actor, id, b.Reason)
	} else {
		c, err = h.svc.Disable(r.Context(), actor, id, b.Reason)
	}
	if err != nil {
		writeAdminErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, companyJSON(c, isAdmin(r)))
}

func companyID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "id inválido")
		return 0, false
	}
	return id, true
}

func writeAdminErr(w http.ResponseWriter, err error) {
	var ve domain.ValidationErrors
	switch {
	case errors.As(err, &ve):
		httpx.JSON(w, http.StatusUnprocessableEntity, map[string]any{"error": "validation_error", "message": "Revisa los datos marcados", "fields": ve})
	case errors.Is(err, domain.ErrTaxIDTaken):
		httpx.JSON(w, http.StatusConflict, map[string]any{"error": "tax_id_taken", "message": "Ya existe una empresa con ese RUT.",
			"fields": domain.ValidationErrors{{Field: "taxId", Message: "Ya existe una empresa con ese RUT."}}})
	case errors.Is(err, domain.ErrReasonShort):
		httpx.JSON(w, http.StatusUnprocessableEntity, map[string]any{"error": "validation_error", "message": "Revisa los datos marcados",
			"fields": domain.ValidationErrors{{Field: "reason", Message: "Indica el motivo (entre 5 y 500 caracteres)."}}})
	case errors.Is(err, domain.ErrSameState):
		httpx.Error(w, http.StatusConflict, "same_state", "La empresa ya está en ese estado; recarga la página.")
	case errors.Is(err, domain.ErrNotFound):
		httpx.Error(w, http.StatusNotFound, "not_found", err.Error())
	default:
		httpx.Internal(w, err, "No se pudo guardar la empresa")
	}
}
