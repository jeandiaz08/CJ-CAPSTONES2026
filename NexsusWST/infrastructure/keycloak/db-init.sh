#!/bin/sh
# Prepara el usuario y el esquema "keycloak" en PostgreSQL (DAS §7.1).
# Es idempotente: se ejecuta en cada arranque (servicio keycloak-db-init) y
# funciona también sobre una base que ya tenía datos (los scripts de
# /docker-entrypoint-initdb.d solo corren cuando el volumen está vacío).
set -eu

: "${KEYCLOAK_DB_PASSWORD:?Falta KEYCLOAK_DB_PASSWORD en .env}"

psql -v ON_ERROR_STOP=1 -v kcpass="$KEYCLOAK_DB_PASSWORD" -v dbname="$POSTGRES_DB" <<'SQL'
SELECT format('CREATE ROLE keycloak LOGIN PASSWORD %L', :'kcpass')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'keycloak') \gexec
SELECT format('ALTER ROLE keycloak PASSWORD %L', :'kcpass') \gexec
CREATE SCHEMA IF NOT EXISTS keycloak AUTHORIZATION keycloak;
ALTER ROLE keycloak SET search_path = keycloak;
SELECT format('GRANT CONNECT ON DATABASE %I TO keycloak', :'dbname') \gexec
-- Keycloak solo ve su esquema: sin permisos sobre las tablas de Nexus.
REVOKE ALL ON SCHEMA public FROM keycloak;
SQL
echo "esquema keycloak listo"
