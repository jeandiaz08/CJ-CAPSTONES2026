# Traspaso: módulo Órdenes (frontend y backend)

- **De:** Jean · **Para:** Cristopher (y su Claude Code) · **Fecha:** 07-11-2026
- **Base:** `jean_dev` (este documento) y `test`, que tiene el mismo contenido. Las rutas son relativas a `NexsusWST/`.
- **Cómo leerlo:**
    - La sección 2 sirve para decidir qué versión conservar si tocaste lo mismo.
    - La 3 es la lista de trabajo de backend.
    - La 7 dice cómo comprobar que todo sigue verde.

---

## 1. Resumen

1. El frontend de Órdenes está terminado. Incluye:
    - Todas las órdenes y Asignación (con cortes, horario y olas de Invas).
    - Empaque.
    - Retiro y entrega, con manifiesto y «Despachar igual».
    - Registro histórico, con las pestañas «Registro órdenes» y «Registro de acciones».
    - Seguimiento y campana de avisos.
    - Favicon de Nexus.
2. El backend nuevo de la Tarea 06 está en `orders-service`:
    - `GET /v1/orders/registry` y `GET /v1/orders/registry/export`.
    - `GET /v1/activity` y `GET /v1/activity/export`.
    - La migración Órdenes `000009_activity_log`.
    - De tus archivos, solo cambiaron las líneas autorizadas (sección 2.3).
3. Tu rama `cristopher_dev`, en 7803d1f, se integró en `jean_dev` con el merge **6473d39**, sin conflictos. Desde entonces `cristopher_dev` no tiene commits nuevos, así que `git log jean_dev..origin/cristopher_dev` sale vacío.
4. `jean_dev` va 17 commits (sin contar merges) por delante de `cristopher_dev`: de bf5d74e (Retiro, Fase 1) a este documento. `test` = `jean_dev` (último merge cfd6016 más el de este documento). `main` no se tocó.
5. **Numeración de migraciones (actualizada el 10-11-2026):** Jean toma **Core 000011** y **Órdenes 000010** (Clientes, Tarea 07), **Core 000012** (solicitudes internas de Usuarios WeStorage, Tarea 08) y **Core 000013** y **Órdenes 000011** (Productos, Tarea 09: auditoría de productos y apartado «Productos» del registro). Tú sigues desde **Core 000014** y **Órdenes 000012**.

---

## 2. Cambios de BACKEND hechos por Jean

### 2.1 Después de integrar tu rama (6473d39 → hoy)

