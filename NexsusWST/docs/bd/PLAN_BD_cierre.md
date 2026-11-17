# Plan de cierre de base de datos · NexusWST vs. esquema `we` de WeStorage

**Fecha del análisis:** 17-11-2026 · **Rama:** `jean_dev` · **Presentación:** 26 y 27-11-2026
**Alcance:** solo base de datos. No se analiza el API Mock (endpoints, Postman, credenciales).
**Regla de oro:** este plan **no integra nada** al esquema `we`. Produce documentos, una seed y capturas. No toca migraciones ni código de backend.

> Para quien ejecute este plan en una sesión nueva: lee las secciones 1 a 5 completas antes de empezar. Las fases (sección 3) están en orden y cada una dice de qué depende. Antes de la fase 3.4 (seed) hay decisiones del usuario (sección 4) que deben estar respondidas.

---

## 1. Entradas (rutas exactas, relativas a `NexsusWST/`)

| # | Ruta | Qué es | Cómo leerla |
|---|---|---|---|
| E1 | `respaldo/api_mock_ismael/pg-beta-local-202611101551.sql` | Volcado de Ismael (`pg_dump`): base `wemanager_beta`, esquema `we`. **Fuente principal.** ~700 KB. | Con scripts o `grep`, nunca entero. `respaldo/` está fuera de git: ábrelo por ruta. |
| E2 | `respaldo/api_mock_ismael/LEEME_correo_Ismael.md` | Correo de la entrega. Sobre la BD solo dice: «base de datos precargada con clientes y productos»; 7 clientes, 40 productos cada uno. | Contiene credenciales de prueba del mock: **no copiar nada de ahí** a archivos versionados. |
| E3 | `docs/bd/Historial de cambios BD NexusWST v1.2 (reconstruido).xlsx` (copia versionada) y `respaldo/docs/` (original local) | 84 filas DB-01 a DB-84. DB-01 a DB-08 del v1.0 (01-11-2026); DB-09 a DB-84 reconstruidas desde migraciones. Hojas: «Historial de cambios», «Leyenda», «Notas de reconstrucción». | Sin openpyxl en el equipo: editar el XML del .xlsx con la biblioteca estándar de Python (ver 3.1). |
| E4 | `backend-core/migrations/*.up.sql` (000001 a 000013) y `orders-service/migrations/*.up.sql` (000001 a 000011) | Nuestro esquema `public`. | Leer completos; son cortos. |
| E5 | `docs/API.md`, `docs/traspaso/Traspaso_Ordenes_frontend_backend.md`, `respaldo/notas/backend_pendientes.md` | Contrato, traspaso a Cristopher, pendientes de backend. | `grep` por tema. |
| E6 | `respaldo/seed/seed_demo.sql`, `respaldo/seed/LEEME.md`, `respaldo/seed/borrar_seed_demo.sql`, `infrastructure/dev-seed/seed_dev.sql` | Seeds actuales (42 órdenes demo, Andes Outdoor id 900001, cuentas demo). | Leer el LEEME primero. |
| E7 | `frontend-web/capturas/pantallas.spec.ts` y `docs/pantallas/README.md` (en git, `HEAD`) | Generador de capturas y su índice. | Ver riesgo R1: la carpeta `docs/pantallas/` está borrada en el árbol local. |
| E8 | `respaldo/docs/Cambios_pantallas_Sprint_Review_1.md`, `respaldo/docs/ERS_v1.2.md`, `respaldo/docs/DAS_v1.2.md` | Acuerdos con el cliente y requisitos. | Solo para la sección 2.5. |

Scripts de análisis usados en esta sesión (para reproducirlos, no son entregables): extraer `CREATE TABLE we.*`, `ALTER TABLE ONLY we.* ADD CONSTRAINT`, `CREATE INDEX` y los bloques `COPY we.<tabla> (...) FROM stdin; ... \.` (ojo: los `COPY` vacíos terminan con `\.` en la línea siguiente; usar `re.M` con `^\\\.$` o se desfasan los conteos). Valor nulo en COPY = `\N`.

---

## 2. Hallazgos

### 2.1 Datos verificados del volcado

- **37 tablas** en `we` (la pista decía 36: faltaba `clients_stores`, 7 filas), **47 FK**, **14 índices** (`CREATE INDEX`) más 56 PK/UNIQUE/CHECK por `ALTER TABLE`. Sin triggers, funciones, vistas, tipos ni extensiones. Solo el esquema `we`.
- **Con datos:** `clients` 7 · `clients_stores` 7 · `products` 1437 · `orders` 700 · `order_items` 1065 · `order_status_history` 700 · `shipments` 677 · `shipment_packages` 701 · `inventory_balances` 1590 · `inventory_snapshots` 7 · `communes` 122 · `regions` 16 · `stores` 3 · `couriers` 11 · `courier_services` 14 · `order_statuses` 10 · `warehouse_statuses` 9 · `shipment_statuses` 11 · `shipment_package_statuses` 11 · `wms_status_mappings` 10 · `inventory_conditions` 6.
- **Vacías (16):** `users`, `roles`, `roles_modules`, `system_modules`, `shipment_tracking_events`, `courier_status_mappings`, `courier_communes`, `courier_service_coverage`, `orders_logs`, `action_logs`, `products_kits`, `common_types`, `economic_activities`, `channel_status_mappings`, `orders_documents`, `orders_items_info`.
- **Pistas confirmadas:** `we.couriers` no tiene hora de corte ni de llegada (tampoco `courier_services`). Recíbelo existe (id 8, `RBL`). `inventory_balances` tiene `lot` y `expiration_date` (330 de 1590 filas con lote) y **no** tiene comprometido. `communes.aliases` es `json`. `orders.order_value` es `real`. `users` trae `password` y `salt`.
- **Hallazgos nuevos:**
  1. `communes.aliases` viene **vacío (`[]`) en las 122 filas**.
  2. `orders.warehouse_id` **no tiene FK**, pero calza con `stores.id` vía `clients_stores` (clientes 1 y 6 → Enea 4; 2, 3 y 4 → San Bernardo; 5 y 7 → Las Condes).
  3. Datos ficticios: clientes «Cliente Prueba NN», RUT `TEST-CLIENT-NN`, correos `@example.test`; `recipient_dni` = número de orden en las 700 órdenes.
  4. Órdenes del 01-01-2026 al 22-10-2026 (491 en septiembre). **Ninguna tiene más de 1 año.**
  5. `order_status_history`: exactamente 1 fila por orden, todas `source = SYSTEM`, `reason = «Estado inicial importado desde el OMS anterior»`, `previous_status_id` nulo. El estado del OMS anterior viene en `metadata.source_oms_status` (LISTO 315, ENTREGADO 198, ENVIADO 152, PAGADO 23, ANULADO 9, PREVENTA 3).
  6. **Inconsistencia:** 188 órdenes están `READY_FOR_SHIPMENT` (4) con bodega `PENDING` (1); otras combinaciones raras: `SHIPPED` + `PENDING` (1), `VALIDATED` + `READY` (7). Ver pregunta 4.
  7. `dispatch_type` 1 (677) y 2 (23): las 23 órdenes sin `shipment` son las de tipo 2 (22) más 1. Significado desconocido (pregunta 2).
  8. `products.type` = 1 en los 1437 productos; `products_kits` vacía. No se sabe qué valor es kit (pregunta 1). Productos por cliente: 40 a 503 (el correo decía 40 cada uno). 406 sin EAN13.
  9. Couriers duplicados: `Bluexpress` (`BLX`) y `bluexpress` (`SHP_BLUEXPRESS`); `chilexpress` con código `SHP_CHILEXPRESS`. 8 `courier_services` son «Servicio legado» (`LEGACY_DEFAULT`). Pregunta 7.
  10. Unicidad de órdenes: solo índice **no único** `(client_id, client_order_number)`; nosotros exigimos `UNIQUE (company_id, channel, external_ref)`.
  11. `action_logs.date` y `orders_logs.date` son `timestamp` **sin zona horaria**; el resto usa `timestamptz`.
  12. **Comunas:** 91 en común con nuestras 113. **31 solo en `we`** (Algarrobo, Alto Bio bío, Arauco, Cabrero, Casablanca, Colbún, Curaco de Vélez, Freirina, Hualañé, Huasco, Isla de Pascua, La Ligua, Lago Ranco, Laja, Lautaro, Limache, Lolol, Longaví, Natales, Olivar, Panguipulli, Pelarco, Penco, Puchuncaví, Quirihue, Renaico, Requínoa, Romeral, Tomé, Vilcún, Villa Alegre) y **22 solo nuestras** (Alhué, Aysén, Caldera, Chillán Viejo, Constitución, El Monte, Illapel, Lo Espejo, Lo Prado, Lota, Machalí, María Pinto, Mejillones, Pozo Almonte, Pucón, Puerto Natales, Putre, Rengo, San José de Maipo, San Pedro, Tiltil, Tocopilla). «Natales» (we) = «Puerto Natales» (nuestra). **33 órdenes** de `we` caen en comunas que no tenemos.
  13. `regions.id` = número CUT (1 Tarapacá … 13 Metropolitana, 14 Los Ríos, 15 Arica y Parinacota, 16 Ñuble). Nuestro `regions.code` es el romano (`I` … `RM`, `XIV`, `XV`, `XVI`). Mapeo 1:1 directo.

