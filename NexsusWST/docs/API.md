# NexusWST — Contrato de la API (v1.1)

Base desde el navegador (siempre a través de Traefik, mismo origen):

| Servicio | Prefijo |
| --- | --- |
| Keycloak (login, token) | `/auth/realms/nexuswst/...` |
| Core | `/api/core` |
| Órdenes | `/api/orders` |
| Webhooks de canales y couriers | `/webhooks/{proveedor}/{idIntegración}` |

## Autenticación (DAS v1.1 §4.5, PV-18)

- La interfaz inicia sesión en **Keycloak** con OIDC **código + PKCE** (cliente público `nexus-web`). El token de acceso dura **10 min**; se renueva con `refresh_token` (sesión de 12 h, se cierra tras 2 h sin uso).
- Cada petición lleva `Authorization: Bearer <access_token>`. Core y Órdenes verifican el token con las claves públicas de Keycloak (RS256) y leen el **perfil de Nexus** en PostgreSQL en cada petición.
- El WebSocket se abre con un ticket: `POST /v1/ws-ticket` (con Bearer) devuelve `{ticket, expiresIn: 30}`. Sirve una sola vez y cada reconexión pide uno nuevo. `?access_token=` ya no se acepta.
- La interfaz web no maneja tokens: llama a `/bff/{core|orders}/...` de Next, que agrega el Bearer en el servidor. `/api/core` y `/api/orders` siguen disponibles con Bearer para Postman y curl.
- Solo en desarrollo (`KC_WEB_DIRECT_GRANTS=true`) se puede pedir un token con usuario y contraseña, para Postman o curl:

```bash
curl -s -X POST http://localhost/auth/realms/nexuswst/protocol/openid-connect/token \
  -d grant_type=password -d client_id=nexus-web -d username=jefe@nexus.local -d 'password=Nexus2026!'
```

### Errores comunes

| HTTP | `error` | Significado |
| --- | --- | --- |
| 401 | `unauthorized` / `token_expired` | Falta el token, es inválido o venció: renovar o volver a iniciar sesión |
| 401 | `user_disabled` | El perfil está deshabilitado (bloqueo inmediato aunque el token siga vigente) |
| 403 | `profile_missing` | Existe en Keycloak pero aún no tiene perfil: llamar primero a `GET /api/core/v1/auth/me` |
| 403 | `company_disabled` | Todas las empresas del usuario están deshabilitadas |
| 403 | `forbidden` | Rol, permiso o empresa no autorizados |
| 404 | `not_found` | No existe **o pertenece a otra empresa** (no se revela cuál) |
| 409 | `conflict` / `invalid_transition` / `not_editable` / `duplicate` | Otra persona ya la cambió, el estado no lo permite o está duplicada |
| 422 | `validation_error` | Datos inválidos; viene `fields: [{field, message}]` |
| 500 | `internal_error` | Viene `ref`: referencia de diagnóstico para buscar en los logs |

Las respuestas de error del Core traen `ref` (igual a la cabecera `X-Request-Id`). En un 500, el mensaje pide informar esa referencia y la causa técnica queda en el registro del servidor con el mismo `request_id`, nunca en la respuesta (H-15-T04). Si Traefik ya envía `X-Request-Id`, se conserva; una referencia con caracteres raros o de más de 64 se reemplaza.

Toda ruta que trae datos acepta `?company_id=N` para acotar a una empresa (se contrasta con el perfil).

---

## Core (`/api/core`)

### Sesión

| Método | Ruta | Quién | Descripción |
| --- | --- | --- | --- |
| GET | `/v1/auth/me` | todos | Perfil: `id, email, name, role, permissions, companies, allCompanies, modules[{module, actions}]`. **Crea el perfil en el primer ingreso** de un usuario creado en la consola de Keycloak. |
| GET | `/v1/roles` | todos | Roles base y permisos (`despacho`, `gestionar_perfiles`) |

### Usuarios (H-02, CU-10) — consulta: Administrador · cambios: permiso **Gestionar perfiles**

Desde la Tarea 07 (Matriz de perfiles v1.2), los `GET` siguen abiertos al perfil Administrador (con o sin el permiso: «Lectura»), y **todas las escrituras** (`POST /v1/users`, `PATCH`, `disable`, `enable`, `password-email` y `password-reset`) exigen el **permiso `gestionar_perfiles`**, no el perfil. Sin él responden **403** `forbidden`: «Necesitas el permiso Gestionar perfiles para crear o modificar perfiles, empresas o solicitudes.». Se valida por permiso para que, a futuro, KAM y SAC reciban un permiso parecido limitado a perfiles con rol Cliente (`TODO(futuro)` en `tenant.CanManageProfiles`).

