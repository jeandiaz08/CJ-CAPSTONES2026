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
- El WebSocket recibe el token en `?access_token=`.
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

Toda ruta que trae datos acepta `?company_id=N` para acotar a una empresa (se contrasta con el perfil).

---

## Core (`/api/core`)

### Sesión

| Método | Ruta | Quién | Descripción |
| --- | --- | --- | --- |
| GET | `/v1/auth/me` | todos | Perfil: `id, email, name, role, permissions, companies, allCompanies, modules[{module, actions}]`. **Crea el perfil en el primer ingreso** de un usuario creado en la consola de Keycloak. |
| GET | `/v1/roles` | todos | Roles base y permisos (`despacho`) |

### Usuarios (H-02, CU-10) — solo Administrador

| Método | Ruta | Cuerpo / filtros | Descripción |
| --- | --- | --- | --- |
| GET | `/v1/users` | `?q=&role=&active=true\|false&limit=&offset=` | Listado `{items, total}` |
| POST | `/v1/users` | `{email, name, role, permissions[], companies[], allCompanies}` | Crea en Keycloak con su rol y le envía el **correo para definir la contraseña**. 409 si el correo existe. |
| GET | `/v1/users/{id}` | | Detalle |
| PATCH | `/v1/users/{id}` | cualquiera de `{name, role, permissions, companies, allCompanies}` | Edita. Nadie cambia **su propio** rol, permisos ni empresas (403). |
| POST | `/v1/users/{id}/disable` | `{reason}` | Deshabilita en Keycloak, cierra sus sesiones y bloquea su siguiente petición. Lo que tenía tomado en empaque vuelve a la bandeja. |
| POST | `/v1/users/{id}/enable` | `{reason}` | Rehabilita |
| POST | `/v1/users/{id}/password-email` | | Reenvía el correo para definir la contraseña |
| GET | `/v1/users/{id}/history` | | Auditoría: quién cambió qué y cuándo |

Reglas: un `cliente` necesita al menos una empresa y no puede tener `allCompanies` ni el permiso `despacho`.

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
| GET | `/v1/orders` | Filtros: `status=a,b`, `view=` (vacío = bandeja: órdenes en curso y Entregadas de hasta 365 días desde `handed_over_at`; nunca Canceladas · `registro` = Canceladas y Entregadas de más de 365 días · `all`), `q` (n°, referencia, destinatario o código courier), `courier`, `channel`, `source=manual\|bulk\|webhook`, `situation=en_cola\|invas_agotado\|fuera_corte\|sin_regla` (valor desconocido: 422), `from`/`to` (AAAA-MM-DD), `priority=true`, `without_wave=true`, `incident_kind`, `ids=1,2`, `sort=recent\|oldest\|updated\|priority` (por defecto, lo más reciente primero; en `view=asignacion`, el orden de la cola: corte, prioridad y antigüedad), `limit` (≤200), `offset`. Respuesta `{data, total, limit, offset}` |
| GET | `/v1/orders/summary` | Conteo por estado de todas las órdenes del alcance; respeta `company_id` y no aplica los demás filtros |
| GET | `/v1/orders/export` | Mismos filtros; `format=xlsx` (defecto) o `csv`. Con `ids=` exporta **los seleccionados**. Máx. 10.000 filas |
| GET | `/v1/orders/warehouses` | Bodegas distintas que ya aparecen en órdenes del alcance. Solo Jefe/Admin. Sugerencias para las reglas de ola. |
| POST | `/v1/orders` | `{companyId, externalRef?, courier, recipientName, recipientPhone?, recipientEmail?, addressLine, commune, regionCode?, dispatchType?, customerNotes?, items[{sku, quantity}]}` → 201 Pendiente. Sin `externalRef` se genera `NX-…`. Roles: admin, jefe, cliente. |
| POST | `/v1/orders/import` | multipart: `file` (.xlsx o .csv, máx. 5 MB) + `companyId`. **201** todo creado · **207** parte con errores: `{ordersCreated, orderIds, totalRows, failedRows, errors[{row, ref, message}]}`. Varias filas con la misma referencia = una orden con varios productos. |
| GET | `/v1/orders/import/template` | Plantilla `.xlsx` con hoja «Instrucciones», listas desplegables de couriers, tipos de despacho y comunas activas y filas de ejemplo; con `company_id` agrega los SKU activos de esa empresa. `?format=csv`: mismas columnas y ejemplos, sin listas. Las filas cuya referencia empieza con `EJEMPLO-` no se importan. |
| GET | `/v1/orders/{id}` | `{order, events[], packages[], documents[], tracking{available, events[]}}` |

### Edición, anulación y cancelación (H-09, CU-06)

| Método | Ruta | Cuerpo | Notas |
| --- | --- | --- | --- |
| PATCH | `/v1/orders/{id}` | `{courier?, courierCode?, destination?{…}, dispatchType?, customerNotes?, items?, reason?}` | Solo antes de llegar a Invas (`editable`). Si cambian productos, destino o courier, se **valida de nuevo** (libera el comprometido). 409 `not_editable` en preparación. |
| POST | `/v1/orders/{id}/cancel` | `{reason}` (obligatorio) | Cancelada (terminal, oculta, no se borra). Si ya está en una ola de Invas pasa a **Incidencia «Cancelación por confirmar»** y el Jefe la confirma con `resolve` + `cancel`. |
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
| GET | `/v1/dispatch/manifests[?status=open\|closed]`, `/v1/dispatch/manifests/{id}` | Detalle con líneas y bultos escaneados |
| POST | `/v1/dispatch/manifests/{id}/scan` | `{code}` marca un bulto (manual o con lector) |
| POST | `/v1/dispatch/manifests/{id}/close` | `{pickerName, pickerId?, vehiclePlate?, notes?}` → `{delivered[], pending[]}`. Las órdenes con bultos faltantes no se entregan y quedan para el siguiente retiro. |

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
- `GET /ws?access_token=…` (WebSocket). Mensajes:
  - `{"type":"order.changed", orderId, companyId, status, version, priority}`
  - `{"type":"notification", audience, notification}`
  - Al reconectar, volver a consultar la API.

### Cola de Invas y situación (regla de cortes)

Hora de operación: America/Santiago. El motor automático evalúa cada orden Validada sin ola:

1. Fuera de `operation_days` o antes de `operation_start`: no asigna.
2. Después de `assignment_deadline`: no asigna, salvo prioritarias con `priority_overrides_deadline=true`.
3. Pasado el último corte activo de su courier: queda retenida, salvo prioritarias. Un corte vale hasta su minuto exacto. Un courier sin cortes activos se asigna al final de la cola.

Orden: próximo corte del courier (el más temprano primero), prioridad y antigüedad (`created_at`). Una prioritaria fuera de corte va al inicio. Lo retenido se libera el próximo día de operación a `operation_start`, en ese mismo orden. `assignmentSituation` es `fuera_corte` mientras la orden está retenida por cualquiera de estas reglas. Los trabajos ya encolados terminan aunque pase la hora límite.
