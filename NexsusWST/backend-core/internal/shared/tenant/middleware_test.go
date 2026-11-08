package tenant

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nexuswst/backend-core/internal/platform/auth"
)

// H-01-T08 (SCRUM-85): lo que ve quien llama a la API según su sesión. La
// verificación criptográfica del token (firma, emisor, vencimiento) está en
// platform/auth/jwt_test.go; aquí, qué responde el servidor en cada caso.

type fakeVerifier map[string]error // token -> error (nil = válido; sub = token)

func (f fakeVerifier) Verify(_ context.Context, tok string) (auth.Claims, error) {
	err, ok := f[tok]
	if !ok {
		return auth.Claims{}, errors.New("token inválido")
	}
	return auth.Claims{Subject: tok}, err
}

type fakeProfiles map[string]Profile

func (f fakeProfiles) ProfileByKeycloakID(_ context.Context, id string) (Profile, error) {
	p, ok := f[id]
	if !ok {
		return Profile{}, ErrProfileNotFound
	}
	return p, nil
}

type fakeProvisioner struct{ calls int }

func (f *fakeProvisioner) Provision(_ context.Context, c auth.Claims) (Profile, error) {
	f.calls++
	if c.Subject == "nuevo" {
		return Profile{UserID: 99, Role: RoleCliente, Active: true, Companies: []int64{1}}, nil
	}
	return Profile{}, ErrProfileNotFound
}

func TestMiddlewareSessionCases(t *testing.T) {
	ver := fakeVerifier{
		"ok": nil, "deshabilitado": nil, "sin-perfil": nil, "nuevo": nil, "empresa-baja": nil,
		"vencido": auth.ErrExpiredToken,
	}
	profiles := fakeProfiles{
		"ok":            {UserID: 1, Role: RoleJefeOps, Active: true, AllCompanies: true},
		"deshabilitado": {UserID: 2, Role: RoleEmpaque, Active: false, AllCompanies: true},
		"empresa-baja":  {UserID: 3, Role: RoleCliente, Active: true, DisabledCompanies: 1},
	}
	prov := &fakeProvisioner{}
	var seen Identity
	h := Middleware(ver, profiles, prov, false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen, _ = From(r.Context())
		w.WriteHeader(http.StatusNoContent)
	}))

	cases := []struct {
		name, header string
		status       int
		code         string
	}{
		{"sesión válida", "Bearer ok", 204, ""},
		{"sin token", "", 401, "unauthorized"},
		{"token manipulado o de otro sistema", "Bearer basura", 401, "unauthorized"},
		{"token vencido", "Bearer vencido", 401, "token_expired"},
		{"usuario deshabilitado (aunque su token siga vigente)", "Bearer deshabilitado", 401, "user_disabled"},
		{"sin perfil en Nexus", "Bearer sin-perfil", 403, "profile_missing"},
		{"primer ingreso: se crea el perfil", "Bearer nuevo", 204, ""},
		{"empresa deshabilitada", "Bearer empresa-baja", 403, "company_disabled"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
			if c.header != "" {
				req.Header.Set("Authorization", c.header)
			}
			seen = Identity{}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != c.status {
				t.Fatalf("estado %d, quería %d (%s)", rec.Code, c.status, rec.Body.String())
			}
			if c.status == 204 && seen.UserID == 0 {
				t.Fatal("la identidad no llegó al handler")
			}
			if c.code != "" {
				var body map[string]string
				_ = json.Unmarshal(rec.Body.Bytes(), &body)
				if body["error"] != c.code || body["message"] == "" {
					t.Fatalf("error %q, quería %q: %v", body["error"], c.code, body)
				}
			}
		})
	}
	if prov.calls != 2 { // sin-perfil y nuevo
		t.Fatalf("se intentó crear perfil %d veces", prov.calls)
	}
}

func TestQueryTokenOnlyForWebSocket(t *testing.T) {
	ver := fakeVerifier{"ok": nil}
	profiles := fakeProfiles{"ok": {UserID: 1, Active: true, AllCompanies: true}}
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) })
	for allow, want := range map[bool]int{true: 204, false: 401} {
		rec := httptest.NewRecorder()
		Middleware(ver, profiles, nil, allow)(ok).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ws?access_token=ok", nil))
		if rec.Code != want {
			t.Fatalf("allowQueryToken=%v: %d, quería %d", allow, rec.Code, want)
		}
	}
}
