package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nexuswst/backend-core/internal/modules/auth_roles/application"
	"github.com/nexuswst/backend-core/internal/modules/auth_roles/domain"
	"github.com/nexuswst/backend-core/internal/modules/auth_roles/ports"
	"github.com/nexuswst/backend-core/internal/platform/xlsx"
	"github.com/nexuswst/backend-core/internal/shared/tenant"
)

// Configuración › Usuarios WeStorage (Tarea 08): /v1/profiles?scope=internal.

const internalQS = "?scope=internal"

// recIdP anota los roles que se fijan en Keycloak (para comprobar la vuelta atrás).
type recIdP struct {
	memIdP
	roles *[]string
}

func (r recIdP) SetRole(_ context.Context, _ string, role string) error {
	*r.roles = append(*r.roles, role)
	return nil
}

// internalFixture: además del Administrador 1 (sin permiso en los datos), un Jefe de Operaciones
// vinculado a Keycloak y un Empaque sin correo.
func internalFixture(t *testing.T, idp ports.IdentityProvider) (*memUsers, *ProfileHandler) {
	t.Helper()
	m, h := newProfilesFixtureWith(idp)
	m.users[1].Permissions, m.users[1].Email = []string{tenant.PermManageProfiles}, "admin@westorage.cl"
	m.users[2] = &ports.Profile{User: domain.User{ID: 2, KeycloakID: "kc-jefe", Email: "jefe@westorage.cl", Name: "María Rojas",
		Role: "jefe_operaciones", Permissions: []string{"despacho"}, AllCompanies: true, Active: true}}
	m.users[3] = &ports.Profile{User: domain.User{ID: 3, KeycloakID: "kc-emp", Username: "juan.bodega", Name: "Juan Pérez",
		Role: "empaque", Permissions: []string{}, AllCompanies: true, Active: true}}
	return m, h
}

func TestInternalProfilesAccess(t *testing.T) {
	_, h := internalFixture(t, memIdP{})
	// El Administrador sin permiso consulta; el Jefe y el Cliente, no.
	for _, path := range []string{"/v1/profiles", "/v1/profiles/2", "/v1/profiles/export"} {
		if rec := serveProfiles(h, reader, req("GET", path+internalQS, "")); rec.Code != http.StatusOK {
			t.Errorf("Administrador sin permiso consulta %s: %d %s", path, rec.Code, rec.Body.String())
		}
		for name, p := range map[string]tenant.Profile{"jefe": jefeOps, "cliente": client} {
			if rec := serveProfiles(h, p, req("GET", path+internalQS, "")); rec.Code != http.StatusForbidden {
				t.Errorf("%s consulta %s interno: %d, quería 403", name, path, rec.Code)
			}
		}
	}
	// Sin «Gestionar perfiles», ningún cambio (403); un Jefe con el permiso tampoco (solo Administrador).
	jefeConPermiso := jefeOps
	jefeConPermiso.Permissions = []string{tenant.PermManageProfiles}
	writes := [][3]string{
		{"POST", "/v1/profiles", `{"name":"Nuevo","email":"n@westorage.cl","role":"empaque"}`},
		{"PATCH", "/v1/profiles/2", `{"name":"Otra","role":"jefe_operaciones"}`},
		{"POST", "/v1/profiles/2/disable", `{"reason":"Ya no trabaja"}`},
		{"POST", "/v1/profiles/3/password", ``},
		{"GET", "/v1/profiles/template", ``},
	}
	for _, w := range writes {
		for name, p := range map[string]tenant.Profile{"admin sin permiso": reader, "jefe con permiso": jefeConPermiso} {
			if rec := serveProfiles(h, p, req(w[0], w[1]+internalQS, w[2])); rec.Code != http.StatusForbidden {
				t.Errorf("%s %s como %s: %d, quería 403", w[0], w[1], name, rec.Code)
			}
		}
	}
	// Un Cliente no existe en el alcance interno, ni un interno en el de clientes.
	if rec := serveProfiles(h, manager, req("GET", "/v1/profiles/5"+internalQS, "")); rec.Code != http.StatusNotFound {
		t.Errorf("Cliente por el alcance interno: %d", rec.Code)
	}
	var list struct {
		Data []application.ProfileView `json:"data"`
	}
	rec := serveProfiles(h, manager, req("GET", "/v1/profiles"+internalQS, ""))
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	for _, p := range list.Data {
		if p.Role == "cliente" {
			t.Fatalf("un Cliente aparece en Usuarios WeStorage: %+v", p)
		}
	}
}

