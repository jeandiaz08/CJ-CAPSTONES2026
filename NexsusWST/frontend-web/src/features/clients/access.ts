// Acceso del módulo Clientes (Matriz de perfiles v1.2, regla 10 y 11).
// La interfaz solo oculta o deshabilita: el servidor responde 403 sin el permiso (regla 7).
import { PERM_MANAGE_PROFILES, type AccessUser } from "@/state/menuAccess";

import type { ProfileScope } from "@/services/clients";

// Alcance de las pantallas de perfiles: «client» = Clientes › Administración clientes;
// «internal» = Configuración › Usuarios WeStorage (Tarea 08, mismos componentes).
export type { ProfileScope };

// ¿Puede crear, editar, activar o desactivar perfiles y empresas, y aprobar solicitudes?
// Se pregunta por el permiso, nunca por el perfil.
// TODO(futuro): KAM y SAC tendrán un permiso parecido limitado a scope «client»
// (perfiles con rol Cliente). Cuando exista, se acepta aquí solo para ese alcance.
export function canManageProfiles(user: AccessUser | null | undefined, scope: ProfileScope): boolean {
  if (!user) return false;
  switch (scope) {
    case "client":
    case "internal":
      return user.permissions.includes(PERM_MANAGE_PROFILES);
    default:
      return false;
  }
}

// Usuarios WeStorage (Tarea 08): roles que se ofrecen y permisos que admite cada uno (Matriz v1.2).
// Analista, KAM y SAC son perfiles futuros (regla 11): no se ofrecen. El servidor valida lo mismo.
export const INTERNAL_ROLES = ["admin", "jefe_operaciones", "empaque"] as const;
export const ROLE_PERMISSIONS: Record<string, string[]> = {
  admin: [PERM_MANAGE_PROFILES],
  jefe_operaciones: ["despacho"],
  empaque: ["despacho"],
};
export const PROFILE_ROLE_LABELS: Record<string, string> = {
  admin: "Administrador",
  jefe_operaciones: "Jefe de Operaciones",
  empaque: "Empaque",
  cliente: "Cliente",
};
export const PERMISSION_LABELS: Record<string, string> = { despacho: "Despacho", [PERM_MANAGE_PROFILES]: "Gestionar perfiles" };
