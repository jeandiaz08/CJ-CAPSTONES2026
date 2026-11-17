# Correcciones del 17-11-2026: hallazgos de las capturas de evidencia

Al generar las capturas de pantalla para la evidencia del Capstone ([docs/pantallas/](../pantallas/README.md)) se revisó cada pantalla con su perfil y aparecieron siete hallazgos. Cinco se corrigieron y dos se dejaron como están a propósito.

- **Rama:** `jean_dev`.
- **Entorno:** stack de desarrollo con Docker Compose y el seed de demostración, con datos ficticios.

## 1. Hallazgos corregidos

| # | Pantalla | Síntoma | Causa | Corrección | Archivos tocados | Prueba que lo cubre |
|---|---|---|---|---|---|---|
| 1 | Empaque › verificación de productos (perfil Empaque) | Cada producto aparecía como «Producto sin descripción» y sin imagen, aunque en Productos sí tenía descripción (por ejemplo AO-1004, «Linterna frontal recargable») | El front cruza el SKU con `GET /api/core/v1/products` y el Core le respondía 403 al perfil Empaque. Las líneas de la orden no traen la descripción | El Core permite a Empaque **leer el listado** (`GET /v1/products`), con los mismos filtros y alcance por empresa. Nada más se abre (ver sección 5) | `backend-core/internal/modules/products/adapters/inbound/http/catalog.go`, `catalog_test.go` | Go: `TestCatalogAccess`, `TestCatalogEmpaqueCannotWrite`. E2E: `e2e/productos.spec.ts` › «permisos…» |
| 2 | Tablas con panel de detalle a 1440 px: Todas las órdenes, Asignación, Registro histórico, Administración clientes (Perfiles, Empresas, Solicitudes), Perfiles empresa, Usuarios WeStorage, Productos e Integraciones | La última columna quedaba cortada detrás del scroll horizontal (Estado, Ola, Permisos) | Anchos fijos de tabla (`min-width` de 850 a 1164 px) y celdas sin quiebre de línea, frente a los 684 px que deja el panel de detalle | Solo CSS para pantallas de 1280 px o más: la tabla ocupa el ancho disponible, con menos padding y texto en dos líneas (corta en espacios; los correos, en cualquier punto). Estados y números de orden no se cortan. Bajo 1280 px no cambia nada. Medido en las 11 tablas: el desborde pasó de 216–480 px a 0 | `frontend-web/src/app/globals.css` (bloque final) | E2E: `e2e/orders-table.spec.ts` (360, 390, 768 y 1024 px sin cambios). Verificación visual en las capturas de desktop |
| 3 | Detalle de orden › Historial de estados | El actor salía como «user:3 Empaque Demo» | El resumen del historial mostraba `actor` sin formatear | Pasa por `actorName()`, la misma función que ya usa el modal «Ver todo» | `frontend-web/src/features/orders/components/OrderDetail.tsx` | Visual (`docs/pantallas/02-ordenes/ordenes-detalle_desktop.png`) |
| 4 | Detalle de orden › Seguimiento | Estados con el código crudo del courier («picked_up · Retirado en bodega») | Se concatenaba `status` y `description` | Se muestra la descripción («Retirado en bodega»); el código solo si el courier no manda descripción. La columna Fecha sigue con scroll horizontal en el panel, a propósito | `frontend-web/src/features/orders/components/TrackingSection.tsx` | Vitest: `tests/notifications.test.jsx` › «Seguimiento en el detalle». E2E: `e2e/seguimiento-avisos.spec.ts` |
| 5 | Campana de avisos | «Orden #4511 (20006) en Incidencia» y «Otro. Otro: Producto dañado…» | El servidor armaba el título con el id interno y anteponía el tipo de incidencia a un motivo que ya lo traía | Título con el número de la orden («Orden #20006 en Incidencia»); también en «Etiqueta lista» y «Despacho incompleto», que comparten la referencia. En incidencias no se repite el tipo si el motivo ya lo trae | `orders-service/internal/application/notices.go`, `notices_test.go` (nuevo) | Go: `TestIncidentNoticeText`, `TestOrderRefFallback`. E2E: `e2e/seguimiento-avisos.spec.ts` › «campana…» |

## 2. Hallazgos revisados y no modificados a propósito

