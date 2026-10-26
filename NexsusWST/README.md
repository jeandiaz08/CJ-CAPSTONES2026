# NexusWST — Entorno dockerizado de desarrollo

Base de código y contenedores para empezar a programar NexusWST (modernización de We Manager para WeStorage), alineada al **DAS v1.1**, **ERS v1.1** y **Casos de uso V1.5** (Keycloak como proveedor de identidad, PV-18):

```
Navegador ──HTTPS/WSS──► Traefik ──┬──► frontend  (Next.js :3000)
                                   ├──► core      (Go monolito modular :8080)  ← /api/core/*
                                   ├──► orders    (Go hexagonal + worker :8081) ← /api/orders/*  (+ WebSocket /api/orders/ws)
                                   └──► keycloak  (identidad :8080)           ← /auth/*  (login, token, correos)
                                              │
                         red "data" (privada) ├──► postgres (:5432)  base única (+ esquema "keycloak")
                                              ├──► redis    (:6379)  caché + Pub/Sub + Streams
                                              └──► mailpit  (solo dev: bandeja de correos de prueba)
```

---

## 1. Qué necesitas instalar (una sola vez)

| Herramienta | Versión | Para qué |
|---|---|---|
| **Docker Desktop** (Windows/macOS) o Docker Engine + Compose v2 (Linux) | Compose ≥ 2.24 | Levantar todo el stack |
| **WSL2 + Ubuntu** (solo Windows) | — | Docker Desktop lo usa como motor; además trabajarás dentro de él |
| **Git** | cualquiera reciente | Clonar / subir a GitLab |
| **VS Code** | — | Editor. Extensiones: *Go*, *Docker*, *WSL*, *ESLint*, *PostgreSQL* (opcional) |
| Go 1.26 y Node 22 **locales** | opcional | Solo para autocompletado del IDE. Para *ejecutar* no hacen falta: todo corre en contenedores |

Recursos: mínimo 8 GB de RAM (ideal 16 GB). En Docker Desktop → *Settings → Resources* asigna al menos 4 GB.

> **Windows (importante):** clona el proyecto **dentro de WSL** (`/home/tu_usuario/proyectos/...`), no en `C:\Users\...`. Desde `C:` los bind mounts son muy lentos y la recarga en caliente falla a menudo. Abre la carpeta con `code .` desde la terminal de Ubuntu.

Verifica la instalación:

```bash
docker --version
docker compose version
docker run --rm hello-world
```

---

## 2. Estructura del repositorio

```
nexuswst/
├── docker-compose.yml           # Servicios BASE (comunes a todos los entornos)
├── docker-compose.override.yml  # DESARROLLO (se carga solo): hot reload, puertos locales, Adminer
├── docker-compose.prod.yml      # PRODUCCIÓN: HTTPS Let's Encrypt, red de datos interna
├── .env.example                 # Plantilla de variables (copiar a .env)
├── Makefile                     # Atajos opcionales
├── infrastructure/
│   ├── traefik/                 # traefik.yml (dev), traefik.prod.yml, dynamic/middlewares.yml
│   ├── postgres/init/           # Extensiones (solo al crear el volumen)
│   └── redis/redis.conf         # AOF + noeviction (no perder trabajos)
├── frontend-web/                # React + Next.js
│   └── src/ app · components · features · services · hooks · state
├── backend-core/                # Monolito modular Go
│   ├── cmd/api/                 # main.go: composición
│   ├── internal/modules/        # products (ejemplo completo), catalogs (Cache-Aside),
│   │                            # clients, auth_roles, inventory, reception, billing (vacíos)
│   ├── internal/platform/       # config, database, cache, httpserver
│   ├── internal/shared/         # httpx, tenant
│   └── migrations/
└── orders-service/              # Servicio de Órdenes Go
    ├── cmd/api/                 # API + worker en el mismo proceso (DA-06)
    ├── internal/domain/         # Orden + máquina de estados (PA-04) + tests
    ├── internal/application/    # Casos de uso
    ├── internal/ports/          # Contratos (repositorio, notifier, WMS)
    ├── internal/adapters/       # inbound/http (REST+WS), outbound/postgres, redisbus, wms (simulado)
    ├── internal/workers/        # Outbox relay + consumidor Redis Streams (PA-06)
    └── migrations/
```

Correspondencia con el diagrama: cada **caja azul del monolito** es una carpeta en `backend-core/internal/modules/`; cada **contenedor del diagrama de despliegue** es un servicio en `docker-compose.yml`.