| Método | Ruta | Cuerpo / filtros | Descripción |
| --- | --- | --- | --- |
| GET | `/v1/users` | `?q=&role=&active=true\|false&limit=&offset=` | Listado `{items, total}` |
| POST | `/v1/users` | `{email, name, role, permissions[], companies[], allCompanies}` o, sin correo, `{username, name, role, …}` | Con correo: crea en Keycloak con su rol y le envía el **correo para definir la contraseña** (409 `email_taken`). Sin correo (H-02-T09; personal interno o perfil Cliente): crea el usuario con una **contraseña temporal** que viene una sola vez en `temporaryPassword` (409 `username_taken`). |
| GET | `/v1/users/{id}` | | Detalle |
| PATCH | `/v1/users/{id}` | cualquiera de `{name, role, permissions, companies, allCompanies}` | Edita. Nadie cambia **su propio** rol, permisos ni empresas (403). |
| POST | `/v1/users/{id}/disable` | `{reason}` | Deshabilita en Keycloak, cierra sus sesiones y bloquea su siguiente petición. Lo que tenía tomado en empaque vuelve a la bandeja. |
| POST | `/v1/users/{id}/enable` | `{reason}` | Rehabilita |
| POST | `/v1/users/{id}/password-email` | | Reenvía el correo para definir la contraseña (422 si el usuario no tiene correo) |
| POST | `/v1/users/{id}/password-reset` | | `{temporaryPassword}`: contraseña temporal nueva que el usuario cambia al entrar; cierra sus sesiones. Para quien no tiene correo (también sirve si perdió su correo). No aplica a uno mismo (422). |
| GET | `/v1/users/{id}/history` | | Auditoría: quién cambió qué y cuándo |

**Usuarios sin correo (H-02-T09).** Personal de bodega o perfil Cliente que no tiene correo (el Cliente, desde la Tarea 07): el usuario es de 3 a 40 caracteres `[a-z0-9._-]`, empieza con letra o número y no lleva `@`. El rol y las empresas los asigna el administrador como a cualquiera. El usuario (vista) trae `username`, `email` (vacío) y `hasEmail: false`; la interfaz muestra «Restablecer contraseña» en vez de «Reenviar correo». `temporaryPassword` solo viene en la respuesta de alta o de restablecimiento (con `Cache-Control: no-store`), no se guarda en Nexus ni en la auditoría. En el login, el campo «Correo o usuario» acepta ambos.

Reglas: un `cliente` necesita al menos una empresa y no puede tener `allCompanies` ni permisos internos (`despacho`, `gestionar_perfiles`).

### Empresas cliente (Tarea 07, RF-04, H-03) — consulta: Administrador o permiso **Gestionar perfiles** · cambios: permiso **Gestionar perfiles**

Clientes › Administración clientes › Empresas. `GET /v1/clients` (abajo, en Ola por cliente) sigue siendo la lista corta para los formularios de órdenes.

| Método | Ruta | Cuerpo / filtros | Descripción |
| --- | --- | --- | --- |
| GET | `/v1/companies` | `?q=&status=active\|inactive&wave=general\|{código}&from=&to=&ids=1,2&limit=&offset=` | `{data, total, limit, offset}`. `q` busca por nombre o RUT (con o sin puntos). `from` y `to` (`AAAA-MM-DD`) son sobre la fecha de creación, en hora de Chile. `limit` de 1 a 200; por defecto 20. |
| GET | `/v1/companies/export` | mismos filtros | Excel «Empresas»: Nombre empresa, RUT, Correo, Teléfono, Ola, N° perfiles, Estado, Fecha de creación (+ Código Invas para el Administrador). Máximo 10.000 filas (422 `too_many_rows`). |
| GET | `/v1/companies/{id}` | | Detalle |
| POST | `/v1/companies` | `{name, taxId, email, phone, waveCode, wmsCode}` | Crea (201). |
| PATCH | `/v1/companies/{id}` | el mismo cuerpo, con todos los campos | Edita. Solo se registra lo que cambió. |
| POST | `/v1/companies/{id}/disable` | `{reason}` (obligatorio, de 5 a 500 caracteres) | Desactiva. Sus usuarios dejan de operar en su siguiente petición (403 `company_disabled` para quien no tenga otra empresa activa); el historial se conserva (H-03, criterio 3). 409 `same_state` si ya estaba inactiva. |
| POST | `/v1/companies/{id}/enable` | `{reason}` opcional | Activa. |

Empresa (respuesta): `{id, name, taxId, email, phone, waveCode, profilesCount, active, createdAt, updatedAt}`, más `wmsCode` solo para el perfil Administrador.

