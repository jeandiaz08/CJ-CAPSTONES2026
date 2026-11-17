DROP TABLE IF EXISTS ops_settings;
ALTER TABLE courier_settings DROP CONSTRAINT IF EXISTS courier_settings_cutoff_before_arrival;
ALTER TABLE courier_settings DROP COLUMN IF EXISTS cutoff_time;
