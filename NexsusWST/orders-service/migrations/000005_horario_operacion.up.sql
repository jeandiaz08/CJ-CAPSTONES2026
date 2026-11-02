-- Regla de cortes del Jefe de Operaciones: horario de operación y orden de la cola de Invas.
INSERT INTO ops_settings (key, value, updated_by) VALUES
    ('operation_start', '06:30', 'migracion'),
    ('operation_days', 'mon,tue,wed,thu,fri', 'migracion'),
    ('priority_overrides_deadline', 'false', 'migracion')
ON CONFLICT (key) DO NOTHING;

-- Corte que define el lugar del trabajo en la cola; NULL va al final.
ALTER TABLE job_outbox ADD COLUMN next_cutoff time;
