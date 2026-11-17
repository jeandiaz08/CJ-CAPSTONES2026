// Package ports define los contratos (interfaces) que usa la aplicación.
// Los adaptadores de salida los implementan (PA-02 Repository).
package ports

import (
	"context"

	"github.com/nexuswst/backend-core/internal/modules/products/domain"
)

// ListFilter: CompanyIDs nil = todas las empresas (solo personal interno).
type ListFilter struct {
	CompanyIDs []int64
	Search     string
	Limit      int
	Offset     int
}

type ProductRepository interface {
	List(ctx context.Context, f ListFilter) ([]domain.Product, error)
	GetByID(ctx context.Context, companyIDs []int64, id int64) (domain.Product, error)
	Create(ctx context.Context, p domain.Product) (domain.Product, error)
	SetWave(ctx context.Context, id int64, code *string) error

	// Composición de kits (H-04-T01)
	BySKUs(ctx context.Context, companyID int64, skus []string) ([]domain.Product, error)
	Components(ctx context.Context, kitID int64) ([]domain.KitComponent, error)
	// ReplaceComponents reemplaza toda la composición en una transacción.
	ReplaceComponents(ctx context.Context, kitID int64, comps []domain.KitComponent) error
}
