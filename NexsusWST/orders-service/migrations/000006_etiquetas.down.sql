BEGIN;
ALTER TABLE notifications DROP COLUMN warehouse;
ALTER TABLE order_events DROP COLUMN warehouse, DROP COLUMN company_id;
DROP TABLE order_labels;
ALTER TABLE courier_settings DROP COLUMN label_strategy;
COMMIT;
