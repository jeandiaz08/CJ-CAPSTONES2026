package http

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nexuswst/backend-core/internal/modules/auth_roles/application"
	"github.com/nexuswst/backend-core/internal/modules/auth_roles/domain"
	"github.com/nexuswst/backend-core/internal/modules/auth_roles/ports"
	"github.com/nexuswst/backend-core/internal/platform/events"
	"github.com/nexuswst/backend-core/internal/shared/tenant"
)

// memRequests imita profile_requests, incluido el índice de una sola pendiente por perfil y tipo
// (uq_profile_requests_pending con NULLS NOT DISTINCT: en las internas, sin empresa, también choca).
type memRequests struct {
	reqs     map[int64]*domain.ProfileRequest
	audits   []string
	lastList ports.RequestFilter
}

func (m *memRequests) Create(_ context.Context, companyID, by int64, d domain.RequestDraft) (int64, error) {
	key := func(r *domain.ProfileRequest) string {
		t := ""
		if r.TargetUserID != nil {
			t = string(rune(*r.TargetUserID))
		}
		login := r.ProfileEmail + r.ProfileUsername
		if login == "" {
			login = strings.ToLower(r.ProfileName)
		}
		return string(r.Scope) + string(r.Kind) + t + login
	}
	scope, company := domain.ProfileScopeClient, "Andes"
	if companyID == 0 {
		scope, company = domain.ProfileScopeInternal, ""
	}
	n := &domain.ProfileRequest{ID: int64(len(m.reqs) + 1), Scope: scope, CompanyID: companyID, CompanyName: company, Kind: d.Kind, ProfileName: d.Name,
		ProfileEmail: d.Email, ProfilePhone: d.Phone, ProfileJobTitle: d.JobTitle, ProfileUsername: d.Username, ProfileRole: d.Role, ProfilePermissions: d.Permissions,
		Reason: d.Reason, Status: domain.RequestPending, RequestedBy: by, RequestedAt: time.Now()}
	if d.Kind != domain.RequestNewProfile {
		t := d.TargetUserID
		n.TargetUserID = &t
	}
	for _, r := range m.reqs {
		if r.Status == domain.RequestPending && r.CompanyID == companyID && key(r) == key(n) {
			return 0, domain.ErrRequestPending
		}
	}
	m.reqs[n.ID] = n
	return n.ID, nil
}

func (m *memRequests) Get(_ context.Context, id int64) (domain.ProfileRequest, error) {
	if r, ok := m.reqs[id]; ok {
		return *r, nil
	}
	return domain.ProfileRequest{}, domain.ErrNotFound
}

func (m *memRequests) List(_ context.Context, f ports.RequestFilter) ([]domain.ProfileRequest, int, ports.RequestCounts, error) {
	m.lastList = f
	out := []domain.ProfileRequest{}
	for _, r := range m.reqs {
		if f.Scope == "" || r.Scope == f.Scope {
			out = append(out, *r)
		}
	}
	return out, len(out), ports.RequestCounts{Pending: len(out)}, nil
}

func (m *memRequests) review(id, by int64, status domain.RequestStatus) error {
	r := m.reqs[id]
	if r.Status != domain.RequestPending {
		return domain.ErrRequestReviewed
	}
	r.Status, r.ReviewedBy = status, &by
	return nil
}

func (m *memRequests) Approve(_ context.Context, id, by int64, _ *int64) error {
	return m.review(id, by, domain.RequestApproved)
}

func (m *memRequests) Reject(_ context.Context, id, by int64, reason string) error {
	m.reqs[id].RejectionReason = reason
	return m.review(id, by, domain.RequestRejected)
}

func (m *memRequests) PendingFor(context.Context, []int64) (map[int64][]domain.RequestKind, error) {
	return map[int64][]domain.RequestKind{}, nil
}

func (m *memRequests) AuditRequest(_ context.Context, _, _ int64, _ *int64, action string, _ map[string]any) error {
	m.audits = append(m.audits, action)
	return nil
}

func (m *memRequests) ManagerEmails(context.Context) ([]string, error) {
	return []string{"admin@nexus.local"}, nil
}

type recNotifier struct{ notices []events.Notice }

func (r *recNotifier) Notify(_ context.Context, n events.Notice) { r.notices = append(r.notices, n) }

type recMailer struct {
	mu   sync.Mutex
	sent [][]string
}

func (r *recMailer) Send(_ context.Context, to []string, subject, _ string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sent = append(r.sent, append([]string{subject}, to...))
	return nil
}

