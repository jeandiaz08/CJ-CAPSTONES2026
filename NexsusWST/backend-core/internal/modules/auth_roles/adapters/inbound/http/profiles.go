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

	"github.com/nexuswst/backend-core/internal/modules/auth_roles/application"
	"github.com/nexuswst/backend-core/internal/modules/auth_roles/domain"
	"github.com/nexuswst/backend-core/internal/modules/auth_roles/ports"
	"github.com/nexuswst/backend-core/internal/shared/httpx"
	"github.com/nexuswst/backend-core/internal/shared/tenant"
)

// ProfileHandler - perfiles. ?scope=client (por defecto): Clientes › Administración clientes
// (Tarea 07); consultan el Administrador y quien gestione perfiles, y el Cliente solo los de su
// empresa (Perfiles empresa: el servidor fija la empresa). ?scope=internal: Configuración › Usuarios
// WeStorage (Tarea 08); consulta solo el Administrador. Crear, editar, activar, desactivar,
// restablecer la contraseña e importar exige «Gestionar perfiles» (403 sin él).
type ProfileHandler struct{ svc *application.Profiles }

func NewProfileHandler(svc *application.Profiles) *ProfileHandler { return &ProfileHandler{svc: svc} }

const maxImportBytes = 5 << 20

func (h *ProfileHandler) Register(mux *http.ServeMux, auth func(http.Handler) http.Handler) {
	view := func(f http.HandlerFunc) http.Handler {
		return auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, _ := tenant.From(r.Context())
			if scopeOf(r) == domain.ProfileScopeInternal {
				if !id.HasRole(tenant.RoleAdmin) {
					httpx.Error(w, http.StatusForbidden, "forbidden", "Usuarios WeStorage es solo para el Administrador.")
					return
				}
			} else if !id.CanViewClientAdmin() && !id.HasRole(tenant.RoleCliente) {
				httpx.Error(w, http.StatusForbidden, "forbidden", "Administración clientes es solo para el Administrador o quien tenga el permiso Gestionar perfiles.")
				return
			}
			f(w, r)
		}))
	}
	// Perfiles de WeStorage: el permiso debe cubrir todo (ProfileScopeAll) y ser Administrador.
	// TODO(futuro): KAM y SAC tendrán un permiso que cubra solo ProfileScopeClient.
	manage := func(f http.HandlerFunc) http.Handler {
		client, internal := tenant.RequireManageProfiles(tenant.ProfileScopeClient)(f), tenant.RequireManageProfiles(tenant.ProfileScopeAll)(f)
		return auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if scopeOf(r) != domain.ProfileScopeInternal {
				client.ServeHTTP(w, r)
				return
			}
			if id, _ := tenant.From(r.Context()); !id.HasRole(tenant.RoleAdmin) {
				httpx.Error(w, http.StatusForbidden, "forbidden", tenant.ErrNoManageProfiles)
				return
			}
			internal.ServeHTTP(w, r)
		}))
	}
	mux.Handle("GET /v1/profiles", view(h.list))
	mux.Handle("GET /v1/profiles/export", view(h.export))
	mux.Handle("GET /v1/profiles/template", manage(h.template))
	mux.Handle("POST /v1/profiles/import", manage(h.importFile))
	mux.Handle("GET /v1/profiles/{id}", view(h.get))
	mux.Handle("POST /v1/profiles", manage(h.create))
	mux.Handle("PATCH /v1/profiles/{id}", manage(h.update))
	mux.Handle("POST /v1/profiles/{id}/disable", manage(h.disable))
	mux.Handle("POST /v1/profiles/{id}/enable", manage(h.enable))
	mux.Handle("POST /v1/profiles/{id}/password", manage(h.password))
}

func scopeOf(r *http.Request) domain.ProfileScope {
	if s := r.URL.Query().Get("scope"); s != "" {
		return domain.ProfileScope(s)
	}
	return domain.ProfileScopeClient
}

