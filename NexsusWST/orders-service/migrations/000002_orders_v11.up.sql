-- Órdenes v1.1 (ERS v1.1, DAS v1.1, CU-01 a CU-08, H-05 a H-15).
-- Dueño: servicio de Órdenes. Lee companies, products, users y communes (Core).
BEGIN;

-- ---------------------------------------------------------------------------
-- Orden: destino, despacho, origen y control de empaque
-- ---------------------------------------------------------------------------
ALTER TABLE orders
    ADD COLUMN IF NOT EXISTS recipient_name     TEXT,
    ADD COLUMN IF NOT EXISTS recipient_phone    TEXT,
    ADD COLUMN IF NOT EXISTS recipient_email    TEXT,
    ADD COLUMN IF NOT EXISTS address_line       TEXT,
    ADD COLUMN IF NOT EXISTS commune            TEXT,      -- nombre normalizado (catálogo communes)
    ADD COLUMN IF NOT EXISTS region_code        TEXT,
    ADD COLUMN IF NOT EXISTS dispatch_type      TEXT NOT NULL DEFAULT 'domicilio', -- domicilio, retiro_oficina, express
    ADD COLUMN IF NOT EXISTS warehouse          TEXT NOT NULL DEFAULT 'principal',
    ADD COLUMN IF NOT EXISTS customer_notes     TEXT,
    ADD COLUMN IF NOT EXISTS courier_code       TEXT,      -- código/seguimiento entregado por el courier
    ADD COLUMN IF NOT EXISTS source             TEXT NOT NULL DEFAULT 'manual' CHECK (source IN ('manual','bulk','webhook')),
    ADD COLUMN IF NOT EXISTS import_id          BIGINT,
    ADD COLUMN IF NOT EXISTS created_by         BIGINT,    -- users.id (NULL = integración)
    ADD COLUMN IF NOT EXISTS incident_kind      TEXT,      -- tipo de la incidencia abierta
    ADD COLUMN IF NOT EXISTS claimed_by         BIGINT,    -- Empaque: quién tomó la orden
    ADD COLUMN IF NOT EXISTS claimed_at         TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS rules_checked_at   TIMESTAMPTZ, -- motor de reglas de ola ya la evaluó
    ADD COLUMN IF NOT EXISTS handed_over_at     TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS cancelled_at       TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_orders_created ON orders(company_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_orders_courier_status ON orders(courier, status);
CREATE INDEX IF NOT EXISTS idx_orders_rules ON orders(id) WHERE status = 'validated' AND wave_id IS NULL AND rules_checked_at IS NULL;

-- ---------------------------------------------------------------------------
-- Bultos (RF-22) y documentos (RF-09/RF-23)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS order_packages (
    id           BIGSERIAL PRIMARY KEY,
    order_id     BIGINT       NOT NULL REFERENCES orders(id),
    code         TEXT         NOT NULL UNIQUE,     -- etiqueta del bulto, p. ej. NX-123-1
    weight_kg    NUMERIC(8,2),
    length_cm    NUMERIC(8,1),
    width_cm     NUMERIC(8,1),
    height_cm    NUMERIC(8,1),
    created_by   BIGINT,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_packages_order ON order_packages(order_id);

CREATE TABLE IF NOT EXISTS order_documents (
    id            BIGSERIAL PRIMARY KEY,
    order_id      BIGINT      NOT NULL REFERENCES orders(id),
    company_id    BIGINT      NOT NULL,
    kind          TEXT        NOT NULL DEFAULT 'otro' CHECK (kind IN ('factura','boleta','guia_despacho','etiqueta','otro')),
    filename      TEXT        NOT NULL,
    content_type  TEXT        NOT NULL,
    size_bytes    BIGINT      NOT NULL,
    sha256        TEXT        NOT NULL,
    storage_key   TEXT        NOT NULL UNIQUE,
    uploaded_by   TEXT        NOT NULL,
    uploaded_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_documents_order ON order_documents(order_id);

-- ---------------------------------------------------------------------------
-- Carga masiva (RF-09): resultado por fila
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS order_imports (
    id          BIGSERIAL PRIMARY KEY,
    company_id  BIGINT      NOT NULL,
    filename    TEXT        NOT NULL,
    total_rows  INT         NOT NULL DEFAULT 0,
    created     INT         NOT NULL DEFAULT 0,
    failed      INT         NOT NULL DEFAULT 0,
    errors      JSONB       NOT NULL DEFAULT '[]'::jsonb,
    created_by  TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------------
-- Reglas de asignación a olas (RF-17, CU-02): gana la primera coincidencia
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS wave_rules (
    id             BIGSERIAL PRIMARY KEY,
    name           TEXT        NOT NULL,
    position       INT         NOT NULL,           -- orden de evaluación (1 = primero)
    active         BOOLEAN     NOT NULL DEFAULT TRUE,
    courier        TEXT,                           -- NULL = cualquiera
    company_id     BIGINT,
    channel        TEXT,
    dispatch_type  TEXT,
    warehouse      TEXT,
    wave_code      TEXT        NOT NULL,           -- código de ola de Invas (GEN, AM, PM...)
    created_by     TEXT        NOT NULL,
    updated_by     TEXT        NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_wave_rules_position ON wave_rules(position) WHERE active;

-- Configuración operativa de couriers (página de prioridades, CU-02/CU-04):
-- hora de llegada (corte), máximo de bultos por envío y cobertura por comuna.
CREATE TABLE IF NOT EXISTS courier_settings (
    courier        TEXT PRIMARY KEY,               -- código del catálogo couriers (Core)
    arrival_time   TIME,                           -- hora de llegada del courier (corte)
    max_packages   INT CHECK (max_packages IS NULL OR max_packages > 0), -- p. ej. Recíbelo = 1
    covers_all     BOOLEAN     NOT NULL DEFAULT TRUE,
    updated_by     TEXT,
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS courier_coverage (
    courier  TEXT NOT NULL,
    commune  TEXT NOT NULL,
    PRIMARY KEY (courier, commune)
);

-- ---------------------------------------------------------------------------
-- Retiro y entrega al courier (RF-23/RF-24, CU-05)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS manifests (
    id             BIGSERIAL PRIMARY KEY,
    courier        TEXT        NOT NULL,
    status         TEXT        NOT NULL DEFAULT 'open' CHECK (status IN ('open','closed')),
    created_by     TEXT        NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    closed_by      TEXT,
    closed_at      TIMESTAMPTZ,
    picker_name    TEXT,                           -- datos de quien retira (PV-09)
    picker_id      TEXT,
    vehicle_plate  TEXT,
    notes          TEXT
);
CREATE TABLE IF NOT EXISTS manifest_lines (
    manifest_id  BIGINT NOT NULL REFERENCES manifests(id),
    order_id     BIGINT NOT NULL REFERENCES orders(id),
    status       TEXT   NOT NULL DEFAULT 'included' CHECK (status IN ('included','moved','cancelled','removed','pending_packages','delivered')),
    moved_to     TEXT,                             -- «Movida a» courier nuevo
    PRIMARY KEY (manifest_id, order_id)
);
CREATE TABLE IF NOT EXISTS manifest_scans (
    manifest_id  BIGINT      NOT NULL REFERENCES manifests(id),
    package_id   BIGINT      NOT NULL REFERENCES order_packages(id),
    scanned_by   TEXT        NOT NULL,
    scanned_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (manifest_id, package_id)
);
-- Una orden solo puede estar en un manifiesto abierto a la vez
CREATE UNIQUE INDEX IF NOT EXISTS idx_manifest_one_open ON manifest_lines(order_id) WHERE status = 'included';

-- ---------------------------------------------------------------------------
-- Seguimiento posterior al retiro (RF-25): no altera el estado interno
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS tracking_events (
    id              BIGSERIAL PRIMARY KEY,
    order_id        BIGINT      NOT NULL REFERENCES orders(id),
    provider        TEXT        NOT NULL,
    event_id        TEXT        NOT NULL,           -- id del evento en el proveedor (deduplicación)
    status_raw      TEXT        NOT NULL,           -- estado tal como lo envía el courier
    description     TEXT,
    occurred_at     TIMESTAMPTZ NOT NULL,
    received_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (provider, event_id)
);
CREATE INDEX IF NOT EXISTS idx_tracking_order ON tracking_events(order_id, occurred_at);

-- ---------------------------------------------------------------------------
-- Integraciones por cliente (RF-05, H-06): secreto cifrado, nunca en claro
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS integrations (
    id               BIGSERIAL PRIMARY KEY,
    company_id       BIGINT      NOT NULL,
    provider         TEXT        NOT NULL,           -- generic, mercadolibre, chilexpress...
    kind             TEXT        NOT NULL DEFAULT 'channel' CHECK (kind IN ('channel','courier')),
    channel          TEXT        NOT NULL,           -- canal que se registra en la orden
    default_courier  TEXT,
    secret_enc       TEXT        NOT NULL,           -- AES-GCM con INTEGRATIONS_KEY
    active           BOOLEAN     NOT NULL DEFAULT TRUE,
    created_by       TEXT        NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE webhook_inbox
    ADD COLUMN IF NOT EXISTS integration_id  BIGINT,
    ADD COLUMN IF NOT EXISTS external_id     TEXT,
    ADD COLUMN IF NOT EXISTS status          TEXT NOT NULL DEFAULT 'received', -- received, processed, duplicate, error
    ADD COLUMN IF NOT EXISTS error           TEXT,
    ADD COLUMN IF NOT EXISTS order_id        BIGINT,
    ADD COLUMN IF NOT EXISTS attempts        INT NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_inbox_pending ON webhook_inbox(id) WHERE processed_at IS NULL;

-- ---------------------------------------------------------------------------
-- Avisos (RF-27, H-15): por empresa y perfil; lectura por usuario
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS notifications (
    id          BIGSERIAL PRIMARY KEY,
    company_id  BIGINT      NOT NULL,
    audience    TEXT        NOT NULL CHECK (audience IN ('internal','client','all')),
    order_id    BIGINT,
    kind        TEXT        NOT NULL,                -- incident, voided, validated, cancelled...
    title       TEXT        NOT NULL,
    body        TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_notifications_company ON notifications(company_id, created_at DESC);
CREATE TABLE IF NOT EXISTS notification_reads (
    notification_id  BIGINT NOT NULL REFERENCES notifications(id),
    user_id          BIGINT NOT NULL,
    read_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (notification_id, user_id)
);

-- ---------------------------------------------------------------------------
-- Vista de solo lectura para el Core (DAS v1.1 §4.2: sin HTTP entre servicios)
-- ---------------------------------------------------------------------------
CREATE OR REPLACE VIEW v_order_history AS
SELECT e.id, e.order_id, o.company_id, o.external_ref, e.event_type, e.event,
       e.from_status, e.to_status, e.actor, e.reason, e.occurred_at
FROM order_events e JOIN orders o ON o.id = e.order_id;

COMMIT;
