// Package application contiene los casos de uso de Productos.
package application

import (
	"context"

	"github.com/nexuswst/backend-core/internal/modules/products/domain"
	"github.com/nexuswst/backend-core/internal/modules/products/ports"
)

type Service struct {
	repo ports.ProductRepository
}

func NewService(repo ports.ProductRepository) *Service { return &Service{repo: repo} }

type CreateInput struct {
	SKU         string
	Description string
	Type        string
}

// List filtra SIEMPRE por las empresas autorizadas (RF-03, RNF-SEG-01).
// companyIDs nil significa "todas" y solo lo entrega tenant.Scope a personal interno.
func (s *Service) List(ctx context.Context, companyIDs []int64, search string, limit, offset int) ([]domain.Product, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return s.repo.List(ctx, ports.ListFilter{CompanyIDs: companyIDs, Search: search, Limit: limit, Offset: offset})
}

func (s *Service) Get(ctx context.Context, companyIDs []int64, id int64) (domain.Product, error) {
	return s.repo.GetByID(ctx, companyIDs, id)
}

func (s *Service) Create(ctx context.Context, companyID int64, in CreateInput) (domain.Product, error) {
	p, err := domain.NewProduct(companyID, in.SKU, in.Description, domain.ProductType(in.Type))
	if err != nil {
		return domain.Product{}, err
	}
	return s.repo.Create(ctx, p)
}
