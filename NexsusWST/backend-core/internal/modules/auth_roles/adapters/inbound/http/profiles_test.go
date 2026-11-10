package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nexuswst/backend-core/internal/modules/auth_roles/application"
	"github.com/nexuswst/backend-core/internal/modules/auth_roles/domain"
	"github.com/nexuswst/backend-core/internal/modules/auth_roles/ports"
	"github.com/nexuswst/backend-core/internal/platform/xlsx"
	"github.com/nexuswst/backend-core/internal/shared/tenant"
)

// Dobles en memoria: usuarios, perfiles (contacto y empresa) y Keycloak.
type memUsers struct {
	ports.UserRepository
	users      map[int64]*ports.Profile
	audits     []string
	nextID     int64
	failUpdate bool       // simula una falla del Core a mitad de una edición
	lock       sync.Mutex // candado de gestores (pg_advisory_xact_lock en PostgreSQL)
}

func (m *memUsers) GetByID(_ context.Context, id int64) (domain.User, error) {
	if p, ok := m.users[id]; ok {
		return p.User, nil
	}
	return domain.User{}, domain.ErrNotFound
}

func (m *memUsers) GetByEmail(_ context.Context, email string) (domain.User, error) {
	for _, p := range m.users {
		if p.Email == email && email != "" {
			return p.User, nil
		}
	}
	return domain.User{}, domain.ErrNotFound
}

func (m *memUsers) GetByUsername(_ context.Context, u string) (domain.User, error) {
	for _, p := range m.users {
		if p.Username == u && u != "" {
			return p.User, nil
		}
	}
	return domain.User{}, domain.ErrNotFound
}

func (m *memUsers) CompaniesExist(_ context.Context, ids []int64) (bool, error) {
	for _, id := range ids {
		if id != 1 && id != 3 {
			return false, nil
		}
	}
	return true, nil
}

func (m *memUsers) Insert(_ context.Context, kc string, d domain.Draft, _ *int64) (domain.User, error) {
	m.nextID++
	u := domain.User{ID: m.nextID, KeycloakID: kc, Email: d.Email, Username: d.Username, Name: d.Name, Role: d.Role,
		Permissions: d.Permissions, Companies: d.Companies, Active: true, CreatedAt: time.Now()}
	p := &ports.Profile{User: u}
	if len(d.Companies) > 0 {
		p.CompanyID, p.CompanyName = d.Companies[0], "Empresa "+string(rune('0'+d.Companies[0]))
	}
	m.users[u.ID] = p
	return u, nil
}

func (m *memUsers) Update(_ context.Context, id int64, d domain.Draft, _ int64) error {
	if m.failUpdate {
		m.failUpdate = false // solo la primera: la vuelta atrás sí funciona
		return fmt.Errorf("falla simulada del Core")
	}
	p := m.users[id]
	p.Name, p.Companies, p.Role, p.Permissions = d.Name, d.Companies, d.Role, d.Permissions
	if len(d.Companies) > 0 {
		p.CompanyID = d.Companies[0]
	}
	return nil
}

func (m *memUsers) SetActive(_ context.Context, id int64, active bool, _ int64) error {
	m.users[id].Active = active
	return nil
}

func (m *memUsers) Audit(_ context.Context, _ *int64, _ int64, action string, _ map[string]any) error {
	m.audits = append(m.audits, action)
	return nil
}

type memProfiles struct{ *memUsers }

func (m memProfiles) ListProfiles(_ context.Context, f ports.ProfileFilter) ([]ports.Profile, int, error) {
	out := []ports.Profile{}
	for _, p := range m.users {
		if (p.Role == "cliente") == (f.Scope == domain.ProfileScopeClient) {
			out = append(out, *p)
		}
	}
	return out, len(out), nil
}

func (m memProfiles) GetProfile(_ context.Context, id int64) (ports.Profile, error) {
	if p, ok := m.users[id]; ok {
		return *p, nil
	}
	return ports.Profile{}, domain.ErrNotFound
}

func (m memProfiles) SetContact(_ context.Context, id int64, phone, job string) error {
	m.users[id].Phone, m.users[id].JobTitle = phone, job
	return nil
}

