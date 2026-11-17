-- Ola por cliente o por producto (PV-19; H-03-T07, H-04-T10).
-- NULL = sin preferencia. El código es el de la ola en Invas (mismo formato
-- que wave_rules.wave_code). Órdenes lo lee por las vistas de abajo, en solo
-- lectura (DA-12): producto → cliente → reglas.
ALTER TABLE companies ADD COLUMN IF NOT EXISTS wave_code TEXT
    CHECK (wave_code IS NULL OR wave_code ~ '^[A-Z0-9_-]{1,20}$');
ALTER TABLE products ADD COLUMN IF NOT EXISTS wave_code TEXT
    CHECK (wave_code IS NULL OR wave_code ~ '^[A-Z0-9_-]{1,20}$');

CREATE OR REPLACE VIEW v_product_waves AS
    SELECT company_id, sku, wave_code FROM products WHERE wave_code IS NOT NULL;
CREATE OR REPLACE VIEW v_company_waves AS
    SELECT id AS company_id, wave_code FROM companies WHERE wave_code IS NOT NULL;