### 2.2 Comparación por dominio (columna a columna)

Notación: ✔ equivalente · ≈ equivalente con diferencia · ✘ solo existe en un lado.

#### a) Clientes y usuarios

| Nuestro (`public`) | `we` | Diferencias |
|---|---|---|
| `companies.id` bigserial | `clients.id` smallint | Tipo (smallint limita a 32 767). |
| `companies.name` text NOT NULL | `clients.name` varchar NOT NULL | ✔ |
| `companies.tax_id` text NOT NULL + índice único normalizado (sin puntos ni guion) | `clients.dni` varchar NOT NULL UNIQUE (literal) | ≈ En `we`, «76.123.456-0» y «761234560» serían dos clientes. |
| `companies.wms_code` text | `clients.wms_code` smallint NOT NULL | ≈ Tipo y obligatoriedad. |
| `companies.email` citext opcional | `clients.email` varchar NOT NULL | ≈ Obligatorio en `we`. |
| `companies.phone`, `wave_code`, `created_by`, `updated_by`, `created_at`, `updated_at` | — | ✘ solo nuestro. |
| — | `clients.url_site`, `clients.parent_id` (holding) | ✘ solo `we`. |
| `users` (perfil; identidad en Keycloak: `keycloak_id`, `last_login_at`) | `users` con `password` + `salt` | ≈ Autenticación propia en `we`; **nosotros nunca guardamos contraseñas** (DAS v1.1, PV-18). |
| `users.email` citext opcional + `username` citext opcional + CHECK «al menos uno» | `users.email` NOT NULL, `users.username` NOT NULL | ≈ `we` no admite operarios sin correo (acuerdo del Sprint Review). |
| `users.role` text CHECK (admin, jefe_operaciones, analista, empaque, cliente) + `permissions text[]` | `users.rol_id` → `roles` (vacía) | ≈ Sin roles cargados en `we`; sin permisos sueltos como «Despacho». |
| `users.active` | — | ✘ `we.users` no tiene «activo». |
| `users.phone`, `job_title`, `all_companies`, `created_by`, `updated_by` | `users.phone`, `users.dni` NOT NULL UNIQUE | `phone` ✔; el resto ✘ en un lado u otro. |
| `user_companies` (N:M usuario-empresa) | `users.client_id` NOT NULL (1 cliente por usuario) | ≈ `we` no permite un usuario con varias empresas ni personal interno sin cliente. |
| `role_modules (role, module, actions text[])` | `roles` + `roles_modules (rol_id, module_id, actions_active json)` + `system_modules` (vacías) | ≈ Misma idea, normalizada en `we`. |
| `user_audit` (actor nullable = sistema; `target_user_id`, `target_company_id`, `request_id`, `target_product_id`; `detail` jsonb) | `action_logs (module, action, value json, reference_id int NOT NULL, user_id smallint NOT NULL, date timestamp)` | ≈ `we` no registra acciones del sistema (user_id NOT NULL) y no tiene zona horaria. |
| `profile_requests`, `user_notes` | — | ✘ solo nuestro. |

#### b) Productos y kits

| Nuestro | `we` | Diferencias |
|---|---|---|
| `products.company_id` bigint | `products.client_id` smallint | Tipo. |
| `products.sku` text, `UNIQUE (company_id, sku)` | `products.code` varchar, `UNIQUE (client_id, code)` | ✔ |
| `products.description` text | `products.title` varchar | ✔ (nombre distinto). |
| `products.type` text CHECK ('simple','kit') | `products.type` smallint DEFAULT 1 | ≈ Codificación desconocida (pregunta 1). |
| `products.wave_code`, `created_at`, `updated_at` | — | ✘ solo nuestro. |
| — | `products.ean13_code` | ✘ solo `we` (útil para el lector de SKU de Empaque: mejora futura). |
| `kit_components (kit_id, component_id, quantity int >0)` PK compuesta, CHECK kit ≠ componente | `products_kits (id, product_id_kit, product_id_recipe, quantity smallint DEFAULT 1)` | ≈ `we` no impide duplicados ni autorreferencia. |

#### c) Inventario

| Nuestro | `we` | Diferencias |
|---|---|---|
| Sin tabla de existencias: el disponible se consulta a Invas en vivo (DA-09) | `inventory_snapshots` (1 por cliente) + `inventory_balances` (por producto, lote, vencimiento, condición, vendible) + `inventory_conditions` | ✘ `we` guarda fotos de stock; nosotros no. |
| `stock_committed (company_id, sku, quantity)`; `order_stock_lines` (lo comprometido por orden, kits expandidos) | — | ✘ `we` **no tiene comprometido** (pedido explícito del cliente). |

#### d) Órdenes, líneas y estados