---

## 3. Primer arranque — paso a paso

### Paso 1. Obtener el código

```bash
# Si ya está en GitLab:
git clone <url-del-repo-de-ismael> nexuswst
cd nexuswst
# Si partes del .zip: descomprímelo dentro de WSL y entra a la carpeta
```

### Paso 2. Crear el archivo de variables

```bash
cp .env.example .env
```

Abre `.env` y cambia los valores que dicen `CAMBIAR`. Usa contraseñas **sin** `@ : / ? #` (van dentro de una URL de conexión). Para los secretos largos (`KEYCLOAK_CORE_CLIENT_SECRET`, `INTEGRATIONS_KEY`) usa `openssl rand -hex 32`.

### Paso 3. Construir y levantar

```bash
docker compose up -d --build
```

La primera vez tarda varios minutos (descarga imágenes, `npm ci`, compila Go). El orden de arranque está controlado por *healthchecks*:

```
postgres, redis ──► migrate-core ──► migrate-orders ──► core, orders ──► (traefik ya enruta)
postgres ──► keycloak-db-init ──► keycloak (importa el realm la primera vez; tarda ~1 min)
```

### Paso 4. Verificar que todo está arriba

```bash
docker compose ps
```

Debes ver `core`, `orders`, `postgres`, `redis` y `keycloak` como **healthy**, y `migrate-core`, `migrate-orders` y `keycloak-db-init` como **exited (0)** (es normal: corren una vez y terminan).

| URL | Qué es |
|---|---|
| http://localhost | Aplicación (login → inicio con sidebar) |
| http://localhost/api/core/health | Salud del Core |
| http://localhost/api/core/ready | Core + PostgreSQL + Redis |
| http://localhost/api/orders/health | Salud de Orders |
| http://127.0.0.1:8088 | Dashboard de Traefik (solo dev) |
| http://127.0.0.1:8089 | Adminer → servidor `postgres`, usuario/clave/base de tu `.env` |
| http://localhost/auth/admin | Consola de Keycloak (solo emergencias) → `KEYCLOAK_ADMIN_USER` / `KEYCLOAK_ADMIN_PASSWORD` de tu `.env` |
| http://127.0.0.1:8025 | Mailpit: aquí llegan los correos de prueba (definir/recuperar contraseña, avisos) |

### Paso 5. Guardar `go.sum` en Git

`backend-core/go.sum` y `orders-service/go.sum` ya vienen en el repositorio (garantizan builds reproducibles). Si agregas una dependencia, regenéralos:

```bash
ls backend-core/go.sum orders-service/go.sum
# Si no existen:
docker compose run --rm --no-deps core go mod tidy
docker compose run --rm --no-deps orders go mod tidy
```

### Paso 6. Entrar a la aplicación

Abre http://localhost → **/login** → botón **Ingresar** → pantalla de **Keycloak** (en español) → vuelves a NexusWST con tu perfil. Usuarios de prueba (existen solo en desarrollo, contraseña **`Nexus2026!`**):

| Correo | Rol | Qué ve |
|---|---|---|
| `admin@nexus.local` | Administrador | Todo |
| `jefe@nexus.local` | Jefe de Operaciones (permiso Despacho) | Operación, olas, incidencias, reglas |
| `empaque@nexus.local` | Empaque (permiso Despacho) | Empaque y retiro |
| `analista@nexus.local` | Analista | Consulta |
| `cliente1@nexus.local` | Cliente (solo Empresa Demo Uno) | Sus órdenes y productos |
| `cliente2@nexus.local` | Cliente (solo Empresa Demo Dos) | Sus órdenes (sirve para probar el aislamiento) |

El perfil de Nexus (rol, permisos, empresas) se vincula solo la primera vez que cada usuario ingresa.

**Recorrido de prueba completo:**

1. Como *jefe*, crea una orden de **Empresa Demo Uno**: courier, destino con comuna (p. ej. Ñuñoa) y SKU `SKU-001`.
2. En segundos pasa sola a **Validada**.
3. Asígnala a una ola (o crea una regla en `/v1/wave-rules`).
4. Invas (simulado) la deja **En preparación** y luego **Preparada**.
5. Desde el detalle: *Tomar para empaque* → *Empaque verificado* → *Entregar al courier*.

Otros casos para probar:

- `SKU-SINSTOCK`: la orden queda **Anulada** y el cliente recibe un aviso y un correo (Mailpit).
- Comuna mal escrita (p. ej. «Providensia»): queda en **Incidencia** con la sugerencia «¿Quisiste decir Providencia?».