func (m memProfiles) CompanyByTaxID(_ context.Context, rut string) (int64, string, bool, error) {
	switch strings.NewReplacer(".", "", "-", "").Replace(rut) {
	case "111111111":
		return 1, "Empresa 1", true, nil
	case "222222222":
		return 2, "Empresa 2", false, nil
	}
	return 0, "", false, domain.ErrNotFound
}

func (m memProfiles) Company(_ context.Context, id int64) (string, bool, error) {
	switch id {
	case 1, 3:
		return "Empresa", true, nil
	case 2:
		return "Empresa 2", false, nil
	}
	return "", false, domain.ErrNotFound
}

func (m memProfiles) ActiveCompanies(context.Context) ([][2]string, error) {
	return [][2]string{{"Empresa 1", "11.111.111-1"}}, nil
}

func (m memProfiles) CountManagers(_ context.Context, except int64) (int, error) {
	n := 0
	for _, p := range m.users {
		if p.ID != except && p.Active && p.Role == "admin" && slices.Contains(p.Permissions, tenant.PermManageProfiles) {
			n++
		}
	}
	return n, nil
}

func (m memProfiles) WithManagerLock(ctx context.Context, fn func(context.Context) error) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	return fn(ctx)
}

func (m memProfiles) AuditCompany(_ context.Context, _, _ int64, action string, _ map[string]any) error {
	m.audits = append(m.audits, action)
	return nil
}

type memIdP struct{ ports.IdentityProvider }

func (memIdP) CreateUser(context.Context, string, string) (string, error)      { return "kc-1", nil }
func (memIdP) CreateLocalUser(context.Context, string, string) (string, error) { return "kc-2", nil }
func (memIdP) SetRole(context.Context, string, string) error                   { return nil }
func (memIdP) SendSetPasswordEmail(context.Context, string) error              { return nil }
func (memIdP) SetTemporaryPassword(context.Context, string, string) error      { return nil }
func (memIdP) SetEnabled(context.Context, string, bool) error                  { return nil }
func (memIdP) Logout(context.Context, string) error                            { return nil }
func (memIdP) UpdateName(context.Context, string, string) error                { return nil }

func newProfilesFixture() (*memUsers, *ProfileHandler) {
	return newProfilesFixtureWith(memIdP{})
}

func newProfilesFixtureWith(idp ports.IdentityProvider) (*memUsers, *ProfileHandler) {
	m := &memUsers{users: map[int64]*ports.Profile{}, nextID: 10}
	m.users[1] = &ports.Profile{User: domain.User{ID: 1, Name: "Admin", Role: "admin", AllCompanies: true, Active: true}}
	m.users[5] = &ports.Profile{User: domain.User{ID: 5, KeycloakID: "kc-5", Email: "esperanza@andes.cl", Name: "Esperanza Gómez", Role: "cliente",
		Companies: []int64{1}, Active: true}, CompanyID: 1, CompanyName: "Andes"}
	svc := application.NewService(m, idp, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return m, NewProfileHandler(application.NewProfiles(svc, memProfiles{m}))
}

func serveProfiles(h *ProfileHandler, p tenant.Profile, req *http.Request) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	h.Register(mux, func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(tenant.With(r.Context(), tenant.Identity{Profile: p})))
		})
	})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

var (
	manager = tenant.Profile{UserID: 1, Role: tenant.RoleAdmin, AllCompanies: true, Permissions: []string{tenant.PermManageProfiles}}
	reader  = tenant.Profile{UserID: 8, Role: tenant.RoleAdmin, AllCompanies: true}
	jefeOps = tenant.Profile{UserID: 2, Role: tenant.RoleJefeOps, AllCompanies: true, Permissions: []string{tenant.PermDespacho}}
	client  = tenant.Profile{UserID: 5, Role: tenant.RoleCliente, Companies: []int64{1}}
)

func req(method, path, body string) *http.Request {
	return httptest.NewRequest(method, path, strings.NewReader(body))
}

