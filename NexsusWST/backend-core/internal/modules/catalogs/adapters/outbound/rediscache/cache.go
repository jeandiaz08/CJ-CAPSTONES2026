package rediscache

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/nexuswst/backend-core/internal/modules/catalogs/domain"
)

// Clave: entorno + versión del formato + nombre (DAS §5.3). Limpieza por prefijo.
type Cache struct {
	rdb    *redis.Client
	prefix string
}

func New(rdb *redis.Client, env string) *Cache {
	return &Cache{rdb: rdb, prefix: "nexus:" + env + ":catalog:v1:"}
}

func (c *Cache) Get(ctx context.Context, name domain.Name) ([]byte, bool, error) {
	raw, err := c.rdb.Get(ctx, c.prefix+string(name)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return raw, true, nil
}

func (c *Cache) Set(ctx context.Context, name domain.Name, data []byte, ttl time.Duration) error {
	return c.rdb.Set(ctx, c.prefix+string(name), data, ttl).Err() // TTL explícito siempre
}

func (c *Cache) Invalidate(ctx context.Context, name domain.Name) error {
	return c.rdb.Del(ctx, c.prefix+string(name)).Err()
}
