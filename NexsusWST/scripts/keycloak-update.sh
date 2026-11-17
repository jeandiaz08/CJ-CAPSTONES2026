#!/usr/bin/env bash
# =============================================================================
#  Aplica al realm «nexuswst» YA CREADO los cambios de configuración que el
#  archivo de importación solo aplica en una instalación nueva
#  (Keycloak importa el realm una sola vez: --import-realm no pisa uno existente).
#
#  Uso:  ./scripts/keycloak-update.sh        (o: make keycloak-update)
#
#  1. Usuarios sin correo (H-02-T09 / SCRUM-231): el nombre de usuario deja de
#     ser el correo y el correo pasa a ser opcional en el perfil de usuario.
#     También deja los correos de acceso con el tema «nexuswst-email» (SCRUM-80).
#  2. Ingreso con Microsoft 365 (H-01-T09 / SCRUM-230): crea o actualiza el
#     proveedor «microsoft» con ENTRA_TENANT_ID, ENTRA_CLIENT_ID y
#     ENTRA_CLIENT_SECRET del .env. Si falta alguno, queda deshabilitado y el
#     botón «Continuar con Microsoft» se ve deshabilitado.
#
#  Es idempotente: se puede correr las veces que haga falta.
#  Necesita python3 en el equipo (lee el perfil de usuario del archivo del realm).
#  Variables opcionales: COMPOSE (p. ej. "docker compose -f docker-compose.yml -f docker-compose.prod.yml").
# =============================================================================
set -euo pipefail
cd "$(dirname "$0")/.."

COMPOSE=${COMPOSE:-docker compose}
REALM=nexuswst
REALM_FILE=infrastructure/keycloak/import/nexuswst-realm.json

# Valores del .env (sin exportar todo el archivo)
envval() { grep -E "^$1=" .env 2>/dev/null | tail -n1 | cut -d= -f2- | sed -e 's/^"//' -e 's/"$//' || true; }
TENANT=${ENTRA_TENANT_ID:-$(envval ENTRA_TENANT_ID)}
CLIENT=${ENTRA_CLIENT_ID:-$(envval ENTRA_CLIENT_ID)}
SECRET=${ENTRA_CLIENT_SECRET:-$(envval ENTRA_CLIENT_SECRET)}
ENABLED=false
if [ -n "$TENANT" ] && [ -n "$CLIENT" ] && [ -n "$SECRET" ]; then ENABLED=true; fi

# kcadm dentro del contenedor de Keycloak, con el administrador de arranque.
kc() { $COMPOSE exec -T keycloak /opt/keycloak/bin/kcadm.sh "$@" --config /tmp/kcadm.config; }
$COMPOSE exec -T keycloak sh -c '/opt/keycloak/bin/kcadm.sh config credentials --config /tmp/kcadm.config \
  --server "http://localhost:8080${KC_HTTP_RELATIVE_PATH:-}" --realm master \
  --user "$KC_BOOTSTRAP_ADMIN_USERNAME" --password "$KC_BOOTSTRAP_ADMIN_PASSWORD"' > /dev/null

echo "1/2 Usuarios sin correo (usuario propio, correo opcional) y textos de los correos en español"
kc update "realms/$REALM" -s registrationEmailAsUsername=false -s emailTheme=nexuswst-email
python3 - "$REALM_FILE" <<'EOF' | kc update users/profile -r "$REALM" -f -
import json, sys
realm = json.load(open(sys.argv[1], encoding="utf-8"))
print(realm["components"]["org.keycloak.userprofile.UserProfileProvider"][0]["config"]["kc.user.profile.config"][0])
EOF

echo "2/2 Ingreso con Microsoft 365: $([ "$ENABLED" = true ] && echo "habilitado (tenant $TENANT)" || echo "deshabilitado (faltan datos de Entra ID en el .env)")"
IDP=$(python3 - "$ENABLED" "$TENANT" "$CLIENT" "$SECRET" <<'EOF'
import json, sys
enabled, tenant, client, secret = sys.argv[1:5]
print(json.dumps({
    "alias": "microsoft", "providerId": "microsoft", "displayName": "Microsoft",
    "enabled": enabled == "true", "trustEmail": True, "storeToken": False, "linkOnly": False,
    "hideOnLogin": False, "firstBrokerLoginFlowAlias": "first broker login",
    "config": {"tenantId": tenant, "clientId": client, "clientSecret": secret,
               "defaultScope": "openid profile email", "prompt": "select_account", "syncMode": "IMPORT"},
}))
EOF
)
if kc get "identity-provider/instances/microsoft" -r "$REALM" > /dev/null 2>&1; then
  echo "$IDP" | kc update "identity-provider/instances/microsoft" -r "$REALM" -f -
else
  echo "$IDP" | kc create "identity-provider/instances" -r "$REALM" -f -
fi
$COMPOSE exec -T keycloak rm -f /tmp/kcadm.config
echo "Listo."
