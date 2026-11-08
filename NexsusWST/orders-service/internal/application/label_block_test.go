package application

import (
	"testing"

	"github.com/nexuswst/orders-service/internal/domain"
)

// Sin etiqueta no se entrega (Traspaso Órdenes A1).
func TestLabelBlock(t *testing.T) {
	cases := []struct {
		strategy, status string
		blocked          bool
	}{
		{domain.LabelStrategyAPI, domain.LabelReady, false},
		{domain.LabelStrategyOffice, domain.LabelOffice, false},
		{domain.LabelStrategyOffice, domain.LabelNone, false}, // oficina no lleva etiqueta de API
		{domain.LabelStrategyAPI, domain.LabelNone, true},
		{domain.LabelStrategyAPI, domain.LabelPending, true},
		{domain.LabelStrategyAPI, domain.LabelFailed, true},
		{domain.LabelStrategyAPI, "", true},
	}
	for _, c := range cases {
		got := labelBlock(domain.Order{LabelStrategy: c.strategy, LabelStatus: c.status}) != ""
		if got != c.blocked {
			t.Errorf("estrategia %q, etiqueta %q: bloqueada=%v, se esperaba %v", c.strategy, c.status, got, c.blocked)
		}
	}
}