| Commit | Archivos (orders-service) | Qué hace | Por qué | Cómo probarlo |
|---|---|---|---|---|
| bbd71a9 | `internal/application/dispatch_anyway.go` (**tuyo**) y `dispatch_anyway_test.go` | `canAuthorizeIncomplete` ahora devuelve `canDispatch(id)` (permiso **Despacho**) en vez de `HasRole(Admin, JefeOps)`. 403: «Necesitas el permiso Despacho para autorizar un despacho con bultos faltantes.» | Lo pide la Matriz de perfiles: «Despacho» es un permiso, no un perfil. Con el seed autorizan Jefe y Empaque; Admin queda en solo lectura | `go test ./internal/application -run DispatchAnyway`: Empaque con permiso OK; Empaque, Jefe y Admin sin permiso → `ErrForbidden` |
| 38cd930 | Nuevos: `internal/ports/registry.go`, `internal/adapters/outbound/postgres/registry.go`, `internal/application/registry.go`, `internal/adapters/inbound/http/registry.go` (`HistoryHandler`), con pruebas. **Tuyos:** `cmd/api/main.go` (+1 línea) y `internal/adapters/outbound/postgres/repository.go` (2 líneas en `List`) | Registro histórico: `GET /v1/orders/registry`. La bandeja excluye exactamente lo que entra al registro | RF-28 / Tarea 06 Fase 1. Regla nueva: Cancelada → de inmediato; cualquier otra → 1 año desde su último cambio de estado (Entregadas: `handed_over_at`) | `go test ./internal/application -run Registry` y `./internal/adapters/outbound/postgres -run Registry` (con `TEST_DATABASE_URL`). La prueba de postgres comprueba que bandeja y registro son complementarios |
| cc121bc | `internal/application/registry.go`, `internal/adapters/inbound/http/registry.go` | `GET /v1/orders/registry/export` (xlsx o csv) | «Excel acciones» del registro | `curl` con Bearer a `/api/orders/v1/orders/registry/export?format=csv` |
| 24da8ce | **Migración `migrations/000009_activity_log.{up,down}.sql`**. Nuevos: `internal/ports/activity.go`, `internal/adapters/outbound/postgres/activity.go`, `internal/application/activity.go`, `internal/adapters/inbound/http/activity.go`, con pruebas. `cmd/api/main.go`: la misma línea de 38cd930, ahora con `application.NewActivity(postgres.NewActivity(db))` | Vista `activity_log` (une `order_events`, creación/cierre de `manifests` y `user_audit`) e índices `idx_order_events_occurred` e `idx_user_audit_created`. `GET /v1/activity` | RF-28 / Tarea 06 Fase 2: registro de acciones de personas y sistema | `go test ./internal/application -run Activity`; la prueba de postgres corre sobre la vista real dentro de una transacción con rollback |
| 3f0d641 | `internal/application/activity_text.go` (nuevo), `activity.go` (application, http y postgres), `internal/ports/activity.go`, `internal/adapters/inbound/http/registry.go` | `GET /v1/activity/export` y textos en español por fila (`actionLabel`, `statusChange`, `courierName`, `transportCompany`, `observation`, `detailLines`). `q=#N` busca el manifiesto N | Tarea 06 Fase 3: la pantalla y el Excel muestran lo mismo | `go test ./internal/application -run Activity` |
| 99e1a1f | `docs/API.md` | Secciones «Registro histórico», «Registro de acciones», «Despachar igual» por permiso y la nueva regla de `view=` | Contrato al día | Lectura |

### 2.2 Migraciones

| Servicio | N° | Archivo | Autor | Qué hace |
|---|---|---|---|---|
| Core | 000006 | `backend-core/migrations/000006_notas_usuario` | Jean (aa567d8) | Notas del usuario (`/v1/me/notes`) |
| Core | 000007 | `backend-core/migrations/000007_courier_recibelo` | Jean (c3d5fe3) | Courier Recíbelo («Buscar en oficina») |
| Core | 000002 / 000004 | `000002_catalogos`, `000004_reservada` | Jean (c3d5fe3) | Separa los catálogos reales de los datos de prueba, que pasan a `infrastructure/dev-seed/seed_dev.sql` |
| Core | 000008–000010 | kits, ola por cliente/producto, usuario sin correo | Cristopher | — |
| Órdenes | 000003 | `orders-service/migrations/000003_cortes_courier` | Jean (aa567d8) | Cortes por courier |
| Órdenes | 000004 | `000004_cortes_multicourier` | Jean (aa567d8) | Cortes con varios couriers y `ops_settings` (hora límite) |
| Órdenes | 000005 | `000005_horario_operacion` | Jean (f99c7b6) | `operation_start`, `operation_days`, `priority_overrides_deadline` en `ops_settings`, y `job_outbox.next_cutoff` |
| Órdenes | 000006 | `000006_etiquetas` | Jean (293ff1f) | `order_labels`: estrategia de etiqueta por courier y reintento |
| Órdenes | 000007–000008 | `order_stock_lines`, `despacho_incompleto` (`order_events.details`) | Cristopher | — |
| Órdenes | **000009** | `000009_activity_log` | Jean (24da8ce) | Vista `activity_log` y 2 índices. Solo lectura: no copia ni borra datos |

**Regla (actualizada el 08-11-2026):** Core **000011** y Órdenes **000010** son de Jean (módulo Clientes, Tarea 07: empresas, perfiles, solicitudes y la vista `activity_log` con empresas y solicitudes). Core **000012** también es de Jean (Tarea 08: `profile_requests` con `scope`, sin empresa en las internas). Core **000013** y Órdenes **000011** también son de Jean (Tarea 09: `user_audit.target_product_id` y la vista `activity_log` con productos). Tus próximas migraciones son **Core 000014** y **Órdenes 000012**. Las tablas de versión son `schema_migrations_orders` y `schema_migrations_core`.

