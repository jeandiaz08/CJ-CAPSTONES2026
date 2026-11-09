-- Deshace 000011. Las filas de auditoría de empresas o solicitudes (sin usuario) se borran,
-- porque la columna target_user_id vuelve a ser obligatoria.
BEGIN;

DELETE FROM user_audit WHERE target_user_id IS NULL;
DROP INDEX IF EXISTS idx_user_audit_company;
ALTER TABLE user_audit DROP CONSTRAINT IF EXISTS user_audit_has_target;
ALTER TABLE user_audit DROP COLUMN IF EXISTS request_id;
ALTER TABLE user_audit DROP COLUMN IF EXISTS target_company_id;
ALTER TABLE user_audit ALTER COLUMN target_user_id SET NOT NULL;

DROP TABLE IF EXISTS profile_requests;

DROP INDEX IF EXISTS idx_user_companies_company;
ALTER TABLE users DROP COLUMN IF EXISTS job_title;
ALTER TABLE users DROP COLUMN IF EXISTS phone;

ALTER TABLE companies DROP COLUMN IF EXISTS updated_by;
ALTER TABLE companies DROP COLUMN IF EXISTS created_by;
ALTER TABLE companies DROP COLUMN IF EXISTS phone;
ALTER TABLE companies DROP COLUMN IF EXISTS email;
DROP INDEX IF EXISTS uq_companies_tax_id_norm;
ALTER TABLE companies ALTER COLUMN tax_id DROP NOT NULL;

COMMIT;
