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

### Paso 3. Compilar el tema de Keycloak

Keycloak monta `infrastructure/keycloak/theme/dist_keycloak/nexuswst-keycloak-theme.jar`. Ese `.jar` es un artefacto y **no va en Git**: cada máquina lo compila antes de levantar Keycloak. Solo requiere Docker.

```bash
make theme
```

Sin `make` (PowerShell), desde la carpeta del repositorio:

```powershell
docker run --rm -v "${PWD}/infrastructure/keycloak/theme:/app" -w /app node:22-bookworm-slim sh docker-build.sh
```

`make up` y `make prod-up` lo compilan solos si falta. Si cambias el tema, vuelve a ejecutar `make theme` y luego `docker compose restart keycloak`.

### Paso 4. Construir y levantar

```bash
docker compose up -d --build
```

La primera vez tarda varios minutos (descarga imágenes, `npm ci`, compila Go). El orden de arranque está controlado por *healthchecks*:

```
postgres, redis ──► migrate-core ──► migrate-orders ──► core, orders ──► (traefik ya enruta)
postgres ──► keycloak-db-init ──► keycloak (importa el realm la primera vez; tarda ~1 min)
```

### Paso 5. Verificar que todo está arriba

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

### Paso 6. Guardar `go.sum` en Git

`backend-core/go.sum` y `orders-service/go.sum` ya vienen en el repositorio (garantizan builds reproducibles). Si agregas una dependencia, regenéralos:

```bash
ls backend-core/go.sum orders-service/go.sum
# Si no existen:
docker compose run --rm --no-deps core go mod tidy
docker compose run --rm --no-deps orders go mod tidy
```

### Paso 7. Entrar a la aplicación

Abre http://localhost → **/login** → botón **Ingresar** → pantalla de **Keycloak** (en español) → vuelves a NexusWST con tu perfil. Usuarios de prueba (existen solo en desarrollo, contraseña **`Nexus2026!`**):

La sesión la maneja Auth.js: los tokens quedan en una cookie httpOnly cifrada y el navegador llama a `/bff/{core|orders}/...`. El WebSocket se abre con un ticket de un solo uso.

| Correo | Rol | Qué ve |
|---|---|---|
| `admin@nexus.local` | Administrador (permiso Gestionar perfiles) | Todo |
| `admin.lectura@nexus.local` | Administrador sin Gestionar perfiles | Todo; Clientes › Administración clientes en solo lectura |
| `jefe@nexus.local` | Jefe de Operaciones (permiso Despacho) | Operación, olas, incidencias, reglas |
| `empaque@nexus.local` | Empaque (permiso Despacho) | Empaque y retiro |
| `analista@nexus.local` | Analista | Consulta |
| `cliente1@nexus.local` | Cliente (solo Empresa Demo Uno) | Sus órdenes y productos |
| `cliente2@nexus.local` | Cliente (solo Empresa Demo Dos) | Sus órdenes (sirve para probar el aislamiento) |

**Realm importado antes del 07-11-2026.** Keycloak importa `nexuswst-realm.json` solo al crear el realm. Si el tuyo es anterior, su perfil de usuario todavía exige correo y no se pueden crear usuarios internos sin correo (SCRUM-231): Keycloak responde `error-user-attribute-required`. Para aplicar el perfil de usuario del repo sin recrear Keycloak (desde la raíz, en Git Bash):

```bash
python -c "import json;r=json.load(open('infrastructure/keycloak/import/nexuswst-realm.json',encoding='utf-8'));open('user-profile.json','w',encoding='utf-8').write(r['components']['org.keycloak.userprofile.UserProfileProvider'][0]['config']['kc.user.profile.config'][0])"
docker compose cp user-profile.json keycloak:/tmp/user-profile.json && rm user-profile.json
docker compose exec keycloak sh -c '/opt/keycloak/bin/kcadm.sh config credentials --server "http://localhost:8080${KC_HTTP_RELATIVE_PATH}" --realm master --user "$KC_BOOTSTRAP_ADMIN_USERNAME" --password "$KC_BOOTSTRAP_ADMIN_PASSWORD" && /opt/keycloak/bin/kcadm.sh update users/profile -r nexuswst -f /tmp/user-profile.json'
```

El correo deja de ser obligatorio en Keycloak; la regla «el perfil Cliente siempre tiene correo» la aplica el Core.

El perfil de Nexus (rol, permisos, empresas) se vincula solo la primera vez que cada usuario ingresa. El servicio `seed-dev` crea estos perfiles al levantar; para recargarlos: `make seed-dev` o `docker compose run --rm seed-dev`.

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

### Pruebas del frontend

Desde `frontend-web/`:

