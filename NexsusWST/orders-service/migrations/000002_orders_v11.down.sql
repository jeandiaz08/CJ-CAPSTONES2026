BEGIN;
DROP VIEW IF EXISTS v_order_history;
DROP TABLE IF EXISTS notification_reads, notifications, integrations, tracking_events,
    manifest_scans, manifest_lines, manifests, courier_coverage, courier_settings,
    wave_rules, order_imports, order_documents, order_packages;
ALTER TABLE webhook_inbox DROP COLUMN IF EXISTS integration_id, DROP COLUMN IF EXISTS external_id,
    DROP COLUMN IF EXISTS status, DROP COLUMN IF EXISTS error, DROP COLUMN IF EXISTS order_id, DROP COLUMN IF EXISTS attempts;
ALTER TABLE orders DROP COLUMN IF EXISTS recipient_name, DROP COLUMN IF EXISTS recipient_phone,
    DROP COLUMN IF EXISTS recipient_email, DROP COLUMN IF EXISTS address_line, DROP COLUMN IF EXISTS commune,
    DROP COLUMN IF EXISTS region_code, DROP COLUMN IF EXISTS dispatch_type, DROP COLUMN IF EXISTS warehouse,
    DROP COLUMN IF EXISTS customer_notes, DROP COLUMN IF EXISTS courier_code, DROP COLUMN IF EXISTS source,
    DROP COLUMN IF EXISTS import_id, DROP COLUMN IF EXISTS created_by, DROP COLUMN IF EXISTS incident_kind,
    DROP COLUMN IF EXISTS claimed_by, DROP COLUMN IF EXISTS claimed_at, DROP COLUMN IF EXISTS rules_checked_at,
    DROP COLUMN IF EXISTS handed_over_at, DROP COLUMN IF EXISTS cancelled_at;
COMMIT;
