ALTER TABLE courier_settings ADD COLUMN cutoff_time time;

-- Vuelve al modelo anterior: el primer corte de cada courier
UPDATE courier_settings cs
SET cutoff_time = x.cutoff_time
FROM (
    SELECT cc.courier, min(c.cutoff_time) AS cutoff_time
    FROM cutoff_couriers cc JOIN cutoffs c ON c.id = cc.cutoff_id AND c.active
    GROUP BY cc.courier
) x
WHERE x.courier = cs.courier;

ALTER TABLE courier_settings
    ADD CONSTRAINT courier_settings_cutoff_before_arrival
    CHECK (cutoff_time IS NULL OR arrival_time IS NULL OR cutoff_time <= arrival_time);

DROP TABLE IF EXISTS cutoff_couriers;
DROP TABLE IF EXISTS cutoffs;
