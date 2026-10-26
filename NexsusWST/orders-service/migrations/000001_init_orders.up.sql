-- Órdenes (dueño: servicio de Órdenes). Lee companies/products del Core.
BEGIN;

CREATE TABLE IF NOT EXISTS orders (
    id                   BIGSERIAL PRIMARY KEY,
    company_id           BIGINT      NOT NULL REFERENCES companies(id),
    external_ref         TEXT        NOT NULL,              -- referencia del canal
    channel              TEXT        NOT NULL,              -- manual, mercadolibre, shopify...
    courier              TEXT,                              -- código del catálogo couriers
    status               TEXT        NOT NULL CHECK (status IN ('pending','validating','validated','picking','picked',
                                     'packing','ready_pickup','handed_over','incident','voided','cancelled')),
    prev_status          TEXT,                              -- para volver al resolver una Incidencia
    priority             BOOLEAN     NOT NULL DEFAULT FALSE,-- «Enviar primero a la cola» (RF-15)
    wave_id              TEXT,                              -- ola de Invas asignada
    wms_ref              TEXT,                              -- OS en Invas
    note                 TEXT,                              -- motivo de incidencia/anulación
    version              INT         NOT NULL DEFAULT 1,    -- concurrencia optimista
    validation_attempts  INT         NOT NULL DEFAULT 0,    -- reintentos de validación
    next_retry_at        TIMESTAMPTZ,
    last_error           TEXT,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (company_id, channel, external_ref)              -- RF-13 duplicados
);
CREATE INDEX IF NOT EXISTS idx_orders_company_status ON orders(company_id, status);
CREATE INDEX IF NOT EXISTS idx_orders_due_validation ON orders(next_retry_at) WHERE status IN ('pending','validating');

CREATE TABLE IF NOT EXISTS order_items (
    id        BIGSERIAL PRIMARY KEY,
    order_id  BIGINT NOT NULL REFERENCES orders(id),
    sku       TEXT   NOT NULL,
    quantity  INT    NOT NULL CHECK (quantity > 0)
);
CREATE INDEX IF NOT EXISTS idx_order_items_order ON order_items(order_id);

-- Historial de la orden: SOLO AGREGADO (DAS §4.3, RF-11/RF-28)
CREATE TABLE IF NOT EXISTS order_events (
    id           BIGSERIAL PRIMARY KEY,
    order_id     BIGINT      NOT NULL REFERENCES orders(id),
    event_type   TEXT        NOT NULL,  -- created, status_change, priority, wave_requested, edit, print...
    event        TEXT        NOT NULL,
    from_status  TEXT,
    to_status    TEXT,
    actor        TEXT        NOT NULL,  -- usuario o proceso
    reason       TEXT,
    occurred_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_order_events_order ON order_events(order_id);

-- Stock comprometido por Nexus (DA-09). Solo Órdenes escribe; el Core lee.
CREATE TABLE IF NOT EXISTS stock_committed (
    company_id  BIGINT      NOT NULL REFERENCES companies(id),
    sku         TEXT        NOT NULL,
    quantity    INT         NOT NULL DEFAULT 0 CHECK (quantity >= 0),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (company_id, sku)
);

-- Bandeja transaccional (outbox) -> Redis Streams (DAS §5.4)
CREATE TABLE IF NOT EXISTS job_outbox (
    id            TEXT PRIMARY KEY,           -- identificador de operación
    type          TEXT        NOT NULL,
    company_id    BIGINT      NOT NULL,
    order_id      BIGINT,
    priority      BOOLEAN     NOT NULL DEFAULT FALSE,
    payload       JSONB       NOT NULL DEFAULT '{}'::jsonb,
    attempts      INT         NOT NULL DEFAULT 0,
    last_error    TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at  TIMESTAMPTZ,
    completed_at  TIMESTAMPTZ,
    failed_at     TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_outbox_unpublished ON job_outbox(priority DESC, created_at) WHERE published_at IS NULL;

-- Webhooks de couriers y canales (DA-14), se procesan por la cola
CREATE TABLE IF NOT EXISTS webhook_inbox (
    id            BIGSERIAL PRIMARY KEY,
    provider      TEXT        NOT NULL,
    payload       TEXT        NOT NULL,
    received_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at  TIMESTAMPTZ
);

COMMIT;
