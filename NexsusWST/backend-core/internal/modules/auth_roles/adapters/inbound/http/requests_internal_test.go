package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/nexuswst/backend-core/internal/modules/auth_roles/application"
	"github.com/nexuswst/backend-core/internal/modules/auth_roles/domain"
	"github.com/nexuswst/backend-core/internal/modules/auth_roles/ports"
	"github.com/nexuswst/backend-core/internal/shared/tenant"
)

// Solicitudes de perfiles de WeStorage (Tarea 08, Fase 2): /v1/profile-requests?scope=internal.

func internalRequestsFixture(t *testing.T) (*memUsers, *memRequests, *recNotifier, *recMailer, *RequestHandler) {
	t.Helper()
	m, reqs, n, mail, h := newRequestsFixture()
	m.users[1] = &ports.Profile{User: domain.User{ID: 1, KeycloakID: "kc-1", Email: "admin@westorage.cl", Name: "Admin", Role: "admin",
		Permissions: []string{tenant.PermManageProfiles}, AllCompanies: true, Active: true}}
	m.users[2] = &ports.Profile{User: domain.User{ID: 2, KeycloakID: "kc-2", Email: "jefe@westorage.cl", Name: "María Rojas", Role: "jefe_operaciones",
		Permissions: []string{"despacho"}, AllCompanies: true, Active: true}}
	m.users[8] = &ports.Profile{User: domain.User{ID: 8, KeycloakID: "kc-8", Email: "lectura@westorage.cl", Name: "Admin Lectura", Role: "admin",
		AllCompanies: true, Active: true}}
	return m, reqs, n, mail, h
}

func TestInternalRequestsAccess(t *testing.T) {
	_, _, _, _, h := internalRequestsFixture(t)
	body := `{"kind":"new_profile","name":"Pedro Bodega","username":"pedro.bodega","role":"empaque","permissions":["despacho"],"reason":"Nuevo operario"}`
	// Solicita el Administrador (con o sin el permiso); el Cliente y el Jefe, no.
	for name, p := range map[string]tenant.Profile{"cliente": client, "jefe": jefeOps} {
		if rec := serveRequests(h, p, "POST", "/v1/profile-requests?scope=internal", body); rec.Code != http.StatusForbidden {
			t.Errorf("%s solicita interna: %d", name, rec.Code)
		}
		if rec := serveRequests(h, p, "GET", "/v1/profile-requests?scope=internal", ""); rec.Code != http.StatusForbidden {
			t.Errorf("%s consulta internas: %d", name, rec.Code)
		}
	}
	// Rol, permisos y usuario no se aceptan en una solicitud de cliente.
	if rec := serveRequests(h, client, "POST", "/v1/profile-requests", `{"kind":"new_profile","name":"X","email":"x@andes.cl","role":"admin","reason":"Nuevo perfil"}`); rec.Code != http.StatusBadRequest {
		t.Errorf("rol en una solicitud de cliente: %d", rec.Code)
	}
	// Validación con las reglas de «Nuevo perfil» de WeStorage.
	rec := serveRequests(h, reader, "POST", "/v1/profile-requests?scope=internal", `{"kind":"new_profile","name":"X","role":"empaque","permissions":["gestionar_perfiles"],"reason":"Nuevo operario"}`)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `"field":"email"`) || !strings.Contains(rec.Body.String(), `"field":"permissions"`) {
		t.Fatalf("validación interna: %d %s", rec.Code, rec.Body.String())
	}
	// Nadie solicita su propia desactivación.
	if rec := serveRequests(h, reader, "POST", "/v1/profile-requests?scope=internal", `{"kind":"deactivation","targetUserId":8,"reason":"Me voy de la empresa"}`); rec.Code != http.StatusForbidden {
		t.Fatalf("desactivación propia: %d %s", rec.Code, rec.Body.String())
	}
	// Un perfil Cliente no existe en el alcance interno.
	if rec := serveRequests(h, reader, "POST", "/v1/profile-requests?scope=internal", `{"kind":"deactivation","targetUserId":5,"reason":"Ya no trabaja aquí"}`); rec.Code != http.StatusNotFound {
		t.Fatalf("Cliente por el alcance interno: %d", rec.Code)
	}
}