func TestInternalProfilesCreate(t *testing.T) {
	_, h := internalFixture(t, memIdP{})
	post := func(body string) *httptest.ResponseRecorder {
		return serveProfiles(h, manager, req("POST", "/v1/profiles"+internalQS, body))
	}
	// Con correo: rol y permisos del formulario, sin contraseña temporal.
	rec := post(`{"name":"Ana Torres","email":"ana@westorage.cl","role":"jefe_operaciones","permissions":["despacho"],"jobTitle":"Supervisora"}`)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"role":"jefe_operaciones"`) ||
		!strings.Contains(rec.Body.String(), `"permissions":["despacho"]`) || strings.Contains(rec.Body.String(), "temporaryPassword") {
		t.Fatalf("crear con correo: %d %s", rec.Code, rec.Body.String())
	}
	// Roles y permisos que no corresponden.
	for body, field := range map[string]string{
		`{"name":"X","email":"x@westorage.cl","role":"analista"}`:                                     "role",
		`{"name":"X","email":"x@westorage.cl","role":"cliente"}`:                                      "role",
		`{"name":"X","email":"x@westorage.cl","role":"empaque","permissions":["gestionar_perfiles"]}`: "permissions",
		`{"name":"X","email":"x@westorage.cl","role":"admin","permissions":["despacho"]}`:             "permissions",
		`{"name":"X","role":"empaque"}`:                                                               "email",
		`{"name":"X","username":"A B","role":"empaque"}`:                                              "username",
	} {
		if rec := post(body); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `"field":"`+field+`"`) {
			t.Errorf("%s: %d %s", body, rec.Code, rec.Body.String())
		}
	}
	// Sin correo: nombre de usuario obligatorio y la contraseña temporal se muestra una vez.
	rec = post(`{"name":"Pedro Bodega","username":"pedro.bodega","role":"empaque","permissions":["despacho"]}`)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"temporaryPassword":"`) || !strings.Contains(rec.Body.String(), `"hasEmail":false`) {
		t.Fatalf("crear sin correo: %d %s", rec.Code, rec.Body.String())
	}
	if rec := post(`{"name":"Otro","username":"juan.bodega","role":"empaque"}`); rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), `"field":"username"`) {
		t.Fatalf("usuario repetido: %d %s", rec.Code, rec.Body.String())
	}
	// En clientes, rol, permisos y usuario no se aceptan.
	if rec := serveProfiles(h, manager, req("POST", "/v1/profiles", `{"name":"X","companyId":1,"email":"x@x.cl","role":"admin"}`)); rec.Code != http.StatusBadRequest {
		t.Fatalf("rol en un perfil Cliente: %d", rec.Code)
	}
	// Restablecer sin correo: contraseña temporal; con correo: enlace.
	if rec := serveProfiles(h, manager, req("POST", "/v1/profiles/3/password"+internalQS, "")); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"temporaryPassword":"`) {
		t.Fatalf("restablecer sin correo: %d %s", rec.Code, rec.Body.String())
	}
	if rec := serveProfiles(h, manager, req("POST", "/v1/profiles/2/password"+internalQS, "")); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"emailSent":true`) {
		t.Fatalf("restablecer con correo: %d %s", rec.Code, rec.Body.String())
	}
}

func TestInternalProfilesProtections(t *testing.T) {
	m, h := internalFixture(t, memIdP{})
	// Nadie se desactiva a sí mismo ni se quita su propio «Gestionar perfiles».
	if rec := serveProfiles(h, manager, req("POST", "/v1/profiles/1/disable"+internalQS, `{"reason":"Me voy de vacaciones"}`)); rec.Code != http.StatusForbidden {
		t.Fatalf("desactivarse a sí mismo: %d %s", rec.Code, rec.Body.String())
	}
	if rec := serveProfiles(h, manager, req("PATCH", "/v1/profiles/1"+internalQS, `{"name":"Admin","role":"admin","permissions":[]}`)); rec.Code != http.StatusForbidden {
		t.Fatalf("quitarse el permiso: %d %s", rec.Code, rec.Body.String())
	}
	// Otro gestor (fuera de los datos) intenta dejar sin Administradores con el permiso: 409.
	other := manager
	other.UserID = 99
	for _, w := range [][3]string{
		{"POST", "/v1/profiles/1/disable", `{"reason":"Ya no trabaja aquí"}`},
		{"PATCH", "/v1/profiles/1", `{"name":"Admin","role":"admin","permissions":[]}`},
		{"PATCH", "/v1/profiles/1", `{"name":"Admin","role":"jefe_operaciones","permissions":["despacho"]}`},
	} {
		rec := serveProfiles(h, other, req(w[0], w[1]+internalQS, w[2]))
		if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "last_manager") {
			t.Fatalf("%s %s %s: %d %s", w[0], w[1], w[2], rec.Code, rec.Body.String())
		}
	}
	if !m.users[1].Active || len(m.users[1].Permissions) != 1 {
		t.Fatalf("el último gestor no debía cambiar: %+v", m.users[1].User)
	}
	// Con otro Administrador gestor activo, sí se puede.
	m.users[7] = &ports.Profile{User: domain.User{ID: 7, Email: "otro@westorage.cl", Name: "Otro Admin", Role: "admin",
		Permissions: []string{tenant.PermManageProfiles}, AllCompanies: true, Active: true}}
	if rec := serveProfiles(h, other, req("POST", "/v1/profiles/1/disable"+internalQS, `{"reason":"Ya no trabaja aquí"}`)); rec.Code != http.StatusOK {
		t.Fatalf("desactivar con otro gestor: %d %s", rec.Code, rec.Body.String())
	}
}

func TestInternalProfileRoleChangeRollsBack(t *testing.T) {
	roles := []string{}
	m, h := internalFixture(t, recIdP{roles: &roles})
	// Cambio de rol correcto: Keycloak y Core, una acción en el registro con el antes y el después.
	m.audits = nil
	rec := serveProfiles(h, manager, req("PATCH", "/v1/profiles/2"+internalQS, `{"name":"María Rojas","role":"empaque","permissions":["despacho"]}`))
	if rec.Code != http.StatusOK || m.users[2].Role != "empaque" || fmt.Sprint(roles) != "[empaque]" || fmt.Sprint(m.audits) != "[updated]" {
		t.Fatalf("cambio de rol: %d %s · keycloak %v · registro %v", rec.Code, rec.Body.String(), roles, m.audits)
	}
	// Si el Core falla a mitad, Keycloak vuelve al rol anterior y el perfil no cambia.
	roles = roles[:0]
	m.failUpdate = true
	rec = serveProfiles(h, manager, req("PATCH", "/v1/profiles/2"+internalQS, `{"name":"María Rojas","role":"jefe_operaciones","permissions":[]}`))
	if rec.Code != http.StatusInternalServerError || fmt.Sprint(roles) != "[jefe_operaciones empaque]" || m.users[2].Role != "empaque" {
		t.Fatalf("vuelta atrás: %d · keycloak %v · rol %s", rec.Code, roles, m.users[2].Role)
	}
}

func TestInternalProfilesImport(t *testing.T) {
	_, h := internalFixture(t, memIdP{})
	var file bytes.Buffer
	_ = xlsx.Write(&file, "Perfiles", []string{"Nombre completo", "Correo", "Rol", "Permisos", "Teléfono", "Cargo"}, [][]any{
		{"EJEMPLO Ana", "ej@westorage.cl", "Empaque", "", "", ""},                     // ejemplo: no se importa
		{"Ana Torres", "ana@westorage.cl", "Jefe de Operaciones", "Despacho", "", ""}, // fila 3: válida
		{"Beto", "beto@westorage.cl", "Analista", "", "", ""},                         // fila 4: rol no ofrecido
		{"Carla", "carla@westorage.cl", "Empaque", "Gestionar perfiles", "", ""},      // fila 5: permiso del rol
		{"Dani", "", "Empaque", "", "", ""},                                           // fila 6: sin correo
		{"Eva", "eva@westorage.cl", "administrador", "gestionar perfiles", "", ""},    // fila 7: válida
		{"Fede", "fede@westorage.cl", "Empaque", "Volar", "", ""},                     // fila 8: permiso desconocido
	})
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, _ := mw.CreateFormFile("file", "perfiles.xlsx")
	_, _ = part.Write(file.Bytes())
	_ = mw.Close()
	r := httptest.NewRequest("POST", "/v1/profiles/import"+internalQS, &body)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	rec := serveProfiles(h, manager, r)
	var res application.ImportResult
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	rows := []string{}
	for _, e := range res.Errors {
		rows = append(rows, fmt.Sprintf("%d:%s", e.Row, e.Field))
	}
	if rec.Code != http.StatusOK || res.Rows != 6 || len(res.Created) != 2 || fmt.Sprint(rows) != "[4:role 5:permissions 6:email 8:permissions]" {
		t.Fatalf("importación: %d %v %s", rec.Code, rows, rec.Body.String())
	}
}
