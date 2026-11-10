package ports

import (
	"context"
	"time"

	"github.com/nexuswst/backend-core/internal/modules/products/domain"
)

// CatalogFilter - página Productos (Tarea 09). CompanyIDs nil = todas (solo personal interno).
type CatalogFilter struct {
	CompanyIDs []int64
	Query      string             // SKU o descripción
	Type       domain.ProductType // "" = todos
	Active     *bool
	Wave       string // "" = todas; "general" = sin ola propia; o el código
	IDs        []int64
	Limit      int
	Offset     int
}

// CatalogItem es el producto con el nombre del cliente y quién hizo la última acción (user_audit).
type CatalogItem struct {
	domain.Product
	CompanyName   string
	UpdatedByName string
	UpdatedByAt   *time.Time
}

// Audit es la acción que queda en user_audit (target_product_id), en la misma transacción.
type Audit struct {
	ActorID int64
	Action  string
	Detail  map[string]any
}

// CatalogRepository - lecturas del catálogo y escrituras con su acción en el registro, todo en una
// transacción (si falla el registro, no queda el cambio).
type CatalogRepository interface {
	ListCatalog(ctx context.Context, f CatalogFilter) ([]CatalogItem, int, error)
	GetCatalog(ctx context.Context, companyIDs []int64, id int64) (CatalogItem, error)
	// CreateProduct crea el producto y, si es kit, su composición. domain.ErrDuplicateSKU si el SKU
	// ya existe en la empresa.
	CreateProduct(ctx context.Context, p domain.Product, comps []domain.KitComponent, a Audit) (domain.Product, error)
	// UpdateProduct cambia descripción y ola; comps nil = la composición no cambia.
	UpdateProduct(ctx context.Context, id int64, description string, wave *string, comps []domain.KitComponent, a Audit) error
	SetActive(ctx context.Context, id int64, active bool, a Audit) error
	// ActiveKitsUsing: SKU de los kits activos que llevan el producto como componente.
	ActiveKitsUsing(ctx context.Context, componentID int64) ([]string, error)
	// Company: nombre y si está activa (para validar el cliente del formulario).
	Company(ctx context.Context, id int64) (name string, active bool, err error)
}
