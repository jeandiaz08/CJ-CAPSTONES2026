-- Rendimiento de la bandeja y del registro histórico (revisión del traspaso de Órdenes).
-- La regla de 000009 («al registro las Canceladas y lo que lleva 1 año sin cambiar de
-- estado») calculaba la fecha del último cambio con una subconsulta a order_events por
-- cada orden: con 50.000 órdenes y 250.000 eventos, la bandeja tardaba ~250 ms (2,7 s
-- en frío). Ahora la fecha vive en orders.status_changed_at, con índice: ~12 ms.
--
-- Misma regla que antes: Entregadas → handed_over_at; si no, el último evento que
-- cambió el estado; sin eventos, created_at. La mantienen triggers (no la aplicación)
-- para que también la respeten los seeds y cualquier carga por SQL.
BEGIN;

ALTER TABLE orders ADD COLUMN IF NOT EXISTS status_changed_at timestamptz;

CREATE OR REPLACE FUNCTION order_status_changed_at(p_order bigint, p_status text, p_handed timestamptz, p_created timestamptz)
RETURNS timestamptz LANGUAGE sql STABLE AS $$
    SELECT COALESCE(CASE WHEN p_status = 'handed_over' THEN p_handed END,
                    (SELECT max(e.occurred_at) FROM order_events e
                      WHERE e.order_id = p_order AND e.to_status IS NOT NULL AND e.to_status IS DISTINCT FROM e.from_status),
                    p_created, now())
$$;

UPDATE orders o SET status_changed_at = order_status_changed_at(o.id, o.status, o.handed_over_at, o.created_at);
-- El DEFAULT es solo respaldo (p. ej. tablas copiadas con LIKE, sin triggers); el valor
-- real lo pone el trigger.
ALTER TABLE orders ALTER COLUMN status_changed_at SET DEFAULT now(),
                   ALTER COLUMN status_changed_at SET NOT NULL;

-- Orden creada o con cambio de estado / entrega / fecha de creación.
CREATE OR REPLACE FUNCTION orders_status_changed_at_trg() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    NEW.status_changed_at := order_status_changed_at(NEW.id, NEW.status, NEW.handed_over_at, NEW.created_at);
    RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS trg_orders_status_changed_at ON orders;
CREATE TRIGGER trg_orders_status_changed_at
    BEFORE INSERT OR UPDATE OF status, handed_over_at, created_at ON orders
    FOR EACH ROW EXECUTE FUNCTION orders_status_changed_at_trg();

-- Eventos de la orden (transitionTx inserta el evento después de cambiar el estado).
-- Por SENTENCIA con tablas de transición: un solo UPDATE por INSERT, aunque se carguen
-- miles de eventos (un trigger por fila hacía un UPDATE por evento). Al insertar solo
-- importan los eventos que cambian el estado; al editar o borrar, todas las órdenes tocadas.
CREATE OR REPLACE FUNCTION order_events_status_changed_at_trg() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        UPDATE orders o SET status_changed_at = order_status_changed_at(o.id, o.status, o.handed_over_at, o.created_at)
        FROM (SELECT DISTINCT n.order_id FROM ev_new n
               WHERE n.to_status IS NOT NULL AND n.to_status IS DISTINCT FROM n.from_status) c
        WHERE o.id = c.order_id;
    ELSIF TG_OP = 'UPDATE' THEN
        UPDATE orders o SET status_changed_at = order_status_changed_at(o.id, o.status, o.handed_over_at, o.created_at)
        FROM (SELECT order_id FROM ev_new UNION SELECT order_id FROM ev_old) c
        WHERE o.id = c.order_id;
    ELSE
        UPDATE orders o SET status_changed_at = order_status_changed_at(o.id, o.status, o.handed_over_at, o.created_at)
        FROM (SELECT DISTINCT order_id FROM ev_old) c
        WHERE o.id = c.order_id;
    END IF;
    RETURN NULL;
END $$;
DROP TRIGGER IF EXISTS trg_order_events_status_changed_at ON order_events;
DROP TRIGGER IF EXISTS trg_order_events_status_changed_ins ON order_events;
DROP TRIGGER IF EXISTS trg_order_events_status_changed_upd ON order_events;
DROP TRIGGER IF EXISTS trg_order_events_status_changed_del ON order_events;
CREATE TRIGGER trg_order_events_status_changed_ins AFTER INSERT ON order_events
    REFERENCING NEW TABLE AS ev_new FOR EACH STATEMENT EXECUTE FUNCTION order_events_status_changed_at_trg();
CREATE TRIGGER trg_order_events_status_changed_upd AFTER UPDATE ON order_events
    REFERENCING OLD TABLE AS ev_old NEW TABLE AS ev_new FOR EACH STATEMENT EXECUTE FUNCTION order_events_status_changed_at_trg();
CREATE TRIGGER trg_order_events_status_changed_del AFTER DELETE ON order_events
    REFERENCING OLD TABLE AS ev_old FOR EACH STATEMENT EXECUTE FUNCTION order_events_status_changed_at_trg();

CREATE INDEX IF NOT EXISTS idx_orders_status_changed ON orders (status_changed_at DESC);

COMMIT;
