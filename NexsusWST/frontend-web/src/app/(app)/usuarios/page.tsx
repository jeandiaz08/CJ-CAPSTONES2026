import { ModulePlaceholder } from "@/components/ModulePlaceholder";

export default function Page() {
  return (
    <ModulePlaceholder
      title="Usuarios y roles"
      rf="RF-02"
      description="Crear, editar, habilitar/deshabilitar usuarios; asignar rol, permisos (p. ej. Despacho) y empresas autorizadas."
      frontend="frontend-web/src/app/(app)/usuarios/page.tsx"
      backend="backend-core/internal/modules/auth_roles/ (tablas users y user_companies)"
      notes={["Roles: Administrador, Jefe de Operaciones, Analista, Empaque y Cliente. Despacho es un permiso, no un rol.", "Para generar el hash de una contraseña: docker compose exec core /app/tmp/api -hash-password 'clave'", "Al deshabilitar un usuario, revoca sus sesiones (Revocations.RevokeUser)."]}
    />
  );
}