Para probar la API con curl o Postman (solo desarrollo, `KC_WEB_DIRECT_GRANTS=true`):

```bash
TOKEN=$(curl -s -X POST http://localhost/auth/realms/nexuswst/protocol/openid-connect/token \
  -d grant_type=password -d client_id=nexus-web -d username=jefe@nexus.local -d 'password=Nexus2026!' \
  | sed -E 's/.*"access_token":"([^"]+)".*/\1/')
curl -H "Authorization: Bearer $TOKEN" http://localhost/api/core/v1/auth/me
curl -H "Authorization: Bearer $TOKEN" http://localhost/api/orders/v1/orders
```

El contrato completo de la API está en **[docs/API.md](docs/API.md)**.

---

## 4. Dónde programar cada cosa

| Quiero programar… | Frontend (Next.js) | Backend (Go) |
|---|---|---|
| **Un ítem del menú lateral** | `frontend-web/src/features/navigation/menu.ts` (+ página en `src/app/(app)/<ruta>/page.tsx`) | — |
| **Estructura general** (sidebar, barra superior, sesión) | `src/app/(app)/layout.tsx`, `src/components/Sidebar.tsx`, `Topbar.tsx`, `src/state/auth.tsx` | — |
| **Bandeja de órdenes** (RF-10) | `src/app/(app)/ordenes/page.tsx`, `src/features/orders/` | `orders-service/internal/adapters/inbound/http/handler.go` → `application/service.go` |
| **Detalle e historial** (RF-11) | `src/app/(app)/ordenes/[id]/page.tsx` | `adapters/outbound/postgres/repository.go` (tabla `order_events`) |
| **Estados de la orden** (PA-04) | `src/features/orders/statuses.ts` (etiquetas y botones) | `orders-service/internal/domain/status.go` (**la regla real**) |
| **Validación automática / stock** (RF-12) | — | `orders-service/internal/application/validator.go` |
| **Olas** (RF-17) y **reglas de olas** | `src/app/(app)/olas/`, `reglas-olas/` | `application/rules.go` (motor de reglas), `workers/consumer.go` |
| **Empaque / bultos** (RF-18 a RF-22) | `src/app/(app)/empaque/` | `orders-service/internal/application/fulfillment.go` (`TakeForPacking`, `ConfirmPacking`) |
| **Retiro y manifiesto** (RF-23/24) | `src/app/(app)/retiro/` | `application/fulfillment.go` (`CreateManifest`, `ScanPackage`, `CloseManifest`) |
| **Carga masiva y exportación Excel** (RF-09/10) | `src/app/(app)/ordenes/` | `application/imports.go` + `platform/xlsx/` |
| **Incidencias** (RF-15/20) | `src/app/(app)/incidencias/` (o «Registro») | `application/service.go` (`ReportIncident`, `Resolve`) |
| **Avisos** (RF-27) | campana en `Topbar.tsx` | `application/notices.go`, `GET /v1/notifications` |
| **Inicio de sesión** (RF-01) | `src/services/oidc.ts`, `src/state/auth.tsx`, `src/app/login/` (puente hasta Auth.js) | Keycloak: `infrastructure/keycloak/import/nexuswst-realm.json` |
| **Integración real con Invas** | — | nuevo `orders-service/internal/adapters/outbound/wms/invas.go` (reemplaza `simulated.go`) |
| **Webhooks** de couriers/canales (H-06/H-14) | `src/app/(app)/integraciones/` | `adapters/inbound/webhooks/`, `application/integrations.go` |
| **Clientes / Productos** (RF-04/06) | `src/app/(app)/clientes/`, `productos/` | `backend-core/internal/modules/clients/`, `products/` |
| **Usuarios y roles** (RF-02) | `src/app/(app)/usuarios/` | `backend-core/internal/modules/auth_roles/` (Admin API de Keycloak en `adapters/outbound/keycloak/`) |
| **Catálogos con caché** (RF-29/31) | `src/services/core.ts` (`getCatalog`) | `backend-core/internal/modules/catalogs/` |

Las páginas sin implementar muestran **qué requisito cubren y en qué archivos programarlas**.

### Reglas que el código ya respeta (DAS v1.1)

