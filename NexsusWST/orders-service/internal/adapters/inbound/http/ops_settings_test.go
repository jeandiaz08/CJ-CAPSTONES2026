package http

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nexuswst/orders-service/internal/application"
	"github.com/nexuswst/orders-service/internal/domain"
	"github.com/nexuswst/orders-service/internal/ports"
	"github.com/nexuswst/orders-service/internal/shared/tenant"
)

type settingsFake struct {
	ports.RuleRepository
	deadline string
	settings ports.CourierSettings
	schedule domain.OperationSchedule
}

func (f *settingsFake) AssignmentDeadline(context.Context) (string, error) { return f.deadline, nil }

func (f *settingsFake) SaveAssignmentDeadline(_ context.Context, v, _ string) error {
	f.deadline = v
	return nil
}

func (f *settingsFake) OperationSchedule(context.Context) (domain.OperationSchedule, error) {
	if f.schedule.OperationStart == "" {
		f.schedule = domain.OperationSchedule{AssignmentDeadline: f.deadline, OperationStart: "06:30", OperationDays: []string{"mon", "tue"}}
	}
	return f.schedule, nil
}

func (f *settingsFake) SaveOperationSchedule(_ context.Context, s domain.OperationSchedule, _ string) error {
	f.schedule = s
	return nil
}

func (f *settingsFake) Cutoffs(context.Context) ([]ports.Cutoff, error) {
	return []ports.Cutoff{{ID: 1, CutoffTime: "11:00"}}, nil
}

func (f *settingsFake) ListCourierSettings(context.Context) ([]ports.CourierSettings, error) {
	return []ports.CourierSettings{f.settings}, nil
}

func (f *settingsFake) SaveCourierSettings(_ context.Context, s ports.CourierSettings, _ string) error {
	f.settings = s
	return nil
}

type catalogFake struct{ ports.CatalogReader }

func (catalogFake) CourierActive(context.Context, string) (bool, error) { return true, nil }

type ordersFake struct {
	ports.OrderRepository
	filter     ports.ListFilter
	summaryIDs []int64
}

func (f *ordersFake) List(_ context.Context, q ports.ListFilter) ([]domain.Order, int, error) {
	f.filter = q
	v := "en_cola"
	return []domain.Order{{ID: 1, Status: domain.StatusValidated, WaveID: "OLA-AM", AssignmentSituation: &v}}, 1, nil
}

func TestSettingsHandlers(t *testing.T) {
	f := &settingsFake{deadline: "12:00"}
	orders := &ordersFake{}
	svc := application.NewService(application.Deps{Rules: f, Catalog: catalogFake{}, Orders: orders})
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewHandler(svc, nil, nil, log, nil)
	for _, tc := range []struct {
		method, path, body, role string
		want                     int
	}{
		{"GET", "/v1/ops-settings/assignment-deadline", "", "admin", 200},
		{"PUT", "/v1/ops-settings/assignment-deadline", `{"assignmentDeadline":"13:05"}`, "jefe_operaciones", 204},
		{"PUT", "/v1/ops-settings/assignment-deadline", `{"assignmentDeadline":"24:00"}`, "admin", 422},
		{"PUT", "/v1/ops-settings/assignment-deadline", `{"assignmentDeadline":"10:59"}`, "admin", 422},
		{"GET", "/v1/ops-settings/operation", "", "jefe_operaciones", 200},
		{"GET", "/v1/ops-settings/operation", "", "cliente", 403},
		{"PUT", "/v1/ops-settings/operation", `{"operationStart":"07:00","operationDays":["sun","mon"]}`, "jefe_operaciones", 204},
		{"PUT", "/v1/ops-settings/operation", `{"operationStart":"11:00","operationDays":["mon"]}`, "admin", 422},
		{"PUT", "/v1/ops-settings/operation", `{"operationStart":"07:00","operationDays":[]}`, "admin", 422},
		{"PUT", "/v1/ops-settings/operation", `{"operationStart":"07:00","operationDays":["lunes"]}`, "admin", 422},
		{"PUT", "/v1/ops-settings/operation", `{"operationStart":"7","operationDays":["mon"]}`, "admin", 422},
		{"PUT", "/v1/ops-settings/operation", `{"operationStart":"07:00","operationDays":["mon"]}`, "cliente", 403},
		{"PUT", "/v1/ops-settings/assignment-deadline", `{"assignmentDeadline":"9:00"}`, "admin", 422},
		{"GET", "/v1/ops-settings/assignment-deadline", "", "cliente", 403},
		{"PUT", "/v1/ops-settings/assignment-deadline", `{"assignmentDeadline":"12:00"}`, "empaque", 403},
		{"PUT", "/v1/couriers/blueexpress/settings", `{"arrivalTime":"10:00","cutoffTime":"09:00","coversAll":true}`, "admin", 400},
		{"GET", "/v1/couriers/settings", "", "admin", 200},
		{"PUT", "/v1/couriers/blueexpress/settings", `{"arrivalTime":"10:00","cutoffTime":"10:00","coversAll":true}`, "admin", 400},
		{"PUT", "/v1/couriers/blueexpress/settings", `{"arrivalTime":"10:00","cutoffTime":"11:00","coversAll":true}`, "admin", 400},
		{"PUT", "/v1/couriers/blueexpress/settings", `{"cutoffTime":"25:00","coversAll":true}`, "admin", 400},
		{"PUT", "/v1/couriers/blueexpress/settings", `{"cutoffTime":"09:00","coversAll":true}`, "cliente", 400},
		{"GET", "/v1/orders?view=asignacion&without_wave=true", "", "admin", 200},
	} {
		t.Run(tc.method+tc.path+tc.role+tc.body, func(t *testing.T) {
			mux := http.NewServeMux()
			mw := func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					next.ServeHTTP(w, r.WithContext(tenant.With(r.Context(), tenant.Identity{Profile: tenant.Profile{UserID: 1, Role: tc.role, AllCompanies: true}})))
				})
			}
			h.Register(mux, mw, mw)
			r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("status %d: %s", w.Code, w.Body)
			}
			if tc.path == "/v1/couriers/settings" && strings.Contains(w.Body.String(), `"cutoffTime"`) {
				t.Fatal(w.Body)
			}
			if strings.Contains(tc.path, "view=asignacion") {
				if orders.filter.View != ports.ViewAssignment || !orders.filter.WithoutWave {
					t.Fatal(orders.filter)
				}
				if !strings.Contains(w.Body.String(), `"assignmentSituation":"en_cola"`) {
					t.Fatal(w.Body)
				}
			}
		})
	}
	if f.deadline != "13:05" {
		t.Fatal(f.deadline)
	}
	if f.schedule.OperationStart != "07:00" || strings.Join(f.schedule.OperationDays, ",") != "mon,sun" {
		t.Fatalf("horario guardado: %+v", f.schedule)
	}
}