### 2.3 Archivos tuyos que cambié (los únicos autorizados)

| Archivo | Cambio exacto | Si tú también lo cambiaste |
|---|---|---|
| `orders-service/cmd/api/main.go` | Una línea: `ordershttp.NewHistoryHandler(application.NewRegistry(postgres.NewRegistry(db)), application.NewActivity(postgres.NewActivity(db)), log).Register(mux, requireAuth) // Registro histórico y de acciones (Tarea 06, Jean)` | Conserva tus cambios y mantén esta línea. Las 4 rutas viven en `HistoryHandler` (`internal/adapters/inbound/http/registry.go:32-35`) |
| `orders-service/internal/adapters/outbound/postgres/repository.go` (`List`) | `case ports.ViewRegistry: where = append(where, inRegistry)` y en `default: where = append(where, "NOT "+inRegistry)`. `inRegistry` está en `postgres/registry.go` | Si cambiaste el filtro de bandeja o registro, aplica la regla nueva (sección 2.1, 38cd930). `GET /v1/orders?view=registro` sigue funcionando |
| `orders-service/internal/application/dispatch_anyway.go` | `canAuthorizeIncomplete` → `canDispatch(id)` y el texto del 403 (bbd71a9) | Mantén el permiso Despacho. El aviso `despacho_incompleto` de `notices.go` no cambió |

### 2.4 Antes de la integración (ya están en tu rama desde `test`)

| Commit | Qué hizo (backend) | Archivos principales |
|---|---|---|
| aa567d8 | Cortes multicourier, `GET /v1/cutoffs`, `/v1/ops-settings/assignment-deadline`, `assignmentSituation` en `view=asignacion`, `packagesCount` en el listado, `manifestId` en los bultos. Core: `/v1/me/notes` | `internal/application/cutoffs.go`, `ops_settings.go`, `postgres/cutoffs.go`, `ops_settings.go`; Core `auth_roles/.../notes.go` |
| eac4273 | Bandeja y registro según CU-06; filtros `source` y `situation` con total correcto; se quita `arrivalTime` de los ajustes de courier (la llegada vive en los cortes) | `postgres/repository.go`, `rules.go`, `application/rules.go` |
| c739de0 | Plantilla de importación con hoja «Instrucciones» y listas desplegables; las filas `EJEMPLO-` no se importan; `itemsEditable` en la orden | `application/imports.go`, `platform/xlsx/xlsx.go` |
| 8167f4b | `POST /resolve` acepta `corrections` (dirección, comuna, región, courier). Cambiar el courier saca la orden del manifiesto abierto; con manifiesto cerrado → 422. También aplica a `PATCH` | `application/service.go`, `domain/order.go` |
| e3dd3c2 | Corte duplicado o posterior a la hora límite → 422. `GET /v1/orders/warehouses` | `application/cutoffs.go`, `ops_settings.go` |
| f99c7b6 | El motor ordena la cola de Invas por próximo corte, prioridad y antigüedad; retiene lo que pasa el último corte; no asigna tras la hora límite ni en días sin operación | `application/rules.go`, `domain/schedule.go`, `workers/outbox_relay.go` |
| 29ee98a | Cola de empaque ordenada por prioridad y `created_at`, `claimedAt`, toma y liberación | `application/fulfillment.go`, `postgres/fulfillment.go` |
| c3d5fe3 | Seed de desarrollo separado (`infrastructure/dev-seed/seed_dev.sql`, servicio `seed-dev`); cuentas demo según `NEXT_PUBLIC_SHOW_DEMO_ACCOUNTS` | Core `migrations/`, `docker-compose.override.yml`, `Makefile` |
| 293ff1f | Estrategia de etiqueta por courier: por API (simulada) queda pendiente y se reintenta sola, con aviso por Pub/Sub; Recíbelo usa «Buscar en oficina» y máximo 1 bulto | `application/labels.go`, `domain/label.go`, `courier/simulated.go`, `postgres/integrations.go` |
| 8ad872d | `POST /v1/ws-ticket`: ticket de un uso, 30 s, en Redis (solo el hash), consumido con `GETDEL` en `/ws?ticket=`. Ya no acepta `?access_token=` | `inbound/http/wsticket.go`, `shared/tenant/tenant.go` |
| ba6117a | Variables del BFF en `.env.example` y `docker-compose.yml` (`CORE_INTERNAL_URL`, `ORDERS_INTERNAL_URL`) | `.env.example`, `docker-compose.yml` |

