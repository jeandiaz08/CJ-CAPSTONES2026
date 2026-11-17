package application

import (
	"context"
	"slices"
	"time"

	"github.com/nexuswst/orders-service/internal/domain"
	"github.com/nexuswst/orders-service/internal/ports"
	"github.com/nexuswst/orders-service/internal/shared/tenant"
)

// Registro de acciones (Tarea 06, Fase 2, RF-28; cambio de Jean; ver backend_pendientes
// «Cambios de backend hechos por Jean»). Solo Administrador y Jefe de Operaciones.
// TODO(PV-21): acotar al Jefe a sus bodegas cuando exista user_warehouses.

// ActivityRetention - el registro no devuelve nada más antiguo (no se borra nada).
const ActivityRetention = 365 * 24 * time.Hour

var (
	ActivityModules = []string{"orders", "packing", "dispatch", "users", "products"}
	activityKinds   = []string{"human", "system"}
)

// activityFields - lista de campos de details que se pueden mostrar. Todo lo demás se
// descarta (por ejemplo keycloakId), así nunca salen credenciales, secretos ni tokens
// aunque algún día se guarden por error.
var activityFields = map[string]bool{
	// órdenes, empaque y retiro
	"fromStatus": true, "toStatus": true, "courier": true, "labelCode": true, "packages": true,
	"kind": true, "manifestId": true, "waveId": true, "reason": true, "authorizedBy": true,
	"packagesTotal": true, "scannedPackages": true, "missingPackages": true,
	"orders": true, "pickerName": true, "pickerId": true, "vehiclePlate": true, "notes": true,
	// usuarios (user_audit del Core)
	"name": true, "role": true, "permissions": true, "companies": true, "allCompanies": true,
	"username": true, "withoutEmail": true, "temporary": true, "source": true, "targetRole": true,
	// empresas cliente y solicitudes de perfiles (Tarea 07, Core 000011)
	"taxId": true, "email": true, "phone": true, "waveCode": true, "wmsCode": true, "active": true,
	"jobTitle": true, "companyName": true, "requestId": true, "requestKind": true,
	"created": true, "rows": true, "failed": true, // carga masiva de perfiles
	"edited": true, // solicitud aceptada con datos editados
	// productos y kits (Tarea 09, Core 000013)
	"sku": true, "description": true, "productType": true, "components": true,
}

// Dentro de un campo permitido, un cambio viene como {from, to} (por ejemplo, el perfil).
var activityChangeFields = map[string]bool{"from": true, "to": true, "allCompanies": true}

func safeDetails(in map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range in {
		if !activityFields[k] {
			continue
		}
		if m, ok := v.(map[string]any); ok {
			change := map[string]any{}
			for ck, cv := range m {
				if activityChangeFields[ck] {
					change[ck] = cv
				}
			}
			v = change
		}
		out[k] = v
	}
	return out
}

// ActivityItem - una acción lista para responder.
type ActivityItem struct {
	ID          string         `json:"id"`
	OccurredAt  time.Time      `json:"occurredAt"`
	Actor       Actor          `json:"actor"`
	Module      string         `json:"module"`
	Action      string         `json:"action"`
	ActionLabel string         `json:"actionLabel"`
	Entity      ActivityEntity `json:"entity"`
	CompanyID   *int64         `json:"companyId,omitempty"`
	CompanyName string         `json:"companyName,omitempty"`
	Warehouse   string         `json:"warehouse,omitempty"` // hoy casi siempre vacío; sirve para PV-21
	Reason      string         `json:"reason,omitempty"`
	Details     map[string]any `json:"details"`
	// Textos listos para la pantalla y el Excel (activity_text.go).
	StatusChange     string   `json:"statusChange,omitempty"` // «Validada → En preparación»
	CourierName      string   `json:"courierName,omitempty"`
	TransportCompany string   `json:"transportCompany,omitempty"` // de notes del manifiesto
	Observation      string   `json:"observation,omitempty"`      // notes sin la empresa del transporte
	DetailLines      []string `json:"detailLines"`                // el antes y el después completo
}

type ActivityEntity struct {
	Type  string `json:"type"` // order | manifest | user
	ID    string `json:"id"`
	Label string `json:"label"`
}

type Activity struct {
	repo ports.ActivityReader
	now  func() time.Time
}

func NewActivity(repo ports.ActivityReader) *Activity { return &Activity{repo: repo, now: time.Now} }

func canReadActivity(id tenant.Identity) bool {
	return id.HasRole(tenant.RoleAdmin, tenant.RoleJefeOps)
}

func (a *Activity) List(ctx context.Context, id tenant.Identity, f ports.ActivityFilter) ([]ActivityItem, int, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	return a.list(ctx, id, f)
}

func (a *Activity) list(ctx context.Context, id tenant.Identity, f ports.ActivityFilter) ([]ActivityItem, int, error) {
	if !canReadActivity(id) {
		return nil, 0, fail(domain.ErrForbidden, "El registro de acciones es solo para Administrador y Jefe de Operaciones.")
	}
	if f.Module != "" && !slices.Contains(ActivityModules, f.Module) {
		return nil, 0, fail(domain.ErrInvalid, "Apartado desconocido: usa orders, packing, dispatch, users o products.")
	}
	if f.Kind != "" && !slices.Contains(activityKinds, f.Kind) {
		return nil, 0, fail(domain.ErrInvalid, "Tipo de actor desconocido: usa human o system.")
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
	f.Since = a.now().Add(-ActivityRetention)
	entries, total, err := a.repo.Activity(ctx, f)
	if err != nil {
		return nil, 0, err
	}
	out := make([]ActivityItem, 0, len(entries))
	for _, e := range entries {
		it := ActivityItem{
			ID: e.ID, OccurredAt: e.OccurredAt,
			Actor:  Actor{ID: e.ActorID, Name: e.ActorName, Role: e.ActorRole, Kind: e.ActorKind},
			Module: e.Module, Action: e.Action,
			Entity:    ActivityEntity{Type: e.EntityType, ID: e.EntityID, Label: e.EntityLabel},
			CompanyID: e.CompanyID, CompanyName: e.CompanyName, Warehouse: e.Warehouse, Reason: e.Reason,
			Details: safeDetails(e.Details),
		}
		presentActivity(&it, e.CourierName)
		out = append(out, it)
	}
	return out, total, nil
}
