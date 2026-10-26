-- DAS v1.1 / PV-18: la identidad (contraseña y sesión) vive en Keycloak.
-- El Core conserva el PERFIL de Nexus: rol, permisos, empresas y si está
-- activo, vinculado por el identificador de Keycloak (claim "sub").
BEGIN;

ALTER TABLE users ADD COLUMN IF NOT EXISTS keycloak_id UUID UNIQUE;
ALTER TABLE users DROP COLUMN IF EXISTS password_hash;   -- ya no se guardan contraseñas
ALTER TABLE users ADD COLUMN IF NOT EXISTS last_login_at TIMESTAMPTZ;
ALTER TABLE users ADD COLUMN IF NOT EXISTS created_by BIGINT REFERENCES users(id);
ALTER TABLE users ADD COLUMN IF NOT EXISTS updated_by BIGINT REFERENCES users(id);

-- Permisos por perfil y módulo (H-02 T01). El menú de la interfaz se arma con
-- esto, pero la autorización definitiva la aplica cada endpoint en el servidor.
CREATE TABLE IF NOT EXISTS role_modules (
    role     TEXT   NOT NULL CHECK (role IN ('admin','jefe_operaciones','analista','empaque','cliente')),
    module   TEXT   NOT NULL,
    actions  TEXT[] NOT NULL DEFAULT '{view}',
    PRIMARY KEY (role, module)
);

INSERT INTO role_modules (role, module, actions) VALUES
  ('admin','dashboard','{view}'), ('admin','ordenes','{view,create,edit,cancel,export}'),
  ('admin','olas','{view,assign}'), ('admin','empaque','{view,pack}'), ('admin','retiro','{view,dispatch}'),
  ('admin','registro','{view}'), ('admin','clientes','{view,create,edit}'), ('admin','productos','{view,create,edit,import,export}'),
  ('admin','usuarios','{view,create,edit,disable}'), ('admin','reglas_olas','{view,edit}'), ('admin','integraciones','{view,edit}'),

  ('jefe_operaciones','dashboard','{view}'), ('jefe_operaciones','ordenes','{view,create,edit,cancel,export,resolve,prioritize}'),
  ('jefe_operaciones','olas','{view,assign}'), ('jefe_operaciones','empaque','{view,pack}'), ('jefe_operaciones','retiro','{view,dispatch}'),
  ('jefe_operaciones','registro','{view}'), ('jefe_operaciones','clientes','{view}'), ('jefe_operaciones','productos','{view}'),
  ('jefe_operaciones','reglas_olas','{view,edit}'),

  ('analista','dashboard','{view}'), ('analista','ordenes','{view,export}'), ('analista','registro','{view}'),
  ('analista','clientes','{view}'), ('analista','productos','{view}'),

  ('empaque','empaque','{view,pack}'), ('empaque','retiro','{view}'),

  ('cliente','dashboard','{view}'), ('cliente','ordenes','{view,create,edit,cancel,export}'),
  ('cliente','registro','{view}'), ('cliente','productos','{view}')
ON CONFLICT (role, module) DO NOTHING;

-- Registro de quién cambia usuarios, roles, permisos y empresas (RF-02).
-- Solo agregado: no se actualiza ni se borra.
CREATE TABLE IF NOT EXISTS user_audit (
    id              BIGSERIAL PRIMARY KEY,
    actor_user_id   BIGINT      REFERENCES users(id),     -- NULL = sistema (primer ingreso)
    target_user_id  BIGINT      NOT NULL REFERENCES users(id),
    action          TEXT        NOT NULL,                  -- created, updated, disabled, enabled, provisioned, linked
    detail          JSONB       NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_user_audit_target ON user_audit(target_user_id, created_at DESC);

COMMIT;