---

## 3. Pendientes de BACKEND (por prioridad)

### Alta

| # | Qué falta | Qué espera el front | TODO en el front | Qué pasa hoy |
|---|---|---|---|---|
| A1 | Regla «sin etiqueta no se entrega» en `POST /v1/dispatch/manifests` y `/close` | 422 si la orden tiene `label.status` en `none`, `pending` o `failed`, salvo los couriers «Buscar en oficina» (Recíbelo) | Bloqueo de fila en `frontend-web/src/features/dispatch/components/PickupList.tsx:75` (sin TODO) | Solo el front bloquea la fila. `Service.CreateManifest` (`internal/application/fulfillment.go:345`) no revisa la etiqueta |
| A2 | Alcance en `GET /v1/dispatch/manifests` | `?status=&courier=` filtrado por las empresas del token (RF-03), con paginación o un límite razonable | `frontend-web/src/services/dispatch.ts:37` | El front filtra por courier. El servidor devuelve hasta 50 manifiestos completos de todas las empresas (`fulfillment.go:394`) |
| A3 | Empresa del transporte como campo de `/close` | `POST /v1/dispatch/manifests/{id}/close` con `transportCompany: string`, guardado en `manifests` y expuesto en `activity_log.details` | `frontend-web/src/features/dispatch/manifest.ts:32`; backend `orders-service/internal/application/activity_text.go:59` | Va como primera línea de `notes` («Empresa del transporte: X») y `/v1/activity` la recupera parseando ese texto |
| A4 | Observación por orden en `/close` | `observations: [{orderId, text}]`, guardada por línea de manifiesto o como evento en `order_events.details` | `frontend-web/src/features/dispatch/manifest.ts:29` | Va en `notes`, una línea por orden: «Observación · #ref · texto» |
| A5 | `TODO(PV-21)`: bodegas por usuario (`user_warehouses`) | El Jefe solo ve las alertas, incidencias, bandejas y acciones de sus bodegas; Admin sin límite; Cliente por empresa | `frontend-web/src/state/auth.tsx:130`, `frontend-web/src/state/menuAccess.ts:3`; backend `orders-service/internal/application/activity.go:15` | El Jefe ve todas las bodegas |

### Media

| # | Qué falta | Qué espera el front | TODO en el front | Qué pasa hoy |
|---|---|---|---|---|
| M1 | Códigos de bulto en la Lista para retiro | `GET /v1/dispatch/couriers/{courier}/orders` → `[{id, externalRef, companyId, recipientName, commune, labelCode, labelStatus, packages:[{code, scanned}]}]` | `frontend-web/src/features/dispatch/hooks/usePickupList.ts:32` | El front pide `GET /v1/orders/{id}` por cada orden (N+1) |
| M2 | Escaneo antes del manifiesto | `POST /v1/dispatch/scan {code}` sin manifiesto, o que `/scan` acepte órdenes en `ready_pickup` | Marcas locales en `frontend-web/src/features/dispatch/pickupList.ts:59` | El front guarda las lecturas en `sessionStorage` y las reenvía a `/scan` al generar el manifiesto |
| M3 | Bulto o producto en la incidencia | `POST /v1/orders/{id}/incident` con `packageCode?` y `sku?` | `frontend-web/src/features/dispatch/components/PickupOrderModal.tsx:43` | Va dentro de `reason` como texto |
| M4 | Aviso de entrega | Notificación `kind: "delivered"` con `orderId`, `externalRef` y `manifestId` al entregar | `frontend-web/src/hooks/useNotificationFeed.ts:45` | La campana lo arma con `order.changed` (`status=handed_over`) y pide la referencia con `GET /v1/orders/{id}` |
| M5 | Aviso en vivo de cambios del manifiesto | Mensaje WS `{"type":"manifest.changed", manifestId, orderId?, code?}` al escanear o cerrar | — | Un escaneo desde otro dispositivo se ve recién al recargar |
| M6 | Perfil del actor al momento de la acción | `order_events`, `manifests` y `user_audit` guardan `actor_role` (o `role` en `details`) | — (ver 6.1) | `/v1/activity` y `/v1/orders/registry` muestran el perfil **actual** |
| M7 | Configuración y catálogos en el registro de acciones | Eventos de cortes, horario, hora límite, reglas de ola (hoy se borran en físico), couriers, integraciones, productos, kits y empresas, con `{from, to}` en `details`; un nuevo `module` en `activity_log` | — | Esos cambios no dejan rastro de quién ni cuándo |
| M8 | Filtrar por permiso los avisos internos del WebSocket (Tarea 07) | Que el aviso `kind: profile_request` (solicitud de perfil nueva) llegue solo a quien tiene el permiso `gestionar_perfiles`. Por ejemplo, un campo `permission` en el mensaje que `ws` (`orders-service/internal/adapters/inbound/http/handler.go`) compare con `id.Permissions` antes de reenviarlo | `frontend-web/src/hooks/useNotificationFeed.ts` (`requestNotice`) | El Core publica el aviso con `audience: internal`: le llega por WebSocket a todo el personal interno de esas empresas (Jefe, Administrador sin el permiso) y el front lo oculta |

