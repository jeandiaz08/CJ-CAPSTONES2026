package tenant

import (
	"net/http"

	"github.com/nexuswst/backend-core/internal/shared/httpx"
)

// PermManageProfiles es el permiso «Gestionar perfiles» (Matriz de perfiles v1.2):
// crear, editar, activar y desactivar perfiles y empresas, y aprobar o rechazar
// solicitudes de perfiles. Como «Despacho», es un permiso y no un perfil.
const PermManageProfiles = "gestionar_perfiles"

// ProfileScope indica sobre qué perfiles se quiere actuar.
type ProfileScope string

const (
	ProfileScopeAll    ProfileScope = "all"    // perfiles de clientes y de WeStorage
	ProfileScopeClient ProfileScope = "client" // solo perfiles con rol Cliente y sus empresas
)

// CanManageProfiles responde «¿tiene un permiso de gestión de perfiles que cubra
// este alcance?». Se valida por permiso, nunca por perfil (regla 11 de la matriz).
//
// TODO(futuro): los perfiles KAM y SAC tendrán un permiso parecido, limitado a
// ProfileScopeClient (crear y gestionar perfiles con rol Cliente). Cuando exista,
// se agrega aquí: «gestionar_perfiles» cubre todo y el nuevo, solo ProfileScopeClient.
func (id Identity) CanManageProfiles(scope ProfileScope) bool {
	switch scope {
	case ProfileScopeAll, ProfileScopeClient:
		return id.HasPermission(PermManageProfiles)
	default:
		return false
	}
}

// CanViewClientAdmin indica si puede consultar Clientes › Administración clientes:
// el Administrador (en lectura si no tiene el permiso) y quien pueda gestionar
// perfiles de clientes.
func (id Identity) CanViewClientAdmin() bool {
	return id.HasRole(RoleAdmin) || id.CanManageProfiles(ProfileScopeClient)
}

// ErrNoManageProfiles es el mensaje del 403 cuando falta el permiso.
const ErrNoManageProfiles = "Necesitas el permiso Gestionar perfiles para crear o modificar perfiles, empresas o solicitudes."

// RequireManageProfiles limita una ruta a quien tenga un permiso de gestión de
// perfiles para el alcance indicado. Responde 403 en otro caso.
func RequireManageProfiles(scope ProfileScope) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, ok := From(r.Context())
			if !ok || !id.CanManageProfiles(scope) {
				httpx.Error(w, http.StatusForbidden, "forbidden", ErrNoManageProfiles)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