| Nuestro `orders` | `we.orders` | Diferencias |
|---|---|---|
| `id` bigserial | `id` integer | Tipo. |
| `company_id` | `client_id` | ✔ |
| `external_ref` + `channel`, `UNIQUE (company_id, channel, external_ref)` | `client_order_number` + `channel_sale` (MAYÚSCULAS) / `channel_id` (nulo), índice no único | ≈ Sin protección contra duplicados en `we`. |
| `status` (11 valores) + `prev_status` | `status_id` → `order_statuses` + `warehouse_status_id` → `warehouse_statuses` | ≈ Dos ejes en `we`; ver 2.3. |
| `recipient_name`, `recipient_phone`, `recipient_email` | ídem | ✔ |
| — | `recipient_dni` | ✘ solo `we`. |
| `address_line` | `shipping_address` NOT NULL | ✔ |
| `commune` text (nombre) + `region_code` | `commune_id` smallint NOT NULL | ≈ Por id; región derivada. |
| `dispatch_type` text (domicilio, retiro_oficina, express) | `dispatch_type` smallint (1, 2) | ≈ Pregunta 2. |
| `warehouse` text DEFAULT 'principal' | `warehouse_id` smallint (≈ `stores.id`, sin FK) | ≈ Pregunta 3. |
| `customer_notes` | `shipping_comments` | ✔ |
| `courier` text, `courier_code` (seguimiento) | `shipments.courier_service_id`, `shipment_packages.tracking_number` | ≈ En `we` el courier va en el envío, por servicio. |
| `wms_ref` text | `wms_ds_id` integer + `wms_ds_date` | ≈ |
| `handed_over_at`, `cancelled_at` | `shipments.dispatched_at`, `shipments.cancelled_at` | ≈ En el envío, no en la orden. |
| `created_at`, `updated_at` | `order_date` date + `created_at`, `updated_at` | ≈ |
| `priority`, `wave_id`, `note`, `version`, `validation_attempts`, `next_retry_at`, `last_error`, `source`, `import_id`, `created_by`, `incident_kind`, `claimed_by`, `claimed_at`, `rules_checked_at` | — | ✘ solo nuestro (prioridad, olas, incidencias, empaque, concurrencia). |
| — | `order_type`, `order_value` real, `client_site` | ✘ solo `we` (`real` no sirve para montos: pregunta 12). |

| Nuestro `order_items` | `we.order_items` | Diferencias |
|---|---|---|
| `sku` text | `product_id` FK | ≈ Nosotros referenciamos por SKU. |
| `quantity` int >0 | `quantity_requested` int >0 | ✔ |
| — | `quantity_prepared` int ≥0 | ✘ solo `we` (en Nexus lo informa Invas). |
| — | `orders_items_info (info json)` vacía | ✘ |

#### e) Historial y logs

| Nuestro | `we` | Diferencias |
|---|---|---|
| `order_events` (event_type, event, from_status, to_status text, actor text, reason, details jsonb, company_id, warehouse, occurred_at) | `order_status_history` (previous/new_status_id, source ∈ USER/SYSTEM/WMS/SALES_CHANNEL/COURIER, changed_by_user_id, reason, comments, external_reference, metadata jsonb, created_at) | ≈ `we` separa motivo y comentario y tiene «fuente»; nuestro actor es texto «user:{id} {nombre}». |
| (acciones sin cambio de estado: edición, prioridad, impresión) en `order_events` | `orders_logs (action, value json, user_id NOT NULL, date timestamp)` vacía | ≈ |
| Vista `activity_log` (une eventos, manifiestos y `user_audit`) | `action_logs` + `orders_logs` | ≈ `we` no tiene vistas. |
| `v_order_history` (vista) | — | ✘ |

#### f) Envíos, couriers y seguimiento

| Nuestro | `we` | Diferencias |
|---|---|---|
| `couriers (code PK, name, active)` | `couriers (id, name, code UNIQUE, tracking_url_template, active, timestamps)` | ≈ Códigos distintos (pregunta 7). |
| — | `courier_services` (nivel de servicio por courier) | ✘ solo `we`. |
| `courier_settings` (arrival_time, max_packages, covers_all, label_strategy) | — | ✘ solo nuestro. |
| `cutoffs`, `cutoff_couriers`, `ops_settings` | — | ✘ solo nuestro (pedido del cliente: horarios por courier). |
| `courier_coverage (courier, commune)` | `courier_communes` (+ external_code/name) y `courier_service_coverage` (por servicio, prioridad, restricciones) — vacías | ≈ `we` es más rico; vacío. |
| `order_packages` (code `NX-{id}-{n}`, peso y medidas) | `shipment_packages` (package_number, external_package_id, tracking_number, label_url, label_format, status_id) | ≈ Medidas solo nuestras; tracking por bulto solo en `we`. |
| `order_labels` (estado de etiqueta, reintentos) | `shipment_packages.label_url/label_format` + `shipment_statuses.LABELS_GENERATED` | ≈ Sin reintentos en `we`. |
| `manifests`, `manifest_lines`, `manifest_scans` | — | ✘ solo nuestro (retiro con RUT y patente, escaneo, «Despachar igual»). |
| `tracking_events` (por orden; provider, event_id único, status_raw) | `shipment_tracking_events` (por bulto; normalized_status_id, location, raw_payload) + `courier_status_mappings` — vacías | ≈ `we` normaliza estados; nosotros guardamos el crudo. |
| `webhook_inbox`, `integrations` | `channel_status_mappings` (vacía) | ✘ |
| `order_documents` (archivo con hash y storage) | `orders_documents (type_id → common_types, link)` vacía | ≈ |

#### g) Geografía

| Nuestro | `we` | Diferencias |
|---|---|---|
| `regions (code text PK, name, sort_order)` | `regions (id smallint, name, active)` | ≈ Mapeo id CUT ↔ código romano (2.1-13). Nombres: `we` usa nombres oficiales largos. |
| `communes (name PK, region_code, active)` 113 | `communes (id, region_id, name, aliases json)` 122 | ≈ 91 en común; ver 2.1-12. |

#### h) Configuración y operación interna

Solo nuestro: `wave_rules`, `companies.wave_code`, `products.wave_code`, vistas `v_product_waves`/`v_company_waves`, `ops_settings`, `job_outbox`, `order_imports`, `notifications`, `notification_reads`, `user_notes`. Solo `we`: `common_types` (catálogo genérico, vacío), `economic_activities` (giros SII, vacío), `stores`, `clients_stores`, `wms_status_mappings` (10 códigos `LEGACY_WMS`).

### 2.3 Mapeo de estados

**Nexus → `we` (para informar a Ismael y para la hoja «Comparación»)**

