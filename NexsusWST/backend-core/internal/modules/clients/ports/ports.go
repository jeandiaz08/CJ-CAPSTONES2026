package ports

import (
	"context"
	"time"

	"github.com/nexuswst/backend-core/internal/modules/clients/domain"
)

// CompanyFilter - Clientes › Administración clientes › Empresas (Tarea 07).
type CompanyFilter struct {
	Query  string     // nombre o RUT (con o sin puntos)
	Active *bool      // nil = todas
	Wave   string     // "" = todas; "general" = sin ola fija (NULL); otro = código de ola
	From   *time.Time // fecha de creación (día, hora de Chile), inclusive
	To     *time.Time
	IDs    []int64 // solo estas (exportar las seleccionadas)
	Limit  int
	Offset int
}

// Audit - acción que queda en user_audit (target_company_id) en la misma transacción.
type Audit struct {
	Action string
	Detail map[string]any
}

// CompanyRepository. companyIDs nil = todas (personal interno).
type CompanyRepository interface {
	List(ctx context.Context, companyIDs []int64, search string) ([]domain.Company, error)
	SetWave(ctx context.Context, id int64, code *string) error

	AdminList(ctx context.Context, f CompanyFilter) ([]domain.Company, int, error)
	Get(ctx context.Context, id int64) (domain.Company, error)
	// Create devuelve domain.ErrTaxIDTaken si el RUT ya existe.
	Create(ctx context.Context, d domain.Draft, actorID int64, a Audit) (domain.Company, error)
	Update(ctx context.Context, id int64, d domain.Draft, actorID int64, a Audit) error
	SetActive(ctx context.Context, id int64, active bool, actorID int64, a Audit) error
}
