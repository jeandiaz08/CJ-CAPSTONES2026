package ports

import (
	"context"
	"time"

	"github.com/nexuswst/backend-core/internal/modules/auth_roles/domain"
)

// RequestFilter - Solicitudes (Administrador: todas; Cliente: las de su empresa).
type RequestFilter struct {
	Scope      domain.ProfileScope // client (por defecto) o internal
	CompanyIDs []int64             // nil = todas
	Status     domain.RequestStatus
	Kind       domain.RequestKind
	Query      string // empresa o nombre del perfil
	From, To   *time.Time
	IDs        []int64
	Limit      int
	Offset     int
}

// RequestCounts - KPIs Pendientes · Aprobadas · Rechazadas (con los mismos filtros, sin el estado).
type RequestCounts struct {
	Pending  int `json:"pending"`
	Approved int `json:"approved"`
	Rejected int `json:"rejected"`
}

type RequestRepository interface {
	// Create devuelve domain.ErrRequestPending si ya hay una pendiente para el mismo perfil y tipo.
	// companyID 0 = solicitud interna de WeStorage (sin empresa).
	Create(ctx context.Context, companyID, requestedBy int64, d domain.RequestDraft) (int64, error)
	Get(ctx context.Context, id int64) (domain.ProfileRequest, error)
	List(ctx context.Context, f RequestFilter) ([]domain.ProfileRequest, int, RequestCounts, error)
	// Approve y Reject devuelven domain.ErrRequestReviewed si ya no está pendiente.
	Approve(ctx context.Context, id, reviewerID int64, createdUserID *int64) error
	Reject(ctx context.Context, id, reviewerID int64, reason string) error
	// PendingFor indica las solicitudes pendientes por perfil (para «Solicitud pendiente»).
	PendingFor(ctx context.Context, userIDs []int64) (map[int64][]domain.RequestKind, error)
	// AuditRequest deja la acción en user_audit (request_id y, si hay, el perfil afectado).
	AuditRequest(ctx context.Context, actorID, requestID int64, targetUserID *int64, action string, detail map[string]any) error
	// ManagerEmails: correos de los usuarios activos con «Gestionar perfiles» (avisos por correo).
	ManagerEmails(ctx context.Context) ([]string, error)
}
