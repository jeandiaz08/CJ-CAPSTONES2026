// Package application contiene los casos de uso de Productos.
package application

import (
	"context"

	"github.com/nexuswst/backend-core/internal/shared/wave"

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
	WaveCode    *string // opcional (PV-19)
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

// Components devuelve la composición de un kit visible para el usuario.
func (s *Service) Components(ctx context.Context, companyIDs []int64, kitID int64) (domain.Product, []domain.KitComponent, error) {
	kit, err := s.repo.GetByID(ctx, companyIDs, kitID)
	if err != nil {
		return domain.Product{}, nil, err
	}
	if kit.Type != domain.TypeKit {
		return kit, nil, domain.ErrNotKit
	}
	comps, err := s.repo.Components(ctx, kit.ID)
	return kit, comps, err
}

// SetComponents reemplaza la composición completa del kit (H-04-T01). Cada
// componente debe existir, estar activo, ser simple y de la misma empresa.
func (s *Service) SetComponents(ctx context.Context, companyIDs []int64, kitID int64, in []domain.ComponentInput) (domain.Product, []domain.KitComponent, error) {
	kit, err := s.repo.GetByID(ctx, companyIDs, kitID)
	if err != nil {
		return domain.Product{}, nil, err
	}
	if kit.Type != domain.TypeKit {
		return kit, nil, domain.ErrNotKit
	}
	lines, err := domain.NormalizeComponents(kit.SKU, in)
	if err != nil {
		return kit, nil, err
	}
	skus := make([]string, len(lines))
	for i, l := range lines {
		skus[i] = l.SKU
	}
	found, err := s.repo.BySKUs(ctx, kit.CompanyID, skus)
	if err != nil {
		return kit, nil, err
	}
	bySKU := make(map[string]domain.Product, len(found))
	for _, p := range found {
		bySKU[p.SKU] = p
	}
	comps := make([]domain.KitComponent, 0, len(lines))
	for _, l := range lines {
		p, ok := bySKU[l.SKU]
		if !ok {
			return kit, nil, &domain.ComponentError{SKU: l.SKU, Reason: "no existe en el catálogo de la empresa"}
		}
		if err := domain.ValidateComponent(kit, p); err != nil {
			return kit, nil, err
		}
		comps = append(comps, domain.KitComponent{ComponentID: p.ID, SKU: p.SKU, Description: p.Description, Quantity: l.Quantity})
	}
	if err := s.repo.ReplaceComponents(ctx, kit.ID, comps); err != nil {
		return kit, nil, err
	}
	return kit, comps, nil
}

func (s *Service) Create(ctx context.Context, companyID int64, in CreateInput) (domain.Product, error) {
	p, err := domain.NewProduct(companyID, in.SKU, in.Description, domain.ProductType(in.Type))
	if err != nil {
		return domain.Product{}, err
	}
	if p.WaveCode, err = wave.Normalize(in.WaveCode); err != nil {
		return domain.Product{}, err
	}
	return s.repo.Create(ctx, p)
}

// SetWave fija o quita la ola propia del producto (PV-19, H-04-T10). Solo
// Administrador y Jefe de Operaciones (lo controla el handler).
func (s *Service) SetWave(ctx context.Context, companyIDs []int64, id int64, code *string) (domain.Product, error) {
	p, err := s.repo.GetByID(ctx, companyIDs, id)
	if err != nil {
		return domain.Product{}, err
	}
	c, err := wave.Normalize(code)
	if err != nil {
		return domain.Product{}, err
	}
	if err := s.repo.SetWave(ctx, p.ID, c); err != nil {
		return domain.Product{}, err
	}
	p.WaveCode = c
	return p, nil
}
