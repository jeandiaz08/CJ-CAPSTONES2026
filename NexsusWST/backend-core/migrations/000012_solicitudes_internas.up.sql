-- Tarea 08 · Fase 2 (H-02): solicitudes de perfiles de WeStorage (Configuración › Usuarios WeStorage).
-- profile_requests guarda también solicitudes internas: sin empresa, con el rol, los permisos y, si
-- el perfil no tiene correo, el nombre de usuario propuestos. Las de clientes no cambian.
BEGIN;

ALTER TABLE profile_requests ADD COLUMN IF NOT EXISTS scope TEXT NOT NULL DEFAULT 'client';
ALTER TABLE profile_requests ADD COLUMN IF NOT EXISTS profile_username CITEXT;
ALTER TABLE profile_requests ADD COLUMN IF NOT EXISTS profile_role TEXT;
ALTER TABLE profile_requests ADD COLUMN IF NOT EXISTS profile_permissions TEXT[];
ALTER TABLE profile_requests ALTER COLUMN company_id DROP NOT NULL;

ALTER TABLE profile_requests ADD CONSTRAINT profile_requests_scope CHECK (scope IN ('client', 'internal'));
-- Una solicitud de cliente siempre es de una empresa; una interna, de ninguna.
ALTER TABLE profile_requests ADD CONSTRAINT profile_requests_company CHECK ((scope = 'client') = (company_id IS NOT NULL));
-- Rol, permisos y usuario propuestos solo en las internas; un perfil interno nuevo siempre trae rol.
ALTER TABLE profile_requests ADD CONSTRAINT profile_requests_internal_fields CHECK (
    scope = 'internal' OR (profile_username IS NULL AND profile_role IS NULL AND profile_permissions IS NULL));
ALTER TABLE profile_requests ADD CONSTRAINT profile_requests_role CHECK (
    profile_role IS NULL OR profile_role IN ('admin', 'jefe_operaciones', 'empaque'));
ALTER TABLE profile_requests ADD CONSTRAINT profile_requests_internal_new CHECK (
    scope = 'client' OR kind <> 'new_profile' OR profile_role IS NOT NULL);

-- Una sola solicitud pendiente por perfil y tipo. Con company_id nulo (internas), NULLS NOT DISTINCT
-- hace que dos pendientes internas iguales choquen (sin él, los NULL nunca son iguales). Un perfil
-- nuevo se identifica por su correo, su usuario o, sin ninguno, por su nombre.
DROP INDEX IF EXISTS uq_profile_requests_pending;
CREATE UNIQUE INDEX uq_profile_requests_pending ON profile_requests (
    scope, company_id, kind,
    COALESCE(target_user_id::text, lower(profile_email::text), lower(profile_username::text), lower(btrim(profile_name))))
    NULLS NOT DISTINCT WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_profile_requests_scope ON profile_requests (scope, requested_at DESC);

COMMIT;
