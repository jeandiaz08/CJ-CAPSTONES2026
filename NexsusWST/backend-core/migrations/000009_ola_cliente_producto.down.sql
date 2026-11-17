DROP VIEW IF EXISTS v_company_waves;
DROP VIEW IF EXISTS v_product_waves;
ALTER TABLE products DROP COLUMN IF EXISTS wave_code;
ALTER TABLE companies DROP COLUMN IF EXISTS wave_code;
