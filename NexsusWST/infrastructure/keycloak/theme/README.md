# Tema de login NexusWST

Tema independiente de Keycloak 26, con Keycloakify 11.16.4, React 19, Vite 6, TypeScript, Inter local y Lucide. El tema envía los formularios a Keycloak; no almacena credenciales ni implementa OIDC.

## Desarrollo y revisión visual

Desde esta carpeta:

```powershell
npm ci
npm run dev
```

Abrir http://127.0.0.1:5173/?screen=gallery para seleccionar pantalla. Los escenarios son exclusivamente de desarrollo; no hacen peticiones al realm ni se incluyen en producción. Sin Storybook ni Playwright instalados.

| Parámetro `screen` | Estado |
|---|---|
| `login` | Normal: Microsoft deshabilitado, mantener sesión visible |
| `login-microsoft` | Proveedor Microsoft configurado |
| `login-error` | Error de credenciales |
| `login-microsoft-error` | Error con proveedor Microsoft |
| `reset` | Recuperación |
| `reset-error` | Error de validación del campo |
| `check-email` | Confirmación genérica de recuperación |
| `check-email-ready` | Reenvío habilitado (solo desarrollo) |
| `check-email-no-user` | Confirmación sin usuario disponible |
| `new-password` | Crear contraseña |
| `new-password-error` | Errores de política/confirmación enviados por servidor |
| `error` | Error general de Keycloak |
| `info` | Confirmación informativa |

Las capturas originales están en `respaldo/login-review/`. La revisión del 29-10-2026 se guarda en `respaldo/login-review/revision-2026-10-29/`, con tres anchos (360×800, 768×1024, 1280×900). Estas carpetas son locales, excluidas de Git. `capture=1` oculta el enlace del catálogo.

## Comportamiento real