- **Lo que exige la base (Core 000011):** `name` obligatorio (máx. 120); `taxId` obligatorio y **único sin importar puntos, guion ni mayúsculas** (409 `tax_id_taken` con `fields:[{field:"taxId"}]`); `email` y `phone` opcionales con formato.
- **RUT:** al crear, o al editarlo, se valida el dígito verificador (módulo 11) y se guarda como `76.123.456-0`. Si al editar no se cambia el RUT, no se vuelve a validar (empresas antiguas con otro formato).
- **Ola (PV-19):** `waveCode: null` es «General (según reglas)»: sin ola fija, el motor usa las reglas. La interfaz obliga a elegir una opción, pero la base acepta NULL. Un código va en mayúsculas, `^[A-Z0-9_-]{1,20}$`.
- **Código Invas (`wmsCode`):** opcional; solo lo ve y lo cambia el perfil Administrador (para otro perfil con el permiso se conserva el valor actual).
- **422 por campo:** `{error:"validation_error", message, fields:[{field, message}]}` con `field` en `name | taxId | email | phone | waveCode | wmsCode | reason`.
- **Registro:** cada cambio queda en `user_audit` (`target_company_id`) con `action` `company_created | company_updated | company_disabled | company_enabled` y `detail` con el antes y el después (`{campo: {from, to}}`) y el motivo. Aparece en `GET /api/orders/v1/activity?module=users` como entidad `company` («Empresa creada», «Empresa desactivada»…).

### Perfiles de clientes (Tarea 07, RF-02, H-02) — consulta: Administrador o permiso **Gestionar perfiles** · cambios: permiso **Gestionar perfiles**

Clientes › Administración clientes › Perfiles. Todas las rutas aceptan `?scope=client` (por defecto). `scope=internal` (perfiles de WeStorage) responde 422 hasta Configuración › Usuarios WeStorage. Un perfil que no es del alcance responde 404.

| Método | Ruta | Cuerpo / filtros | Descripción |
| --- | --- | --- | --- |
| GET | `/v1/profiles` | `?q=&company_id=&status=active\|inactive&from=&to=&ids=&limit=&offset=` | `{data, total, limit, offset}`. `q` busca por nombre, correo, usuario o empresa. |
| GET | `/v1/profiles/export` | mismos filtros | Excel «Perfiles» (máx. 10.000 filas). |
| GET | `/v1/profiles/{id}` | | Detalle |
| POST | `/v1/profiles` | `{name, companyId, email}` o, sin correo, `{name, companyId, username}`; más `phone`, `jobTitle` | Crea el perfil con rol **Cliente fijo**, sin permisos y una empresa activa. Con correo, Keycloak envía el enlace para definir la contraseña. Sin correo, `temporaryPassword` viene una sola vez (`Cache-Control: no-store`). 409 con `fields` si el correo o el usuario ya existen. |
| PATCH | `/v1/profiles/{id}` | `{name, companyId, phone, jobTitle}` | Edita. El correo y el usuario no se editan. Una sola acción `updated` con el antes y el después. |
| POST | `/v1/profiles/{id}/disable` | `{reason}` obligatorio (5 a 500) | Keycloak lo deshabilita y cierra sus sesiones. 409 `same_state` si ya estaba inactivo. |
| POST | `/v1/profiles/{id}/enable` | `{reason}` opcional | Activa. |
| POST | `/v1/profiles/{id}/password` | | Con correo: `{emailSent: true}` (Keycloak envía el enlace). Sin correo: `{temporaryPassword}` una sola vez, y cierra sus sesiones. 422 si está inactivo o sin cuenta en Keycloak. |
| GET | `/v1/profiles/template` | | Plantilla `.xlsx`: hoja «Perfiles» (Nombre completo, RUT empresa, Correo, Usuario si no tiene correo, Teléfono, Cargo; la fila `EJEMPLO…` no se importa) y hoja «Instrucciones» con las empresas activas. |
| POST | `/v1/profiles/import` | multipart `file` (.xlsx, máx. 5 MB, 500 filas) | `{rows, created:[{row, id, name, login, temporaryPassword?}], errors:[{row, field?, message}]}`. Crea las filas válidas e informa las inválidas con su fila y motivo: sin nombre, RUT de empresa inexistente o desactivada, correo inválido, repetido en el archivo o ya registrado. |

Perfil (respuesta): `{id, name, email, username?, hasEmail, phone, jobTitle, role, permissions, companyId, companyName, active, linked, createdAt, lastLoginAt}`.

- **Registro:** `created`, `updated` (con `phone`, `jobTitle`, `companyName` en antes y después), `disabled` y `enabled` (con motivo), `password_reset` y `password_email_sent` quedan en `user_audit` del perfil. La carga masiva deja además, por empresa, `profiles_imported` `{created, rows, failed}`. Todo aparece en Registro de acciones › Usuarios.
- **`/v1/users`** sigue existiendo para la administración general (consulta: Administrador; cambios: permiso).

### Productos y kits (H-04, RF-06) — Administrador, Jefe de Operaciones y Cliente (su empresa)