func TestProfilesAccess(t *testing.T) {
	_, h := newProfilesFixture()
	writes := [][3]string{
		{"POST", "/v1/profiles", `{"name":"Nuevo","companyId":1,"email":"n@x.cl"}`},
		{"PATCH", "/v1/profiles/5", `{"name":"Otra","companyId":1}`},
		{"POST", "/v1/profiles/5/disable", `{"reason":"Ya no trabaja"}`},
		{"POST", "/v1/profiles/5/enable", ``},
		{"POST", "/v1/profiles/5/password", ``},
		{"GET", "/v1/profiles/template", ``},
		{"POST", "/v1/profiles/import", ``},
	}
	for _, w := range writes {
		for name, p := range map[string]tenant.Profile{"admin sin permiso": reader, "jefe": jefeOps, "cliente": client} {
			if rec := serveProfiles(h, p, req(w[0], w[1], w[2])); rec.Code != http.StatusForbidden {
				t.Errorf("%s %s como %s: %d, quería 403", w[0], w[1], name, rec.Code)
			}
		}
	}
	for _, path := range []string{"/v1/profiles", "/v1/profiles/5", "/v1/profiles/export"} {
		if rec := serveProfiles(h, reader, req("GET", path, "")); rec.Code != http.StatusOK {
			t.Errorf("el Administrador sin permiso consulta %s: %d %s", path, rec.Code, rec.Body.String())
		}
		// Fase 5: el Cliente consulta los perfiles de su empresa (Perfiles empresa).
		if rec := serveProfiles(h, client, req("GET", path, "")); rec.Code != http.StatusOK {
			t.Errorf("el Cliente consulta los perfiles de su empresa (%s): %d", path, rec.Code)
		}
	}
	// Un perfil de otra empresa no existe para el Cliente.
	other := client
	other.Companies = []int64{3}
	if rec := serveProfiles(h, other, req("GET", "/v1/profiles/5", "")); rec.Code != http.StatusNotFound {
		t.Errorf("perfil de otra empresa: %d", rec.Code)
	}
	// Un perfil interno no existe para el alcance «client»; un alcance desconocido es 422.
	if rec := serveProfiles(h, reader, req("GET", "/v1/profiles/1", "")); rec.Code != http.StatusNotFound {
		t.Errorf("perfil interno por el alcance client: %d", rec.Code)
	}
	if rec := serveProfiles(h, reader, req("GET", "/v1/profiles?scope=otro", "")); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("alcance desconocido: %d", rec.Code)
	}
}

