package ports

import (
	"context"
	"time"

	"github.com/nexuswst/backend-core/internal/modules/auth_roles/domain"
)

// ProfileFilter - perfiles de Clientes (Tarea 07) y de Usuarios WeStorage (Tarea 08).
type ProfileFilter struct {
	Scope     domain.ProfileScope
	Query     string // nombre, correo, usuario o empresa
	CompanyID int64
	Active    *bool
	From, To  *time.Time // fecha de creación (día de Chile), inclusive
	IDs       []int64
	Limit     int
	Offset    int
}

// Profile es el usuario más sus datos de contacto y su empresa (los perfiles Cliente tienen una;
// los internos, ninguna).
type Profile struct {
	domain.User
	Phone       string
	JobTitle    string
	CompanyID   int64
	CompanyName string
}

// ProfileRepository - lecturas y datos de contacto de los perfiles (users.phone, users.job_title).
type ProfileRepository interface {
	ListProfiles(ctx context.Context, f ProfileFilter) ([]Profile, int, error)
	GetProfile(ctx context.Context, id int64) (Profile, error)
	SetContact(ctx context.Context, id int64, phone, jobTitle string) error
	// CompanyByTaxID busca una empresa por RUT (con o sin puntos y guion), para la importación.
	CompanyByTaxID(ctx context.Context, taxID string) (id int64, name string, active bool, err error)
	// Company devuelve el nombre y si está activa (para validar el formulario).
	Company(ctx context.Context, id int64) (name string, active bool, err error)
	// ActiveCompanies: nombre y RUT de las empresas activas (hoja «Instrucciones» de la plantilla).
	ActiveCompanies(ctx context.Context) ([][2]string, error)
	// AuditCompany deja una acción sobre una empresa en user_audit (resumen de la importación).
	AuditCompany(ctx context.Context, actorID, companyID int64, action string, detail map[string]any) error
	// CountManagers: Administradores activos con «Gestionar perfiles», sin contar a exceptID (Tarea 08).
	CountManagers(ctx context.Context, exceptID int64) (int, error)
}