| Método | Ruta | Cuerpo / filtros | Descripción |
| --- | --- | --- | --- |
| GET | `/v1/products` | `?search=&limit=&offset=&company_id=` | Listado `{data}` acotado a las empresas autorizadas |
| GET | `/v1/products/{id}` | | Detalle |
| POST | `/v1/products` | `{companyId, sku, description, type: simple\|kit, waveCode?}` | Crea. 409 si el SKU ya existe en la empresa. `waveCode` solo lo envían Administrador o Jefe (403 para el cliente) |
| PATCH | `/v1/products/{id}/wave` | `{waveCode: "AM" \| null}` | Ola propia del producto (PV-19). Solo Administrador y Jefe de Operaciones; `null` la quita |
| GET | `/v1/products/{id}/components` | | Composición del kit: `{kit, components: [{productId, sku, description, quantity}]}` |
| PUT | `/v1/products/{id}/components` | `{components: [{sku, quantity}]}` | **Reemplaza** la composición completa (transacción). Responde igual que el GET |

Reglas de la composición (SCRUM-100): solo para productos `kit` (409 `not_kit` si es simple); de 1 a 50 componentes; cada componente debe existir en la **misma empresa**, estar activo y ser `simple` (no se permiten kits dentro de kits); cantidad > 0 y sin SKU repetidos. Si algo no se cumple responde 422 `invalid_component` con el SKU y el motivo, y no se guarda nada. Un kit de otra empresa responde 404.

El kit **no tiene stock propio**: al validar una orden, Órdenes expande cada kit en sus componentes (cantidad del kit × cantidad del componente), consulta su stock y registra el comprometido sobre ellos. Una orden con un kit sin composición pasa a Incidencia con el motivo.

### Ola por cliente y por producto (PV-19, H-03-T07, H-04-T10, H-10-T11)

| Método | Ruta | Cuerpo | Descripción |
| --- | --- | --- | --- |
| GET | `/v1/clients` | | Cada empresa trae `waveCode` (ola por defecto o `null`) |
| PATCH | `/v1/clients/{id}/wave` | `{waveCode: "PM" \| null}` | Ola por defecto del cliente. Solo Administrador y Jefe de Operaciones |

El código es el de la ola en Invas (letras, números, `-` o `_`, máximo 20; se guarda en mayúsculas). Los productos también traen `waveCode`. Órdenes lo lee por las vistas `v_product_waves` y `v_company_waves`, sin llamar al Core.

Cómo decide el motor de asignación, antes de las reglas: **producto → cliente → reglas → sin regla que calce**. Si los productos de la orden que tienen ola coinciden en una sola, va a esa; si tienen olas distintas o ninguno tiene, va a la ola del cliente; si tampoco hay, decide la primera regla que calce; si no, queda Validada sin ola para el Jefe. Si la ola pedida no existe hoy en Invas, la orden no se asigna y queda un evento `no_wave` en su historial. Los cortes y el horario no cambian: deciden cuándo sale la orden.

### Catálogos (caché Redis + ETag)

`GET /v1/catalogs/{regions|couriers|communes|dispatch_types}`

---

## Órdenes (`/api/orders`)

### Estados

`pending` Pendiente · `validating` En validación · `validated` Validada · `picking` En preparación · `picked` Preparada · `packing` En empaque · `ready_pickup` Lista para retiro · `handed_over` Entregada al courier · `incident` Incidencia · `voided` Anulada · `cancelled` Cancelada. `GET /v1/meta` devuelve estados, tipos de incidencia y catálogos con sus etiquetas en español.

### Orden (respuesta)

Contiene estos campos:

- **Identificación:** `id, companyId, externalRef, channel, source (manual|bulk|webhook)`.
- **Courier:** `courier, courierCode`.
- **Destino:** `recipientName, recipientPhone, recipientEmail, addressLine, commune, regionCode, dispatchType, customerNotes`.
- **Estado:** `status, statusLabel, prevStatus, incidentKind, incidentLabel, note (motivo vigente)`.
- **Cola e Invas:** `priority, waveId, wmsRef`.
- **Empaque y control:** `claimedBy, claimedByName, claimedAt, editable, version, items[{sku, quantity}]`.
- **Etiqueta:** `label{strategy (api|office), status (none|pending|ready|failed|office), trackingCode, documentId, downloadPath, nextAttemptAt}`, `courierMaxPackages`. Sigue disponible con la orden Lista para retiro.
- **Fechas:** `createdAt, updatedAt, handedOverAt, cancelledAt`.

### Ingreso y consulta (H-05, H-08, CU-01, CU-08)