| Nexus `orders.status` | `we.order_statuses` | `we.warehouse_statuses` | Nota |
|---|---|---|---|
| `pending` Pendiente | 1 RECEIVED | 1 PENDING | |
| `validating` En validación | 1 RECEIVED | 1 PENDING | `we` no distingue la consulta de stock en curso. |
| `validated` Validada | 2 VALIDATED | 1 PENDING (sin OS) / 2 RECEIVED (con OS en Invas) | |
| `picking` En preparación | 3 PROCESSING | 3 PICKING | |
| `picked` Preparada | 3 PROCESSING | 5 READY | Preparada por Invas, aún sin empaque Nexus. |
| `packing` En empaque | 3 PROCESSING | 4 PACKING | En `we` «packing» es de bodega; en Nexus es el módulo Empaque. |
| `ready_pickup` Lista para retiro | 4 READY_FOR_SHIPMENT | 5 READY | |
| `handed_over` Entregada al courier | 5 SHIPPED | 5 READY | |
| `incident` Incidencia (`diferencia_empaque`) | 8 INCIDENT | 7 DAMAGED_PRODUCT o el previo | Otros tipos: 8 INCIDENT + estado de bodega previo. |
| `voided` Anulada (sin stock) | 8 INCIDENT (propuesta) | 6 OUT_OF_STOCK | Pregunta 4. |
| `cancelled` Cancelada | 10 CANCELLED | 8 CANCELLED (si estaba en Invas) / 9 NOT_APPLICABLE | |
| — (seguimiento, RF-25) | 6 PARTIALLY_DELIVERED, 7 DELIVERED | — | Nexus no tiene estados posteriores a la entrega al courier; vienen de `tracking_events`. |
| — | 9 ON_HOLD | — | Sin equivalente; en los datos corresponde a «PREVENTA». |

**`we` → Nexus (para la seed, fase 3.4)** — se decide por la pareja (estado, bodega):

| `we` (status_id, warehouse_status_id) | Filas | → Nexus | Comentario |
|---|---|---|---|
| (7 DELIVERED, *) | 198 | `handed_over` + evento de seguimiento «Entregado» fabricado | |
| (5 SHIPPED, *) | 152 | `handed_over` | |
| (4 READY_FOR_SHIPMENT, 5 READY) | 67 | `ready_pickup` | |
| (4 READY_FOR_SHIPMENT, 1 PENDING o 2 RECEIVED) | 239 | **decisión D4**: recomendado `validated` (sin ola) | Inconsistencia de origen («LISTO» del OMS antiguo). |
| (4 READY_FOR_SHIPMENT, 8 CANCELLED o 9 NOT_APPLICABLE) | 9 (5 + 4) | **decisión D4**: recomendado excluir de la seed | Combinación sin interpretación clara; no se adivina. |
| (2 VALIDATED, *) | 23 | `validated` | |
| (10 CANCELLED, *) | 9 | `cancelled` | |
| (9 ON_HOLD, *) | 3 | **decisión D4**: recomendado `pending` | Preventa. |

### 2.4 Veredicto por tabla de `we`

Criterio: quedan ~9 días y **no se integra nada todavía**. «ADAPTAR» = sus datos se reutilizan transformados en la seed o su idea se documenta como mejora; nunca se cambia nuestro esquema ahora.

| # | Tabla `we` | Equivalente nuestro | Veredicto | Justificación |
|---|---|---|---|---|
| 1 | `clients` | `companies` | ADAPTAR | Datos útiles para la seed (dni→tax_id, wms_code→texto); `parent_id` y `url_site` no se adoptan. |
| 2 | `clients_stores` | — (`orders.warehouse` texto) | DIFERIR | Bodegas por cliente/usuario es PV-21, pendiente de backend. |
| 3 | `stores` | — | DIFERIR | Sin tabla de bodegas en Nexus; los nombres pueden usarse como texto en la seed (D7). |
| 4 | `users` | `users` + Keycloak | MANTENER LA NUESTRA | `password`/`salt` contradicen Keycloak; `we` exige correo y un solo cliente. |
| 5 | `roles` | `users.role` (CHECK) | MANTENER LA NUESTRA | Vacía; nuestros roles están acordados (sin Picker; Despacho es permiso). |
| 6 | `roles_modules` | `role_modules` | MANTENER LA NUESTRA | Misma idea, vacía en `we`. |
| 7 | `system_modules` | `role_modules.module` | MANTENER LA NUESTRA | Vacía. |
| 8 | `action_logs` | `user_audit` + vista `activity_log` | MANTENER LA NUESTRA | `we` no registra al sistema (user_id NOT NULL) y no guarda zona horaria. |
| 9 | `products` | `products` | ADAPTAR | 1437 productos reutilizables en la seed (code→sku, title→description). `ean13_code` como mejora futura. |
| 10 | `products_kits` | `kit_components` | MANTENER LA NUESTRA | Vacía; la nuestra impide duplicados y autorreferencia. |
| 11 | `inventory_balances` | — (Invas en vivo + `stock_committed`) | DIFERIR | Nexus no guarda stock (DA-09); lote y vencimiento son mejora futura. |
| 12 | `inventory_snapshots` | — | DIFERIR | Ídem. |
| 13 | `inventory_conditions` | — | DIFERIR | Ídem. |
| 14 | `orders` | `orders` | ADAPTAR | 700 órdenes reutilizables en la seed con el mapeo 2.3. |
| 15 | `order_items` | `order_items` | ADAPTAR | product_id → sku vía `products.code`. |
| 16 | `orders_items_info` | — | DESCARTAR | Vacía y sin uso definido. |
| 17 | `order_statuses` | `orders.status` (dominio Go) | MANTENER LA NUESTRA | Estados acordados (PA-04); se documenta el mapeo. |
| 18 | `warehouse_statuses` | `picking`/`picked`/`packing` + Invas | MANTENER LA NUESTRA | Ídem. |
| 19 | `order_status_history` | `order_events` | MANTENER LA NUESTRA | Sus filas alimentan el evento inicial de la seed. |
| 20 | `orders_logs` | `order_events` / `activity_log` | MANTENER LA NUESTRA | Vacía. |
| 21 | `orders_documents` | `order_documents` | MANTENER LA NUESTRA | Vacía; la nuestra guarda hash y almacenamiento. |
| 22 | `common_types` | CHECK y constantes en código | DESCARTAR | Vacía; catálogo genérico sin integridad. |
| 23 | `economic_activities` | — | DESCARTAR | Vacía y fuera de alcance. |
| 24 | `regions` | `regions` | MANTENER LA NUESTRA | Mapeo 1:1 id CUT ↔ código romano. |
| 25 | `communes` | `communes` | MANTENER LA NUESTRA | Completar catálogo (31 comunas) es mejora futura vía migración de Core (D5). |
| 26 | `couriers` | `couriers` | MANTENER LA NUESTRA | Se mapean códigos; duplicados en `we` (pregunta 7). |
| 27 | `courier_services` | — | DIFERIR | Nexus no maneja nivel de servicio. |
| 28 | `courier_communes` | `courier_coverage` | MANTENER LA NUESTRA | Vacía; `external_code` como mejora futura. |
| 29 | `courier_service_coverage` | `courier_coverage` | DIFERIR | Depende de `courier_services`. |
| 30 | `courier_status_mappings` | normalización en Go | DIFERIR | Vacía; útil cuando haya seguimiento real. |
| 31 | `shipments` | `orders.courier`, `courier_code`, `handed_over_at` + `manifests` | ADAPTAR | Solo como fuente de fechas y tracking en la seed. |
| 32 | `shipment_packages` | `order_packages` + `order_labels` | ADAPTAR | Fuente de bultos (package_number → `NX-{id}-{n}`) y tracking. |
| 33 | `shipment_statuses` | `manifest_lines.status`, `order_labels.status` | MANTENER LA NUESTRA | Modelo distinto (retiro con manifiesto). |
| 34 | `shipment_package_statuses` | — | DIFERIR | Seguimiento por bulto es mejora futura. |
| 35 | `shipment_tracking_events` | `tracking_events` | MANTENER LA NUESTRA | Vacía; la seed fabrica eventos. |
| 36 | `channel_status_mappings` | — | DIFERIR | Vacía. |
| 37 | `wms_status_mappings` | traducción en Go | DIFERIR | Se cita como referencia de códigos `LEGACY_WMS`; no se crea tabla. |