func (r *recMailer) wait(t *testing.T, n int) [][]string {
	t.Helper()
	for i := 0; i < 100; i++ {
		r.mu.Lock()
		if len(r.sent) >= n {
			defer r.mu.Unlock()
			return r.sent
		}
		r.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("no llegaron %d correos", n)
	return nil
}

func newRequestsFixture() (*memUsers, *memRequests, *recNotifier, *recMailer, *RequestHandler) {
	m := &memUsers{users: map[int64]*ports.Profile{}, nextID: 10}
	m.users[5] = &ports.Profile{User: domain.User{ID: 5, KeycloakID: "kc-5", Email: "esperanza@andes.cl", Name: "Esperanza Gómez", Role: "cliente",
		Companies: []int64{1}, Active: true}, CompanyID: 1, CompanyName: "Andes"}
	m.users[6] = &ports.Profile{User: domain.User{ID: 6, KeycloakID: "kc-6", Email: "pablo@andes.cl", Name: "Pablo Carpio", Role: "cliente",
		Companies: []int64{1}, Active: false}, CompanyID: 1, CompanyName: "Andes"}
	m.users[7] = &ports.Profile{User: domain.User{ID: 7, Email: "otro@casa.cl", Name: "Otro", Role: "cliente", Companies: []int64{3}, Active: true},
		CompanyID: 3, CompanyName: "Casa"}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := application.NewService(m, memIdP{}, nil, log)
	reqs := &memRequests{reqs: map[int64]*domain.ProfileRequest{}}
	n, mail := &recNotifier{}, &recMailer{}
	h := NewRequestHandler(application.NewRequests(application.NewProfiles(svc, memProfiles{m}), reqs, n, mail, "http://localhost", log))
	return m, reqs, n, mail, h
}

func serveRequests(h *RequestHandler, p tenant.Profile, method, path, body string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	h.Register(mux, func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(tenant.With(r.Context(), tenant.Identity{Profile: p})))
		})
	})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req(method, path, body))
	return rec
}

