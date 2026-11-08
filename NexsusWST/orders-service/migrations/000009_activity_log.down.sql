BEGIN;
DROP VIEW IF EXISTS activity_log;
DROP INDEX IF EXISTS idx_user_audit_created;
DROP INDEX IF EXISTS idx_order_events_occurred;
COMMIT;
