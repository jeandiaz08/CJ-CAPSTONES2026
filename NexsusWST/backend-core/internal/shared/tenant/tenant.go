// Package tenant resuelve QUIÉN hace la petición y QUÉ empresas puede ver
// (RF-01/02/03, DAS v1.1 §4.5).
//
//  1. Verifica el token de Keycloak localmente (claves públicas en caché).
//  2. Lee el PERFIL de Nexus en PostgreSQL en cada petición: activo, rol,
//     permisos y empresas. Por eso desactivar un usuario lo bloquea en su
//     siguiente petición aunque su token siga vigente (sin lista en Redis).
//  3. La empresa NUNCA se toma libremente del navegador: ?company_id=N se
//     contrasta con las empresas del perfil.
package tenant

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/nexuswst/backend-core/internal/platform/auth"
	"github.com/nexuswst/backend-core/internal/shared/httpx"
)

type ctxKey struct{}

// Roles base (son roles de Keycloak). "despacho" es un PERMISO, no un rol.
const (
	RoleAdmin    = "admin"
	RoleJefeOps  = "jefe_operaciones"
	RoleAnalista = "analista"
	RoleEmpaque  = "empaque"
	RoleCliente  = "cliente"

	PermDespacho = "despacho"
)

var BaseRoles = []string{RoleAdmin, RoleJefeOps, RoleAnalista, RoleEmpaque, RoleCliente}

// Profile es el perfil de Nexus guardado por el Core (tabla users).
type Profile struct {
	UserID            int64    `json:"id"`
	KeycloakID        string   `json:"keycloakId"`
	Email             string   `json:"email"`
	Name              string   `json:"name"`
	Role              string   `json:"role"`
	Permissions       []string `json:"permissions"`
	Companies         []int64  `json:"companies"` // solo empresas ACTIVAS
	AllCompanies      bool     `json:"allCompanies"`
	Active            bool     `json:"active"`
	DisabledCompanies int      `json:"-"` // empresas asignadas pero deshabilitadas
}

// Identity = perfil + datos de la sesión de Keycloak.
type Identity struct {
	Profile
	SessionID      string `json:"-"`
	TokenExpiresAt int64  `json:"-"` // unix; el WebSocket se cierra al vencer
}

var (
	ErrForbiddenCompany = errors.New("empresa no autorizada")
	ErrProfileNotFound  = errors.New("perfil no encontrado")
)

// ProfileStore lee el perfil por el identificador de Keycloak.
type ProfileStore interface {
	ProfileByKeycloakID(ctx context.Context, keycloakID string) (Profile, error)
}

// Provisioner crea o vincula el perfil en el primer ingreso (solo el Core:
// es el dueño de la tabla users). En Órdenes es nil.
type Provisioner interface {
	Provision(ctx context.Context, c auth.Claims) (Profile, error)
}

type TokenVerifier interface {
	Verify(ctx context.Context, token string) (auth.Claims, error)
}

func With(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

func From(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(ctxKey{}).(Identity)
	return id, ok
}

// Scope devuelve las empresas que la petición puede ver.
// ids == nil y all == true significa "todas" (personal interno).
func (id Identity) Scope(r *http.Request) (ids []int64, all bool, err error) {
	if raw := r.URL.Query().Get("company_id"); raw != "" {
		cid, perr := strconv.ParseInt(raw, 10, 64)
		if perr != nil || !id.CanAccess(cid) {
			return nil, false, ErrForbiddenCompany
		}
		return []int64{cid}, false, nil
	}
	if id.AllCompanies {
		return nil, true, nil
	}
	if id.Companies == nil {
		return []int64{}, false, nil // nunca nil: nil significa "todas"
	}
	return id.Companies, false, nil
}

func (id Identity) CanAccess(companyID int64) bool {
	if id.AllCompanies {
		return true
	}
	for _, c := range id.Companies {
		if c == companyID {
			return true
		}
	}
	return false
}

func (id Identity) HasRole(roles ...string) bool {
	for _, r := range roles {
		if id.Role == r {
			return true
		}
	}
	return false
}

func (id Identity) HasPermission(p string) bool {
	for _, x := range id.Permissions {
		if x == p {
			return true
		}
	}
	return false
}

// Actor identifica a la persona en historiales y auditoría.
func (id Identity) Actor() string {
	return "user:" + strconv.FormatInt(id.UserID, 10) + " " + id.Name
}

// Middleware verifica el token y carga el perfil. allowQueryToken: solo para
// WebSocket (el navegador no puede enviar cabeceras al abrirlo).
func Middleware(v TokenVerifier, profiles ProfileStore, prov Provisioner, allowQueryToken bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tok := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if tok == "" && allowQueryToken {
				tok = r.URL.Query().Get("access_token")
			}
			if tok == "" {
				httpx.Error(w, http.StatusUnauthorized, "unauthorized", "falta el token de acceso")
				return
			}
			c, err := v.Verify(r.Context(), tok)
			if err != nil {
				code := "unauthorized"
				if errors.Is(err, auth.ErrExpiredToken) {
					code = "token_expired"
				}
				httpx.Error(w, http.StatusUnauthorized, code, err.Error())
				return
			}
			p, err := profiles.ProfileByKeycloakID(r.Context(), c.Subject)
			if errors.Is(err, ErrProfileNotFound) && prov != nil {
				p, err = prov.Provision(r.Context(), c)
			}
			switch {
			case errors.Is(err, ErrProfileNotFound):
				httpx.Error(w, http.StatusForbidden, "profile_missing", "tu usuario aún no tiene perfil en NexusWST: ingresa primero por la aplicación")
				return
			case err != nil:
				httpx.Error(w, http.StatusServiceUnavailable, "unavailable", "no se pudo verificar tu perfil")
				return
			case !p.Active:
				httpx.Error(w, http.StatusUnauthorized, "user_disabled", "tu usuario está deshabilitado")
				return
			case !p.AllCompanies && len(p.Companies) == 0 && p.DisabledCompanies > 0:
				httpx.Error(w, http.StatusForbidden, "company_disabled", "tu empresa está deshabilitada en NexusWST")
				return
			}
			next.ServeHTTP(w, r.WithContext(With(r.Context(), Identity{Profile: p, SessionID: c.SessionID, TokenExpiresAt: c.ExpiresAt})))
		})
	}
}

// RequireRole limita una ruta a ciertos roles (además del alcance por empresa).
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, ok := From(r.Context())
			if !ok || !id.HasRole(roles...) {
				httpx.Error(w, http.StatusForbidden, "forbidden", "tu rol no permite esta acción")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