### Baja

| # | Qué falta | Qué espera el front | TODO en el front | Qué pasa hoy |
|---|---|---|---|---|
| B1 | Productos por bulto | `packages:[{code, items:[{sku, quantity}]}]` en `GET /v1/orders/{id}` y en el manifiesto | `frontend-web/src/features/dispatch/components/ManifestAccordions.tsx:56`, `PickupOrderModal.tsx:71` | El acordeón «Bultos» muestra todos los productos de la orden en cada bulto |
| B2 | Courier anterior | `previousCourier` en la orden | `frontend-web/src/features/dispatch/components/PickupOrderModal.tsx:59` | El chip «Cambiado de {courier}» no se muestra; solo queda el texto en el historial |
| B3 | Campana persistente | El front cargará `GET /v1/notifications?unread=true` al iniciar (el endpoint ya existe) | `frontend-web/src/hooks/useNotificationFeed.ts:9` | Avisos solo en memoria (últimos 20); recargar los borra. Es un cambio del front, no del back |
| B4 | Incidencias en la respuesta de `/close` | `removed:[{orderId, reason}]` | — | El resumen pide `GET /v1/orders/{id}` de cada una para leer el motivo |
| B5 | Quitar o agregar órdenes a un manifiesto abierto | `DELETE /v1/dispatch/manifests/{id}/orders/{orderId}` y `POST …/orders` | — | Con un manifiesto abierto hay que confirmarlo antes de generar otro |
| B6 | Estado de seguimiento normalizado | `tracking.events[].statusLabel` en español (o un código común) | — | Se muestra el `status` crudo del courier más su descripción |

---

## 4. Endpoints y front

### 4.1 Existen y el front no los usa

| Endpoint | Comentario |
|---|---|
| `GET /v1/notifications`, `POST /v1/notifications/read` | Ver B3 |
| `GET /v1/orders/{id}/tracking` | El front usa `tracking` dentro de `GET /v1/orders/{id}` (`handler.go:505`) |
| `summary` de `GET /v1/dispatch/manifests/{id}` | El front calcula el resumen en `features/dispatch/components/ManifestModal.tsx`. Se puede cambiar para usar el del servidor |
| `POST /v1/waves/run-rules` | Sin botón en la UI |
| `GET/POST /v1/integrations…` (rotate-secret, enable, disable) | No hay pantalla de integraciones en Órdenes |
| `GET /v1/orders?view=registro` | El front usa `GET /v1/orders/registry`; `view=registro` queda por compatibilidad |
| Core `GET/POST /v1/me/notes`, `PATCH/DELETE /v1/me/notes/{id}` | Recordatorios del Home; el Home todavía no los muestra |

### 4.2 El front depende de algo que no existe

