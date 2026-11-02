package http

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/nexuswst/orders-service/internal/application"
	"github.com/nexuswst/orders-service/internal/domain"
	"github.com/nexuswst/orders-service/internal/shared/tenant"
)

func (f *ordersFake) Summary(_ context.Context, ids []int64) (map[domain.Status]int, error) {
	f.summaryIDs = ids
	return map[domain.Status]int{domain.StatusPending: 1}, nil
}

func TestListFiltersSummaryAndCourierSettings(t *testing.T) {
	settings := &settingsFake{deadline: "12:00"}
	orders := &ordersFake{}
	svc := application.NewService(application.Deps{Rules: settings, Catalog: catalogFake{}, Orders: orders})
	h := NewHandler(svc, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	mux := http.NewServeMux()
	mw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(tenant.With(r.Context(), tenant.Identity{Profile: tenant.Profile{UserID: 1, Role: tenant.RoleAdmin, AllCompanies: true}})))
		})
	}
	h.Register(mux, mw, mw)
	call := func(method, path, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
		return w
	}

	if w := call("GET", "/v1/orders?source=bulk&situation=sin_regla", ""); w.Code != 200 {
		t.Fatalf("filtros válidos: %d %s", w.Code, w.Body)
	}
	if orders.filter.Source != "bulk" || orders.filter.Situation != "sin_regla" {
		t.Fatalf("filtros no llegan al repositorio: %+v", orders.filter)
	}
	for _, path := range []string{"/v1/orders?source=otro", "/v1/orders?situation=otra", "/v1/orders/export?source=otro"} {
		if w := call("GET", path, ""); w.Code != 422 {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body)
		}
	}

	if w := call("GET", "/v1/orders/summary?company_id=7", ""); w.Code != 200 {
		t.Fatalf("resumen: %d %s", w.Code, w.Body)
	}
	if !slices.Equal(orders.summaryIDs, []int64{7}) {
		t.Fatalf("el resumen ignora company_id: %v", orders.summaryIDs)
	}

	if w := call("PUT", "/v1/couriers/blueexpress/settings", `{"arrivalTime":"10:00","coversAll":true}`); w.Code != 400 {
		t.Fatalf("arrivalTime debe rechazarse: %d %s", w.Code, w.Body)
	}
	if w := call("PUT", "/v1/couriers/blueexpress/settings", `{"coversAll":true}`); w.Code != 204 {
		t.Fatalf("ajustes sin arrivalTime: %d %s", w.Code, w.Body)
	}
	if w := call("GET", "/v1/couriers/settings", ""); w.Code != 200 || strings.Contains(w.Body.String(), "arrivalTime") {
		t.Fatalf("GET expone arrivalTime: %d %s", w.Code, w.Body)
	}
}
