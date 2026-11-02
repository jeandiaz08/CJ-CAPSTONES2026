package application

import (
	"context"
	"errors"
	"testing"

	"github.com/nexuswst/orders-service/internal/domain"
	"github.com/nexuswst/orders-service/internal/ports"
)

type resolveOrders struct {
	ports.OrderRepository
	order           domain.Order
	saved           *domain.Order
	diff            []string
	opts            ports.TransitionOpts
	savedResolution bool
}

func (f *resolveOrders) Get(context.Context, []int64, int64) (domain.Order, error) {
	return f.order, nil
}

func (f *resolveOrders) SaveResolution(_ context.Context, _, after domain.Order, _ domain.Status, diff []string, _, _ string, opts ports.TransitionOpts) (domain.Order, error) {
	f.saved, f.diff, f.opts, f.savedResolution = &after, diff, opts, true
	return after, nil
}

type resolveCatalog struct{ ports.CatalogReader }

func (resolveCatalog) Communes(context.Context) ([]string, error) {
	return []string{"Ñuñoa", "Maipú"}, nil
}
func (resolveCatalog) CourierActive(_ context.Context, code string) (bool, error) {
	return code != "inactivo", nil
}

type resolveRules struct{ ports.RuleRepository }

func (resolveRules) CourierSettings(_ context.Context, courier string) (ports.CourierSettings, error) {
	switch courier {
	case "sin_cobertura":
		return ports.CourierSettings{Courier: courier, Communes: []string{"Maipú"}}, nil
	case "un_bulto":
		limit := 1
		return ports.CourierSettings{Courier: courier, CoversAll: true, MaxPackages: &limit}, nil
	}
	return ports.CourierSettings{Courier: courier, CoversAll: true}, nil
}

type resolveNotifications struct{ ports.NotificationRepository }

func resolveService(o domain.Order) (*Service, *resolveOrders) {
	orders := &resolveOrders{order: o}
	return NewService(Deps{Orders: orders, Catalog: resolveCatalog{}, Rules: resolveRules{}, Notifier: silentNotifier{}, Notifications: resolveNotifications{}}), orders
}

func incident(prev domain.Status, wave string) domain.Order {
	return domain.Order{ID: 1, CompanyID: 7, ExternalRef: "REF-1", Courier: "starken", CourierCode: "OLD-1", Status: domain.StatusIncident, PrevStatus: prev,
		IncidentKind: domain.IncidentCourierCover, WaveID: wave, PackagesCount: 2, DispatchType: "domicilio", Channel: "manual",
		Destination: domain.Destination{RecipientName: "Ana", AddressLine: "Calle 123", Commune: "Ñuñoa"},
		Items:       []domain.Item{{SKU: "AO-1", Quantity: 1}}}
}

func text(s string) *string { return &s }

func TestResolveCorrections(t *testing.T) {
	for _, tc := range []struct {
		name         string
		order        domain.Order
		corrections  domain.Corrections
		wantStatus   domain.Status
		wantRelease  bool
		wantCourier  string
		wantCommune  string
		wantCodeGone bool
	}{
		{"Lista para retiro vuelve a su etapa con otro courier", incident(domain.StatusReadyPickup, "OLA"), domain.Corrections{Courier: text("recibelo")}, domain.StatusReadyPickup, false, "recibelo", "Ñuñoa", true},
		{"en preparación se corrige sin revalidar", incident(domain.StatusPacking, "OLA"), domain.Corrections{AddressLine: text("Calle 456"), Commune: text("maipu")}, domain.StatusPacking, false, "starken", "Maipú", false},
		{"Validada en ola no revalida", incident(domain.StatusValidated, "OLA"), domain.Corrections{AddressLine: text("Calle 456")}, domain.StatusValidated, false, "starken", "Ñuñoa", false},
		{"Validada sin ola revalida y libera stock", incident(domain.StatusValidated, ""), domain.Corrections{AddressLine: text("Calle 456")}, domain.StatusPending, true, "starken", "Ñuñoa", false},
		{"Pendiente revalida", incident(domain.StatusPending, ""), domain.Corrections{Commune: text("Maipú")}, domain.StatusPending, false, "starken", "Maipú", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, orders := resolveService(tc.order)
			out, err := s.Resolve(context.Background(), admin(), nil, 1, ResolveResume, "Corregido", &tc.corrections)
			if err != nil {
				t.Fatal(err)
			}
			if out.Status != tc.wantStatus || out.PrevStatus != "" || out.IncidentKind != "" {
				t.Fatalf("estado: %s (prev %s, tipo %s), esperado %s", out.Status, out.PrevStatus, out.IncidentKind, tc.wantStatus)
			}
			if out.Courier != tc.wantCourier || out.Commune != tc.wantCommune {
				t.Fatalf("datos: %s, %s", out.Courier, out.Commune)
			}
			if orders.opts.ReleaseCommitted != tc.wantRelease {
				t.Fatalf("liberar stock: %v", orders.opts.ReleaseCommitted)
			}
			if tc.wantCodeGone != (out.CourierCode == "") {
				t.Fatalf("código del courier: %q", out.CourierCode)
			}
			if len(orders.diff) == 0 {
				t.Fatal("el cambio debe quedar en el historial")
			}
		})
	}
}

func TestResolveCorrectionsRejected(t *testing.T) {
	for _, tc := range []struct {
		name        string
		corrections domain.Corrections
		field       string
	}{
		{"courier inactivo", domain.Corrections{Courier: text("inactivo")}, "courier"},
		{"courier sin cobertura", domain.Corrections{Courier: text("sin_cobertura")}, "courier"},
		{"courier no admite los bultos", domain.Corrections{Courier: text("un_bulto")}, "courier"},
		{"comuna desconocida", domain.Corrections{Commune: text("Gotham")}, "commune"},
		{"dirección sin número", domain.Corrections{AddressLine: text("Calle sin numero")}, "addressLine"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, orders := resolveService(incident(domain.StatusReadyPickup, "OLA"))
			_, err := s.Resolve(context.Background(), admin(), nil, 1, ResolveResume, "Corregido", &tc.corrections)
			var problems domain.ValidationErrors
			if !errors.As(err, &problems) {
				t.Fatalf("debe responder por campo: %v", err)
			}
			if problems[0].Field != tc.field {
				t.Fatalf("campo: %+v", problems)
			}
			if orders.savedResolution {
				t.Fatal("no debe guardar")
			}
		})
	}
}

func TestResolveCorrectionsRules(t *testing.T) {
	s, orders := resolveService(incident(domain.StatusReadyPickup, "OLA"))
	if _, err := s.Resolve(context.Background(), admin(), nil, 1, ResolveResume, "Sin cambios", &domain.Corrections{Courier: text("starken")}); err == nil {
		t.Fatal("una corrección sin cambios debe rechazarse")
	}
	if _, err := s.Resolve(context.Background(), admin(), nil, 1, ResolveRetry, "x", &domain.Corrections{Courier: text("recibelo")}); err == nil {
		t.Fatal("las correcciones solo van con resume")
	}
	if orders.savedResolution {
		t.Fatal("no debe guardar")
	}
}
