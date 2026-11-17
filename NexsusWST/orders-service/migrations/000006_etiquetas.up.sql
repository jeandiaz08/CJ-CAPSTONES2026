-- Etiqueta por courier (H-12 T09): por API con reintento programado o «Buscar en oficina».
BEGIN;

ALTER TABLE courier_settings
    ADD COLUMN label_strategy TEXT NOT NULL DEFAULT 'api' CHECK (label_strategy IN ('api','office'));

-- Recíbelo no tiene API de etiquetas y admite una sola etiqueta por orden.
INSERT INTO courier_settings (courier, max_packages, label_strategy, updated_by)
VALUES ('recibelo', 1, 'office', 'migracion')
ON CONFLICT (courier) DO UPDATE SET max_packages = 1, label_strategy = 'office', updated_by = 'migracion', updated_at = now();

CREATE TABLE order_labels (
    order_id        BIGINT      PRIMARY KEY REFERENCES orders(id) ON DELETE CASCADE,
    company_id      BIGINT      NOT NULL,
    warehouse       TEXT        NOT NULL,
    status          TEXT        NOT NULL CHECK (status IN ('pending','ready','failed')),
    attempts        INT         NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_error      TEXT,
    document_id     BIGINT      REFERENCES order_documents(id) ON DELETE SET NULL,
    requested_by    TEXT        NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_order_labels_due ON order_labels(next_attempt_at) WHERE status = 'pending';

-- Todo evento o aviso guarda la empresa y la bodega de la orden.
ALTER TABLE order_events ADD COLUMN company_id BIGINT, ADD COLUMN warehouse TEXT;
UPDATE order_events e SET company_id = o.company_id, warehouse = o.warehouse FROM orders o WHERE o.id = e.order_id;
ALTER TABLE notifications ADD COLUMN warehouse TEXT;
UPDATE notifications n SET warehouse = o.warehouse FROM orders o WHERE o.id = n.order_id;

COMMIT;