**Resumen:** 0 ADOPTAR · 7 ADAPTAR (solo datos para la seed) · 15 MANTENER LA NUESTRA · 12 DIFERIR · 3 DESCARTAR.

### 2.5 Cruce con lo pedido por el cliente en el Sprint Review

| Pedido | Nexus hoy | `we` | Conclusión |
|---|---|---|---|
| Stock «comprometido» | `stock_committed` + `order_stock_lines` (se libera al confirmar la ola o al cancelar) | No existe; `inventory_balances` solo cantidad | Mantener lo nuestro; pregunta 6. |
| Horarios por courier (llegada y corte) | `cutoffs`, `cutoff_couriers`, `ops_settings` | No existe | Mantener; pregunta 5. |
| Historial por antigüedad (1 año) | Calculado en `GET /v1/orders/registry` (último cambio de estado o `handed_over_at`), sin tabla | No existe política | La seed debe traer órdenes con más de 1 año (las de `we` no sirven tal cual). |
| Perfiles creados por el administrador | `users` + Keycloak; `profile_requests` | `users` con contraseña | Mantener; pregunta 8. |
| Sin perfil Picker | Roles: admin, jefe_operaciones, analista, empaque, cliente; Despacho = permiso | `roles` vacía | Coherente. |
| Seguimiento por webhook | `webhook_inbox`, `tracking_events` | `shipment_tracking_events` + `courier_status_mappings` (vacías) | Mantener; mapeo de estados como mejora. |
| Descuento de inventario al despachar | Lo hace Invas; Nexus solo libera su comprometido | `inventory_balances` por snapshot | Sin cambio de esquema; pregunta 6. |
| Comentario al cancelar y al cambiar dirección | `order_events.reason` (cancelación con motivo; edición registra «Campo: x → y») | `order_status_history.comments` / `orders_logs.value` | Equivalente; pregunta 20. |

---

## 3. Fases (en orden)

Calendario: hoy 17-11 (martes). Entrega real: lunes 24-11 (deja martes 25 de reserva). Presentación 26 y 27-11.

| Fase | Qué | Depende de | Esfuerzo | Fecha objetivo |
|---|---|---|---|---|
| 3.0 | Este plan (hecho) | — | — | 17-11 |
| 3.1 | Excel v1.3 | sección 4 (D1, D2) | 3 h | 18-11 |
| 3.2 | Preguntas para Ismael (enviar) | — | 1 h | 18-11 (**enviar ese día**) |
| 3.3 | Informe `docs/bd/comparacion_bd_ismael.md` | 3.1 | 3 h | 19-11 |
| 3.4 | Seed definitiva | D3 a D8; respuestas de Ismael si llegan (si no, usar las recomendaciones) | 8–10 h | 20 al 21-11 |
| 3.5 | Regenerar capturas | 3.4, D9 | 2–3 h | 22-11 |
| 3.6 | Adopción ahora vs. mejora futura (sección en el informe) | 3.3 | 1 h | 22-11 |
| 3.7 | Revisión final, actualizar el informe con respuestas de Ismael, commit | todas | 2 h | 24-11 |

Total ≈ 20–23 h de trabajo.

### 3.1 Excel v1.3

- **Base:** `docs/bd/Historial de cambios BD NexusWST v1.2 (reconstruido).xlsx`. **Salida:** `docs/bd/Historial de cambios BD NexusWST v1.3.xlsx` (y copia en `respaldo/docs/`). No modificar el v1.2.
- **Herramienta:** no hay openpyxl. Editar el XML del .xlsx con `zipfile` + `re` de la biblioteca estándar (celdas `inlineStr`, estilo de datos `s="4"`, encabezado `s="3"`, título `s="1"`, subtítulo `s="2"`). Validar abriendo con Excel por COM (`Excel.Application`, solo lectura) y comprobando que el XML esté bien formado.
- **Columna I «Equivalente en esquema we (Ismael)» en las 84 filas** con la tabla 3.1-A (ninguna queda «Por verificar»). Formato obligatorio: `we.tabla.columna (nota)` · `Sin equivalente en we: <razón>` · `No aplica: <razón>` · `Sin equivalente en we; consultar a Ismael (pregunta N)`.
- **Columna K «Estado»:** según la tabla 3.1-B.
- **Reemplazos:** agregar en la columna G de la fila reemplazada, al final, «Reemplazada por DB-NN.» solo si D2 lo autoriza; si no, anotarlo en «Notas de reconstrucción». Pares: DB-17 elimina DB-01; DB-16 elimina DB-02; DB-11 a DB-15 reemplazan el modelo de DB-01; DB-79 reemplaza DB-60; DB-82 reemplaza DB-66; DB-68 modifica DB-45; DB-84 modifica DB-68.
- **Hoja nueva «Comparación»** (después de «Leyenda»): columnas `Tabla we | Filas en el volcado | Equivalente nuestro | Diferencias | Veredicto | Justificación`, con las 37 filas de la tabla 2.4 (diferencias resumidas de 2.2). Encabezado con el estilo del encabezado del historial.
- **Hoja «Notas de reconstrucción»:** agregar al final las notas de v1.3 (qué cambió, fecha, regla de llenado de la columna I, decisión D1).
- No tocar la columna «Informado a Ismael» salvo que ya se hayan enviado las preguntas (entonces poner la fecha de envío solo en las filas que citan una pregunta).

**Tabla 3.1-A · Columna I por fila**