- **Identidad (Keycloak, DA-15)**: Keycloak inicia la sesión y emite el token (RS256, **10 min**, sesión 12 h / 2 h sin uso). Core y Órdenes lo verifican **por sí mismos** con las claves públicas de Keycloak (JWKS en caché, rotación automática). Traefik solo enruta. **Sin lista de revocación**: en cada petición se lee el perfil en PostgreSQL, así que deshabilitar un usuario lo bloquea de inmediato. Nexus no guarda contraseñas.
- **Empresa**: nunca se toma del navegador sin contrastarla con el perfil (`tenant.Scope`). Personal interno ve todas; el cliente solo las suyas. Empresa deshabilitada = sus usuarios no operan.
- **Órdenes nunca llama al Core**: lee `companies`/`products` directo de PostgreSQL (`ProductReader`). Si el Core cae, Órdenes sigue operando con las sesiones vigentes.
- **Comprometido** (`stock_committed`): solo lo escribe Órdenes. Se suma al validar, se resta cuando Invas confirma la ola o al anular/cancelar.
- **Historial** (`order_events`): solo agregado, en la misma transacción que el cambio de estado.
- **Prioridad**: «Enviar primero a la cola» → el Worker envía primero esas órdenes a Invas.
- **Caché de catálogos** en el Core (Redis + `ETag` + `Cache-Control`), con refresco manual `POST /v1/catalogs/{nombre}/refresh`.

---

## 5. Trabajo diario

| Quiero… | Hago… |
|---|---|
| Programar en Go | Edito y guardo: Air recompila y reinicia solo (`docker compose logs -f core orders`) |
| Programar en el front | Edito y guardo: Next recarga el navegador |
| Ejecutar tests Go | `docker compose run --rm --no-deps core go test ./...` (igual con `orders`) |
| Consola SQL | `docker compose exec postgres psql -U nexus -d nexuswst` |
| Ver la cola de trabajos | `docker compose exec redis redis-cli -a <REDIS_PASSWORD> XINFO GROUPS nexus:jobs:orders` |
| Simular que Invas no responde | `WMS_SIMULATE_DOWN=true` en `.env` y `docker compose up -d orders` |
| Ver los correos enviados (contraseñas, avisos) | http://127.0.0.1:8025 (Mailpit) |
| Crear un usuario | `POST /api/core/v1/users` como admin (le llega el correo para definir su contraseña) |
| Más rápido que Invas simulado | `WMS_SIM_PICK_SECONDS=5` en `.env` y `docker compose up -d orders` |
| Instalar un paquete npm | `docker compose exec frontend npm install <pkg>` y luego `docker compose up -d --build -V frontend` |
| Detener todo | `docker compose down` (los datos se conservan) |
| Borrar la BD y empezar de cero | `docker compose down -v` ⚠️ |

### Crear una migración nueva

```bash
docker run --rm -v "$PWD/backend-core/migrations:/migrations" migrate/migrate:v4.18.3 \
  create -ext sql -dir /migrations -seq crear_tabla_x
docker compose up migrate-core
```

Cada servicio migra solo **sus** tablas (cada tabla tiene un solo dueño, DA-03).

---

## 6. Despliegue en DigitalOcean (resumen)

