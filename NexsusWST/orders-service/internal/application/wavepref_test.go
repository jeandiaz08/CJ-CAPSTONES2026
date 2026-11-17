package application

import (
	"slices"
	"strings"
	"testing"

	"github.com/nexuswst/orders-service/internal/domain"
)

// PV-19 (H-10-T11): producto → cliente → reglas → sin regla que calce.
func TestEngineWaveByProductAndClient(t *testing.T) {
	now := when(2, 9, 0) // lunes, operación abierta
	orders := []domainOrder{
		{1, "producto con ola FRIO gana a la regla"},
		{2, "olas de producto distintas → ola del cliente AM"},
		{3, "sin ola de producto → ola del cliente AM"},
		{4, "sin ola de producto ni cliente → regla (GEN)"},
		{5, "ola de producto que hoy no existe en Invas → no se asigna"},
	}
	cat := queueCatalog{
		products: map[int64][]string{1: {"FRIO"}, 2: {"AM", "FRIO"}, 5: {"NOCHE"}},
		company:  map[int64]string{2: "AM", 3: "AM"},
	}
	e, repo := engineWith(now, schedule(), cat, false, toOrders(orders)...)
	run(t, e)
	want := map[int64]string{1: "OLA-FRIO", 2: "OLA-AM", 3: "OLA-AM", 4: "OLA-GEN"}
	for id, w := range want {
		if repo.waves[id] != w {
			t.Errorf("orden %d (%s): ola %q, esperada %q", id, orders[id-1].desc, repo.waves[id], w)
		}
	}
	if _, ok := repo.waves[5]; ok {
		t.Fatal("la orden 5 no debe asignarse a una ola inexistente")
	}
	if !slices.Contains(repo.checked, 5) || len(repo.events) != 1 || !strings.Contains(repo.events[0], "ficha del producto pide la ola NOCHE") {
		t.Fatalf("la orden 5 debe quedar revisada con aviso: checked=%v events=%v", repo.checked, repo.events)
	}
	if !strings.Contains(repo.reasons[2], "olas distintas") || !strings.Contains(repo.reasons[1], "Ola del producto") ||
		!strings.Contains(repo.reasons[3], "Ola por defecto del cliente") || !strings.Contains(repo.reasons[4], "Regla «Todas»") {
		t.Fatalf("motivos en el historial: %v", repo.reasons)
	}
}

// Sin reglas activas, la ola del cliente igual se aplica; sin nada, queda para el Jefe.
func TestEngineWaveWithoutRules(t *testing.T) {
	cat := queueCatalog{company: map[int64]string{1: "AM"}}
	e, repo := engineWith(when(2, 9, 0), schedule(), cat, true, toOrders([]domainOrder{{1, "cliente AM"}, {2, "nada"}})...)
	run(t, e)
	if repo.waves[1] != "OLA-AM" {
		t.Fatalf("orden 1 debía ir a AM: %v", repo.waves)
	}
	if _, ok := repo.waves[2]; ok || !slices.Contains(repo.checked, 2) {
		t.Fatalf("orden 2 debía quedar sin ola y revisada: waves=%v checked=%v", repo.waves, repo.checked)
	}
}

type domainOrder struct {
	id   int64
	desc string
}

func toOrders(in []domainOrder) []domain.Order {
	out := make([]domain.Order, 0, len(in))
	for _, o := range in {
		out = append(out, queueOrder(o.id, "tarde", false, when(2, 8, 0)))
	}
	return out
}