| Llamada o dato del front | Falta | Sección |
|---|---|---|
| `closeNotes()` en `features/dispatch/manifest.ts:33` | Campos `transportCompany` y `observations` en `/close` | A3, A4 |
| Lecturas antes del manifiesto (`features/dispatch/pickupList.ts`) | Escaneo sin manifiesto | M2 |
| Bultos de la lista de retiro (`usePickupList.ts:32`) | Endpoint por courier con bultos | M1 |
| Aviso «entregada» (`useNotificationFeed.ts:45`) | `kind: delivered` | M4 |
| Productos por bulto (`ManifestAccordions.tsx:56`) | `packages[].items` | B1 |

---

## 5. Frontend terminado, por pantalla

| Pantalla (ruta) | Qué hace | Endpoints |
|---|---|---|
| Todas las órdenes (`/ordenes/todas/listado`) | KPI como filtro rápido, búsqueda, Filtros (modal centrado), tabla paginada, detalle a la derecha (modal bajo 1280 px), Nueva orden, Editar, Cancelar, Resolver anulación o incidencia, prioridad, documentos, Excel (importar, plantilla y exportar), historial de estados con «Ver todo» horizontal, seguimiento. `/ordenes/[id]` redirige aquí con `?order=` (o al registro) | `GET /v1/orders`, `/summary`, `/export`, `/warehouses`, `/meta`; `POST /v1/orders`, `/import`, `GET /import/template`; `GET/PATCH /v1/orders/{id}`; `POST …/cancel`, `/recover`, `/resolve`, `/priority`; `…/documents`. Core: `/v1/clients`, `/v1/products`, `/v1/catalogs/{name}` |
| Asignación (`/ordenes/todas/asignacion`) | Tarjetas de cortes, chips de situación, Configurar cortes, horario, hora límite y reglas de ola; Asignar a ola de Invas; prioridad | `GET /v1/orders?view=asignacion&situation=`; `/v1/cutoffs` (CRUD); `/v1/ops-settings/operation` y `/assignment-deadline`; `/v1/couriers/settings` y `/{code}/settings`; `GET /v1/waves`, `POST /v1/waves/assign`; `/v1/wave-rules` (CRUD y `/reorder`) |
| Empaque (`/ordenes/empaque`, `/ordenes/empaque/[id]`) | Cola «Por empacar / Mi empaque», tomar y soltar, lector de bultos, registrar bultos, etiqueta e impresión | `GET /v1/packing/queue`; `POST /v1/orders/{id}/packing/take`, `/release`, `/confirm`; `PUT /v1/orders/{id}/packages`; `POST /v1/orders/{id}/label` |
| Retiro y entrega (`/ordenes/retiro`) | Lista por courier con lector, detalle e «Informar problema», manifiesto en modal (acordeones Órdenes y Bultos, productos completos), «Despachar igual», «Agregar observación», «Confirmar entrega» (Quien retira: nombre conductor, RUT, patente; empresa del transporte; notas) y resumen | `GET /v1/dispatch/couriers`; `GET /v1/orders?status=ready_pickup&courier=`; `GET/POST /v1/dispatch/manifests`; `…/{id}/scan`; `…/{id}/close` (con `dispatchAnyway`); `POST /v1/orders/{id}/incident` |
| Registro órdenes (`/ordenes/registro`) | Tabla con filtros en la URL, detalle con «Historial de eventos», Excel acciones (seleccionadas, por filtros, masivo). Cliente: solo su empresa | `GET /v1/orders/registry`, `/registry/export`; `GET /v1/orders/{id}` (historial) |
| Registro de acciones (`/ordenes/registro?tab=acciones`) | Solo Admin y Jefe. Chips Órdenes · Empaque · Retiro y entrega · Usuarios; filtros (por defecto 7 días), fila expandible, enlaces a la orden o al manifiesto, Exportar registro | `GET /v1/activity`, `/v1/activity/export` |
| Campana y seguimiento (todas las pantallas) | Avisos en vivo (incidencia, etiqueta disponible, despacho incompleto, entrega), últimos 20 en memoria; sección «Seguimiento» del detalle | `POST /v1/ws-ticket`, `GET /ws?ticket=`; `GET /v1/orders/{id}` (`tracking`) |

