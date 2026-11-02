-- orders-service · migración 4: cortes como entidad propia (varios couriers por corte)
-- Reemplaza courier_settings.cutoff_time (migración 3). Un corte tiene su hora y
-- puede incluir varios couriers, cada uno con su hora de llegada a ese corte.
-- Un mismo courier puede estar en varios cortes (ej. Recíbelo a las 09:15 y 10:15).

CREATE TABLE cutoffs (
    id          bigserial PRIMARY KEY,
    name        text,                      -- opcional; si es NULL se muestra «Corte N» según el orden por hora
    cutoff_time time NOT NULL,
    active      boolean NOT NULL DEFAULT true,
    updated_by  text,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX cutoffs_active_time_unique ON cutoffs (cutoff_time) WHERE active;

CREATE TABLE cutoff_couriers (
    cutoff_id    bigint NOT NULL REFERENCES cutoffs(id) ON DELETE CASCADE,
    courier      text   NOT NULL REFERENCES couriers(code),
    arrival_time time   NOT NULL,
    PRIMARY KEY (cutoff_id, courier)
);

CREATE INDEX idx_cutoff_couriers_courier ON cutoff_couriers (courier);

-- Migrar los cortes existentes (uno por cada hora distinta de la migración 3)
INSERT INTO cutoffs (cutoff_time, updated_by)
SELECT DISTINCT cutoff_time, 'migracion'
FROM courier_settings
WHERE cutoff_time IS NOT NULL;

INSERT INTO cutoff_couriers (cutoff_id, courier, arrival_time)
SELECT c.id, cs.courier, COALESCE(cs.arrival_time, cs.cutoff_time)
FROM courier_settings cs
JOIN cutoffs c ON c.cutoff_time = cs.cutoff_time AND c.active
WHERE cs.cutoff_time IS NOT NULL;

ALTER TABLE courier_settings DROP CONSTRAINT IF EXISTS courier_settings_cutoff_before_arrival;
ALTER TABLE courier_settings DROP COLUMN IF EXISTS cutoff_time;
