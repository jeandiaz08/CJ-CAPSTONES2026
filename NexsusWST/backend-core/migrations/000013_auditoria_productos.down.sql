-- Deshace 000013. Antes, bajar Órdenes 000011 (la vista activity_log lee target_product_id).
-- Las acciones que solo apuntaban a un producto se borran.
BEGIN;

DELETE FROM user_audit WHERE target_product_id IS NOT NULL
    AND num_nonnulls(target_user_id, target_company_id, request_id) = 0;
DROP INDEX IF EXISTS idx_user_audit_product;
ALTER TABLE user_audit DROP CONSTRAINT IF EXISTS user_audit_has_target;
ALTER TABLE user_audit DROP COLUMN IF EXISTS target_product_id;
ALTER TABLE user_audit ADD CONSTRAINT user_audit_has_target
    CHECK (num_nonnulls(target_user_id, target_company_id, request_id) >= 1);

COMMIT;