**Común:**
- Todo pasa por el BFF de Next (`/bff/orders/...`, `/bff/core/...`) con la sesión de Auth.js.
- Los modales comparten un bloqueo de scroll (`frontend-web/src/hooks/scrollLock.ts`).
- El modal de filtros es `frontend-web/src/components/FiltersModal.tsx`.

---

## 6. Limitaciones conocidas

1. **Perfil actual, no el histórico.** `closedBy.role` (registro) y `actor.role` (acciones) salen del perfil actual del usuario, porque ningún evento guarda el del momento. Los actores sin `user:{id}` (datos antiguos o del seed) salen sin perfil. Ver M6.
2. **Notas del manifiesto.** La empresa del transporte y las observaciones por orden viajan como texto en `manifests.notes`. «Despachar igual» ya no va en notas: viaja en `dispatchAnyway` y queda en `order_events.details`. Ver A3 y A4.
3. **Configuración fuera del registro.** Cortes, horario, hora límite, reglas de ola, couriers, integraciones y catálogos del Core no dejan rastro (M7). Inicio y cierre de sesión quedan en Keycloak.
4. **PV-21.** No existe `user_warehouses`: el Jefe ve todas las bodegas (A5).
5. **Sin empresa.** Los manifiestos y los usuarios no tienen empresa: un filtro `company_id` explícito en `/v1/activity` los excluye.
6. **Retención.** `/v1/activity` no devuelve nada de más de 1 año (no borra nada).

---

## 7. Cómo verificar

Desde `NexsusWST/`, con Docker. En Windows sin `make`, usa el comando de cada objetivo del `Makefile`.

| Paso | Comando | Qué esperar |
|---|---|---|
| Levantar y migrar | `make up` (corre `migrate-core`, `migrate-orders` y `seed-dev`) | `schema_migrations_orders` en **10** y `schema_migrations_core` en **11** (desde la Tarea 07 de Jean), ambos con `dirty = false` |
| Ver versiones | `make psql`, luego `select version, dirty from schema_migrations_orders; select version, dirty from schema_migrations_core;` | `10 f` y `11 f` |
| Recargar datos ficticios | `make seed-dev` | Empresas, perfiles y productos de `infrastructure/dev-seed/seed_dev.sql` |
| Tema de Keycloak | `make theme`, después `docker compose restart keycloak` | `infrastructure/keycloak/theme/dist_keycloak/nexuswst-keycloak-theme.jar` regenerado |
| Go | `make test-orders` y `make test-core` | Todo `ok`. Las pruebas de postgres (registro y actividad) necesitan `TEST_DATABASE_URL`; sin ella se saltan (`t.Skip`) |
| Front: unitarias | `cd frontend-web && npm test` | 53 pruebas en 8 archivos |
| Front: calidad | `npm run typecheck`, `npm run lint`, `npm run build` | Sin errores. Lint deja 1 advertencia ya conocida (`<img>` en `Sidebar.tsx`) |
| Front: e2e | `npm run test:e2e` (Playwright con el Chrome instalado; `E2E_BASE` por defecto `http://localhost`) | 35 pruebas en 6 archivos |

**Ojo con los e2e:**
- `registro.spec.ts`, `retiro.spec.ts` y `ordenes-toques.spec.ts` necesitan el **seed de demostración de Jean**. Son 46 órdenes, entre ellas las envejecidas `48257769242`–`48257769245`.
- Ese seed vive solo en su máquina (`respaldo/seed/seed_demo.sql`, que no está en git) y no es `seed_dev.sql`. Pídeselo a Jean o salta esas specs.
- `retiro.spec.ts` consume datos (entrega órdenes), así que hay que recargar el seed antes de repetirla.
- Una corrida larga puede perder la sesión de vez en cuando; la spec afectada pasa sola. Es un problema conocido y no se investigó.

**Prueba rápida de los endpoints nuevos.** Con un Bearer de Jefe, sin imprimir tokens:

| Llamada | Esperado |
|---|---|
| `GET /api/orders/v1/orders/registry?limit=5` | `200 {data, total, limit, offset}` |
| `GET /api/orders/v1/activity?module=dispatch` | `200 {items, total, limit, offset}` |
| `GET /api/orders/v1/activity/export` (sin `module`) | `422` |
| `GET /api/orders/v1/activity` como Empaque o Cliente | `403` |
