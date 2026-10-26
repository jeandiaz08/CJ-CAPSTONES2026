// Package inventory - módulo de negocio del Core (pendiente de implementar).
//
// Sigue la misma estructura hexagonal que el módulo "products":
//
//	domain/       entidades y reglas (sin dependencias técnicas)
//	ports/        interfaces que usa la aplicación
//	application/  casos de uso
//	adapters/     inbound (HTTP) y outbound (PostgreSQL, Redis, WMS...)
//
// Regístralo en cmd/api/main.go cuando tenga rutas.
package inventory
