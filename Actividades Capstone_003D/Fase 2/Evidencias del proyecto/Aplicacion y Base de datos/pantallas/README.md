# Pantallas de NexusWST

Capturas del sistema para evidencia del Capstone. Cada pantalla se capturó con el perfil que le corresponde, usando los usuarios de prueba del seed, y muestra solo lo que ese perfil puede ver.

- **Desktop:** 1440 × 900 (todas las pantallas).
- **Tablet:** 768 × 1024 y **Móvil:** 390 × 844 (login, home, todas las órdenes, empaque, retiro y entrega, y productos).
- PNG de página completa cuando la vista tiene scroll. Los modales y menús desplegables se capturan en el tamaño de la ventana.

| Perfil | Usuario de prueba |
|---|---|
| Administrador | `admin@nexus.local` |
| Jefe de Operaciones | `jefe@nexus.local` |
| Empaque (con permiso Despacho) | `empaque@nexus.local` |
| Cliente (Empresa Demo Uno SpA) | `cliente1@nexus.local` |

## 00 · Login

| Pantalla | Perfil | Desktop | Tablet | Móvil |
|---|---|---|---|---|
| Inicio de sesión (NexusWST) | Sin sesión | [desktop](00-login/login-inicio_desktop.png) | [tablet](00-login/login-inicio_tablet.png) | [móvil](00-login/login-inicio_movil.png) |
| Formulario de Keycloak (vacío) | Sin sesión | [desktop](00-login/login-keycloak_desktop.png) | [tablet](00-login/login-keycloak_tablet.png) | [móvil](00-login/login-keycloak_movil.png) |

## 01 · Home

| Pantalla | Perfil | Desktop | Tablet | Móvil |
|---|---|---|---|---|
| Home | Administrador | [desktop](01-home/home-admin_desktop.png) | [tablet](01-home/home-admin_tablet.png) | [móvil](01-home/home-admin_movil.png) |
| Home | Jefe de Operaciones | [desktop](01-home/home-jefe_desktop.png) | — | — |
| Home | Empaque | [desktop](01-home/home-empaque_desktop.png) | [tablet](01-home/home-empaque_tablet.png) | [móvil](01-home/home-empaque_movil.png) |
| Home | Cliente | [desktop](01-home/home-cliente_desktop.png) | — | — |

## 02 · Órdenes

| Pantalla | Perfil | Desktop | Tablet | Móvil |
|---|---|---|---|---|
| Todas las órdenes (KPIs, búsqueda, filtros y tabla) | Administrador | [desktop](02-ordenes/ordenes-bandeja-admin_desktop.png) | [tablet](02-ordenes/ordenes-bandeja-admin_tablet.png) | [móvil](02-ordenes/ordenes-bandeja-admin_movil.png) |
| Todas las órdenes | Jefe de Operaciones | [desktop](02-ordenes/ordenes-bandeja-jefe_desktop.png) | — | — |
| Todas las órdenes (solo su empresa) | Cliente | [desktop](02-ordenes/ordenes-bandeja-cliente_desktop.png) | — | — |
| Filtros | Administrador | [desktop](02-ordenes/ordenes-filtros_desktop.png) | — | — |
| KPI «Incidencias» aplicado como filtro | Administrador | [desktop](02-ordenes/ordenes-filtro-kpi-incidencias_desktop.png) | — | — |
| Asignación de órdenes | Jefe de Operaciones | [desktop](02-ordenes/ordenes-asignacion_desktop.png) | — | — |
| Detalle de orden con historial | Administrador | [desktop](02-ordenes/ordenes-detalle_desktop.png) | — | — |
| Historial de estados completo («Ver todo») | Administrador | [desktop](02-ordenes/ordenes-detalle-historial_desktop.png) | — | — |
| Ingreso manual («Nueva orden») | Administrador | [desktop](02-ordenes/ordenes-ingreso-manual_desktop.png) | — | — |
| Menú «Excel acciones» | Administrador | [desktop](02-ordenes/ordenes-excel-acciones_desktop.png) | — | — |
| Ingreso por Excel («Importar órdenes») | Administrador | [desktop](02-ordenes/ordenes-ingreso-excel_desktop.png) | — | — |
| Empaque: cola «Por empacar» | Empaque | [desktop](02-ordenes/empaque_desktop.png) | [tablet](02-ordenes/empaque_tablet.png) | [móvil](02-ordenes/empaque_movil.png) |
| Empaque: verificación de productos y bultos | Empaque | [desktop](02-ordenes/empaque-orden_desktop.png) | [tablet](02-ordenes/empaque-orden_tablet.png) | [móvil](02-ordenes/empaque-orden_movil.png) |
| Retiro y entrega (courier elegido) | Jefe de Operaciones | [desktop](02-ordenes/retiro-entrega_desktop.png) | [tablet](02-ordenes/retiro-entrega_tablet.png) | [móvil](02-ordenes/retiro-entrega_movil.png) |
| Retiro y entrega: manifiesto abierto | Jefe de Operaciones | [desktop](02-ordenes/retiro-entrega-manifiesto_desktop.png) | — | [móvil](02-ordenes/retiro-entrega-manifiesto_movil.png) |
| Seguimiento del courier (detalle de orden entregada) | Jefe de Operaciones | [desktop](02-ordenes/ordenes-seguimiento_desktop.png) | — | — |
| Campana de avisos | Jefe de Operaciones | [desktop](02-ordenes/ordenes-campana-avisos_desktop.png) | — | — |
| Registro histórico | Administrador | [desktop](02-ordenes/registro-historico_desktop.png) | — | — |
| Registro histórico: detalle de una orden | Administrador | [desktop](02-ordenes/registro-historico-detalle_desktop.png) | — | — |
| Registro de acciones | Administrador | [desktop](02-ordenes/registro-acciones_desktop.png) | — | — |