| Filas | Valor |
|---|---|
| DB-01 | Sin equivalente en we: we.couriers y we.courier_services no tienen hora de corte; consultar a Ismael (pregunta 5) |
| DB-02 | No aplica: restricción de una columna eliminada (DB-16, DB-17) |
| DB-03 | Sin equivalente en we: no hay tabla de parámetros de operación (we.common_types es un catálogo genérico, vacío y sin auditoría) |
| DB-04 | Sin equivalente en we; consultar a Ismael (pregunta 5) |
| DB-05, DB-09, DB-10 | Sin equivalente en we: los recordatorios del Home son propios de Nexus |
| DB-06 | we.couriers (id 8, code RBL «Recibelo») |
| DB-07 | No aplica: descartado; we.order_items no tiene lote (sí we.inventory_balances.lot, a nivel de stock) |
| DB-08 | Sin equivalente en we: situación calculada; we.orders.warehouse_status_id solo da el estado en bodega |
| DB-11 a DB-15 | Sin equivalente en we; consultar a Ismael (pregunta 5) |
| DB-16 | No aplica: elimina la restricción de DB-02 |
| DB-17 | No aplica: elimina la columna de DB-01 |
| DB-18 | we.regions (id = número CUT 1–16; sin código romano ni sort_order) |
| DB-19 | we.couriers.code (SHP_CHILEXPRESS; BLX o SHP_BLUEXPRESS; SKN; Mercado Envíos ¿CML?); consultar a Ismael (pregunta 7) |
| DB-20 | No aplica: retiro de datos de prueba, no es esquema |
| DB-21 a DB-23 | Sin equivalente en we: no hay horario de operación; consultar a Ismael (pregunta 5) |
| DB-24 | No aplica: cola técnica interna de Nexus (outbox) |
| DB-25, DB-26 | Sin equivalente en we; consultar a Ismael (pregunta 15) |
| DB-27 | we.shipment_packages.label_url y label_format (+ we.shipment_statuses LABELS_GENERATED); sin reintentos |
| DB-28 | No aplica: índice técnico de reintentos |
| DB-29 | No aplica: en we se obtiene por we.order_status_history.order_id → we.orders.client_id |
| DB-30 | No aplica: en we se obtiene por we.order_status_history.order_id → we.orders.warehouse_id |
| DB-31 | Sin equivalente en we: no hay tabla de avisos |
| DB-32 | we.products_kits (product_id_kit, product_id_recipe, quantity); consultar a Ismael (pregunta 1) |
| DB-33 | No aplica: índice técnico |
| DB-34 | Sin equivalente en we: we no registra comprometido; consultar a Ismael (pregunta 6) |
| DB-35 a DB-38 | Sin equivalente en we; consultar a Ismael (pregunta 21) |
| DB-39 | we.order_status_history.metadata (jsonb) |
| DB-40 | we.users.email (NOT NULL en we); consultar a Ismael (pregunta 8) |
| DB-41 | we.users.username (NOT NULL, sin regla de formato) |
| DB-42 | Sin equivalente en we: we exige email y username (ambos NOT NULL) |
| DB-43 | we.order_status_history (índice order_status_history_order_date_idx sobre order_id, created_at DESC) |
| DB-44, DB-67, DB-83 | No aplica: índice técnico; we.action_logs no tiene índices |
| DB-45, DB-68, DB-84 | Sin equivalente en we: we no tiene vistas; lo más cercano son we.action_logs y we.orders_logs (vacías) |
| DB-46 | we.clients.dni (NOT NULL, UNIQUE) |
| DB-47 | Sin equivalente en we: we.clients_dni_unique compara el texto literal; consultar a Ismael (pregunta 13) |
| DB-48 | we.clients.email (NOT NULL en we) |
| DB-49 | Sin equivalente en we: we.clients no tiene teléfono |
| DB-50, DB-51 | Sin equivalente en we: we.clients no registra quién crea o edita |
| DB-52 | we.users.phone |
| DB-53 | Sin equivalente en we: we.users no tiene cargo |
| DB-54 | No aplica: en we cada usuario tiene un solo cliente (we.users.client_id); consultar a Ismael (pregunta 8) |
| DB-55 | Sin equivalente en we: no hay solicitudes de perfiles (flujo propio de Nexus) |
| DB-56 a DB-62, DB-69 a DB-80 | No aplica: objeto de profile_requests, que no existe en we |
| DB-63 | we.action_logs.reference_id (id genérico, NOT NULL) |
| DB-64, DB-65, DB-81 | we.action_logs.module + reference_id (un solo id genérico por módulo) |
| DB-66, DB-82 | No aplica: we.action_logs usa un único reference_id NOT NULL |

**Tabla 3.1-B · Estados**

| Filas | Estado actual | Estado nuevo | Motivo |
|---|---|---|---|
| DB-01, DB-02 | Propuesto | En repositorio (y nota «Reemplazada por DB-17 / DB-16») | La migración orders 000003 está en el repo; su efecto lo deshace la 000004. Ver D2. |
| DB-03, DB-04, DB-05, DB-06 | Propuesto | En repositorio | orders 000003, core 000006 y core 000007 están en origin/jean_dev. |
| DB-07 | Descartado | Descartado | Sin cambio. |
| DB-08 | Propuesto | En repositorio | El campo `assignmentSituation` está en el código de orders-service. |
| DB-09 a DB-84 | En repositorio | En repositorio | Sin cambio. «Informado a Ismael» solo cuando se envíen las preguntas. |

### 3.2 Preguntas para Ismael (enviar tal cual)

Guardar en `docs/bd/preguntas_ismael.md` y enviarlas por correo **el 18-11**. Texto:

