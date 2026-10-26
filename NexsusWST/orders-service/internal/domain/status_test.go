package domain

import (
	"errors"
	"testing"
)

func validDraft() Draft {
	return Draft{CompanyID: 1, ExternalRef: "ML-123", Channel: "mercadolibre", Courier: "chilexpress",
		Destination: Destination{RecipientName: "Ana Pérez", AddressLine: "Av. Siempre Viva 742", Commune: "Ñuñoa"},
		Items:       []Item{{SKU: "sku-1", Quantity: 2}}}
}

func newOrder(t *testing.T) Order {
	o, err := NewOrder(validDraft(), "manual")
	if err != nil {
		t.Fatal(err)
	}
	return o
}

func TestHappyPath(t *testing.T) {
	o := newOrder(t)
	for _, ev := range []Event{EvStartValidation, EvValidate, EvWaveConfirmed, EvPickConfirmed, EvStartPacking, EvPacked, EvHandOver} {
		if _, err := o.Apply(ev); err != nil {
			t.Fatalf("evento %s desde %s: %v", ev, o.Status, err)
		}
	}
	if o.Status != StatusHandedOver {
		t.Fatalf("estado final %s", o.Status)
	}
}

// "Una orden no pasa de pendiente a entregada por crear una etiqueta" (DAS PA-04)
func TestRejectsSkippingStates(t *testing.T) {
	o := newOrder(t)
	if _, err := o.Apply(EvHandOver); err != ErrInvalidTransition {
		t.Fatal("pending -> handed_over debe rechazarse")
	}
	if o.Status != StatusPending {
		t.Fatal("el estado no debe cambiar ante una transición rechazada")
	}
}

// Incidencia vuelve al estado donde ocurrió (ERS §3.2.7)
func TestIncidentReturnsToPreviousState(t *testing.T) {
	o := newOrder(t)
	for _, ev := range []Event{EvStartValidation, EvValidate, EvWaveConfirmed, EvPickConfirmed, EvStartPacking} {
		_, _ = o.Apply(ev)
	}
	if _, err := o.Apply(EvIncident); err != nil || o.Status != StatusIncident || o.PrevStatus != StatusPacking {
		t.Fatalf("incidencia mal registrada: %+v %v", o, err)
	}
	if _, err := o.Apply(EvResolve); err != nil || o.Status != StatusPacking {
		t.Fatalf("debía volver a En empaque, quedó %s (%v)", o.Status, err)
	}
}

// Sin stock -> Anulada (no Incidencia); Anulada es recuperable
func TestVoidedIsRecoverable(t *testing.T) {
	o := newOrder(t)
	_, _ = o.Apply(EvStartValidation)
	if _, err := o.Apply(EvVoid); err != nil || o.Status != StatusVoided {
		t.Fatal("sin stock debe pasar a Anulada")
	}
	if _, err := o.Apply(EvRevalidate); err != nil || o.Status != StatusPending {
		t.Fatal("Anulada debe poder corregirse y volver a Pendiente")
	}
}

func TestCancelledIsFinal(t *testing.T) {
	o := newOrder(t)
	_, _ = o.Apply(EvCancel)
	for _, ev := range []Event{EvResolve, EvRevalidate, EvStartValidation} {
		if _, err := o.Apply(ev); err != ErrInvalidTransition {
			t.Fatalf("una orden cancelada no acepta %s", ev)
		}
	}
}

func TestNewOrderValidatesItems(t *testing.T) {
	d := validDraft()
	d.Items = nil
	if _, err := NewOrder(d, "manual"); !errors.Is(err, ErrInvalid) {
		t.Fatal("orden sin productos debe rechazarse")
	}
	d = validDraft()
	d.Items = []Item{{SKU: "A", Quantity: 0}}
	if _, err := NewOrder(d, "manual"); !errors.Is(err, ErrInvalid) {
		t.Fatal("cantidad 0 debe rechazarse")
	}
}