// clientCompanyOf: el Cliente solo consulta su empresa (0 = no es Cliente). Sin empresa activa, -1
// (no ve nada).
func clientCompanyOf(r *http.Request) int64 {
	id, _ := tenant.From(r.Context())
	if !id.HasRole(tenant.RoleCliente) {
		return 0
	}
	if len(id.Companies) == 0 {
		return -1
	}
	return id.Companies[0]
}

func profileFilter(r *http.Request) (ports.ProfileFilter, error) {
	q := r.URL.Query()
	f := ports.ProfileFilter{Scope: scopeOf(r), Query: q.Get("q")}

	if v := q.Get("company_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 {
			return f, errors.New("empresa inválida")
		}
		f.CompanyID = id
	}
	switch q.Get("status") {
	case "":
	case "active", "inactive":
		v := q.Get("status") == "active"
		f.Active = &v
	default:
		return f, errors.New("estado inválido: usa active o inactive")
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
	if c := clientCompanyOf(r); c != 0 {
		f.Scope, f.CompanyID = domain.ProfileScopeClient, c // la empresa del Cliente manda sobre ?company_id
	}
	return f, nil
}

func (h *ProfileHandler) list(w http.ResponseWriter, r *http.Request) {
	f, err := profileFilter(r)
	if err != nil {
		httpx.Error(w, http.StatusUnprocessableEntity, "validation_error", err.Error())
		return
	}
	items, total, err := h.svc.List(r.Context(), f)
	if err != nil {
		writeProfileErr(w, err)
		return
	}
	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": items, "total": total, "limit": limit, "offset": max(f.Offset, 0)})
}

func (h *ProfileHandler) export(w http.ResponseWriter, r *http.Request) {
	f, err := profileFilter(r)
	if err != nil {
		httpx.Error(w, http.StatusUnprocessableEntity, "validation_error", err.Error())
		return
	}
	var buf bytes.Buffer
	if err := h.svc.Export(r.Context(), f, &buf); err != nil {
		writeProfileErr(w, err)
		return
	}
	sendXLSX(w, fmt.Sprintf("perfiles-%s.xlsx", time.Now().Format("20060102-1504")), buf.Bytes())
}

func (h *ProfileHandler) template(w http.ResponseWriter, r *http.Request) {
	var buf bytes.Buffer
	if err := h.svc.Template(r.Context(), scopeOf(r), &buf); err != nil {
		writeProfileErr(w, err)
		return
	}
	sendXLSX(w, "plantilla-perfiles.xlsx", buf.Bytes())
}

func sendXLSX(w http.ResponseWriter, name string, data []byte) {
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
	_, _ = w.Write(data)
}

func (h *ProfileHandler) importFile(w http.ResponseWriter, r *http.Request) {
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
	res, err := h.svc.Import(r.Context(), actor, scopeOf(r), data)
	if err != nil {
		writeProfileErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

func (h *ProfileHandler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	v, err := h.svc.Get(r.Context(), scopeOf(r), id)
	if err != nil {
		writeProfileErr(w, err)
		return
	}
	if c := clientCompanyOf(r); c != 0 && v.CompanyID != c {
		writeProfileErr(w, domain.ErrNotFound) // un perfil de otra empresa no existe para el Cliente
		return
	}
	httpx.JSON(w, http.StatusOK, v)
}

// profileBody: username, role y permissions solo en el alcance interno (en clientes, 400).
type profileBody struct {
	Name        string   `json:"name"`
	CompanyID   int64    `json:"companyId"`
	Email       string   `json:"email"`
	Username    string   `json:"username"`
	Phone       string   `json:"phone"`
	JobTitle    string   `json:"jobTitle"`
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
}

func (b profileBody) draft() domain.ProfileDraft {
	return domain.ProfileDraft{Name: b.Name, CompanyID: b.CompanyID, Email: b.Email, Username: b.Username, Phone: b.Phone,
		JobTitle: b.JobTitle, Role: b.Role, Permissions: b.Permissions}
}

// decodeProfile lee el cuerpo y rechaza los campos internos en el alcance de clientes.
func decodeProfile(w http.ResponseWriter, r *http.Request, msg string) (profileBody, bool) {
	var b profileBody
	err := httpx.Decode(r, &b)
	if err == nil && scopeOf(r) != domain.ProfileScopeInternal && (b.Username != "" || b.Role != "" || b.Permissions != nil) {
		err = errors.New("campos internos en un perfil Cliente")
	}
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", msg)
		return b, false
	}
	return b, true
}

func (h *ProfileHandler) create(w http.ResponseWriter, r *http.Request) {
	b, ok := decodeProfile(w, r, "JSON inválido")
	if !ok {
		return
	}
	actor, _ := tenant.From(r.Context())
	v, err := h.svc.Create(r.Context(), actor, scopeOf(r), b.draft())
	if err != nil {
		writeProfileErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, v)
}

func (h *ProfileHandler) update(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	b, ok := decodeProfile(w, r, "JSON inválido (el correo y el usuario no se editan)")
	if !ok {
		return
	}
	if b.Email != "" || b.Username != "" {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "JSON inválido (el correo y el usuario no se editan)")
		return
	}
	actor, _ := tenant.From(r.Context())
	v, err := h.svc.Update(r.Context(), actor, scopeOf(r), id, b.draft())
	if err != nil {
		writeProfileErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, v)
}

