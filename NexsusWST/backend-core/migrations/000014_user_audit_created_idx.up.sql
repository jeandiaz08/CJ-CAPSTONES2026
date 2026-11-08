-- Índice para el registro de acciones (GET /v1/activity de Órdenes lee user_audit
-- ordenado por fecha). Órdenes 000009 ya lo crea; aquí queda en el servicio dueño
-- de la tabla. IF NOT EXISTS: no falla si ya existe.
CREATE INDEX IF NOT EXISTS idx_user_audit_created ON user_audit (created_at DESC);
