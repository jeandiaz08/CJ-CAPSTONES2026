package application_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/nexuswst/backend-core/internal/modules/products/application"
	"github.com/nexuswst/backend-core/internal/modules/products/domain"
)

// Pruebas de la composición de kits (H-04-T01 / SCRUM-100).

func kitFixture(t *testing.T) (*application.Service, *fakeRepo, domain.Product) {
	t.Helper()
	repo := &fakeRepo{}
	svc := application.NewService(repo)
	ctx := context.Background()
	mk := func(cid int64, sku, typ string) domain.Product {
		p, err := svc.Create(ctx, cid, application.CreateInput{SKU: sku, Description: "Producto " + sku, Type: typ})
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	kit := mk(1, "KIT-1", "kit")
	mk(1, "A", "simple")
	mk(1, "B", "simple")
	mk(1, "KIT-2", "kit")
	mk(2, "OTRA", "simple")
	off := mk(1, "INACTIVO", "simple")
	repo.items[off.ID-1].Active = false
	return svc, repo, kit
}

func TestSetComponentsOK(t *testing.T) {
	svc, repo, kit := kitFixture(t)
	_, comps, err := svc.SetComponents(context.Background(), nil, kit.ID,
		[]domain.ComponentInput{{SKU: " a ", Quantity: 2}, {SKU: "B", Quantity: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if len(comps) != 2 || comps[0].SKU != "A" || comps[0].Quantity != 2 {
		t.Fatalf("composición inesperada: %+v", comps)
	}
	if len(repo.comps[kit.ID]) != 2 {
		t.Fatal("no se guardó la composición")
	}
}

func TestSetComponentsRejects(t *testing.T) {
	cases := map[string]struct {
		in   []domain.ComponentInput
		want string
	}{
		"vacío":        {nil, "al menos un componente"},
		"cantidad 0":   {[]domain.ComponentInput{{SKU: "A", Quantity: 0}}, "mayor que 0"},
		"repetido":     {[]domain.ComponentInput{{SKU: "A", Quantity: 1}, {SKU: "a", Quantity: 1}}, "repetido"},
		"a sí mismo":   {[]domain.ComponentInput{{SKU: "KIT-1", Quantity: 1}}, "sí mismo"},
		"kit en kit":   {[]domain.ComponentInput{{SKU: "KIT-2", Quantity: 1}}, "kits dentro de kits"},
		"otra empresa": {[]domain.ComponentInput{{SKU: "OTRA", Quantity: 1}}, "no existe"},
		"inactivo":     {[]domain.ComponentInput{{SKU: "INACTIVO", Quantity: 1}}, "inactivo"},
		"no existe":    {[]domain.ComponentInput{{SKU: "ZZZ", Quantity: 1}}, "no existe"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			svc, repo, kit := kitFixture(t)
			_, _, err := svc.SetComponents(context.Background(), nil, kit.ID, c.in)
			if !errors.Is(err, domain.ErrInvalidComponent) || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("se esperaba error con %q, llegó %v", c.want, err)
			}
			if len(repo.comps[kit.ID]) != 0 {
				t.Fatal("una composición inválida no debe guardarse")
			}
		})
	}
}

func TestSetComponentsOnlyKits(t *testing.T) {
	svc, _, _ := kitFixture(t)
	// id 2 = producto simple "A"
	if _, _, err := svc.SetComponents(context.Background(), nil, 2, []domain.ComponentInput{{SKU: "B", Quantity: 1}}); !errors.Is(err, domain.ErrNotKit) {
		t.Fatalf("se esperaba ErrNotKit, llegó %v", err)
	}
}

// Prueba negativa de aislamiento: un cliente de la empresa 2 no ve ni edita el kit de la 1.
func TestKitCompanyIsolation(t *testing.T) {
	svc, _, kit := kitFixture(t)
	ctx := context.Background()
	if _, _, err := svc.Components(ctx, []int64{2}, kit.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("la empresa 2 no debe ver el kit: %v", err)
	}
	if _, _, err := svc.SetComponents(ctx, []int64{2}, kit.ID, []domain.ComponentInput{{SKU: "OTRA", Quantity: 1}}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("la empresa 2 no debe editar el kit: %v", err)
	}
}
