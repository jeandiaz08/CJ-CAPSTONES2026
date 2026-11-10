package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nexuswst/backend-core/internal/modules/products/domain"
	"github.com/nexuswst/backend-core/internal/modules/products/ports"
)

// Catálogo contra PostgreSQL real (Tarea 09): alta con composición y registro en una transacción,
// SKU único por empresa, «Modificado por» desde user_audit y filtros del listado.
func TestCatalogRepository(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL habilita PostgreSQL")
	}
	ctx := context.Background()
	db, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var company, actor int64
	var actorName string
	if err := db.QueryRow(ctx, `SELECT c.id, u.id, u.name FROM companies c, users u WHERE c.active AND u.role = 'admin' ORDER BY c.id, u.id LIMIT 1`).Scan(&company, &actor, &actorName); err != nil {
		t.Skip("requiere una empresa activa y un Administrador locales")
	}
	r := NewRepository(db)
	stamp := fmt.Sprint(time.Now().UnixNano())
	var ids []int64
	defer func() {
		_, _ = db.Exec(ctx, `DELETE FROM user_audit WHERE target_product_id = ANY($1)`, ids)
		_, _ = db.Exec(ctx, `DELETE FROM kit_components WHERE kit_id = ANY($1)`, ids)
		_, _ = db.Exec(ctx, `DELETE FROM products WHERE id = ANY($1)`, ids)
	}()
	a := ports.Audit{ActorID: actor, Action: "product_created", Detail: map[string]any{"sku": "x"}}
	comp, err := r.CreateProduct(ctx, domain.Product{CompanyID: company, SKU: "T-C-" + stamp, Description: "Componente", Type: domain.TypeSimple}, nil, a)
	if err != nil {
		t.Fatal(err)
	}
	ids = append(ids, comp.ID)
	kit, err := r.CreateProduct(ctx, domain.Product{CompanyID: company, SKU: "T-K-" + stamp, Description: "Kit", Type: domain.TypeKit},
		[]domain.KitComponent{{ComponentID: comp.ID, Quantity: 2}}, a)
	if err != nil {
		t.Fatal(err)
	}
	ids = append(ids, kit.ID)
	if _, err := r.CreateProduct(ctx, domain.Product{CompanyID: company, SKU: "T-C-" + stamp, Description: "Repetido", Type: domain.TypeSimple}, nil, a); !errors.Is(err, domain.ErrDuplicateSKU) {
		t.Fatalf("SKU repetido: %v", err)
	}
	// Composición y «Modificado por».
	comps, err := r.Components(ctx, kit.ID)
	if err != nil || len(comps) != 1 || comps[0].Quantity != 2 {
		t.Fatalf("composición: %+v %v", comps, err)
	}
	it, err := r.GetCatalog(ctx, nil, kit.ID)
	if err != nil || it.UpdatedByName != actorName || it.UpdatedByAt == nil || it.CompanyName == "" {
		t.Fatalf("modificado por: %+v %v", it, err)
	}
	// Kits activos que usan el componente; desactivar deja la acción y no repite el estado.
	if kits, _ := r.ActiveKitsUsing(ctx, comp.ID); len(kits) != 1 || kits[0] != "T-K-"+stamp {
		t.Fatalf("kits que lo usan: %v", kits)
	}
	if err := r.SetActive(ctx, kit.ID, false, ports.Audit{ActorID: actor, Action: "product_disabled", Detail: map[string]any{"reason": "Prueba"}}); err != nil {
		t.Fatal(err)
	}
	if err := r.SetActive(ctx, kit.ID, false, ports.Audit{ActorID: actor, Action: "product_disabled"}); !errors.Is(err, domain.ErrSameState) {
		t.Fatalf("mismo estado: %v", err)
	}
	if kits, _ := r.ActiveKitsUsing(ctx, comp.ID); len(kits) != 0 {
		t.Fatalf("un kit inactivo no bloquea: %v", kits)
	}
	// Editar composición y ola en una transacción.
	w := "AM"
	if err := r.UpdateProduct(ctx, kit.ID, "Kit editado", &w, []domain.KitComponent{{ComponentID: comp.ID, Quantity: 5}}, ports.Audit{ActorID: actor, Action: "product_updated"}); err != nil {
		t.Fatal(err)
	}
	if comps, _ := r.Components(ctx, kit.ID); len(comps) != 1 || comps[0].Quantity != 5 {
		t.Fatalf("composición editada: %+v", comps)
	}
	// Filtros: tipo, estado y ola.
	inactive := false
	list, total, err := r.ListCatalog(ctx, ports.CatalogFilter{CompanyIDs: []int64{company}, Query: stamp, Type: domain.TypeKit, Active: &inactive, Wave: "AM", Limit: 10})
	if err != nil || total != 1 || list[0].ID != kit.ID {
		t.Fatalf("filtros: %d %+v %v", total, list, err)
	}
	if _, total, _ := r.ListCatalog(ctx, ports.CatalogFilter{CompanyIDs: []int64{company}, Query: stamp, Wave: "general", Limit: 10}); total != 1 {
		t.Fatalf("ola general: %d", total)
	}
	var actions int
	_ = db.QueryRow(ctx, `SELECT count(*) FROM user_audit WHERE target_product_id = ANY($1)`, ids).Scan(&actions)
	if actions != 4 {
		t.Fatalf("acciones en el registro: %d, quería 4", actions)
	}
}