func (h *ProfileHandler) disable(w http.ResponseWriter, r *http.Request) { h.setActive(w, r, false) }
func (h *ProfileHandler) enable(w http.ResponseWriter, r *http.Request)  { h.setActive(w, r, true) }

func (h *ProfileHandler) setActive(w http.ResponseWriter, r *http.Request, active bool) {
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
	v, err := h.svc.SetActive(r.Context(), actor, scopeOf(r), id, active, b.Reason)
	if err != nil {
		writeProfileErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, v)
}

func (h *ProfileHandler) password(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	actor, _ := tenant.From(r.Context())
	temp, err := h.svc.ResetPassword(r.Context(), actor, scopeOf(r), id)
	if err != nil {
		writeProfileErr(w, err)
		return
	}
	if temp != "" { // perfil interno sin correo: la contraseña temporal se muestra una sola vez
		w.Header().Set("Cache-Control", "no-store")
		httpx.JSON(w, http.StatusOK, map[string]any{"emailSent": false, "temporaryPassword": temp})
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"emailSent": true})
}

func writeProfileErr(w http.ResponseWriter, err error) {
	var fields domain.ValidationErrors
	var ve application.ValidationError
	switch {
	case application.IsConflict(err) && errors.As(err, &fields):
		httpx.JSON(w, http.StatusConflict, map[string]any{"error": "conflict", "message": fields.Error(), "fields": fields})
	case errors.As(err, &fields):
		httpx.JSON(w, http.StatusUnprocessableEntity, map[string]any{"error": "validation_error", "message": "Revisa los datos marcados", "fields": fields})
	case errors.Is(err, application.ErrReasonRequired):
		httpx.JSON(w, http.StatusUnprocessableEntity, map[string]any{"error": "validation_error", "message": "Revisa los datos marcados",
			"fields": domain.ValidationErrors{{Field: "reason", Message: "Indica el motivo (entre 5 y 500 caracteres)."}}})
	case errors.Is(err, application.ErrScopeUnavailable), errors.Is(err, application.ErrExportTooBig):
		httpx.Error(w, http.StatusUnprocessableEntity, "validation_error", err.Error())
	case errors.Is(err, application.ErrLastManager):
		httpx.Error(w, http.StatusConflict, "last_manager", "Debe quedar al menos un Administrador activo con «Gestionar perfiles». Asígnale el permiso a otro Administrador antes de este cambio.")
	case errors.Is(err, application.ErrSameState):
		httpx.Error(w, http.StatusConflict, "same_state", "El perfil ya está en ese estado; recarga la página.")
	case errors.As(err, &ve):
		httpx.Error(w, http.StatusUnprocessableEntity, "validation_error", ve.Msg)
	default:
		writeErr(w, err) // los mismos códigos que /v1/users (404, 403 propio, 503 Keycloak, 500)
	}
}