func TestProfilesCreateAndEdit(t *testing.T) {
	m, h := newProfilesFixture()
	// Obligatorios por campo.
	rec := serveProfiles(h, manager, req("POST", "/v1/profiles", `{"name":"","companyId":0}`))
	var body struct {
		Fields []domain.FieldError `json:"fields"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if rec.Code != http.StatusUnprocessableEntity || len(body.Fields) != 3 {
		t.Fatalf("validación: %d %s", rec.Code, rec.Body.String())
	}
	// Empresa desactivada.
	if rec := serveProfiles(h, manager, req("POST", "/v1/profiles", `{"name":"X","companyId":2,"email":"x@x.cl"}`)); !strings.Contains(rec.Body.String(), "desactivada") {
		t.Fatalf("empresa desactivada: %d %s", rec.Code, rec.Body.String())
	}
	// Con correo: rol Cliente fijo, sin permisos y con teléfono y cargo.
	rec = serveProfiles(h, manager, req("POST", "/v1/profiles", `{"name":"Luis Pérez","companyId":1,"email":"Luis@Andes.cl","phone":"+56 9 1111 2222","jobTitle":"Bodega"}`))
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"role":"cliente"`) || !strings.Contains(rec.Body.String(), `"jobTitle":"Bodega"`) ||
		!strings.Contains(rec.Body.String(), `"permissions":[]`) || strings.Contains(rec.Body.String(), "temporaryPassword") {
		t.Fatalf("crear con correo: %d %s", rec.Code, rec.Body.String())
	}
	// Correo repetido: 409 en el campo.
	if rec := serveProfiles(h, manager, req("POST", "/v1/profiles", `{"name":"Otro","companyId":1,"email":"luis@andes.cl"}`)); rec.Code != http.StatusConflict ||
		!strings.Contains(rec.Body.String(), `"field":"email"`) {
		t.Fatalf("correo repetido: %d %s", rec.Code, rec.Body.String())
	}
	// El perfil Cliente siempre tiene correo: sin correo es 422 en el campo, y «username» ya no se acepta.
	rec = serveProfiles(h, manager, req("POST", "/v1/profiles", `{"name":"Pablo","companyId":1}`))
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `"field":"email"`) {
		t.Fatalf("cliente sin correo: %d %s", rec.Code, rec.Body.String())
	}
	if rec := serveProfiles(h, manager, req("POST", "/v1/profiles", `{"name":"Pablo","companyId":1,"username":"pablo.bodega"}`)); rec.Code != http.StatusBadRequest {
		t.Fatalf("username no se acepta: %d", rec.Code)
	}
	// Restablecer contraseña: siempre por enlace al correo.
	if rec := serveProfiles(h, manager, req("POST", "/v1/profiles/5/password", ``)); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"emailSent":true`) {
		t.Fatalf("restablecer: %d %s", rec.Code, rec.Body.String())
	}
	// Editar: una sola acción en el registro.
	m.audits = nil
	if rec := serveProfiles(h, manager, req("PATCH", "/v1/profiles/5", `{"name":"Esperanza Gómez","companyId":3,"phone":"+56 2 2222 3333","jobTitle":"RRHH"}`)); rec.Code != http.StatusOK {
		t.Fatalf("editar: %d %s", rec.Code, rec.Body.String())
	}
	if len(m.audits) != 1 || m.audits[0] != "updated" {
		t.Fatalf("una acción por edición: %v", m.audits)
	}
	// Desactivar exige motivo; solo el botón que corresponde.
	if rec := serveProfiles(h, manager, req("POST", "/v1/profiles/5/disable", `{"reason":"no"}`)); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `"field":"reason"`) {
		t.Fatalf("motivo: %d %s", rec.Code, rec.Body.String())
	}
	if rec := serveProfiles(h, manager, req("POST", "/v1/profiles/5/disable", `{"reason":"Ya no trabaja en la empresa"}`)); rec.Code != http.StatusOK {
		t.Fatalf("desactivar: %d %s", rec.Code, rec.Body.String())
	}
	if rec := serveProfiles(h, manager, req("POST", "/v1/profiles/5/disable", `{"reason":"Otra vez lo mismo"}`)); rec.Code != http.StatusConflict {
		t.Fatalf("ya inactivo: %d", rec.Code)
	}
}

func TestProfilesImport(t *testing.T) {
	m, h := newProfilesFixture()
	var file bytes.Buffer
	_ = xlsx.Write(&file, "Perfiles", []string{"Nombre completo", "RUT empresa", "Correo", "Teléfono", "Cargo"}, [][]any{
		{"EJEMPLO Juan", "11.111.111-1", "ej@x.cl", "", ""},        // ejemplo: no se importa
		{"Ana Soto", "11.111.111-1", "ana@andes.cl", "", "Ventas"}, // fila 3: válida
		{"", "11.111.111-1", "b@x.cl", "", ""},                     // fila 4: sin nombre
		{"Carlos", "99.999.999-9", "c@x.cl", "", ""},               // fila 5: empresa inexistente
		{"Dora", "22.222.222-2", "d@x.cl", "", ""},                 // fila 6: empresa desactivada
		{"Eva", "111111111", "eva@andes.cl", "", ""},               // fila 7: válida (RUT sin puntos)
		{"Ana Dos", "11.111.111-1", "ana@andes.cl", "", ""},        // fila 8: repetida en el archivo
		{"Fede", "11.111.111-1", "", "", ""},                       // fila 9: sin correo
	})
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, _ := mw.CreateFormFile("file", "perfiles.xlsx")
	_, _ = part.Write(file.Bytes())
	_ = mw.Close()
	r := httptest.NewRequest("POST", "/v1/profiles/import", &body)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	rec := serveProfiles(h, manager, r)
	var res application.ImportResult
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	if rec.Code != http.StatusOK || res.Rows != 7 || len(res.Created) != 2 || len(res.Errors) != 5 {
		t.Fatalf("importación: %d %s", rec.Code, rec.Body.String())
	}
	rows := []int{}
	for _, e := range res.Errors {
		rows = append(rows, e.Row)
	}
	if fmt.Sprint(rows) != "[4 5 6 8 9]" {
		t.Fatalf("filas con error %v: %+v", rows, res.Errors)
	}
	if res.Errors[4].Field != "email" {
		t.Fatalf("fila sin correo: %+v", res.Errors[4])
	}
	if m.audits[len(m.audits)-1] != "profiles_imported" {
		t.Fatalf("la carga masiva queda en el registro: %v", m.audits)
	}
}
