-- Deshace 000012. Las solicitudes internas (y su auditoría) se borran: la tabla vuelve a exigir empresa.
BEGIN;

DELETE FROM user_audit WHERE request_id IN (SELECT id FROM profile_requests WHERE scope = 'internal') AND target_user_id IS NULL;
UPDATE user_audit SET request_id = NULL WHERE request_id IN (SELECT id FROM profile_requests WHERE scope = 'internal');
DELETE FROM profile_requests WHERE scope = 'internal';

DROP INDEX IF EXISTS idx_profile_requests_scope;
DROP INDEX IF EXISTS uq_profile_requests_pending;
ALTER TABLE profile_requests DROP CONSTRAINT IF EXISTS profile_requests_internal_new;
ALTER TABLE profile_requests DROP CONSTRAINT IF EXISTS profile_requests_role;
ALTER TABLE profile_requests DROP CONSTRAINT IF EXISTS profile_requests_internal_fields;
ALTER TABLE profile_requests DROP CONSTRAINT IF EXISTS profile_requests_company;
ALTER TABLE profile_requests DROP CONSTRAINT IF EXISTS profile_requests_scope;
ALTER TABLE profile_requests ALTER COLUMN company_id SET NOT NULL;
ALTER TABLE profile_requests DROP COLUMN IF EXISTS profile_permissions;
ALTER TABLE profile_requests DROP COLUMN IF EXISTS profile_role;
ALTER TABLE profile_requests DROP COLUMN IF EXISTS profile_username;
ALTER TABLE profile_requests DROP COLUMN IF EXISTS scope;

CREATE UNIQUE INDEX IF NOT EXISTS uq_profile_requests_pending ON profile_requests (
    company_id, kind,
    COALESCE(target_user_id::text, lower(profile_email::text), lower(btrim(profile_name))))
    WHERE status = 'pending';

COMMIT;