| Método | Ruta | Detalle |
| --- | --- | --- |
| GET | `/v1/orders` | Filtros: `status=a,b`, `view=` (vacío = bandeja: todo lo que **no** está en el Registro histórico · `registro` = Canceladas y órdenes con más de 1 año sin cambiar de estado, ver «Registro histórico» · `all`), `q` (n°, referencia, destinatario o código courier), `courier`, `channel`, `source=manual\|bulk\|webhook`, `situation=en_cola\|invas_agotado\|fuera_corte\|sin_regla` (valor desconocido: 422), `from`/`to` (AAAA-MM-DD), `priority=true`, `without_wave=true`, `incident_kind`, `ids=1,2`, `sort=recent\|oldest\|updated\|priority` (por defecto, lo más reciente primero; en `view=asignacion`, el orden de la cola: corte, prioridad y antigüedad), `limit` (≤200), `offset`. Respuesta `{data, total, limit, offset}` |
| GET | `/v1/orders/summary` | Conteo por estado de todas las órdenes del alcance; respeta `company_id` y no aplica los demás filtros |
| GET | `/v1/orders/export` | Mismos filtros; `format=xlsx` (defecto) o `csv`. Con `ids=` exporta **los seleccionados**. Máx. 10.000 filas |
| GET | `/v1/orders/warehouses` | Bodegas distintas que ya aparecen en órdenes del alcance. Solo Jefe/Admin. Sugerencias para las reglas de ola. |
| POST | `/v1/orders` | `{companyId, externalRef?, courier, recipientName, recipientPhone?, recipientEmail?, addressLine, commune, regionCode?, dispatchType?, customerNotes?, items[{sku, quantity}]}` → 201 Pendiente. Sin `externalRef` se genera `NX-…`. Roles: admin, jefe, cliente. |
| POST | `/v1/orders/import` | multipart: `file` (.xlsx o .csv, máx. 5 MB) + `companyId`. **201** todo creado · **207** parte con errores: `{ordersCreated, orderIds, totalRows, failedRows, errors[{row, ref, message}]}`. Varias filas con la misma referencia = una orden con varios productos. |
| GET | `/v1/orders/import/template` | Plantilla `.xlsx` con hoja «Instrucciones», listas desplegables de couriers, tipos de despacho y comunas activas y filas de ejemplo; con `company_id` agrega los SKU activos de esa empresa. `?format=csv`: mismas columnas y ejemplos, sin listas. Las filas cuya referencia empieza con `EJEMPLO-` no se importan. |
| GET | `/v1/orders/{id}` | `{order, events[], packages[], documents[], tracking{available, events[]}}` |

### Registro histórico (Tarea 06, RF-28) — Administrador, Jefe de Operaciones y Cliente (su empresa)

Una orden entra al registro **Cancelada** (de inmediato) o cuando pasa **1 año desde su último cambio de estado** (cualquier estado; para las Entregadas cuenta `handed_over_at`). El último cambio es el último evento con `to_status` distinto de `from_status`; sin eventos, `created_at`. La bandeja (`GET /v1/orders` sin `view`) excluye exactamente esas órdenes. Empaque y Analista reciben 403; el Cliente ve solo su empresa (alcance del token, `company_id` ajeno = 403).

| Método | Ruta | Detalle |
| --- | --- | --- |
| GET | `/v1/orders/registry` | Filtros: `q` (n° de orden), `status=a,b`, `courier`, `company_id`, `from`/`to` (AAAA-MM-DD, sobre la fecha del cierre), `ids=1,2`, `limit` (1–200, defecto 20), `offset`. Respuesta `{data:[{id, externalRef, companyId, companyName, courier, courierName, status, statusLabel, previousStatus, previousStatusLabel, closedAt, closedBy:{id?, name, role?, kind: human\|system}, reason, motive, stale}], total, limit, offset}`. Orden: cierre más reciente primero |
| GET | `/v1/orders/registry/export` | Mismos filtros; `format=xlsx` (defecto) o `csv`. Columnas: N° orden, Cliente, Cerrada por, Perfil, Motivo, Estado, Estaba en, Courier, Fecha. Máx. 10.000 filas |

- `closedBy`: actor del último cambio de estado. `kind: system` para procesos (Invas, worker); `role` es el perfil **actual** del usuario (el evento no guarda el de ese momento) y falta en actores antiguos sin `user:{id}`.
- `previousStatus`: «Estaba en», el estado antes del cierre. `stale: true` si entró por tiempo y no por estar Cancelada.
- `motive`: para las que entran por tiempo, «Más de 1 año sin movimiento · {motivo original}»; para las Canceladas, el motivo sin el prefijo «Cancelada por {nombre} ({perfil}): » (quién ya va en `closedBy`).

### Registro de acciones (Tarea 06, RF-28) — solo Administrador y Jefe de Operaciones

Se lee de la vista `activity_log` (migración Órdenes `000009_activity_log`), que une `order_events` (órdenes, empaque, etiqueta y «Despachar igual»), la creación y el cierre de `manifests` y `user_audit` del Core. Solo lectura; retención de **1 año** (no devuelve nada más antiguo y no borra nada). Cliente, Empaque y Analista reciben 403. `TODO(PV-21)`: acotar al Jefe a sus bodegas.

