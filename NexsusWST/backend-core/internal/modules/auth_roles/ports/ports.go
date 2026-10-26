package ports

import (
	"context"

	"github.com/nexuswst/backend-core/internal/modules/auth_roles/domain"
)

type ListFilter struct {
	Query  string // correo o nombre
	Role   string
	Active *bool
	Limit  int
	Offset int
}

// UserRepository - perfiles de Nexus en PostgreSQL (dueño: Core).
type UserRepository interface {
	GetByID(ctx context.Context, id int64) (domain.User, error)
	GetByEmail(ctx context.Context, email string) (domain.User, error)
	List(ctx context.Context, f ListFilter) ([]domain.User, int, error)
	Insert(ctx context.Context, keycloakID string, d domain.Draft, actorID *int64) (domain.User, error)
	Update(ctx context.Context, id int64, d domain.Draft, actorID int64) error
	SetActive(ctx context.Context, id int64, active bool, actorID int64) error
	LinkKeycloak(ctx context.Context, id int64, keycloakID string) error
	RelinkKeycloak(ctx context.Context, id int64, oldID, newID string) error
	TouchLogin(ctx context.Context, id int64) error
	CompaniesExist(ctx context.Context, ids []int64) (bool, error)
	Audit(ctx context.Context, actorID *int64, targetID int64, action string, detail map[string]any) error
	AuditOf(ctx context.Context, targetID int64, limit int) ([]domain.AuditEntry, error)
	Modules(ctx context.Context, role string) ([]domain.ModuleAccess, error)
}

// IdentityProvider - Admin API de Keycloak (cuenta de servicio nexus-core).
type IdentityProvider interface {
	CreateUser(ctx context.Context, email, name string) (keycloakID string, err error) // domain.ErrEmailTaken si existe
	FindByEmail(ctx context.Context, email string) (keycloakID string, err error)      // domain.ErrNotFound si no existe
	Exists(ctx context.Context, keycloakID string) (bool, error)
	UpdateName(ctx context.Context, keycloakID, name string) error
	SetRole(ctx context.Context, keycloakID, role string) error // deja un solo rol base
	SetEnabled(ctx context.Context, keycloakID string, enabled bool) error
	Logout(ctx context.Context, keycloakID string) error // cierra todas sus sesiones
	SendSetPasswordEmail(ctx context.Context, keycloakID string) error
	DeleteUser(ctx context.Context, keycloakID string) error // solo para deshacer un alta fallida
}