- `login.ftl`, `login-reset-password.ftl`, `login-update-password.ftl`, `info.ftl` y `error.ftl` personalizados; otras páginas y flujos opcionales sin diseño definido usan `DefaultPage` de Keycloakify.
- Microsoft siempre visible: buscar alias exacto `microsoft` en `social.providers` (Keycloak filtra proveedores deshabilitados). Si existe, utilizar su `loginUrl`; si falta, botón nativo disabled con aria-disabled y sin URL. En ambos casos: «Solo personal WeStorage: usa tu cuenta @westorage.cl». No se configura Entra.
- `rememberMe` depende del realm. Se conservan `credentialId`, `logout-sessions` y cancelación de acciones iniciadas por la aplicación.
- Mensajes globales y por campo proceden de Keycloak, con HTML sanitizado. No se inventan intentos máximos ni duraciones de enlace. La coincidencia de contraseñas se señala también al escribir.
- Guía/barra orientativa desde `kcContext.passwordPolicies`, no desde valores duplicados: longitud y composición, además de no ser igual al username cuando se dispone de él. Si falta el username, esa regla queda para validación del servidor; si falta la política, no se inventa una. Chip de identidad solo si llega username/correo. Los ejemplos de desarrollo contienen una política de prueba; el código de producción solo lee el contexto.
- Keycloak 26.8.1 entrega PasswordPoliciesBean a todas las páginas. Se extiende el tipo de `login-update-password.ftl` de Keycloakify 11.16.4 para declararlo; [fuente del proveedor](https://github.com/keycloak/keycloak/blob/26.8.1/services/src/main/java/org/keycloak/forms/login/freemarker/FreeMarkerLoginFormsProvider.java).
- Keycloak 26.8.1 vuelve a `login.ftl` con `emailSentMessage` después de pedir recuperación; se presenta como «Revisa tu correo», de forma idéntica para cuentas existentes e inexistentes. Otros mensajes de éxito mantienen su significado.
- La confirmación incluye el dato ingresado solo si viene de `auth.attemptedUsername` o `login.username`; nunca consulta si la cuenta existe. El remitente es `properties.NEXUS_MAIL_FROM`, configurable mediante `environmentVariables` de Keycloakify. El valor por defecto está en Vite y el archivo generado, no duplicado en la página. Compose reutiliza `SMTP_FROM` salvo que se defina `NEXUS_MAIL_FROM`.
- Reenvío: espera visual de 60 s, accesible, calculada contra un plazo real; el texto visible cambia por segundo pero la región viva anuncia solo los tramos de 30/10 segundos y disponibilidad. Es ayuda de interfaz, no una protección de seguridad. Después, el botón abre el formulario nativo de recuperación con `login_hint` y el usuario precargado; «Enviar enlace» vuelve a ejecutar el flujo de Keycloak y al retornar reinicia el contador. Se elige este regreso porque la confirmación pertenece al nuevo flujo de login, no a la ejecución POST de recuperación anterior. No se reutiliza una acción vencida ni se crean endpoints. Si falta el dato, el formulario permite ingresarlo; las sesiones vencidas siguen el tratamiento nativo de Keycloak.
- Los colores y medidas se definen una vez en `src/login/tokens.css`, según la imagen de paleta y las correcciones aprobadas. El naranja usa texto oscuro para mantener contraste legible.
- Logos oficiales optimizados sin recolorear. La recoloración de una barra de NexusWST espera SVG y aprobación; propuesta: tercera barra azul → acento `#FFAC61`.

## Verificación y compilación

```powershell
npm run lint
npm run typecheck
npm run build
npm run build-keycloak-theme
```

El último comando requiere JDK y Apache Maven. En esta máquina se autorizó Maven 3.9.16 portátil desde Apache, verificando SHA-512, con JDK 11 existente. No se modificó el PATH permanente:

```powershell
$env:JAVA_HOME = 'C:/Program Files/Java/jdk-11'
$taskMavenBin = Join-Path $env:TEMP 'nexuswst-build-tools/apache-maven-3.9.16/bin'
$env:PATH = "$taskMavenBin;$env:JAVA_HOME/bin;$env:PATH"
npm run build-keycloak-theme
```

`npx --no-install` usa la CLI ya instalada, sin descargar otra versión; además evita un fallo observado al invocar directamente la CLI dentro del script npm en Windows. Java/Maven, logs, dependencias, recursos de desarrollo y resultados de compilación no se versionan.

Resultado: `dist_keycloak/nexuswst-keycloak-theme.jar`.

## Integración con Docker

Antes de levantar Docker desde un checkout nuevo, compilar el JAR con el comando anterior. El artefacto no se versiona. El servicio `keycloak` monta:

```yaml
volumes:
  - ./infrastructure/keycloak/theme/dist_keycloak/nexuswst-keycloak-theme.jar:/opt/keycloak/providers/nexuswst-keycloak-theme.jar:ro
```

El realm versionado selecciona `loginTheme=nexuswst`, `rememberMe=true` y política `length(8) and upperCase(1) and digits(1) and notUsername`. Un realm existente no se actualiza con la importación al arrancar: con kcadm previamente autenticado como administrador se aplica:

```powershell
docker compose exec keycloak /opt/keycloak/bin/kcadm.sh update realms/nexuswst -s rememberMe=true -s 'passwordPolicy=length(8) and upperCase(1) and digits(1) and notUsername' -s loginTheme=nexuswst
```

En esta entrega se autorizó la activación y se recreó únicamente Keycloak mediante `docker compose up -d --no-deps --force-recreate keycloak`; después se actualizaron esos campos con kcadm. No se borraron volúmenes ni se cambiaron roles/proveedores.

Se verificó el tema real en 360/768/1280, credenciales incorrectas, ingreso y vuelta al frontend provisional, cierre de sesión, recuperación y un reenvío nativo con dos correos en Mailpit. El enlace más reciente abrió creación de contraseña; Keycloak rechazó la falta de mayúscula o de número y guardó una contraseña válida. Se conservó la credencial de desarrollo original y se comprobó un ingreso posterior con ella. Capturas y reporte local: `respaldo/login-review/deployment-2026-10-29/`. Auth.js/BFF permanece fuera de esta entrega y requiere una tarea aparte.

Referencias primarias: [Keycloakify](https://docs.keycloakify.dev/), [recuperación en Keycloak 26.8.1](https://github.com/keycloak/keycloak/blob/26.8.1/services/src/main/java/org/keycloak/authentication/authenticators/resetcred/ResetCredentialEmail.java), [Apache Maven](https://maven.apache.org/download.cgi).
