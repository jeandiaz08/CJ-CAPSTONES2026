-- Core: empresas cliente, usuarios/roles, productos y catálogos.
-- Cada tabla tiene UN dueño que escribe (DA-03). Dueño de estas: el Core.
-- Órdenes las LEE directamente (DA-12).
BEGIN;

CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE IF NOT EXISTS companies (
    id           BIGSERIAL PRIMARY KEY,
    name         TEXT        NOT NULL,
    tax_id       TEXT        UNIQUE,             -- RUT
    wms_code     TEXT,                           -- código de vinculación con Invas (RF-04)
    active       BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Usuarios (RF-01/02). Roles del DAS v1.0 §4.5. "despacho" es un permiso.
CREATE TABLE IF NOT EXISTS users (
    id             BIGSERIAL PRIMARY KEY,
    email          CITEXT      NOT NULL UNIQUE,
    name           TEXT        NOT NULL,
    password_hash  TEXT        NOT NULL,
    role           TEXT        NOT NULL CHECK (role IN ('admin','jefe_operaciones','analista','empaque','cliente')),
    permissions    TEXT[]      NOT NULL DEFAULT '{}',
    all_companies  BOOLEAN     NOT NULL DEFAULT FALSE,  -- personal interno (PV-02)
    active         BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS user_companies (
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    company_id  BIGINT NOT NULL REFERENCES companies(id),
    PRIMARY KEY (user_id, company_id)
);

CREATE TABLE IF NOT EXISTS products (
    id           BIGSERIAL PRIMARY KEY,
    company_id   BIGINT      NOT NULL REFERENCES companies(id),
    sku          TEXT        NOT NULL,
    description  TEXT        NOT NULL,
    type         TEXT        NOT NULL DEFAULT 'simple' CHECK (type IN ('simple','kit')),
    active       BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (company_id, sku)
);
CREATE INDEX IF NOT EXISTS idx_products_company ON products(company_id);

-- Catálogos de baja variabilidad (RF-29)
CREATE TABLE IF NOT EXISTS regions (
    code        TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    sort_order  INT  NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS couriers (
    code        TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    active      BOOLEAN NOT NULL DEFAULT TRUE
);

COMMIT;
