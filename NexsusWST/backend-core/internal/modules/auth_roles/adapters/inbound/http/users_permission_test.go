package http

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nexuswst/backend-core/internal/modules/auth_roles/application"
	"github.com/nexuswst/backend-core/internal/modules/auth_roles/domain"
	"github.com/nexuswst/backend-core/internal/modules/auth_roles/ports"
	"github.com/nexuswst/backend-core/internal/shared/tenant"
)

// usersFake: ningún usuario existe; la lista viene vacía. Basta para saber si la
// petición pasó el guardia de la ruta (403) o llegó al caso de uso (otro código).
type usersFake struct{ ports.UserRepository }

func (usersFake) GetByID(context.Context, int64) (domain.User, error) {
	return domain.User{}, domain.ErrNotFound
}

func (usersFake) List(context.Context, ports.ListFilter) ([]domain.User, int, error) {
	return nil, 0, nil
}

// Tarea 07: las escrituras de /v1/users exigen el permiso «Gestionar perfiles»,
// no el perfil Administrador. Sin el permiso, el Administrador solo consulta.
func TestUsersWritesRequireManageProfiles(t *testing.T) {
	svc := application.NewService(usersFake{}, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	h := NewHandler(svc)

	adminRead := tenant.Profile{UserID: 1, Role: tenant.RoleAdmin, AllCompanies: true}
	adminManage := tenant.Profile{UserID: 1, Role: tenant.RoleAdmin, AllCompanies: true, Permissions: []string{tenant.PermManageProfiles}}
	jefeDespacho := tenant.Profile{UserID: 2, Role: tenant.RoleJefeOps, AllCompanies: true, Permissions: []string{tenant.PermDespacho}}
	cliente := tenant.Profile{UserID: 5, Role: tenant.RoleCliente, Companies: []int64{1}}

	writes := []struct{ method, path, body string }{
		{"POST", "/v1/users", `{}`},
		{"PATCH", "/v1/users/9", `{"name":"Otro"}`},
		{"POST", "/v1/users/9/disable", `{"reason":"prueba"}`},
		{"POST", "/v1/users/9/enable", `{}`},
		{"POST", "/v1/users/9/password-email", ``},
		{"POST", "/v1/users/9/password-reset", ``},
	}
	reads := []struct{ method, path string }{
		{"GET", "/v1/users"},
		{"GET", "/v1/users/9"},
		{"GET", "/v1/users/9/history"},
	}

	do := func(p tenant.Profile, method, path, body string) *httptest.ResponseRecorder {
		mux := http.NewServeMux()
		h.Register(mux, func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				next.ServeHTTP(w, r.WithContext(tenant.With(r.Context(), tenant.Identity{Profile: p})))
			})
		})
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
		return rec
	}

	for _, w := range writes {
		t.Run("sin permiso "+w.method+" "+w.path, func(t *testing.T) {
			for name, p := range map[string]tenant.Profile{"admin": adminRead, "jefe con despacho": jefeDespacho, "cliente": cliente} {
				rec := do(p, w.method, w.path, w.body)
				if rec.Code != http.StatusForbidden {
					t.Fatalf("%s: código %d, quería 403", name, rec.Code)
				}
				if name == "admin" && !strings.Contains(rec.Body.String(), "Gestionar perfiles") {
					t.Fatalf("el 403 debe nombrar el permiso: %s", rec.Body.String())
				}
			}
		})
		t.Run("con permiso "+w.method+" "+w.path, func(t *testing.T) {
			// Pasa el guardia: el caso de uso responde 422 (datos) o 404 (no existe).
			if rec := do(adminManage, w.method, w.path, w.body); rec.Code == http.StatusForbidden {
				t.Fatalf("con el permiso no debe responder 403: %s", rec.Body.String())
			}
		})
	}
	for _, r := range reads {
		t.Run("lectura "+r.method+" "+r.path, func(t *testing.T) {
			if rec := do(adminRead, r.method, r.path, ""); rec.Code == http.StatusForbidden {
				t.Fatalf("el Administrador sin el permiso debe poder consultar: %d", rec.Code)
			}
			if rec := do(cliente, r.method, r.path, ""); rec.Code != http.StatusForbidden {
				t.Fatalf("el Cliente no consulta usuarios: %d", rec.Code)
			}
		})
	}
}