- `npm test`: pruebas de comportamiento (`tests/`, vitest + happy-dom, con los servicios simulados).
- `npm run test:e2e`: pruebas de punta a punta (`e2e/`, Playwright con el Chrome instalado). Requieren el stack de desarrollo levantado. Antes de cada corrida, `e2e/global-setup.ts` recarga los datos: corre `seed-dev` y luego el seed de demostración (`respaldo/seed/seed_demo.sql`, solo local), así las pruebas no dependen de lo que dejó la corrida anterior. Si ese archivo no existe, avisa y sigue; `E2E_SKIP_SEED=1` omite la recarga. Otro navegador: `E2E_CHANNEL=msedge`.

### Pruebas de Go y el *healthcheck* de desarrollo

- `make test-core` y `make test-orders` corren `go test ./...` en un contenedor aparte, sin tocar el servicio en ejecución. Las pruebas de PostgreSQL necesitan `TEST_DATABASE_URL`; sin ella se saltan.
- En desarrollo, Air recompila al guardar y deja el binario en `./tmp/api`. Si se guardan dos archivos casi a la vez, dos compilaciones seguidas pueden dejar `./tmp` sin el binario aunque el servidor siga corriendo. Antes el *healthcheck* ejecutaba ese binario, así que el servicio quedaba `unhealthy` y Traefik dejaba de enviarle tráfico (el front respondía en `/api/core`). Desde el 08-11-2026, en `docker-compose.override.yml`, el *healthcheck* de `core` y `orders` consulta `/health` por HTTP. Si alguna vez pasa algo parecido: `docker compose restart core`.

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
4. Las migraciones traen solo catálogos reales. Los datos ficticios (`infrastructure/dev-seed`, usuarios de `infrastructure/keycloak/dev-users`) y las cuentas demo de `/login` (`NEXT_PUBLIC_SHOW_DEMO_ACCOUNTS`) los activa solo `docker-compose.override.yml`, que producción no carga.
5. Keycloak en producción (PV-18):
   - `APP_PUBLIC_URL=https://<dominio>`, `KEYCLOAK_PUBLIC_URL=https://auth.westorage.cl/auth`, `AUTH_DOMAIN=auth.westorage.cl` (DNS apuntando al servidor).
   - `KC_WEB_DIRECT_GRANTS=false`.
   - `SMTP_*` con el correo de WeStorage (remitente `no-reply@westorage.cl`; SPF/DKIM los configura TI).
   - Secretos nuevos y largos para `KEYCLOAK_*` e `INTEGRATIONS_KEY`.
   - Keycloak tiene un límite de memoria de 1,36 GB en `docker-compose.prod.yml` (PV-15); el stack completo usa unos 2,6–2,9 GB del plan de 4 GB.
   - Antes de actualizar la versión de Keycloak, respalda la base (`make backup`; incluye el esquema `keycloak`).
6. Compila el tema de Keycloak con `make theme` (paso 3).
7. Levanta:

```bash
docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d --build
```

En producción: imágenes compiladas sin herramientas de desarrollo, usuario sin privilegios, sin dashboard de Traefik, sin puertos de BD expuestos y la red `data` es interna. Si WeStorage entrega un PostgreSQL administrado, quita el servicio `postgres` y apunta `DATABASE_URL` a esa instancia con `sslmode=require`.

### Producción detrás de Cloudflare (SCRUM-247, PV-20)

