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
}
