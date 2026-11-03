// Package application - casos de uso de Clientes.
// Punto de partida para RF-04: agrega aquí Crear, Editar y Activar/Desactivar.
package application

import (
	"context"

	"github.com/nexuswst/backend-core/internal/shared/wave"

	"github.com/nexuswst/backend-core/internal/modules/clients/domain"
	"github.com/nexuswst/backend-core/internal/modules/clients/ports"
)

type Service struct{ repo ports.CompanyRepository }

func NewService(r ports.CompanyRepository) *Service { return &Service{repo: r} }

func (s *Service) List(ctx context.Context, companyIDs []int64, search string) ([]domain.Company, error) {
	return s.repo.List(ctx, companyIDs, search)
}

// SetWave fija o quita la ola por defecto del cliente (PV-19, H-03-T07).
func (s *Service) SetWave(ctx context.Context, id int64, code *string) (*string, error) {
	c, err := wave.Normalize(code)
	if err != nil {
		return nil, err
	}
	return c, s.repo.SetWave(ctx, id, c)
}