1. Droplet Ubuntu con Docker y Compose; DNS del dominio apuntando a su IP; puertos 80/443 abiertos.
2. Clona el repo en el servidor, crea `.env` con valores **reales** (`APP_ENV=production`, `APP_DOMAIN=nexus.tudominio.cl`, `LOG_LEVEL=info`).
3. Edita el correo en `infrastructure/traefik/traefik.prod.yml` (Let's Encrypt).
4. Elimina o reemplaza `backend-core/migrations/000002_seed_dev.*` y `000004_seed_dev_v11.*` (datos y perfiles ficticios). Los usuarios de prueba de Keycloak están solo en `docker-compose.override.yml`, que producción no usa.
5. Keycloak en producción (PV-18):
   - `APP_PUBLIC_URL=https://<dominio>`, `KEYCLOAK_PUBLIC_URL=https://auth.westorage.cl/auth`, `AUTH_DOMAIN=auth.westorage.cl` (DNS apuntando al servidor).
   - `KC_WEB_DIRECT_GRANTS=false`.
   - `SMTP_*` con el correo de WeStorage (remitente `no-reply@westorage.cl`; SPF/DKIM los configura TI).
   - Secretos nuevos y largos para `KEYCLOAK_*` e `INTEGRATIONS_KEY`.
   - Unos 1,25 GB de RAM para Keycloak.
   - Antes de actualizar la versión de Keycloak, respalda la base (incluye el esquema `keycloak`).
6. Levanta:

```bash
docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d --build
```

En producción: imágenes compiladas sin herramientas de desarrollo, usuario sin privilegios, sin dashboard de Traefik, sin puertos de BD expuestos y la red `data` es interna. Si WeStorage entrega un PostgreSQL administrado, quita el servicio `postgres` y apunta `DATABASE_URL` a esa instancia con `sslmode=require`.

---

## 7. Solución de problemas

| Síntoma | Causa probable / solución |
|---|---|
| `port is already allocated` (80, 5432, 6379) | Otro programa usa el puerto. Cambia `HTTP_PORT`, `POSTGRES_HOST_PORT`… en `.env` |
| Traefik: `client version 1.24 is too old` | Traefik antiguo con Docker ≥ 29. Usa `TRAEFIK_VERSION=v3.6` o superior (ya es el valor por defecto) |
| 404 en http://localhost | Traefik aún no detecta el contenedor o éste no está healthy. `docker compose ps` y `docker compose logs traefik` |
| `core` queda *unhealthy* en el primer arranque | La primera compilación con Air descarga dependencias; espera 1–2 min. Revisa `docker compose logs core` |
| `migrate-*` sale con error `Dirty database version N` | Una migración falló a medias. Corrige el SQL y ejecuta: `docker compose run --rm migrate-core -path=/migrations -database "<url>" force <N-1>` |
| No recarga al guardar (Windows) | Proyecto en `C:\`: muévelo a WSL. Para el front también puedes cambiar el script a `next dev --webpack` (usa `WATCHPACK_POLLING`) |
| `exec ... no such file or directory` en scripts | Finales de línea CRLF. El `.gitattributes` fuerza LF; vuelve a clonar o ejecuta `git add --renormalize .` |
| Cambié `.env` y no se aplica | `docker compose up -d` recrea los contenedores afectados (no basta `restart`) |
| Al ingresar: «redirect_uri inválido» o «token inválido» | `APP_PUBLIC_URL` / `KEYCLOAK_PUBLIC_URL` no coinciden con la URL que usas en el navegador (incluye el puerto si `HTTP_PORT` no es 80). El realm se importa **una sola vez**: si cambias la URL, bórralo en la consola de Keycloak o recrea el esquema |
| 403 `profile_missing` en Órdenes | El usuario aún no entra por la app: el perfil se crea al llamar `GET /api/core/v1/auth/me` (lo hace el login) |
| `keycloak` tarda en quedar *healthy* | Normal la primera vez (crea sus tablas e importa el realm, ~1–2 min). `docker compose logs -f keycloak` |

---

## 8. Qué es provisional (y por qué)

Rotulado explícitamente para no presentarlo como funcionalidad terminada (DAS §6.3):

| Elemento | Estado | Se cierra con |
|---|---|---|
| `wms.Simulated` (stock 100 / `SINSTOCK` 0 / `POCO` 3, olas, asignación y preparación) | Simulado, no llama a Invas | PV-03/PV-04 → `invas.go` |
| Botón "Confirmar preparación (sim.)" | Solo en desarrollo; en producción lo informa Invas | PV-03/PV-04 |
| Etiquetas de courier y detalle de Mercado Libre (`courier.Simulated`) | PDF y datos de prueba rotulados «SIMULACIÓN» | PV-08 |
| Catálogo de comunas | RM completa + principales comunas (113); falta el listado oficial (346) | Carga CUT/SUBDERE |
| Login con PKCE sin librería (`src/services/oidc.ts`) | Puente funcional hasta Auth.js | H-01 T05/T06 (Jean) |
| Tema WeStorage en el login de Keycloak | Tema por defecto con el nombre NexusWST · WeStorage | H-01 T07 (Jean) |
| Alcance por empresa del personal interno (`all_companies`) | Propuesta | PV-02 |
| `000002_seed_dev`, `000004_seed_dev_v11`, `dev-users` de Keycloak | Datos y usuarios ficticios | Retirar antes de producción |

## Próximos pasos sugeridos

1. Subir al repositorio GitLab/GitHub del equipo (`go.sum` incluidos).
2. Pipeline CI: `go test`, `npm run build`, SonarQube y build de imágenes.
3. Pantallas de Jean sobre la API v1.1: Registro, empaque con conteo, retiro/manifiesto, reglas de ola, usuarios, integraciones y campana de avisos.
4. CRUD de clientes y productos con kits e importación (H-03/H-04).
