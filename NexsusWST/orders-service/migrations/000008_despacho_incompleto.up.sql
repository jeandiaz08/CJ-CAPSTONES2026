-- H-13-T08 (SCRUM-243): «Despachar igual» con bultos faltantes.
-- order_events solo guardaba el motivo como texto; details guarda los datos
-- ordenados del evento (bultos faltantes, manifiesto, ola, quién autorizó).
-- Sirve también para otros eventos que necesiten más que un texto.
BEGIN;
ALTER TABLE order_events ADD COLUMN IF NOT EXISTS details JSONB;
COMMIT;
