package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nexuswst/orders-service/internal/domain"
	"github.com/nexuswst/orders-service/internal/ports"
	"github.com/nexuswst/orders-service/internal/shared/tenant"
)

type fakeActivity struct{ got ports.ActivityFilter }

func (f *fakeActivity) Activity(_ context.Context, af ports.ActivityFilter) ([]ports.ActivityEntry, int, error) {
	f.got = af
	return []ports.ActivityEntry{{ID: "ua-1", ActorName: "Sistema", ActorKind: "system", Module: "users", Action: "linked",
		Details: map[string]any{"keycloakId": "abc", "password": "x", "token": "y", "role": "empaque",
			"permissions": map[string]any{"from": []any{}, "to": []any{"despacho"}, "secret": "z"}}}}, 1, nil
}

func TestActivityAccessAndValidation(t *testing.T) {
	ctx := context.Background()
	for _, role := range []string{tenant.RoleCliente, tenant.RoleEmpaque, tenant.RoleAnalista} {
		_, _, err := NewActivity(&fakeActivity{}).List(ctx, who(role, tenant.PermDespacho), ports.ActivityFilter{})
		if !errors.Is(err, domain.ErrForbidden) {
			t.Fatalf("%s: quería 403, llegó %v", role, err)
		}
	}
	for _, f := range []ports.ActivityFilter{{Module: "config"}, {Kind: "robot"}} {
		if _, _, err := NewActivity(&fakeActivity{}).List(ctx, who(tenant.RoleJefeOps), f); !errors.Is(err, domain.ErrInvalid) {
			t.Fatalf("%+v: quería 422, llegó %v", f, err)
		}
	}
}

func TestActivityRetentionLimitAndSafeDetails(t *testing.T) {
	now := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	repo := &fakeActivity{}
	a := NewActivity(repo)
	a.now = func() time.Time { return now }
	for _, role := range []string{tenant.RoleAdmin, tenant.RoleJefeOps} {
		items, total, err := a.List(context.Background(), who(role), ports.ActivityFilter{Module: "users", Limit: 5000})
		if err != nil || total != 1 {
			t.Fatalf("%s: %v", role, err)
		}
		if !repo.got.Since.Equal(now.Add(-ActivityRetention)) || repo.got.Limit != 50 {
			t.Fatalf("retención %v, límite %d", repo.got.Since, repo.got.Limit)
		}
		d := items[0].Details
		for _, k := range []string{"keycloakId", "password", "token"} {
			if _, ok := d[k]; ok {
				t.Fatalf("%s no debería salir: %v", k, d)
			}
		}
		perm := d["permissions"].(map[string]any)
		if d["role"] != "empaque" || perm["secret"] != nil || perm["to"] == nil {
			t.Fatalf("detalle %v", d)
		}
	}
}
