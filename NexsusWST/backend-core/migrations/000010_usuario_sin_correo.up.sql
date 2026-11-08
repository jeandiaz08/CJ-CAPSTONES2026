-- H-02-T09 (SCRUM-231): usuarios de bodega sin correo.
-- El administrador los crea con nombre de usuario y contraseña temporal; como
-- no pueden recuperar la contraseña por correo, el administrador la restablece.
-- Cada perfil tiene correo, nombre de usuario o ambos.
BEGIN;
ALTER TABLE users ALTER COLUMN email DROP NOT NULL;
ALTER TABLE users ADD COLUMN IF NOT EXISTS username CITEXT UNIQUE
    CHECK (username IS NULL OR username ~ '^[a-z0-9][a-z0-9._-]{2,39}$');
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_email_or_username;
ALTER TABLE users ADD CONSTRAINT users_email_or_username CHECK (email IS NOT NULL OR username IS NOT NULL);
COMMIT;