> Hola Ismael. Revisamos el volcado `pg-beta-local-202611101551.sql` (esquema `we`) y lo comparamos con la base de NexusWST. No vamos a integrar nada antes de la presentación; queremos dejar documentadas las equivalencias. Te dejamos estas preguntas, numeradas para poder citarlas:
>
> 1. `products.type` vale 1 en los 1437 productos. ¿Qué valor corresponde a un kit? ¿`products_kits.product_id_recipe` es el componente y `quantity` la cantidad por kit?
> 2. `orders.dispatch_type` tiene los valores 1 y 2. ¿Qué significa cada uno? Las órdenes con 2 no tienen envío.
> 3. `orders.warehouse_id` coincide con `stores.id` según `clients_stores`, pero no tiene FK. ¿Es intencional? ¿Una orden puede salir de una bodega distinta a la principal del cliente?
> 4. Estados: 188 órdenes están «Lista para despacho» con bodega «Pendiente de envío al WMS». ¿Cómo se interpreta esa combinación? ¿`ON_HOLD` equivale a «preventa»? En Nexus, una orden sin stock queda «Anulada» (recuperable): ¿la registrarían como `INCIDENT` con bodega `OUT_OF_STOCK`?
> 5. El cliente pidió hora de llegada y hora de corte por courier (y varios cortes por día), además de un horario de operación. En `we` no vemos dónde guardarlo. ¿Lo agregarán, por ejemplo en `courier_services`?
> 6. `inventory_balances` no tiene stock comprometido. En Nexus el comprometido lo lleva el OMS hasta que Invas confirma la ola. ¿Ustedes lo calcularían o se agregará una columna? ¿Cada cuánto se cargan los snapshots desde Invas?
> 7. Couriers: hay dos Blue Express (`BLX` y `SHP_BLUEXPRESS`) y Chilexpress usa `SHP_CHILEXPRESS`. ¿Cuál es el código oficial de cada uno? ¿«Colecta MELI» es Mercado Envíos?
> 8. `users` guarda `password` y `salt`. Nexus usa Keycloak y no guarda contraseñas. ¿Podrían vincular por un identificador externo? Además: ¿qué `client_id` llevan los usuarios internos de WeStorage (es NOT NULL)? ¿Un usuario puede ver varias empresas? ¿Puede haber operarios sin correo (`email` es NOT NULL)?
> 9. `users` no tiene un campo «activo». ¿Cómo se desactiva un perfil?
> 10. `action_logs.user_id` es NOT NULL: ¿cómo registran acciones del sistema o de integraciones? `action_logs.date` y `orders_logs.date` no tienen zona horaria: ¿en qué zona se guardan?
> 11. ¿Para qué se usa `clients.parent_id`?
> 12. `orders.order_value` es `real`. Para montos en pesos, ¿lo cambiarían a `numeric`?
> 13. `clients.dni` es único como texto. ¿Se guarda el RUT normalizado (sin puntos ni guion)? Nexus normaliza para evitar duplicados.
> 14. Las órdenes no tienen restricción única por cliente y número de orden. ¿Cómo evitan duplicados cuando un canal reenvía la misma orden?
> 15. Couriers sin etiqueta por API (Recíbelo, «Buscar en oficina») y límite de bultos por orden: ¿dónde lo guardarían?
> 16. Retiro: Nexus registra un manifiesto con quién retira (nombre, RUT, patente), los bultos escaneados y «Despachar igual» con motivo. ¿Lo contemplan en `we`?
> 17. ¿Hay una política de retención o archivo para órdenes de más de un año?
> 18. ¿Podemos guardar en nuestro repositorio una seed derivada de este volcado (son datos ficticios con `example.test`)?
> 19. `communes` tiene 122 comunas y `aliases` vacío. ¿Cargarán las 346 oficiales? Vimos «Natales» (nosotros «Puerto Natales») y «Alto Bio bío».
> 20. Comentarios: ¿el motivo de cancelación y el cambio de dirección van en `order_status_history.comments` o en `orders_logs.value`?
> 21. Olas de Invas por cliente y por producto (acordado en el Sprint Review): ¿se quedan solo en Nexus o las agregarán a `we`?
> 22. `wms_status_mappings` (`LEGACY_WMS`): ¿son los códigos que devuelve Invas hoy? `order_status_id` está vacío en las 10 filas: ¿se completará?
>
> Gracias. Jean y Cristopher.

### 3.3 Informe para la profesora: `docs/bd/comparacion_bd_ismael.md`

Público: profesora del Capstone (no técnica en el detalle). Máximo ~4 páginas. Estructura:
1. Contexto (qué es `we`, qué es Nexus, por qué se comparan; que no se integra antes del 26-11).
2. Resumen en cifras (37 tablas, 16 vacías; veredictos 0/7/15/12/3).
3. Diferencias de diseño que importan: identidad (Keycloak vs. contraseña), stock comprometido, cortes por courier, manifiesto de retiro, estados en dos ejes, auditoría del sistema, tipos (`real`, `smallint`, `timestamp` sin zona).
4. Mapeo de estados (tabla 2.3).
5. Tabla de veredictos (2.4).
6. Lo que se adopta ahora y lo que queda como mejora futura (3.6).
7. Preguntas enviadas a Ismael (resumen y fecha) y, si llegan, sus respuestas.
Sin datos personales ni credenciales. Enlazar el Excel v1.3.

### 3.4 Seed definitiva (después de las decisiones D3 a D8)

**Dónde:** según D3 (recomendado: `respaldo/seed/seed_we_demo.sql` + `respaldo/seed/borrar_seed_we_demo.sql`, fuera de git hasta que Ismael responda la pregunta 18). Generada por un script Python (`respaldo/seed/generar_seed_we.py`) que lee el volcado y escribe SQL; así se puede regenerar. No modifica `seed_demo.sql` ni `seed_dev.sql`.

**Reutilizar del volcado (con transformación):**

| Dato `we` | → Nexus | Transformación |
|---|---|---|
| `clients` (7) | `companies` | `name` («Cliente Prueba NN»: decidir si se renombran, D6); `dni` `TEST-CLIENT-NN` no es RUT → generar RUT ficticio válido con dígito verificador (el Core normaliza y exige único); `wms_code` → texto; `email` → `email`. Ids altos fijos (p. ej. 910001–910007) para que la limpieza los reconozca, como Andes Outdoor 900001. |
| `products` (1437) | `products` | `code` → `sku`, `title` → `description`, `type` 1 → `simple`. Limitar a los productos usados en órdenes + 40 por cliente si el volumen ralentiza la demo. |
| `regions` (16) | — | No se cargan (ya están por migración); solo mapa id CUT → código romano para traducir. |
| `communes` (122) | — | No se cargan; se mapean por nombre normalizado (sin tildes, minúsculas). «Natales» → «Puerto Natales». Las órdenes en las 31 comunas faltantes se excluyen o se reasignan según D5. |
| `orders` (700) | `orders` | `client_order_number` → `external_ref`; `channel_sale` → minúsculas (`shopify`, `mercadolibre`, `api`…); dirección, destinatario y comuna; estado por la tabla 2.3 (we → Nexus); `warehouse` por D7; courier por `shipments.courier_service_id` → `courier_services.courier_id` → `couriers.code` → código Nexus (D8); `source` = `webhook` si viene de canal, `bulk` para `API`; fechas: conservar `created_at` y **desplazar un subconjunto a hace más de 1 año** (ver abajo). |
| `order_items` (1065) | `order_items` | `product_id` → `sku`; `quantity_requested` → `quantity`. |
| `order_status_history` (700) | `order_events` | Un evento `created` con `actor = «Sistema»` y `reason` = «Importada desde el OMS anterior»; `metadata` → `details`. Agregar los eventos intermedios que exige cada estado Nexus (validación, ola, preparación, empaque, entrega) con fechas crecientes, usando los mismos `event_type`/`event` del código (ver `seed_demo.sql`). |
| `shipments` + `shipment_packages` | `order_packages`, `order_labels`, `orders.courier_code`, `handed_over_at` | `package_number` → `code` `NX-{order_id}-{n}`; `tracking_number` → `orders.courier_code`; `dispatched_at` → `handed_over_at`; etiqueta `ready` si hay tracking o `label_url`. |
| `inventory_balances` | — | No se carga (Nexus consulta a Invas). Solo si D3 lo pide: usar `quantity` de `DISPONIBLE` como stock del simulador de Invas en desarrollo. |

**Fabricar (no existe en el volcado):**