## 03 · Clientes

| Pantalla | Perfil | Desktop | Tablet | Móvil |
|---|---|---|---|---|
| Perfiles empresa | Cliente | [desktop](03-clientes/clientes-perfiles-empresa_desktop.png) | — | — |
| Administración clientes · Perfiles | Administrador | [desktop](03-clientes/clientes-administracion-perfiles_desktop.png) | — | — |
| Administración clientes · Empresas | Administrador | [desktop](03-clientes/clientes-administracion-empresas_desktop.png) | — | — |
| Administración clientes · Solicitudes | Administrador | [desktop](03-clientes/clientes-administracion-solicitudes_desktop.png) | — | — |

## 04 · Configuración

| Pantalla | Perfil | Desktop | Tablet | Móvil |
|---|---|---|---|---|
| Usuarios WeStorage · Perfiles | Administrador | [desktop](04-configuracion/configuracion-usuarios_desktop.png) | — | — |
| Usuarios WeStorage · Solicitudes | Administrador | [desktop](04-configuracion/configuracion-usuarios-solicitudes_desktop.png) | — | — |

## 05 · Productos

| Pantalla | Perfil | Desktop | Tablet | Móvil |
|---|---|---|---|---|
| Productos | Administrador | [desktop](05-productos/productos-admin_desktop.png) | [tablet](05-productos/productos-admin_tablet.png) | [móvil](05-productos/productos-admin_movil.png) |
| Productos (solo su empresa) | Cliente | [desktop](05-productos/productos-cliente_desktop.png) | — | — |

## 06 · Integraciones

| Pantalla | Perfil | Desktop | Tablet | Móvil |
|---|---|---|---|---|
| Catálogo de integraciones | Administrador | [desktop](06-integraciones/integraciones-catalogo_desktop.png) | — | — |
| Conexiones de clientes | Administrador | [desktop](06-integraciones/integraciones-conexiones_desktop.png) | — | — |
| Detalle de una conexión (secreto enmascarado) | Administrador | [desktop](06-integraciones/integraciones-conexion-detalle_desktop.png) | — | — |

## Cómo regenerarlas

Requisitos: el stack de desarrollo levantado (`docker compose up -d` en la raíz), Google Chrome instalado y el seed de demostración en `respaldo/seed/seed_demo.sql` (solo local). Desde `frontend-web/`:

```bash
npx playwright test -c playwright.capturas.config.ts
```

- El script es [`frontend-web/capturas/pantallas.spec.ts`](../../frontend-web/capturas/pantallas.spec.ts). Reutiliza el login y los helpers de las pruebas e2e (`frontend-web/e2e/helpers.ts`).
- Antes de capturar recarga el seed (el mismo `global-setup` de las e2e), así las imágenes siempre salen con los mismos datos. La última captura (campana) crea una incidencia de prueba; la próxima corrida la borra al recargar el seed.
- Sobrescribe las imágenes de esta carpeta. Si cambia la lista de pantallas, actualiza también este índice.
- Por seguridad, el script no escribe contraseñas en el login, no abre el diálogo que muestra secretos de Integraciones, oculta la lista de usuarios de prueba de `/login` (solo existe en desarrollo) y falla si detecta una contraseña de prueba, un token o un secreto a la vista.

---

Capturas generadas con Playwright sobre el entorno de pruebas, con datos ficticios del seed de demostración.
