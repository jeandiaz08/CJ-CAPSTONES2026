-- Deshace 000010: vuelve a la vista activity_log de la 000009.
BEGIN;

DROP VIEW IF EXISTS activity_log;
CREATE OR REPLACE VIEW activity_log AS
WITH raw AS (
    -- Órdenes, empaque y retiro (eventos de cada orden).
    SELECT 'oe-' || e.id AS id,
           e.occurred_at,
           e.actor AS actor_raw,
           CASE
               WHEN e.details->>'kind' = 'despacho_incompleto' OR e.event_type = 'manifest' OR e.event = 'manifest_leave' THEN 'dispatch'
               WHEN e.event IN ('start_packing', 'release_packing', 'packed') OR e.event_type IN ('packages', 'print', 'label') THEN 'packing'
               ELSE 'orders'
           END AS module,
           CASE
               WHEN e.details->>'kind' = 'despacho_incompleto' THEN 'dispatch_anyway'
               WHEN e.event_type = 'label' THEN 'label_' || e.event
               WHEN e.event_type = 'print' THEN 'print_' || e.event
               ELSE e.event
           END AS action,
           'order' AS entity_type,
           e.order_id::text AS entity_id,
           o.external_ref AS entity_label,
           COALESCE(e.company_id, o.company_id) AS company_id,
           COALESCE(e.warehouse, o.warehouse) AS warehouse,
           e.reason,
           COALESCE(e.details, '{}'::jsonb) || jsonb_strip_nulls(jsonb_build_object(
               'fromStatus', e.from_status,
               'toStatus', e.to_status,
               'courier', o.courier,
               'labelCode', o.courier_code,
               'packages', (SELECT count(*) FROM order_packages p WHERE p.order_id = e.order_id))) AS details
    FROM order_events e
    JOIN orders o ON o.id = e.order_id

    UNION ALL
    -- Manifiesto creado.
    SELECT 'mc-' || m.id, m.created_at, m.created_by, 'dispatch', 'manifest_created',
           'manifest', m.id::text, '#' || m.id, NULL::bigint, NULL::text, NULL::text,
           jsonb_build_object('courier', m.courier,
               'orders', (SELECT count(*) FROM manifest_lines l WHERE l.manifest_id = m.id),
               'packages', (SELECT count(*) FROM order_packages p JOIN manifest_lines l ON l.order_id = p.order_id WHERE l.manifest_id = m.id))
    FROM manifests m

    UNION ALL
    -- Manifiesto cerrado: quién retira, RUT, patente y notas (la empresa del transporte va en notes).
    SELECT 'mx-' || m.id, m.closed_at, COALESCE(m.closed_by, ''), 'dispatch', 'manifest_closed',
           'manifest', m.id::text, '#' || m.id, NULL::bigint, NULL::text, NULLIF(m.notes, ''),
           jsonb_strip_nulls(jsonb_build_object('courier', m.courier,
               'orders', (SELECT count(*) FROM manifest_lines l WHERE l.manifest_id = m.id AND l.status = 'delivered'),
               'packages', (SELECT count(*) FROM order_packages p JOIN manifest_lines l ON l.order_id = p.order_id WHERE l.manifest_id = m.id AND l.status = 'delivered'),
               'pickerName', NULLIF(m.picker_name, ''), 'pickerId', NULLIF(m.picker_id, ''),
               'vehiclePlate', NULLIF(m.vehicle_plate, ''), 'notes', NULLIF(m.notes, '')))
    FROM manifests m
    WHERE m.closed_at IS NOT NULL

    UNION ALL
    -- Usuarios (Core): altas, cambios, activar/desactivar, contraseñas y vínculos con Keycloak.
    SELECT 'ua-' || a.id, a.created_at,
           CASE WHEN a.actor_user_id IS NULL THEN 'Sistema' ELSE 'user:' || a.actor_user_id || ' ' || COALESCE(au.name, '') END,
           'users', a.action, 'user', a.target_user_id::text, COALESCE(tu.name, tu.email::text, '#' || a.target_user_id),
           NULL::bigint, NULL::text, NULLIF(a.detail->>'reason', ''),
           a.detail || jsonb_strip_nulls(jsonb_build_object('targetRole', tu.role))
    FROM user_audit a
    LEFT JOIN users au ON au.id = a.actor_user_id
    LEFT JOIN users tu ON tu.id = a.target_user_id
)
SELECT r.id,
       r.occurred_at,
       substring(r.actor_raw FROM '^user:([0-9]+)')::bigint AS actor_id,
       CASE
           WHEN r.actor_raw ~ '^user:' THEN COALESCE(NULLIF(trim(regexp_replace(r.actor_raw, '^user:[0-9]+\s*', '')), ''), 'Usuario #' || substring(r.actor_raw FROM '^user:([0-9]+)'))
           WHEN trim(r.actor_raw) = '' OR r.actor_raw ~* '^(system:|sistema$|invas$|worker$)' THEN 'Sistema'
           ELSE trim(r.actor_raw)
       END AS actor_name,
       u.role AS actor_role,
       CASE WHEN r.actor_raw !~ '^user:' AND (trim(r.actor_raw) = '' OR r.actor_raw ~* '^(system:|sistema$|invas$|worker$)') THEN 'system' ELSE 'human' END AS actor_kind,
       r.module,
       r.action,
       r.entity_type,
       r.entity_id,
       r.entity_label,
       r.company_id,
       r.warehouse,
       r.reason,
       r.details
FROM raw r
LEFT JOIN users u ON u.id = substring(r.actor_raw FROM '^user:([0-9]+)')::bigint;

COMMIT;