| # | Pantalla | Qué se vio | Por qué no se cambió |
|---|---|---|---|
| 6 | Detalle de orden › «Ver todo» | El texto del último paso visible queda cortado en el borde derecho del modal | Es el borde del scroll horizontal acordado para el historial completo, no un desborde. Lo cubre `e2e/ordenes-toques.spec.ts` («Ver todo» en horizontal) |
| 7 | Detalle de una orden Entregada | «Resolver incidencia», «Editar» y «Cancelar» aparecen deshabilitados | El patrón de pantalla muestra siempre las acciones y las deshabilita según el estado y el perfil |

## 3. Pruebas nuevas y actualizadas

**Nuevas**
- `TestCatalogEmpaqueCannotWrite` (Core, `catalog_test.go`): Empaque recibe 403 en `POST /v1/products`, `PATCH /v1/products/{id}`, `POST …/disable`, `POST …/enable`, `POST /v1/products/import` y `GET /v1/products/template`. Además verifica que el producto no cambió.
- `orders-service/internal/application/notices_test.go`:
  - `TestIncidentNoticeText` prueba el título «Orden #20006 en Incidencia» y el cuerpo en tres casos: motivo que ya trae el tipo, motivo sin el tipo y motivo vacío.
  - `TestOrderRefFallback` prueba que sin número externo se usa `#{id}`.

**Actualizadas**
- `TestCatalogAccess` (Core): Empaque ahora lista con 200, y `GET /v1/products/export` y `GET /v1/products/{id}` siguen en 403. Cambió porque la prueba fijaba la regla anterior (Empaque sin acceso).
- `e2e/productos.spec.ts` › «permisos…»: esperaba 403 de Empaque en `GET /v1/products`. Ahora espera 200 y comprueba 403 en crear, editar, desactivar, exportar y detalle.
- `tests/notifications.test.jsx` › «Seguimiento en el detalle»: esperaba «in_transit · En tránsito a Santiago». Ahora espera «En tránsito a Santiago» y, sin descripción, el código («delivered»).

## 4. Resultados de la corrida (17-11-2026, `jean_dev`)

| Prueba | Resultado |
|---|---|
| `go test ./...` en Core (contenedor `core`), con `gofmt` y `go vet` | Todo `ok` |
| `go test ./...` en orders-service (contenedor `orders`), con `gofmt` y `go vet` | Todo `ok` |
| Vitest (`npx vitest run`) | 124/124 en 17 archivos |
| E2E (`npm run test:e2e`, con el seed de demostración recargado por `global-setup`) | 78/78 en la primera corrida, sin reintentos |
| Typecheck (`npx tsc --noEmit`) | Sin errores |
| Lint (`npm run lint`) | 0 errores. Queda la advertencia conocida de `<img>` en `Sidebar.tsx` |

- El inicio de sesión intermitente de las e2e (**QA-08**) **no apareció** en esta corrida.
- No se corrió `npm run build`.
- Las pruebas de Go contra PostgreSQL real necesitan `TEST_DATABASE_URL`; sin ella se saltan, igual que antes.

## 5. Cambio de regla de negocio

**Empaque puede leer el listado de productos** (`GET /api/core/v1/products`), solo en lectura, para ver la descripción de cada SKU al contar en Empaque.

| Ruta | Empaque antes | Empaque ahora |
|---|---|---|
| `GET /v1/products` (listado) | 403 | **200** (mismos filtros y alcance por empresa) |
| `GET /v1/products/{id}` (detalle) | 403 | 403 |
| `GET /v1/products/export` | 403 | 403 |
| `GET /v1/products/template` | 403 | 403 |
| `POST /v1/products/import` | 403 | 403 |
| `POST`, `PATCH`, `disable`, `enable` | 403 | 403 |

El menú de Empaque no cambia: sigue sin la pantalla Productos. El contrato está en [docs/API.md](../API.md), sección Productos. La Matriz de perfiles pasa a v1.3 con este cambio; es un documento local, fuera del repositorio.

**Mejora futura (no hecha):** que orders-service agregue la descripción a cada línea de la orden (`items[{sku, quantity, description}]`). Así Empaque no necesitaría leer el catálogo del Core. Está anotada como B7 en el [traspaso](../traspaso/Traspaso_Ordenes_frontend_backend.md).

## 6. Pendiente

- **Verificar el seguimiento de órdenes con datos completos.** Hoy solo una orden del seed tiene eventos de seguimiento (48257769230). Queda para cuando esté el seed definitivo.