| Método | Ruta | Detalle |
| --- | --- | --- |
| GET | `/v1/activity` | Filtros: `module=orders\|packing\|dispatch\|users` (otro valor: 422), `from`/`to` (AAAA-MM-DD), `actor` (id o parte del nombre), `role`, `kind=human\|system`, `q` (n° de orden, de manifiesto o nombre de usuario), `company_id` (solo esa empresa; sin él, el alcance del usuario más lo que no tiene empresa), `limit` (1–200, defecto 50), `offset`. Respuesta `{items:[{id, occurredAt, actor:{id?, name, role?, kind}, module, action, actionLabel, entity:{type: order\|manifest\|user, id, label}, companyId?, companyName?, warehouse?, reason?, details, statusChange?, courierName?, transportCompany?, observation?, detailLines[]}], total, limit, offset}`. Lo más reciente primero. `q=#12` busca el manifiesto 12 (creación, cierre y órdenes despachadas igual en él) |
| GET | `/v1/activity/export` | Mismos filtros; `module` obligatorio (422 sin él). Excel con las columnas del apartado: Órdenes (Fecha y hora, Actor, Perfil, Humano/Sistema, Acción, N° orden, Cliente, Estado anterior → nuevo, Motivo u observación, Detalle), Empaque (… Bultos, Código de etiqueta, Observación), Retiro y entrega (… N° manifiesto, Courier, Órdenes, Bultos, Quien retira, RUT, Patente, Empresa del transporte, Observación) y Usuarios (… Usuario afectado, Perfil afectado, Detalle). Máx. 10.000 filas |

- `action`: el evento de la orden (`create`, `validate`, `cancel`, `hand_over`, `start_packing`, `release_packing`, `register`, `packed`, `add`, …), `label_{estado}` y `print_label` (empaque), `dispatch_anyway`, `manifest_created`, `manifest_closed`, o la acción de `user_audit` (`created`, `updated`, `enabled`, `disabled`, `password_reset`, `password_email_sent`, `linked`, …).
- Textos listos para mostrar (los mismos del Excel): `actionLabel` (acción en español), `statusChange` («Validada → En preparación»), `courierName`, `transportCompany` y `observation` (de las notas del manifiesto: la primera línea «Empresa del transporte: …» y el resto) y `detailLines` (el antes y el después completo, una línea por dato, por ejemplo «Perfil: Empaque → Jefe de Operaciones»).
- `actor`: `kind: system` para el worker y los procesos (Invas, «Sistema»); `role` es el perfil **actual** del usuario (los eventos no guardan el de ese momento).
- `details` sale filtrado por una lista de campos permitidos: `fromStatus`, `toStatus`, `courier`, `labelCode`, `packages`, `orders`, `kind`, `manifestId`, `waveId`, `reason`, `authorizedBy`, `packagesTotal`, `scannedPackages`, `missingPackages`, `pickerName`, `pickerId`, `vehiclePlate`, `notes`, `name`, `role`, `permissions`, `companies`, `allCompanies`, `username`, `withoutEmail`, `temporary`, `source` y `targetRole`; un cambio viene como `{from, to}`. Nunca salen credenciales, secretos, tokens ni el `keycloakId`.
- Fuera de alcance (mejora futura): cambios de configuración (cortes, horario de operación, olas, couriers, integraciones) y catálogos del Core.

### Edición, anulación y cancelación (H-09, CU-06)

| Método | Ruta | Cuerpo | Notas |
| --- | --- | --- | --- |
| PATCH | `/v1/orders/{id}` | `{courier?, courierCode?, destination?{…}, dispatchType?, customerNotes?, items?, reason?}` | Solo antes de llegar a Invas (`editable`). Si cambian productos, destino o courier, se **valida de nuevo** (libera el comprometido). 409 `not_editable` en preparación. |
| POST | `/v1/orders/{id}/cancel` | `{reason}` (obligatorio) | Cancelada (terminal, oculta, no se borra). Si ya está en una ola de Invas pasa a **Incidencia «Cancelación por confirmar»** y el Jefe la confirma con `resolve` + `cancel`. El **cliente** solo cancela antes de la preparación (H-09-T07): con la orden ya en una ola de Invas recibe 409 `invalid_transition` y debe pedirlo a WeStorage; Jefe y Admin sí pueden. |
| POST | `/v1/orders/{id}/recover` | `{note}` | «Resolver anulación»: Anulada → Pendiente (Jefe/Admin) |

### Incidencias (H-11, CU-03)