| Dato | Cómo |
|---|---|
| Órdenes en estados activos (pendiente, en validación, en preparación, preparada, en empaque, incidencia, anulada) | Mantener las 42 de `seed_demo.sql` (Andes Outdoor y Empresa Demo Uno) y/o derivar ~5 por estado desde órdenes `we` recientes. |
| Historial de más de 1 año (pantalla Registro) | Desplazar ~20 órdenes entregadas o canceladas de `we` a 2024–2025 (último cambio > 1 año). |
| Eventos de seguimiento | 1 a 4 `tracking_events` por orden entregada (`provider` = courier, `event_id` único, estados crudos tipo «En tránsito», «Entregado»). |
| Incidencias | Usar los tipos reales del dominio (`datos`, `invas`, `diferencia_empaque`, `courier_cobertura`, `courier_bultos`, `cancelacion`, `retiro`). |
| Kits | 2 a 3 kits por cliente con `kit_components` sobre productos simples del mismo cliente. |
| Usuarios y perfiles | Perfiles `users` + `user_companies` para 1 Cliente por empresa `we`; cuentas en Keycloak solo para las que inician sesión (script como `crear_usuario_demo_keycloak.sh`). Sin contraseñas en SQL. |
| Manifiestos | 1 cerrado y 1 abierto por courier principal, con líneas, escaneos y un «Despachar igual». |
| Cortes y reglas de ola | Reutilizar los de `seed_demo.sql`. |

**Criterios de aceptación:** se ejecuta 2 veces sin duplicar; el borrado deja la base como antes; todas las pantallas tienen datos de los 4 perfiles; el Registro muestra órdenes de más de 1 año; las e2e existentes siguen pasando (en `frontend-web/`: `npm run test:e2e`, que es `playwright test`); typecheck, lint y build sin errores.

### 3.5 Regenerar capturas

1. Resolver D9 (carpeta `docs/pantallas/` borrada en el árbol local).
2. Cargar la seed definitiva; levantar el stack (`make up`).
3. En `frontend-web/`: `npx playwright test -c playwright.capturas.config.ts` (ejecuta `capturas/pantallas.spec.ts`; escribe en `docs/pantallas/`). Si la seed agrega o cambia los usuarios de prueba, ajustar `e2e/helpers` antes.
4. Revisar a mano 1 captura por módulo; actualizar `docs/pantallas/README.md` si cambian pantallas o usuarios.

### 3.6 Qué se adopta ahora y qué queda como mejora futura

**Ahora (sin cambiar esquema):** datos de `we` en la seed (clientes, productos, órdenes, líneas, bultos, tracking); mapeo de estados documentado; equivalencias en el Excel v1.3.
**Mejora futura (después del 27-11, con Cristopher y respuestas de Ismael):** EAN13 en productos (lector de Empaque); lote y vencimiento (`inventory_balances`); niveles de servicio de courier (`courier_services`, `courier_service_coverage`); cobertura con código externo (`courier_communes`); seguimiento por bulto y estados normalizados (`shipment_tracking_events`, `courier_status_mappings`); bodegas (`stores`, `clients_stores`, PV-21); catálogo oficial de 346 comunas con alias; `source` y `comments` separados en el historial; empresa matriz (`parent_id`). Las migraciones de Core y Órdenes deben coordinarse con Cristopher (sus próximos números: Core 000014, Órdenes 000012).

### 3.7 Cierre

Actualizar el informe con las respuestas de Ismael si llegaron, marcar «Informado a Ismael» en el Excel, commit `docs(bd): ...` en `jean_dev` y push. Sin merge.

---

## 4. Decisiones del usuario (con recomendación)

| # | Decisión | Recomendación |
|---|---|---|
| D1 | Responsable de DB-01 a DB-08: el v1.0 dice Cristopher; git dice que los commits son de Jean. | Confirmar con Cristopher quién escribió el SQL (el LEEME de la seed indica que se preparó en `respaldo/seed/migraciones` antes de pasar al repo). Hasta entonces, **no cambiar** y dejar la nota en «Notas de reconstrucción». |
| D2 | ¿Se permite editar DB-01 a DB-08 (columnas I, K y la nota de reemplazo en G)? | Sí: cambiar I y K y agregar «Reemplazada por DB-NN» en G; registrar cada cambio en las notas. |
| D3 | Dónde vive la seed definitiva y si se versiona. | `respaldo/seed/` (local) hasta que Ismael autorice (pregunta 18); después, `infrastructure/dev-seed/`. |
| D4 | Mapeo de las combinaciones dudosas de `we` (239 «Lista para despacho» con bodega pendiente o recibida; 9 «Lista para despacho» con bodega cancelada o «no aplica»; 3 `ON_HOLD`). | `validated` sin ola, excluir y `pending`, respectivamente; documentarlo en el informe. |
| D5 | 33 órdenes en 31 comunas que no tenemos. | Excluirlas de la seed (no crear migración ahora). Completar comunas queda como mejora futura con Cristopher. |
| D6 | Nombres «Cliente Prueba NN» y RUT `TEST-CLIENT-NN`. | Conservar los nombres (trazabilidad con Ismael) y generar RUT ficticios válidos; mantener Andes Outdoor como protagonista de la demo. |
| D7 | Valor de `orders.warehouse` para las órdenes `we`. | Usar el nombre corto de `stores` («Enea 4», «San Bernardo», «Las Condes») como texto, sin crear tabla. |
| D8 | Couriers de `we` que no existen en Nexus (Home Delivery Corp, Flota WeStorage, falaflex, EnviaFlash, Welivery). | Mapear los equivalentes (BLX/SHP_BLUEXPRESS → `blueexpress`, SKN → `starken`, SHP_CHILEXPRESS → `chilexpress`, RBL → `recibelo`, CML → `mercadoenvios`) y agregar los demás como filas de catálogo **solo en la seed** (no por migración). |
| D9 | `docs/pantallas/` está borrada en el árbol local (57 archivos, sin commit). | Si no fue intencional: `git restore docs/pantallas` antes de regenerar. No hacer commit de ese borrado por accidente. |
| D10 | Migraciones base (25-10) fuera del historial. | Mantenerlas fuera de las filas; la hoja «Comparación» ya cubre esas tablas. |

---

## 5. Riesgos y qué NO hacer

**Riesgos**
- **R1:** borrado local de `docs/pantallas/`; un `git add -A` lo subiría. Agregar siempre archivos por ruta.
- **R2:** las respuestas de Ismael pueden no llegar antes del 24-11. El plan funciona con las recomendaciones de la sección 4; las preguntas quedan como «enviadas» en el informe.
- **R3:** volumen de la seed (1437 productos, 700 órdenes) puede hacer lentas las pantallas o romper e2e que asumen pocos datos. Medir y recortar si hace falta.
- **R4:** RUT ficticios mal formados hacen fallar la migración Core 000011 o el índice único normalizado.
- **R5:** fechas de la seed relativas a la carga: recargar el día de la grabación o de la presentación.
- **R6:** el Excel se edita a mano por XML: validar siempre con Excel y con un parser XML antes de guardar.

**Qué NO hacer**
- No crear ni editar migraciones ni código de backend; no crear el esquema `we` en nuestra base; no integrar el API Mock.
- No copiar el volcado, credenciales del correo ni datos del mock a archivos versionados (salvo la seed derivada si Ismael lo autoriza).
- No modificar `seed_demo.sql`, `seed_dev.sql`, el v1.0 ni el v1.2 del Excel.
- No hacer merge ni push a `main`; no usar `git add -A`.
- No inventar equivalencias: si no hay base en el volcado, va a «consultar a Ismael (pregunta N)».
- No cambiar el responsable de DB-01 a DB-08 sin la decisión D1.