func TestDraftValidation(t *testing.T) {
	d := validDraft()
	d.Items = append(d.Items, Item{SKU: " SKU-1 ", Quantity: 3}) // mismo SKU: se suma
	if errs := d.Normalize(); errs != nil {
		t.Fatalf("borrador válido rechazado: %v", errs)
	}
	if len(d.Items) != 1 || d.Items[0].Quantity != 5 || d.Items[0].SKU != "SKU-1" {
		t.Fatalf("líneas mal normalizadas: %+v", d.Items)
	}
	bad := Draft{CompanyID: 1, Courier: "", Destination: Destination{AddressLine: "sin numero"}, Items: []Item{{SKU: "", Quantity: 0}}}
	errs := bad.Normalize()
	fields := map[string]bool{}
	for _, e := range errs {
		fields[e.Field] = true
	}
	for _, f := range []string{"courier", "recipientName", "addressLine", "commune", "items[0]"} {
		if !fields[f] {
			t.Errorf("faltó error en %s: %v", f, errs)
		}
	}
	if !errors.Is(errs, ErrInvalid) {
		t.Error("ValidationErrors debe envolver ErrInvalid")
	}
	m := Draft{CompanyID: 1, Courier: "starken", Destination: validDraft().Destination, Items: validDraft().Items}
	if errs := m.Normalize(); errs != nil || m.ExternalRef == "" || m.Channel != "manual" {
		t.Fatalf("orden manual sin referencia debe generar una: %+v %v", m, errs)
	}
}

func TestEditRules(t *testing.T) {
	o := newOrder(t)
	courier := "starken"
	n, diff, err := o.ApplyChanges(Changes{Courier: &courier})
	if err != nil || n.Courier != "starken" || len(diff) != 1 || !o.NeedsRevalidation(n) {
		t.Fatalf("edición en Pendiente: %v %v", diff, err)
	}
	_, _ = o.Apply(EvStartValidation)
	_, _ = o.Apply(EvValidate)
	o.WaveID = "OLA-1" // ya pedida a Invas
	if _, _, err := o.ApplyChanges(Changes{Courier: &courier}); !errors.Is(err, ErrNotEditable) {
		t.Fatal("una orden con ola solicitada no se edita desde Nexus")
	}
	_, _ = o.Apply(EvWaveConfirmed)
	if _, _, err := o.ApplyChanges(Changes{Courier: &courier}); !errors.Is(err, ErrNotEditable) {
		t.Fatal("En preparación no es editable")
	}
	// Incidencia en Lista para retiro: se corrige courier, no productos
	for _, ev := range []Event{EvPickConfirmed, EvStartPacking, EvPacked} {
		_, _ = o.Apply(ev)
	}
	_, _ = o.Incident(IncidentCourierCover)
	if _, _, err := o.ApplyChanges(Changes{Courier: &courier}); err != nil {
		t.Fatalf("debe poder cambiar el courier en la incidencia: %v", err)
	}
	items := []Item{{SKU: "OTRO", Quantity: 1}}
	if _, _, err := o.ApplyChanges(Changes{Items: &items}); !errors.Is(err, ErrNotEditable) {
		t.Fatal("no se cambian productos de una orden ya preparada")
	}
}

func TestCommuneMatching(t *testing.T) {
	cat := []string{"Ñuñoa", "Providencia", "Maipú", "Las Condes"}
	if off, _ := MatchCommune("nunoa", cat); off != "Ñuñoa" {
		t.Fatalf("sin tildes debe coincidir: %q", off)
	}
	if off, sug := MatchCommune("Providensia", cat); off != "" || sug != "Providencia" {
		t.Fatalf("debe sugerir Providencia: %q %q", off, sug)
	}
	if off, sug := MatchCommune("Tokio", cat); off != "" || sug != "" {
		t.Fatalf("no debe sugerir nada: %q %q", off, sug)
	}
}

func TestPackingDiscrepancies(t *testing.T) {
	exp := []Item{{"A", 2}, {"B", 1}}
	if d := PackingDiscrepancies(exp, []Item{{"b", 1}, {"A", 2}}); len(d) != 0 {
		t.Fatalf("debía coincidir: %v", d)
	}
	d := PackingDiscrepancies(exp, []Item{{"A", 1}, {"C", 1}})
	if len(d) != 3 {
		t.Fatalf("esperaba falta A, falta B y C ajeno: %v", d)
	}
}
