-- Tarea 07 (Jean): módulo Clientes — empresas, perfiles de clientes y solicitudes de perfiles
-- (H-02, H-03, RF-02 a RF-05; Matriz de perfiles v1.2). Manda la base de datos: lo que aquí es
-- obligatorio o único también lo exige la pantalla.
BEGIN;

-- 1) Empresas: el RUT pasa a obligatorio. Si alguna empresa no lo tiene, la migración se detiene
--    con un mensaje claro: no se inventan RUT.
DO $$
DECLARE
    missing TEXT;
BEGIN
    SELECT string_agg(format('#%s «%s»', id, name), ', ' ORDER BY id) INTO missing
    FROM companies WHERE tax_id IS NULL OR btrim(tax_id) = '';
    IF missing IS NOT NULL THEN
        RAISE EXCEPTION 'Migración 000011: hay empresas sin RUT (%). Completa companies.tax_id y vuelve a migrar.', missing;
    END IF;
END $$;
ALTER TABLE companies ALTER COLUMN tax_id SET NOT NULL;
-- El RUT es único sin importar puntos, guion ni mayúsculas («76.123.456-0» = «761234560»).
CREATE UNIQUE INDEX IF NOT EXISTS uq_companies_tax_id_norm ON companies (upper(regexp_replace(tax_id, '[^0-9kK]', '', 'g')));

-- Correo y teléfono de contacto (opcionales) y quién creó o editó por última vez.
ALTER TABLE companies ADD COLUMN IF NOT EXISTS email CITEXT
    CHECK (email IS NULL OR email ~ '^[^@\s]+@[^@\s]+\.[^@\s]+$');
ALTER TABLE companies ADD COLUMN IF NOT EXISTS phone TEXT
    CHECK (phone IS NULL OR length(phone) BETWEEN 6 AND 30);
ALTER TABLE companies ADD COLUMN IF NOT EXISTS created_by BIGINT REFERENCES users(id);
ALTER TABLE companies ADD COLUMN IF NOT EXISTS updated_by BIGINT REFERENCES users(id);

-- 2) Perfiles: teléfono y cargo (opcionales).
ALTER TABLE users ADD COLUMN IF NOT EXISTS phone TEXT
    CHECK (phone IS NULL OR length(phone) BETWEEN 6 AND 30);
ALTER TABLE users ADD COLUMN IF NOT EXISTS job_title TEXT
    CHECK (job_title IS NULL OR length(job_title) BETWEEN 1 AND 80);

-- Contar los perfiles de cada empresa sin recorrer toda la tabla.
CREATE INDEX IF NOT EXISTS idx_user_companies_company ON user_companies (company_id);

-- 3) Solicitudes de perfiles: el Cliente solicita un perfil nuevo, una activación o una
--    desactivación; aprueba o rechaza quien tenga «Gestionar perfiles».
CREATE TABLE IF NOT EXISTS profile_requests (
    id                BIGSERIAL   PRIMARY KEY,
    company_id        BIGINT      NOT NULL REFERENCES companies(id),
    kind              TEXT        NOT NULL CHECK (kind IN ('new_profile', 'activation', 'deactivation')),
    -- Perfil afectado: vacío solo al pedir un perfil nuevo.
    target_user_id    BIGINT      REFERENCES users(id),
    -- Datos del perfil (en activación y desactivación, el nombre es una copia del perfil).
    profile_name      TEXT        NOT NULL CHECK (length(btrim(profile_name)) BETWEEN 1 AND 120),
    profile_email     CITEXT      CHECK (profile_email IS NULL OR profile_email ~ '^[^@\s]+@[^@\s]+\.[^@\s]+$'),
    profile_phone     TEXT        CHECK (profile_phone IS NULL OR length(profile_phone) BETWEEN 6 AND 30),
    profile_job_title TEXT        CHECK (profile_job_title IS NULL OR length(profile_job_title) BETWEEN 1 AND 80),
    reason            TEXT        NOT NULL CHECK (length(btrim(reason)) BETWEEN 1 AND 500),
    status            TEXT        NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected')),
    requested_by      BIGINT      NOT NULL REFERENCES users(id),
    requested_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    reviewed_by       BIGINT      REFERENCES users(id),
    reviewed_at       TIMESTAMPTZ,
    rejection_reason  TEXT,
    -- Perfil creado al aprobar una solicitud de perfil nuevo.
    created_user_id   BIGINT      REFERENCES users(id),
    CONSTRAINT profile_requests_target CHECK ((kind = 'new_profile') = (target_user_id IS NULL)),
    CONSTRAINT profile_requests_review CHECK ((status = 'pending') = (reviewed_by IS NULL AND reviewed_at IS NULL)),
    CONSTRAINT profile_requests_rejection CHECK (
        (status = 'rejected' AND length(btrim(COALESCE(rejection_reason, ''))) BETWEEN 1 AND 500)
        OR (status <> 'rejected' AND rejection_reason IS NULL)),
    CONSTRAINT profile_requests_created CHECK (created_user_id IS NULL OR (kind = 'new_profile' AND status = 'approved'))
);
-- No se permite una segunda solicitud pendiente para el mismo perfil y tipo. Un perfil nuevo se
-- identifica por su correo o, sin correo, por su nombre, dentro de la empresa.
CREATE UNIQUE INDEX IF NOT EXISTS uq_profile_requests_pending ON profile_requests (
    company_id, kind,
    COALESCE(target_user_id::text, lower(profile_email::text), lower(btrim(profile_name))))
    WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_profile_requests_status ON profile_requests (status, requested_at DESC);
CREATE INDEX IF NOT EXISTS idx_profile_requests_company ON profile_requests (company_id, requested_at DESC);

-- 4) Auditoría: user_audit también registra acciones sobre empresas y solicitudes, para que
--    aparezcan en Registro de acciones › Usuarios (vista activity_log, Órdenes 000010).
ALTER TABLE user_audit ALTER COLUMN target_user_id DROP NOT NULL;
ALTER TABLE user_audit ADD COLUMN IF NOT EXISTS target_company_id BIGINT REFERENCES companies(id);
ALTER TABLE user_audit ADD COLUMN IF NOT EXISTS request_id BIGINT REFERENCES profile_requests(id);
ALTER TABLE user_audit DROP CONSTRAINT IF EXISTS user_audit_has_target;
ALTER TABLE user_audit ADD CONSTRAINT user_audit_has_target
    CHECK (num_nonnulls(target_user_id, target_company_id, request_id) >= 1);
CREATE INDEX IF NOT EXISTS idx_user_audit_company ON user_audit (target_company_id, created_at DESC)
    WHERE target_company_id IS NOT NULL;

COMMIT;
