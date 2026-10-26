package ports

import (
	"context"
	"time"

	"github.com/nexuswst/backend-core/internal/modules/catalogs/domain"
)

// Source es la fuente autorizada (PostgreSQL). Devuelve el catálogo ya en JSON.
type Source interface {
	Load(ctx context.Context, name domain.Name) ([]byte, error)
}

// Cache es la copia temporal (Redis). hit=false si no existe o expiró.
type Cache interface {
	Get(ctx context.Context, name domain.Name) (data []byte, hit bool, err error)
	Set(ctx context.Context, name domain.Name, data []byte, ttl time.Duration) error
	Invalidate(ctx context.Context, name domain.Name) error
}
