-- Composición de kits (RF-06, H-04-T01 / SCRUM-100).
-- Un kit no tiene stock propio: su disponibilidad sale de sus componentes.
-- Los componentes son productos simples de la misma empresa (lo valida el
-- Core); no se permiten kits dentro de kits, así no hay ciclos.
CREATE TABLE IF NOT EXISTS kit_components (
    kit_id        BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    component_id  BIGINT NOT NULL REFERENCES products(id),
    quantity      INT    NOT NULL CHECK (quantity > 0),
    PRIMARY KEY (kit_id, component_id),
    CHECK (kit_id <> component_id)
);
CREATE INDEX IF NOT EXISTS idx_kit_components_component ON kit_components(component_id);
