-- Kits (RF-06, SCRUM-100): un kit no tiene stock propio. Al validar, cada
-- línea de kit se expande en sus componentes y el comprometido se registra
-- sobre ellos. Se guarda lo que se comprometió para liberar exactamente lo
-- mismo aunque después cambie la composición del kit.
CREATE TABLE IF NOT EXISTS order_stock_lines (
    order_id  BIGINT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    sku       TEXT   NOT NULL,
    quantity  INT    NOT NULL CHECK (quantity > 0),
    PRIMARY KEY (order_id, sku)
);
