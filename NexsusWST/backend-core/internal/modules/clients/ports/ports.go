package ports

import (
	"context"

	"github.com/nexuswst/backend-core/internal/modules/clients/domain"
)

// CompanyRepository. companyIDs nil = todas (personal interno).
// TODO (RF-04): Create, Update, SetActive.
type CompanyRepository interface {
	List(ctx context.Context, companyIDs []int64, search string) ([]domain.Company, error)
}