| Método | Ruta | Cuerpo |
| --- | --- | --- |
| POST | `/v1/orders/{id}/incident` | `{kind, reason}` — `kind`: `datos, invas, diferencia_empaque, courier_cobertura, courier_bultos, cancelacion, retiro, integracion, otro` |
| POST | `/v1/orders/{id}/resolve` | `{action: resume\|retry\|cancel, note, corrections?{addressLine?, commune?, regionCode?, courier?}}` — vuelve al estado donde ocurrió; `retry` reencola el trabajo fallido. `corrections` solo con `resume`: se guardan junto con la resolución, aunque la orden ya esté en Invas o lista para retiro, y se validan comuna, courier activo, cobertura y máximo de bultos (422 por campo). Antes de llegar a Invas la orden vuelve a validarse. Si cambia el courier y la orden está en un manifiesto abierto, sale de él (línea `moved`); con el manifiesto cerrado responde 422. Solo Jefe/Admin. |

### Olas, prioridad y reglas (H-10, CU-02)

| Método | Ruta | Detalle |
| --- | --- | --- |
| POST | `/v1/orders/{id}/priority` | `{priority, reason}` «Enviar primero a la cola» |
| GET | `/v1/waves` | Olas de Invas `{id, code, name, type}` |
| POST | `/v1/waves/assign` | `{orderIds, waveId}` → 202 `{queued, skipped}` (asignar = encolar; pasa a En preparación cuando Invas confirma). Fuera de corte y en días sin operación se permite; después de la hora límite de un día de operación responde 422, salvo prioritarias con `priority_overrides_deadline` |
| POST | `/v1/waves/run-rules` | Aplica las reglas ahora |
| GET/POST | `/v1/wave-rules` | `{name, position?, active, courier?, companyId?, channel?, dispatchType?, warehouse?, waveCode}` — gana la primera que coincide; sin regla queda Validada sin ola |
| PUT/DELETE | `/v1/wave-rules/{id}` | |
| POST | `/v1/wave-rules/reorder` | `{ids:[…]}` en el orden deseado |
| GET | `/v1/couriers/settings` | Máximo de bultos y cobertura por courier |
| PUT | `/v1/couriers/{code}/settings` | `{maxPackages?, coversAll, communes[]}` — solo Jefe/Admin. La llegada vive en los cortes (`/v1/cutoffs`); enviar `arrivalTime` responde 400. |
| GET | `/v1/ops-settings/operation` | `{assignmentDeadline, operationStart, operationDays[], priorityOverridesDeadline, operationDay, beforeStart, afterDeadline, nextRelease}`. Solo Jefe/Admin. Los últimos cuatro describen el momento actual, en hora de America/Santiago. |
| PUT | `/v1/ops-settings/operation` | `{operationStart:"HH:MM", operationDays:["mon",…], priorityOverridesDeadline?}` — inicio anterior al primer corte y a la hora límite, y al menos un día (422). Solo Jefe/Admin. |

### Empaque (H-12, CU-04)

| Método | Ruta | Detalle |
| --- | --- | --- |
| GET | `/v1/packing/queue` | Preparadas + las que tengo En empaque. Jefe/Admin también reciben las tomadas por usuarios deshabilitados. Orden: prioridad y luego `created_at`. |
| POST | `/v1/orders/{id}/packing/take` | Preparada → En empaque. Si otra persona la tomó: 409 |
| POST | `/v1/orders/{id}/packing/release` | Devolver a la bandeja |
| PUT | `/v1/orders/{id}/packages` | `{packages:[{weightKg?, lengthCm?, widthCm?, heightCm?}]}` → códigos `NX-{orden}-{n}` |
| POST | `/v1/orders/{id}/packing/confirm` | `{items:[{sku, quantity}] contados, version}` → Lista para retiro, o Incidencia si no coincide (`discrepancies[]`) o si el courier no admite esa cantidad de bultos. Dos confirmaciones a la vez: la segunda recibe 409. |
| POST | `/v1/orders/{id}/label` | Etiqueta del courier (PDF, hoy **simulada**). Lista: 201 `{document, downloadPath, label}`. Pendiente: 202 `{label}`; se reintenta sola y avisa por WebSocket y notificación. Courier «Buscar en oficina»: 409. Si cambia la cantidad de bultos, la etiqueta queda sin efecto. |

### Documentos (H-05 esc. 3)

`GET /v1/orders/{id}/documents` · `POST /v1/orders/{id}/documents` (multipart `file` + `kind`: factura, boleta, guia_despacho, etiqueta, otro; PDF/PNG/JPG/XML, máx. 10 MB, se valida el contenido real) · `GET /v1/orders/{id}/documents/{docId}/download`

### Retiro y entrega (H-13, CU-05) — permiso **Despacho**

