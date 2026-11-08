package application

import (
	"context"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/nexuswst/orders-service/internal/domain"
	"github.com/nexuswst/orders-service/internal/platform/xlsx"
	"github.com/nexuswst/orders-service/internal/ports"
	"github.com/nexuswst/orders-service/internal/shared/tenant"
)

// Registro histórico de órdenes (Tarea 06, cambio de Jean; ver backend_pendientes
// «Cambios de backend hechos por Jean»). Lo ven el Administrador y el Jefe de
// Operaciones (todas sus empresas) y el Cliente (solo la suya, por el alcance del
// token, como en la Matriz de perfiles: «Propio»).

// StaleMotive - motivo de las órdenes que entran por tiempo, no por estar Canceladas.
const StaleMotive = "Más de 1 año sin movimiento"

var RoleLabels = map[string]string{
	tenant.RoleAdmin: "Administrador", tenant.RoleJefeOps: "Jefe de Operaciones", tenant.RoleAnalista: "Analista",
	tenant.RoleEmpaque: "Empaque", tenant.RoleCliente: "Cliente",
}

// Actor - quién hizo una acción, a partir del texto de order_events.actor.
// Role es el perfil ACTUAL del usuario: los eventos no guardan el de ese momento.
type Actor struct {
	ID   *int64 `json:"id,omitempty"`
	Name string `json:"name"`
	Role string `json:"role,omitempty"`
	Kind string `json:"kind"` // human | system
}

var userActor = regexp.MustCompile(`^user:(\d+)\s*(.*)$`)

// Cancel guarda «Cancelada por {nombre} ({perfil}): {motivo}»; quién ya va en «Cerrada por».
var cancelledBy = regexp.MustCompile(`^Cancelada por .+? \([^)]*\): `)

// ParseActor lee «user:{id} {nombre}» (la API), «system:…», «Sistema» o «Invas»
// (procesos), o un nombre suelto (datos antiguos o del seed).
func ParseActor(raw, role string) Actor {
	raw = strings.TrimSpace(raw)
	if m := userActor.FindStringSubmatch(raw); m != nil {
		id, _ := strconv.ParseInt(m[1], 10, 64)
		name := strings.TrimSpace(m[2])
		if name == "" {
			name = "Usuario #" + m[1]
		}
		return Actor{ID: &id, Name: name, Role: role, Kind: "human"}
	}
	lower := strings.ToLower(raw)
	if raw == "" || strings.HasPrefix(lower, "system:") || lower == "sistema" || lower == "invas" || lower == "worker" {
		return Actor{Name: "Sistema", Kind: "system"}
	}
	return Actor{Name: raw, Kind: "human"}
}

// RegistryRow - una orden del registro lista para mostrar o exportar.
type RegistryRow struct {
	ports.RegistryEntry
	ClosedBy Actor
	Motive   string
}

// registryMotive - «Más de 1 año sin movimiento» para las que entran por tiempo, junto al
// motivo original si lo tienen; para las Canceladas, su motivo.
func registryMotive(e ports.RegistryEntry) string {
	reason := strings.TrimSpace(cancelledBy.ReplaceAllString(e.Reason, ""))
	switch {
	case !e.Stale:
		return reason
	case reason == "":
		return StaleMotive
	default:
		return StaleMotive + " · " + reason
	}
}

type Registry struct{ repo ports.RegistryReader }

func NewRegistry(repo ports.RegistryReader) *Registry { return &Registry{repo: repo} }

func canReadRegistry(id tenant.Identity) bool {
	return id.HasRole(tenant.RoleAdmin, tenant.RoleJefeOps, tenant.RoleCliente)
}

// List - el filtro ya trae las empresas del alcance del usuario (CompanyIDs).
func (r *Registry) List(ctx context.Context, id tenant.Identity, f ports.RegistryFilter) ([]RegistryRow, int, error) {
	if !canReadRegistry(id) {
		return nil, 0, fail(domain.ErrForbidden, "El registro histórico es para Administrador, Jefe de Operaciones y Cliente.")
	}
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 20
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
	entries, total, err := r.repo.Registry(ctx, f)
	if err != nil {
		return nil, 0, err
	}
	out := make([]RegistryRow, 0, len(entries))
	for _, e := range entries {
		out = append(out, RegistryRow{RegistryEntry: e, ClosedBy: ParseActor(e.Actor, e.ActorRole), Motive: registryMotive(e)})
	}
	return out, total, nil
}

var registryHeader = []string{"N° orden", "Cliente", "Cerrada por", "Perfil", "Motivo", "Estado", "Estaba en", "Courier", "Fecha"}

// Export - Excel con lo filtrado (o con las seleccionadas en f.IDs).
func (r *Registry) Export(ctx context.Context, id tenant.Identity, f ports.RegistryFilter, w io.Writer) error {
	f.Offset = 0
	if !canReadRegistry(id) {
		return fail(domain.ErrForbidden, "El registro histórico es para Administrador, Jefe de Operaciones y Cliente.")
	}
	f.Limit = MaxExportRows
	entries, total, err := r.repo.Registry(ctx, f)
	if err != nil {
		return err
	}
	if total > MaxExportRows {
		return fail(domain.ErrInvalid, "El filtro tiene %d órdenes: exporta como máximo %d (acota fechas o estado).", total, MaxExportRows)
	}
	loc, _ := time.LoadLocation("America/Santiago")
	if loc == nil {
		loc = time.Local
	}
	rows := make([][]any, 0, len(entries))
	for _, e := range entries {
		actor := ParseActor(e.Actor, e.ActorRole)
		courier := e.CourierName
		if courier == "" {
			courier = e.Courier
		}
		rows = append(rows, []any{e.ExternalRef, e.CompanyName, actor.Name, RoleLabels[actor.Role], registryMotive(e),
			domain.Labels[domain.Status(e.Status)], domain.Labels[domain.Status(e.PreviousStatus)], courier,
			e.ClosedAt.In(loc).Format("2006-01-02 15:04")})
	}
	return xlsx.Write(w, "Registro de órdenes", registryHeader, rows)
}
