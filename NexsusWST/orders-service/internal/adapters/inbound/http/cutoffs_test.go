package http

import (
	"context"
	"github.com/nexuswst/orders-service/internal/application"
	"github.com/nexuswst/orders-service/internal/domain"
	"github.com/nexuswst/orders-service/internal/ports"
	"github.com/nexuswst/orders-service/internal/shared/tenant"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type cutoffCatalog struct{ ports.CatalogReader }

func (cutoffCatalog) CourierActive(_ context.Context, code string) (bool, error) {
	return code != "inactive", nil
}

type cutoffFake struct{ ports.RuleRepository }

func (cutoffFake) Cutoffs(context.Context) ([]ports.Cutoff, error) {
	return []ports.Cutoff{{ID: 1, Number: 1, CutoffTime: "08:00", Couriers: []ports.CutoffCourier{}}}, nil
}
func (cutoffFake) SaveCutoff(_ context.Context, c ports.Cutoff, _ string) (ports.Cutoff, error) {
	if c.CutoffTime == "08:00" {
		return c, domain.ErrCutoffTimeTaken
	}
	return c, nil
}
func (cutoffFake) AssignmentDeadline(context.Context) (string, error) { return "12:00", nil }
func (cutoffFake) DeleteCutoff(context.Context, int64, string) error  { return nil }
func TestCutoffHandlers(t *testing.T) {
	for _, tc := range []struct {
		method, path, body, role string
		want                     int
	}{
		{"POST", "/v1/cutoffs", `{"cutoffTime":"09:00","couriers":[{"code":"inactive","arrivalTime":"10:00"}]}`, "admin", 422},
		{"GET", "/v1/cutoffs", "", "admin", 200}, {"GET", "/v1/cutoffs", "", "cliente", 403},
		{"POST", "/v1/cutoffs", `{"cutoffTime":"09:00","couriers":[{"code":"starken","arrivalTime":"09:30"}]}`, "jefe_operaciones", 201},
		{"PUT", "/v1/cutoffs/1", `{"cutoffTime":"09:00","couriers":[{"code":"starken","arrivalTime":"09:30"}]}`, "admin", 200},
		{"POST", "/v1/cutoffs", `{"cutoffTime":"08:00","couriers":[{"code":"starken","arrivalTime":"09:30"}]}`, "admin", 422},
		{"POST", "/v1/cutoffs", `{"cutoffTime":"13:00","couriers":[{"code":"starken","arrivalTime":"13:30"}]}`, "admin", 422},
		{"POST", "/v1/cutoffs", `{"cutoffTime":"25:00","couriers":[]}`, "admin", 422},
		{"POST", "/v1/cutoffs", `{"cutoffTime":"09:00","couriers":[{"code":"starken","arrivalTime":"08:00"}]}`, "admin", 422},
		{"DELETE", "/v1/cutoffs/1", "", "admin", 204}, {"DELETE", "/v1/cutoffs/1", "", "empaque", 403},
	} {
		t.Run(tc.method+tc.path+tc.body+tc.role, func(t *testing.T) {
			svc := application.NewService(application.Deps{Rules: cutoffFake{}, Catalog: cutoffCatalog{}})
			h := NewHandler(svc, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
			mux := http.NewServeMux()
			mw := func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					next.ServeHTTP(w, r.WithContext(tenant.With(r.Context(), tenant.Identity{Profile: tenant.Profile{UserID: 1, Role: tc.role}})))
				})
			}
			h.Register(mux, mw, mw)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)))
			if w.Code != tc.want {
				t.Fatalf("%d %s", w.Code, w.Body)
			}
		})
	}
}
