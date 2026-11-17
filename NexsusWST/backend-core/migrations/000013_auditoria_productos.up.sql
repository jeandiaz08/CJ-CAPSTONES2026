-- Tarea 09 · Productos (H-04): las acciones sobre productos y kits (crear, editar,
-- activar, desactivar, importar) quedan en user_audit con el producto afectado, para el Registro
-- de acciones › Productos (vista activity_log, Órdenes 000011) y el «Modificado por» del panel
-- (la última acción). No se borra nada: los productos solo se desactivan.
BEGIN;

ALTER TABLE user_audit ADD COLUMN IF NOT EXISTS target_product_id BIGINT REFERENCES products(id);
ALTER TABLE user_audit DROP CONSTRAINT IF EXISTS user_audit_has_target;
ALTER TABLE user_audit ADD CONSTRAINT user_audit_has_target
    CHECK (num_nonnulls(target_user_id, target_company_id, request_id, target_product_id) >= 1);
CREATE INDEX IF NOT EXISTS idx_user_audit_product ON user_audit (target_product_id, created_at DESC)
    WHERE target_product_id IS NOT NULL;

COMMIT;
