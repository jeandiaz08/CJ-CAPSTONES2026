package ports

import (
	"context"
	"time"
)

// Registro de acciones (Tarea 06, Fase 2, RF-28; cambio de Jean; ver backend_pendientes
// «Cambios de backend hechos por Jean»). Se lee de la vista activity_log (migración 000009).

// ActivityFilter - filtros de GET /v1/activity.
type ActivityFilter struct {
	CompanyIDs []int64 // alcance del usuario: nil = todas; incluye lo que no tiene empresa (manifiestos, usuarios)
	CompanyID  *int64  // filtro explícito por empresa: solo lo de esa empresa
	Module     string  // orders | packing | dispatch | users
	Actor      string  // id de usuario o parte del nombre
	Role       string
	Kind       string // human | system
	Search     string // N° de orden, de manifiesto («#12») o nombre de usuario
	From, To   *time.Time
	Since      time.Time // retención: nada anterior (lo fija la aplicación)
	Limit      int
	Offset     int
}

// ActivityEntry - una fila de activity_log. Details viene completo de la base; la
// aplicación lo filtra por una lista de campos permitidos antes de responder.
type ActivityEntry struct {
	ID          string
	OccurredAt  time.Time
	ActorID     *int64
	ActorName   string
	ActorRole   string // perfil ACTUAL del usuario
	ActorKind   string
	Module      string
	Action      string
	EntityType  string
	EntityID    string
	EntityLabel string
	CompanyID   *int64
	CompanyName string
	CourierName string // nombre del courier de details.courier
	Warehouse   string
	Reason      string
	Details     map[string]any
}

type ActivityReader interface {
	Activity(ctx context.Context, f ActivityFilter) ([]ActivityEntry, int, error)
}
