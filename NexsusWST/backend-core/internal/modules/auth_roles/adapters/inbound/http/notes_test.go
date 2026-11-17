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

type notesFake struct {
	ports.UserRepository
	owner   int64
	note    domain.Note
	changes domain.NoteChanges
}

func (f *notesFake) ListNotes(_ context.Context, u int64) ([]domain.Note, error) {
	if u != f.owner {
		return []domain.Note{}, nil
	}
	return []domain.Note{f.note}, nil
}

func (f *notesFake) CreateNote(_ context.Context, u int64, c domain.NoteChanges) (domain.Note, error) {
	f.owner = u
	f.changes = c
	f.note = domain.Note{ID: 8, Body: *c.Body}
	return f.note, nil
}

func (f *notesFake) UpdateNote(_ context.Context, u, n int64, c domain.NoteChanges) (domain.Note, error) {
	if u != f.owner || n != 8 {
		return domain.Note{}, domain.ErrNotFound
	}
	f.changes = c
	return f.note, nil
}

func (f *notesFake) DeleteNote(_ context.Context, u, n int64) error {
	if u != f.owner || n != 8 {
		return domain.ErrNotFound
	}
	return nil
}

func TestNotesHandlers(t *testing.T) {
	f := &notesFake{owner: 1, note: domain.Note{ID: 8, Body: "mía"}}
	svc := application.NewService(f, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	h := NewHandler(svc)
	for _, tc := range []struct {
		method, path, body string
		user               int64
		want               int
	}{
		{"GET", "/v1/me/notes", "", 1, 200}, {"GET", "/v1/me/notes", "", 2, 200},
		{"POST", "/v1/me/notes", `{"body":"  Recordatorio  ","dueDate":"2026-11-02"}`, 1, 201},
		{"POST", "/v1/me/notes", `{"body":" "}`, 1, 422},
		{"POST", "/v1/me/notes", `{"body":"hola","dueDate":"2026-02-30"}`, 1, 422},
		{"POST", "/v1/me/notes", `{"body":"hola","userId":2}`, 1, 400},
		{"POST", "/v1/me/notes", `{"body":null}`, 1, 400},
		{"PATCH", "/v1/me/notes/8", `{"done":true}`, 1, 200},
		{"PATCH", "/v1/me/notes/8", `{"dueDate":null}`, 1, 200},
		{"PATCH", "/v1/me/notes/8", `{}`, 1, 422},
		{"PATCH", "/v1/me/notes/8", `{"body":"ajena"}`, 2, 404},
		{"DELETE", "/v1/me/notes/8", "", 2, 404},
		{"DELETE", "/v1/me/notes/8", "", 1, 204},
	} {
		t.Run(tc.method+tc.path+tc.body+string(rune(tc.user+'0')), func(t *testing.T) {
			mux := http.NewServeMux()
			h.Register(mux, func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					next.ServeHTTP(w, r.WithContext(tenant.With(r.Context(), tenant.Identity{Profile: tenant.Profile{UserID: tc.user, Role: "cliente"}})))
				})
			})
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)))
			if w.Code != tc.want {
				t.Fatalf("status %d: %s", w.Code, w.Body)
			}
			if tc.method == "GET" && tc.user == 2 && strings.Contains(w.Body.String(), "mía") {
				t.Fatal("nota ajena expuesta")
			}
			if tc.body == `{"dueDate":null}` && (!f.changes.DueDateSet || f.changes.DueDate != nil) {
				t.Fatal("no distinguió null")
			}
		})
	}
}