Capa opcional sobre la de producción (`docker-compose.cloudflare.yml` + `infrastructure/traefik/traefik.cloudflare.yml`). Sin ella, producción sigue igual (Let's Encrypt por HTTP).

1. En Cloudflare: registro `A` del dominio de Nexus apuntando al droplet, con el proxy activo (nube naranja), y en SSL/TLS el modo **Full (strict)**.
2. Crea un token de API con permiso **Zone → DNS → Edit** sobre la zona y ponlo en `CLOUDFLARE_DNS_API_TOKEN` del `.env`. Traefik lo usa para pedir el certificado de Let's Encrypt por DNS (funciona con el proxy activo). Cambia el correo de `certificatesResolvers` en `traefik.cloudflare.yml`, igual que en `traefik.prod.yml`.
3. `make prod-cf-up` (para bajar: `make prod-cf-down`).

Qué cambia:

- **Solo entra tráfico de Cloudflare**: quien vaya directo a la IP del servidor recibe 403, aunque falsee las cabeceras de Cloudflare.
- **IP real del visitante**: Traefik solo acepta `X-Forwarded-For` de las IP de Cloudflare, así Keycloak (bloqueo por fuerza bruta) y los registros ven la IP real y nadie la puede falsear.
- Los rangos de Cloudflare están en los dos archivos de Traefik; `make cloudflare-ips` los compara con los publicados (conviene antes de cada despliegue).
- Recomendado además en el droplet: firewall que solo abra 80/443 a esos rangos (y 22 a quien administra).

### Integración continua (SCRUM-197)

El pipeline corre los mismos pasos en **GitHub Actions** (`.github/workflows/ci.yml`, en cada push a `main`, `test`, `jean_dev` o `cristopher_dev` y en cada PR) y en **GitLab CI** (`.gitlab-ci.yml`, si el proyecto se sube a GitLab). Los dos llaman a `scripts/ci.sh`:

| Paso | Qué revisa |
| --- | --- |
| Go · core / orders | `gofmt`, `go vet` y pruebas con detector de carreras y cobertura (`coverage.out`). Las pruebas que necesitan PostgreSQL se saltan. |
| Frontend | `npm ci`, lint, tipos (`tsc`) y build de producción |
| Imágenes | Construye las imágenes de producción (sin publicarlas) |
| SonarQube | Análisis con la cobertura de Go (`sonar-project.properties`). Se salta solo si no está configurado. |

Para activar SonarQube: en GitHub, el secreto `SONAR_TOKEN` y las variables `SONAR_HOST_URL` (y `SONAR_ORGANIZATION` si es SonarCloud); en GitLab, las mismas como variables de CI/CD. Antes de subir puedes correr lo mismo con `make ci` (necesita Go 1.26 y Node 22).

### Cambios de configuración de Keycloak en un realm ya creado

Keycloak importa `infrastructure/keycloak/import/nexuswst-realm.json` **solo la primera vez**. Si el realm ya existe (tu entorno o el servidor), aplica los cambios con:

```bash
make keycloak-update        # ./scripts/keycloak-update.sh (se puede repetir sin problema)
```

- **Usuarios sin correo** (SCRUM-231): el usuario deja de ser el correo y el correo pasa a ser opcional.
- **Correos en español** (SCRUM-80): tema `nexuswst-email` (`infrastructure/keycloak/email-theme`), con remitente `SMTP_FROM`.
- **Microsoft 365 / Entra ID** (SCRUM-230, opcional): con `ENTRA_TENANT_ID`, `ENTRA_CLIENT_ID` y `ENTRA_CLIENT_SECRET` en el `.env` se habilita «Continuar con Microsoft»; sin ellos queda deshabilitado. El administrador de Microsoft de WeStorage registra la aplicación con la URI de redirección `<KEYCLOAK_PUBLIC_URL>/realms/nexuswst/broker/microsoft/endpoint`.
- En producción: `export COMPOSE="docker compose -f docker-compose.yml -f docker-compose.prod.yml"` antes del comando. Necesita `python3` en el servidor.

### Respaldo y restauración de PostgreSQL (SCRUM-199)

El respaldo incluye **toda la base**: Core, Órdenes y el esquema `keycloak` (usuarios y roles). Redis no se respalda: guarda caché (se reconstruye) y colas con AOF.

```bash
make backup                                            # ./scripts/backup.sh
make restore file=backups/nexuswst_20261103-2100.dump  # pide escribir RESTAURAR
```

- El respaldo queda en `./backups/nexuswst_<fecha>.dump` (formato *custom* de `pg_dump`, comprimido). Se verifica con `pg_restore --list` antes de guardarlo y se conservan los últimos 7 (`BACKUP_KEEP`). La carpeta `backups/` está en `.gitignore`.
- La restauración detiene Keycloak, Core y Órdenes, restaura **en una sola transacción** (si falla, la base queda como estaba) y los vuelve a levantar.
- En producción, antes de cada comando: `export COMPOSE="docker compose -f docker-compose.yml -f docker-compose.prod.yml"`.
- Respaldo diario en el servidor (cron, 03:00):

```bash
0 3 * * * cd /opt/nexuswst/NexsusWST && COMPOSE="docker compose -f docker-compose.yml -f docker-compose.prod.yml" ./scripts/backup.sh >> backups/backup.log 2>&1
```

- Copia los respaldos **fuera del servidor** (por ejemplo, a tu PC con `scp`): si se pierde el droplet, se pierden también los respaldos que estén en él.
- Prueba la restauración al menos una vez en el ambiente de pruebas (PV-15).

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
| `keycloak` no arranca y `nexuswst-keycloak-theme.jar` aparece como carpeta | Se levantó sin compilar el tema y Docker creó una carpeta en su lugar. Bórrala, ejecuta `make theme` y levanta de nuevo |
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
| Alcance por empresa del personal interno (`all_companies`) | Propuesta | PV-02 |
| `infrastructure/dev-seed`, `dev-users` de Keycloak | Datos y usuarios ficticios | Solo desarrollo (`docker-compose.override.yml`) |

## Próximos pasos sugeridos

1. Subir al repositorio GitLab/GitHub del equipo (`go.sum` incluidos).
2. Pipeline CI: `go test`, `npm run build`, SonarQube y build de imágenes.
3. Pantallas de Jean sobre la API v1.1: Registro, empaque con conteo, retiro/manifiesto, reglas de ola, usuarios, integraciones y campana de avisos.
4. CRUD de clientes y productos con kits e importación (H-03/H-04).
