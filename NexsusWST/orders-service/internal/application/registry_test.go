package application

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/nexuswst/orders-service/internal/domain"
	"github.com/nexuswst/orders-service/internal/ports"
	"github.com/nexuswst/orders-service/internal/shared/tenant"
)

func TestParseActor(t *testing.T) {
	for _, c := range []struct {
		raw, role, name, kind string
		id                    int64
	}{
		{"user:2 Jefe de Operaciones Demo", tenant.RoleJefeOps, "Jefe de Operaciones Demo", "human", 2},
		{"user:7", "", "Usuario #7", "human", 7},
		{"system:invas", "", "Sistema", "system", 0},
		{"Sistema", "", "Sistema", "system", 0},
		{"Invas", "", "Sistema", "system", 0},
		{"", "", "Sistema", "system", 0},
		{"Marco Rojas Pérez", "", "Marco Rojas Pérez", "human", 0},
	} {
		a := ParseActor(c.raw, c.role)
		if a.Name != c.name || a.Kind != c.kind || a.Role != c.role || (c.id == 0) != (a.ID == nil) || (a.ID != nil && *a.ID != c.id) {
			t.Fatalf("%q → %+v", c.raw, a)
		}
	}
}

func TestRegistryMotive(t *testing.T) {
	for _, c := range []struct {
		e    ports.RegistryEntry
		want string
	}{
		{ports.RegistryEntry{Reason: "Compra desistida"}, "Compra desistida"},
		{ports.RegistryEntry{Reason: "Cancelada por Ana Pérez (cliente): Compra desistida"}, "Compra desistida"},
		{ports.RegistryEntry{Stale: true}, StaleMotive},
		{ports.RegistryEntry{Stale: true, Reason: " Sin stock "}, StaleMotive + " · Sin stock"},
	} {
		if got := registryMotive(c.e); got != c.want {
			t.Fatalf("%+v → %q", c.e, got)
		}
	}
}

type fakeRegistry struct{ got ports.RegistryFilter }

func (f *fakeRegistry) Registry(_ context.Context, rf ports.RegistryFilter) ([]ports.RegistryEntry, int, error) {
	f.got = rf
	return []ports.RegistryEntry{{OrderID: 1, Actor: "user:2 Jefe", ActorRole: tenant.RoleJefeOps, Stale: true}}, 1, nil
}

func TestRegistryAccess(t *testing.T) {
	ctx := context.Background()
	for _, c := range []struct {
		role string
		ok   bool
	}{
		{tenant.RoleAdmin, true}, {tenant.RoleJefeOps, true}, {tenant.RoleCliente, true},
		{tenant.RoleEmpaque, false}, {tenant.RoleAnalista, false},
	} {
		repo := &fakeRegistry{}
		reg := NewRegistry(repo)
		rows, _, err := reg.List(ctx, who(c.role, tenant.PermDespacho), ports.RegistryFilter{Limit: 999})
		if c.ok != (err == nil) || (!c.ok && !errors.Is(err, domain.ErrForbidden)) {
			t.Fatalf("%s: err %v", c.role, err)
		}
		if !c.ok {
			if err := reg.Export(ctx, who(c.role), ports.RegistryFilter{}, "xlsx", &bytes.Buffer{}); !errors.Is(err, domain.ErrForbidden) {
				t.Fatalf("%s exporta: %v", c.role, err)
			}
			continue
		}
		if repo.got.Limit != 20 {
			t.Fatalf("límite fuera de rango → 20, llegó %d", repo.got.Limit)
		}
		var out bytes.Buffer
		if err := reg.Export(ctx, who(c.role), ports.RegistryFilter{}, "csv", &out); err != nil || !bytes.Contains(out.Bytes(), []byte("N° orden;Cliente;Cerrada por")) {
			t.Fatalf("%s csv: %v %q", c.role, err, out.String())
		}
		if rows[0].Motive != StaleMotive || rows[0].ClosedBy.Name != "Jefe" || rows[0].ClosedBy.Role != tenant.RoleJefeOps {
			t.Fatalf("fila %+v", rows[0])
		}
	}
}
