package ports

import (
	"context"
	"time"
)

// Registro histórico de órdenes (Tarea 06, cambio de Jean; ver backend_pendientes
// «Cambios de backend hechos por Jean»): las Canceladas y las órdenes con más de
// 1 año sin cambiar de estado. La bandeja excluye exactamente esas mismas.

// RegistryFilter - filtros de GET /v1/orders/registry. From y To son sobre la
// fecha del cierre (el último cambio de estado).
type RegistryFilter struct {
	CompanyIDs    []int64 // nil = todas las empresas
	Search        string  // N° de orden (referencia externa o id)
	Statuses      []string
	Courier       string
	IDs           []int64 // exportar solo las seleccionadas
	From, To      *time.Time
	Limit, Offset int
}

// RegistryEntry - una orden del registro con su último cambio de estado.
type RegistryEntry struct {
	OrderID        int64
	ExternalRef    string
	CompanyID      int64
	CompanyName    string
	Courier        string
	CourierName    string
	Status         string
	PreviousStatus string // «Estaba en»: el estado antes del cierre
	ClosedAt       time.Time
	Actor          string // texto de order_events.actor («user:{id} {nombre}», «Sistema», …)
	ActorRole      string // perfil ACTUAL del usuario (el evento no guarda el de ese momento)
	Reason         string
	Stale          bool // entró por más de 1 año sin movimiento (no por estar Cancelada)
}

type RegistryReader interface {
	Registry(ctx context.Context, f RegistryFilter) ([]RegistryEntry, int, error)
}
