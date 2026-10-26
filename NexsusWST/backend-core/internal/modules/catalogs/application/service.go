// Package application implementa Cache-Aside (PA-07) en el Core (DA-13).
//
// 1) Redis -> 2) si falta, PostgreSQL -> 3) guardar en Redis con TTL.
// Si Redis falla se responde desde PostgreSQL (degradación controlada).
// Órdenes NO usa este caché: lee PostgreSQL directamente (DA-12).
package application

import (
	"context"
	"log/slog"
	"time"

	"github.com/nexuswst/backend-core/internal/modules/catalogs/domain"
	"github.com/nexuswst/backend-core/internal/modules/catalogs/ports"
)

type Service struct {
	src   ports.Source
	cache ports.Cache
	ttl   time.Duration
	log   *slog.Logger
}

func NewService(src ports.Source, cache ports.Cache, ttl time.Duration, log *slog.Logger) *Service {
	return &Service{src: src, cache: cache, ttl: ttl, log: log}
}

// Get devuelve el JSON del catálogo y si vino de caché (para trazas X-Cache).
func (s *Service) Get(ctx context.Context, name domain.Name) ([]byte, bool, error) {
	if !domain.Known(name) {
		return nil, false, domain.ErrUnknownCatalog
	}
	if data, hit, err := s.cache.Get(ctx, name); err == nil && hit {
		return data, true, nil
	} else if err != nil {
		s.log.Warn("caché no disponible, se usa PostgreSQL", "catalog", name, "err", err)
	}
	data, err := s.src.Load(ctx, name)
	if err != nil {
		return nil, false, err
	}
	if err := s.cache.Set(ctx, name, data, s.ttl); err != nil {
		s.log.Warn("no se pudo guardar en caché", "catalog", name, "err", err)
	}
	return data, false, nil
}

// Refresh = invalidación + recarga (refresco manual y precarga, RF-29/31).
func (s *Service) Refresh(ctx context.Context, name domain.Name) error {
	if !domain.Known(name) {
		return domain.ErrUnknownCatalog
	}
	data, err := s.src.Load(ctx, name)
	if err != nil {
		return err
	}
	return s.cache.Set(ctx, name, data, s.ttl)
}

// Preload carga todos los catálogos conocidos (al arrancar / inicio de jornada).
func (s *Service) Preload(ctx context.Context) error {
	for _, n := range []domain.Name{domain.Regions, domain.Couriers, domain.Communes, domain.DispatchTypes} {
		if err := s.Refresh(ctx, n); err != nil {
			return err
		}
	}
	return nil
}
