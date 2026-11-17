ALTER TABLE job_outbox DROP COLUMN IF EXISTS next_cutoff;
DELETE FROM ops_settings WHERE key IN ('operation_start', 'operation_days', 'priority_overrides_deadline');