func TestRequestsFlow(t *testing.T) {
	users, reqs, notices, mail, h := newRequestsFixture()

	// El perfil nuevo siempre con correo; el motivo es obligatorio.
	rec := serveRequests(h, client, "POST", "/v1/profile-requests", `{"kind":"new_profile","name":"Luis Pérez","reason":"no"}`)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `"field":"email"`) || !strings.Contains(rec.Body.String(), `"field":"reason"`) {
		t.Fatalf("validación: %d %s", rec.Code, rec.Body.String())
	}
	// Solo el Cliente solicita (el Administrador gestiona directo).
	if rec := serveRequests(h, manager, "POST", "/v1/profile-requests", `{"kind":"new_profile","name":"X","email":"x@x.cl","reason":"Nuevo encargado"}`); rec.Code != http.StatusForbidden {
		t.Fatalf("solo el Cliente solicita: %d", rec.Code)
	}
	body := `{"kind":"new_profile","name":"Luis Pérez","email":"luis@andes.cl","jobTitle":"Bodega","reason":"Nuevo encargado en bodega"}`
	rec = serveRequests(h, client, "POST", "/v1/profile-requests", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("crear: %d %s", rec.Code, rec.Body.String())
	}
	// Una segunda pendiente para el mismo perfil y tipo se rechaza.
	if rec := serveRequests(h, client, "POST", "/v1/profile-requests", body); rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "request_pending") {
		t.Fatalf("pendiente duplicada: %d %s", rec.Code, rec.Body.String())
	}
	// Aviso interno en la campana y correo a quienes gestionan perfiles.
	if len(notices.notices) != 1 || notices.notices[0].Audience != "internal" || notices.notices[0].Kind != "profile_request" {
		t.Fatalf("campana: %+v", notices.notices)
	}
	if sent := mail.wait(t, 1); sent[0][1] != "admin@nexus.local" || !strings.Contains(sent[0][0], "Nueva solicitud de perfil") {
		t.Fatalf("correo: %v", sent)
	}

	// Aprobar y rechazar: solo con el permiso.
	for name, p := range map[string]tenant.Profile{"admin sin permiso": reader, "jefe": jefeOps, "cliente": client} {
		if rec := serveRequests(h, p, "POST", "/v1/profile-requests/1/approve", ""); rec.Code != http.StatusForbidden {
			t.Errorf("aprobar como %s: %d", name, rec.Code)
		}
		if rec := serveRequests(h, p, "POST", "/v1/profile-requests/1/reject", `{"reason":"No corresponde"}`); rec.Code != http.StatusForbidden {
			t.Errorf("rechazar como %s: %d", name, rec.Code)
		}
	}
	// Aprobar crea el perfil con los datos de la solicitud y avisa al Cliente.
	rec = serveRequests(h, manager, "POST", "/v1/profile-requests/1/approve", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"approved"`) {
		t.Fatalf("aprobar: %d %s", rec.Code, rec.Body.String())
	}
	var created *ports.Profile
	for _, u := range users.users {
		if u.Email == "luis@andes.cl" {
			created = u
		}
	}
	if created == nil || created.Role != "cliente" || created.CompanyID != 1 || created.JobTitle != "Bodega" {
		t.Fatalf("perfil creado: %+v", created)
	}
	last := notices.notices[len(notices.notices)-1]
	if last.Audience != "client" || last.Kind != "profile_request_reviewed" || last.Title != "Tu solicitud fue aprobada" {
		t.Fatalf("aviso al Cliente: %+v", last)
	}
	if sent := mail.wait(t, 2); sent[1][1] != "esperanza@andes.cl" {
		t.Fatalf("correo al Cliente: %v", sent)
	}
	if rec := serveRequests(h, manager, "POST", "/v1/profile-requests/1/reject", `{"reason":"Tarde"}`); rec.Code != http.StatusConflict {
		t.Fatalf("ya respondida: %d", rec.Code)
	}

	// Activación: se aplica al perfil. Rechazar exige motivo.
	rec = serveRequests(h, client, "POST", "/v1/profile-requests", `{"kind":"activation","targetUserId":6,"reason":"Vuelve de vacaciones"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("activación: %d %s", rec.Code, rec.Body.String())
	}
	if rec := serveRequests(h, client, "POST", "/v1/profile-requests", `{"kind":"deactivation","targetUserId":6,"reason":"Ya no trabaja"}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("desactivar un perfil inactivo: %d", rec.Code)
	}
	if rec := serveRequests(h, manager, "POST", "/v1/profile-requests/2/approve", ""); rec.Code != http.StatusOK || !users.users[6].Active {
		t.Fatalf("aprobar activación: %d %s", rec.Code, rec.Body.String())
	}
	rec = serveRequests(h, client, "POST", "/v1/profile-requests", `{"kind":"deactivation","targetUserId":6,"reason":"Ya no trabaja aquí"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("desactivación: %d %s", rec.Code, rec.Body.String())
	}
	if rec := serveRequests(h, manager, "POST", "/v1/profile-requests/3/reject", `{"reason":"no"}`); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `"field":"reason"`) {
		t.Fatalf("rechazo sin motivo: %d %s", rec.Code, rec.Body.String())
	}
	if rec := serveRequests(h, manager, "POST", "/v1/profile-requests/3/reject", `{"reason":"Debe seguir activo"}`); rec.Code != http.StatusOK || !users.users[6].Active {
		t.Fatalf("rechazar: %d %s", rec.Code, rec.Body.String())
	}
	if want := "request_created request_approved request_created request_approved request_created request_rejected"; strings.Join(reqs.audits, " ") != want {
		t.Fatalf("registro: %v", reqs.audits)
	}

	// Aislamiento: el perfil de otra empresa no existe para este Cliente; la lista va filtrada.
	if rec := serveRequests(h, client, "POST", "/v1/profile-requests", `{"kind":"deactivation","targetUserId":7,"reason":"Otra empresa"}`); rec.Code != http.StatusNotFound {
		t.Fatalf("perfil de otra empresa: %d", rec.Code)
	}
	if rec := serveRequests(h, client, "GET", "/v1/profile-requests?company_id=3", ""); rec.Code != http.StatusOK || len(reqs.lastList.CompanyIDs) != 1 || reqs.lastList.CompanyIDs[0] != 1 {
		t.Fatalf("el Cliente solo ve su empresa: %d %+v", rec.Code, reqs.lastList)
	}
	var list struct {
		Counts ports.RequestCounts `json:"counts"`
	}
	_ = json.Unmarshal(serveRequests(h, manager, "GET", "/v1/profile-requests", "").Body.Bytes(), &list)
	if list.Counts.Pending == 0 {
		t.Fatalf("KPIs: %+v", list.Counts)
	}
	if rec := serveRequests(h, jefeOps, "GET", "/v1/profile-requests", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("el Jefe no ve solicitudes: %d", rec.Code)
	}
}

func TestRequestsEditAndAccept(t *testing.T) {
	users, _, _, _, h := newRequestsFixture()
	serveRequests(h, client, "POST", "/v1/profile-requests", `{"kind":"new_profile","name":"Luiz Peres","email":"luiz@andes.cl","reason":"Nuevo encargado"}`)
	rec := serveRequests(h, manager, "POST", "/v1/profile-requests/1/approve", `{"name":"Luis Pérez","email":"luis.perez@andes.cl","phone":"","jobTitle":"Bodega"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("editar y aceptar: %d %s", rec.Code, rec.Body.String())
	}
	for _, u := range users.users {
		if u.Email == "luis.perez@andes.cl" && u.Name == "Luis Pérez" && u.CompanyID == 1 {
			return
		}
	}
	t.Fatal("el perfil se crea con los datos editados")
}