func TestInternalRequestsFlow(t *testing.T) {
	users, reqs, notices, mail, h := internalRequestsFixture(t)
	body := `{"kind":"new_profile","name":"Pedro Bodega","username":"Pedro.Bodega","role":"empaque","permissions":["despacho"],"reason":"Nuevo operario de bodega"}`
	rec := serveRequests(h, reader, "POST", "/v1/profile-requests?scope=internal", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("solicitar: %d %s", rec.Code, rec.Body.String())
	}
	var created application.RequestView
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	if created.Scope != "internal" || created.CompanyID != 0 || created.Profile.Username != "pedro.bodega" || created.Profile.Role != "empaque" ||
		fmt.Sprint(created.Profile.Permissions) != "[despacho]" {
		t.Fatalf("solicitud interna: %+v", created)
	}
	// Una segunda pendiente para el mismo perfil y tipo se rechaza (sin empresa también).
	if rec := serveRequests(h, reader, "POST", "/v1/profile-requests?scope=internal", body); rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "request_pending") {
		t.Fatalf("pendiente interna duplicada: %d %s", rec.Code, rec.Body.String())
	}
	// Campana (canal interno) y correo a quienes gestionan perfiles, con el enlace de Configuración.
	if len(notices.notices) != 1 || notices.notices[0].Kind != "internal_profile_request" || notices.notices[0].CompanyID != 0 {
		t.Fatalf("campana: %+v", notices.notices)
	}
	if sent := mail.wait(t, 1); !strings.Contains(sent[0][0], "Nueva solicitud de perfil WeStorage") {
		t.Fatalf("correo: %v", sent)
	}
	// Las internas no se mezclan con las de clientes, ni un Cliente las ve.
	if rec := serveRequests(h, manager, "GET", fmt.Sprintf("/v1/profile-requests/%d", created.ID), ""); rec.Code != http.StatusNotFound {
		t.Fatalf("interna por el alcance client: %d", rec.Code)
	}
	if rec := serveRequests(h, client, "GET", "/v1/profile-requests?scope=internal", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("Cliente consulta internas: %d", rec.Code)
	}
	if rec := serveRequests(h, client, "GET", "/v1/profile-requests", ""); rec.Code != http.StatusOK || reqs.lastList.Scope != domain.ProfileScopeClient {
		t.Fatalf("el Cliente solo lista las de clientes: %d %+v", rec.Code, reqs.lastList)
	}
	// Sin el permiso no se aprueba.
	if rec := serveRequests(h, reader, "POST", fmt.Sprintf("/v1/profile-requests/%d/approve?scope=internal", created.ID), ""); rec.Code != http.StatusForbidden {
		t.Fatalf("aprobar sin permiso: %d", rec.Code)
	}
	// Aprobar: crea el perfil con rol y permisos; sin correo, la contraseña temporal va una vez a quien aprueba.
	rec = serveRequests(h, manager, "POST", fmt.Sprintf("/v1/profile-requests/%d/approve?scope=internal", created.ID), "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"temporaryPassword":"`) || !strings.Contains(rec.Body.String(), `"status":"approved"`) {
		t.Fatalf("aprobar: %d %s", rec.Code, rec.Body.String())
	}
	var newUser *ports.Profile
	for _, u := range users.users {
		if u.Username == "pedro.bodega" {
			newUser = u
		}
	}
	if newUser == nil || newUser.Role != "empaque" || fmt.Sprint(newUser.Permissions) != "[despacho]" {
		t.Fatalf("perfil creado: %+v", newUser)
	}
	// La respuesta avisa solo a quien la pidió (UserID), por el canal interno.
	last := notices.notices[len(notices.notices)-1]
	if last.Kind != "internal_profile_request_reviewed" || last.UserID != reader.UserID {
		t.Fatalf("aviso de respuesta: %+v", last)
	}
}

func TestInternalRequestCannotLeaveNoManager(t *testing.T) {
	users, _, _, _, h := internalRequestsFixture(t)
	// El Administrador sin permiso pide desactivar al único gestor; al aprobarla otro gestor
	// (fuera de los datos), la regla del último gestor responde 409 y la solicitud sigue pendiente.
	rec := serveRequests(h, reader, "POST", "/v1/profile-requests?scope=internal", `{"kind":"deactivation","targetUserId":1,"reason":"Ya no trabaja aquí"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("solicitar: %d %s", rec.Code, rec.Body.String())
	}
	var r application.RequestView
	_ = json.Unmarshal(rec.Body.Bytes(), &r)
	other := manager
	other.UserID = 99
	rec = serveRequests(h, other, "POST", fmt.Sprintf("/v1/profile-requests/%d/approve?scope=internal", r.ID), "")
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "last_manager") || !users.users[1].Active {
		t.Fatalf("aprobar la desactivación del último gestor: %d %s", rec.Code, rec.Body.String())
	}
	if rec := serveRequests(h, other, "GET", fmt.Sprintf("/v1/profile-requests/%d?scope=internal", r.ID), ""); !strings.Contains(rec.Body.String(), `"status":"pending"`) {
		t.Fatalf("la solicitud sigue pendiente: %s", rec.Body.String())
	}
	// Rechazar avisa al que la pidió.
	if rec := serveRequests(h, other, "POST", fmt.Sprintf("/v1/profile-requests/%d/reject?scope=internal", r.ID), `{"reason":"Es el único gestor"}`); rec.Code != http.StatusOK {
		t.Fatalf("rechazar: %d %s", rec.Code, rec.Body.String())
	}
}
