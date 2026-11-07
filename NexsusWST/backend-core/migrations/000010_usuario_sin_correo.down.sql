-- Revertir exige que no queden usuarios sin correo (si los hay, falla a propósito).
BEGIN;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_email_or_username;
ALTER TABLE users DROP COLUMN IF EXISTS username;
ALTER TABLE users ALTER COLUMN email SET NOT NULL;
COMMIT;
