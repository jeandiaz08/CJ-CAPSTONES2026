-- Se ejecuta SOLO la primera vez que se crea el volumen de PostgreSQL.
-- Las tablas NO se crean aquí: viven en las migraciones de cada servicio
-- (backend-core/migrations y orders-service/migrations).

CREATE EXTENSION IF NOT EXISTS pgcrypto;   -- gen_random_uuid()
CREATE EXTENSION IF NOT EXISTS citext;     -- textos sin distinguir mayúsculas
