package tenant

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCanManageProfiles(t *testing.T) {
	with := Identity{Profile: Profile{Role: RoleAdmin, Permissions: []string{PermManageProfiles}}}
	without := Identity{Profile: Profile{Role: RoleAdmin}}
	jefe := Identity{Profile: Profile{Role: RoleJefeOps, Permissions: []string{PermManageProfiles}}}

	for _, s := range []ProfileScope{ProfileScopeAll, ProfileScopeClient} {
		if !with.CanManageProfiles(s) {
			t.Fatalf("con el permiso debe gestionar el alcance %q", s)
		}
		if without.CanManageProfiles(s) {
			t.Fatalf("ser Administrador no basta para el alcance %q: se valida por permiso", s)
		}
	}
	if with.CanManageProfiles("otro") {
		t.Fatal("un alcance desconocido no se autoriza")
	}
	// Se valida por permiso, no por perfil: otro perfil con el permiso también entra.
	if !jefe.CanViewClientAdmin() || !without.CanViewClientAdmin() {
		t.Fatal("Administración clientes: el Administrador en lectura y quien tenga el permiso")
	}
	if (Identity{Profile: Profile{Role: RoleJefeOps}}).CanViewClientAdmin() {
		t.Fatal("el Jefe sin el permiso no ve Administración clientes")
	}
}

func TestRequireManageProfiles(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	for _, c := range []struct {
		id   Identity
		want int
	}{
		{Identity{Profile: Profile{Role: RoleAdmin}}, http.StatusForbidden},
		{Identity{Profile: Profile{Role: RoleAdmin, Permissions: []string{PermDespacho}}}, http.StatusForbidden},
		{Identity{Profile: Profile{Role: RoleAdmin, Permissions: []string{PermManageProfiles}}}, http.StatusNoContent},
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/x", nil)
		RequireManageProfiles(ProfileScopeClient)(ok).ServeHTTP(rec, req.WithContext(With(req.Context(), c.id)))
		if rec.Code != c.want {
			t.Fatalf("permisos %v: código %d, quería %d", c.id.Permissions, rec.Code, c.want)
		}
	}
}
