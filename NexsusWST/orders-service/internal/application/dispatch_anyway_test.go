package application

import (
	"context"
	"errors"
	"testing"

	"github.com/nexuswst/orders-service/internal/domain"
	"github.com/nexuswst/orders-service/internal/ports"
	"github.com/nexuswst/orders-service/internal/shared/tenant"
)

func who(role string, perms ...string) tenant.Identity {
	return tenant.Identity{Profile: tenant.Profile{UserID: 7, Name: "Ana", Role: role, AllCompanies: true, Permissions: perms}}
}

func pk(code string, scanned bool) ports.ManifestBundle {
	return ports.ManifestBundle{Code: code, Scanned: scanned}
}

func testManifest() ports.Manifest {
	return ports.Manifest{ID: 9, Courier: "starken", Status: "open", Lines: []ports.ManifestLine{
		{OrderID: 1, CompanyID: 10, Status: "included", Packages: []ports.ManifestBundle{pk("NX-1-1", true), pk("NX-1-2", false), pk("NX-1-3", true)}},
		{OrderID: 2, CompanyID: 10, Status: "included", Packages: []ports.ManifestBundle{pk("NX-2-1", true)}},
		{OrderID: 3, CompanyID: 20, Status: "included", Packages: []ports.ManifestBundle{pk("NX-3-1", false)}},
		{OrderID: 4, CompanyID: 20, Status: "moved", MovedTo: "chilexpress", Packages: []ports.ManifestBundle{pk("NX-4-1", false)}},
	}}
}

func TestSplitPackages(t *testing.T) {
	sc, mi := splitPackages(testManifest().Lines[0].Packages)
	if len(sc) != 2 || sc[0] != "NX-1-1" || sc[1] != "NX-1-3" || len(mi) != 1 || mi[0] != "NX-1-2" {
		t.Fatalf("escaneados %v faltantes %v", sc, mi)
	}
}

func TestCheckDispatchAnyway(t *testing.T) {
	m := testManifest()
	jefe := who(tenant.RoleJefeOps, tenant.PermDespacho)
	cases := []struct {
		name string
		id   tenant.Identity
		in   []DispatchAnyway
		want error
	}{
		{"sin autorizaciones no exige rol", who(tenant.RoleEmpaque, tenant.PermDespacho), nil, nil},
		{"jefe con motivo", jefe, []DispatchAnyway{{OrderID: 1, Reason: "bulto 2 dañado, sale mañana"}}, nil},
		// Lo decide el permiso Despacho, no el perfil.
		{"empaque con permiso Despacho", who(tenant.RoleEmpaque, tenant.PermDespacho), []DispatchAnyway{{OrderID: 1, Reason: "se reenvía"}}, nil},
		{"empaque sin permiso Despacho", who(tenant.RoleEmpaque), []DispatchAnyway{{OrderID: 1, Reason: "se reenvía"}}, domain.ErrForbidden},
		{"jefe sin permiso Despacho", who(tenant.RoleJefeOps), []DispatchAnyway{{OrderID: 1, Reason: "se reenvía"}}, domain.ErrForbidden},
		{"admin sin permiso Despacho", who(tenant.RoleAdmin), []DispatchAnyway{{OrderID: 1, Reason: "se reenvía"}}, domain.ErrForbidden},
		{"motivo vacío", jefe, []DispatchAnyway{{OrderID: 1, Reason: "   "}}, domain.ErrInvalid},
		{"motivo muy corto", jefe, []DispatchAnyway{{OrderID: 1, Reason: "ok"}}, domain.ErrInvalid},
		{"orden fuera del manifiesto", jefe, []DispatchAnyway{{OrderID: 99, Reason: "se reenvía"}}, domain.ErrInvalid},
		{"orden movida a otro courier", jefe, []DispatchAnyway{{OrderID: 4, Reason: "se reenvía"}}, domain.ErrInvalid},
		{"orden completa no necesita", jefe, []DispatchAnyway{{OrderID: 2, Reason: "se reenvía"}}, domain.ErrInvalid},
		{"ningún bulto escaneado", jefe, []DispatchAnyway{{OrderID: 3, Reason: "se reenvía"}}, domain.ErrInvalid},
		{"orden repetida", jefe, []DispatchAnyway{{OrderID: 1, Reason: "se reenvía"}, {OrderID: 1, Reason: "otra vez"}}, domain.ErrInvalid},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := checkDispatchAnyway(c.id, m, c.in)
			if c.want == nil {
				if err != nil {
					t.Fatalf("error inesperado: %v", err)
				}
				if len(got) != len(c.in) {
					t.Fatalf("autorizadas %v", got)
				}
				return
			}
			if !errors.Is(err, c.want) {
				t.Fatalf("quería %v, llegó %v", c.want, err)
			}
		})
	}
}

type nameCatalog struct{ ports.CatalogReader }

func (nameCatalog) CompanyName(_ context.Context, id int64) (string, error) {
	return map[int64]string{10: "Tienda Uno", 20: "Tienda Dos"}[id], nil
}

func TestSummarizeManifest(t *testing.T) {
	s := &Service{Deps: Deps{Catalog: nameCatalog{}}}
	m := testManifest()
	s.summarize(context.Background(), &m)
	sum := m.Summary
	// la orden 4 se movió a otro courier: no cuenta
	if sum.Orders != 3 || sum.Packages != 5 || sum.Scanned != 3 {
		t.Fatalf("totales %+v", sum)
	}
	if len(sum.Companies) != 2 || sum.Companies[0].Name != "Tienda Uno" || sum.Companies[0].Orders != 2 || sum.Companies[0].Packages != 4 ||
		sum.Companies[1].Orders != 1 || sum.Companies[1].Packages != 1 {
		t.Fatalf("empresas %+v", sum.Companies)
	}
}