| Método | Ruta | Detalle |
| --- | --- | --- |
| GET | `/v1/dispatch/couriers` | Órdenes y bultos listos por courier |
| POST | `/v1/dispatch/manifests` | `{courier, orderIds?}` → manifiesto abierto. Las comunas sin cobertura salen y pasan a Incidencia (`excluded[]`). |
| GET | `/v1/dispatch/manifests[?status=open\|closed]`, `/v1/dispatch/manifests/{id}` | Detalle con líneas y bultos escaneados. El detalle trae `summary {orders, packages, scanned, companies[{id, name, orders, packages}]}` (H-13-T09); las líneas movidas o canceladas no cuentan |
| POST | `/v1/dispatch/manifests/{id}/scan` | `{code}` marca un bulto (manual o con lector) |
| POST | `/v1/dispatch/manifests/{id}/close` | `{pickerName, pickerId?, vehiclePlate?, notes?, dispatchAnyway?[{orderId, reason}]}` → `{delivered[], pending[], incomplete[]}`. Las órdenes con bultos faltantes no se entregan y quedan para el siguiente retiro, salvo las de `dispatchAnyway` («Despachar igual», H-13-T08). |

**«Despachar igual» (H-13-T08, PV-09).** Lo autoriza quien tenga el **permiso Despacho**, sin importar el perfil (403 sin el permiso), igual que para cerrar el manifiesto. Cada orden necesita un motivo de al menos 5 caracteres, debe estar incluida en el manifiesto, tener bultos sin escanear y al menos uno escaneado (si no, 422 y el manifiesto no se cierra). La orden pasa a «Entregada al courier» y queda en `incomplete[]`. En su historial (`GET /v1/orders/{id}`) el evento trae `details`:

```json
{"kind": "despacho_incompleto", "manifestId": 12, "courier": "starken", "waveId": "OLA-AM",
 "reason": "Bulto 2 dañado, se reenvía mañana", "authorizedBy": "user:2 Jefe",
 "packagesTotal": 3, "scannedPackages": ["NX-150-1", "NX-150-3"], "missingPackages": ["NX-150-2"]}
```

Además se crea un aviso interno (`kind: despacho_incompleto`) para el Jefe y el Admin en la campana y en vivo; el cliente no lo ve. Límite: los bultos no registran qué productos lleva cada uno, así que se informa qué bultos faltaron, no qué productos iban en ellos.

### Seguimiento (H-14)

`GET /v1/orders/{id}/tracking` → `{available, events[{status, description, occurredAt, provider}], message}`. Los eventos del courier se guardan tal como llegan y **no cambian** el estado interno.

### Integraciones (H-06, RF-05) — solo Administrador

| Método | Ruta | Detalle |
| --- | --- | --- |
| GET | `/v1/integrations` | Sin secretos |
| POST | `/v1/integrations` | `{companyId, provider, kind: channel\|courier, channel?, defaultCourier?}` → devuelve `secret` **una sola vez** y `webhookPath` |
| POST | `/v1/integrations/{id}/rotate-secret` · `/enable` · `/disable` | |

**Webhook de entrada** `POST /webhooks/{provider}/{id}` (sin JWT):

- Firma: `X-Nexus-Signature: <hex HMAC-SHA256 del cuerpo con el secreto>`, o token `X-Nexus-Token` / `?token=` para proveedores que no firman.
- Responde 200 de inmediato; se procesa por la cola.
- `generic` (canal): `{externalRef, courier?, recipientName, addressLine, commune, items[]}`.
- `mercadolibre`: `{topic, resource}`. El detalle se consulta por API; hoy está **simulado**.
- Courier: `{eventId, courierCode | orderRef, status, description, occurredAt}`.
- Un reenvío no duplica la orden y queda registrado en su historial.

### Avisos y tiempo real (H-15)

- `GET /v1/notifications?unread=true` → `{data, unread}` · `POST /v1/notifications/read {ids}`
- `POST /v1/ws-ticket` → `{ticket, expiresIn}` · `GET /ws?ticket=…` (WebSocket; ticket inválido, usado o vencido: 401). Mensajes:
  - `{"type":"order.changed", orderId, companyId, status, version, priority}`
  - `{"type":"notification", audience, notification}`
  - Al reconectar, volver a consultar la API.

### Cola de Invas y situación (regla de cortes)

Hora de operación: America/Santiago. El motor automático evalúa cada orden Validada sin ola:

1. Fuera de `operation_days` o antes de `operation_start`: no asigna.
2. Después de `assignment_deadline`: no asigna, salvo prioritarias con `priority_overrides_deadline=true`.
3. Pasado el último corte activo de su courier: queda retenida, salvo prioritarias. Un corte vale hasta su minuto exacto. Un courier sin cortes activos se asigna al final de la cola.

Orden: próximo corte del courier (el más temprano primero), prioridad y antigüedad (`created_at`). Una prioritaria fuera de corte va al inicio. Lo retenido se libera el próximo día de operación a `operation_start`, en ese mismo orden. `assignmentSituation` es `fuera_corte` mientras la orden está retenida por cualquiera de estas reglas. Los trabajos ya encolados terminan aunque pase la hora límite.
