package application

import (
	"testing"

	"github.com/nexuswst/orders-service/internal/ports"
)

// Alcance y paginación de los manifiestos (Traspaso Órdenes A2).
func TestScopeManifest(t *testing.T) {
	base := func() ports.Manifest {
		return ports.Manifest{Lines: []ports.ManifestLine{{OrderID: 1, CompanyID: 10}, {OrderID: 2, CompanyID: 20}, {OrderID: 3, CompanyID: 10}}}
	}
	m := base()
	if !scopeManifest(&m, nil) || len(m.Lines) != 3 {
		t.Fatalf("sin alcance se ve todo: %+v", m.Lines)
	}
	m = base()
	if !scopeManifest(&m, []int64{10}) || len(m.Lines) != 2 || m.Lines[0].OrderID != 1 || m.Lines[1].OrderID != 3 {
		t.Fatalf("empresa 10: %+v", m.Lines)
	}
	orig := base()
	m = orig
	if scopeManifest(&m, []int64{30}) || len(m.Lines) != 0 {
		t.Fatalf("empresa sin órdenes en el manifiesto: %+v", m.Lines)
	}
	if len(orig.Lines) != 3 || orig.Lines[1].CompanyID != 20 {
		t.Fatalf("no debe pisar las líneas originales: %+v", orig.Lines)
	}
}

func TestClampPage(t *testing.T) {
	for _, c := range []struct{ limit, offset, wantLimit, wantOffset int }{
		{0, 0, 20, 0}, {-5, -3, 20, 0}, {50, 40, 50, 40}, {500, 0, 100, 0},
	} {
		if l, o := clampPage(c.limit, c.offset); l != c.wantLimit || o != c.wantOffset {
			t.Errorf("clampPage(%d,%d) = %d,%d; se esperaba %d,%d", c.limit, c.offset, l, o, c.wantLimit, c.wantOffset)
		}
	}
}
