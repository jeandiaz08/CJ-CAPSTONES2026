package domain

import "testing"

func TestNormalizeRUT(t *testing.T) {
	for _, c := range []struct {
		in, want string
		ok       bool
	}{
		{"76.123.456-0", "76.123.456-0", true},
		{"76.123.456-7", "", false},
		{"11.111.111-1", "11.111.111-1", true},
		{"111111111", "11.111.111-1", true},
		{"76086428-5", "76.086.428-5", true},
		{"12.345.678-5", "12.345.678-5", true},
		{"10.000.013-k", "10.000.013-K", true},
		{"12.345.678-9", "", false},
		{"SEED-76.100.001-1", "", false},
		{"1-9", "", false},
		{"", "", false},
	} {
		got, ok := NormalizeRUT(c.in)
		if ok != c.ok || got != c.want {
			t.Errorf("NormalizeRUT(%q) = %q, %v; quería %q, %v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestDraftNormalize(t *testing.T) {
	am := " am "
	d := Draft{Name: "  Andes   Outdoor SpA ", TaxID: "760864285", Email: " Contacto@Andes.CL ", Phone: "+56 9 1234 5678", WaveCode: &am}
	if errs := d.Normalize(true); len(errs) > 0 {
		t.Fatalf("borrador válido rechazado: %v", errs)
	}
	if d.Name != "Andes Outdoor SpA" || d.TaxID != "76.086.428-5" || d.Email != "contacto@andes.cl" || *d.WaveCode != "AM" {
		t.Fatalf("normalización inesperada: %+v", d)
	}

	general := Draft{Name: "X", TaxID: "11.111.111-1"}
	if errs := general.Normalize(true); len(errs) > 0 || general.WaveCode != nil {
		t.Fatalf("sin ola es «General (según reglas)» (NULL): %v %v", errs, general.WaveCode)
	}

	bad := "ola mala!"
	b := Draft{Name: "", TaxID: "12.345.678-9", Email: "no-es-correo", Phone: "12", WaveCode: &bad}
	fields := map[string]bool{}
	for _, f := range b.Normalize(true) {
		fields[f.Field] = true
	}
	for _, f := range []string{"name", "taxId", "email", "phone", "waveCode"} {
		if !fields[f] {
			t.Errorf("falta el error del campo %q", f)
		}
	}

	// Al editar sin cambiar el RUT no se vuelve a validar el dígito (empresas antiguas).
	legacy := Draft{Name: "Demo", TaxID: "SEED-76.100.001-1"}
	if errs := legacy.Normalize(false); len(errs) > 0 {
		t.Fatalf("editar sin cambiar el RUT no debe validarlo: %v", errs)
	}
}

func TestNormalizeReason(t *testing.T) {
	if _, err := NormalizeReason(" corto ", true); err != nil {
		t.Fatal("5 caracteres bastan")
	}
	if _, err := NormalizeReason("no", true); err == nil {
		t.Fatal("el motivo de desactivar es obligatorio")
	}
	if r, err := NormalizeReason("", false); err != nil || r != "" {
		t.Fatal("al activar el motivo es opcional")
	}
}
