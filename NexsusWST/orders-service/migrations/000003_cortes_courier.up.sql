-- orders-service · migración 3: cortes por courier y hora límite de asignación
-- El Jefe de Operaciones define a mano la hora de llegada de cada courier
-- (arrival_time, ya existía) y su hora de corte (cutoff_time). La hora límite
-- general (assignment_deadline) es el último momento del día para asignar a ola.

ALTER TABLE courier_settings
    ADD COLUMN cutoff_time time;

ALTER TABLE courier_settings
    ADD CONSTRAINT courier_settings_cutoff_before_arrival
    CHECK (cutoff_time IS NULL OR arrival_time IS NULL OR cutoff_time <= arrival_time);

CREATE TABLE ops_settings (
    key        text PRIMARY KEY,
    value      text NOT NULL,
    updated_by text,
    updated_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO ops_settings (key, value, updated_by)
VALUES ('assignment_deadline', '12:00', 'migracion')
ON CONFLICT (key) DO NOTHING;
