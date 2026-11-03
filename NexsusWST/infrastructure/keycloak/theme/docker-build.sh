#!/bin/sh
# Compila el .jar del tema dentro de un contenedor de Node, sin instalar Node, JDK ni Maven
# en la máquina. Se ejecuta con `make theme` (ver README, paso 3).
# Compila en una copia: Keycloakify mueve el .jar desde node_modules, y así tampoco se toca
# el node_modules de la máquina, que puede ser de otro sistema operativo.
set -e
apt-get update -qq
apt-get install -y -qq --no-install-recommends maven >/dev/null
build=/tmp/theme-build
mkdir -p "$build"
tar -C /app --exclude=./node_modules --exclude=./dist --exclude=./dist_keycloak --exclude=./.keycloakify -cf - . | tar -C "$build" -xf -
cd "$build"
npm ci --no-audit --no-fund
npm run build-keycloak-theme
mkdir -p /app/dist_keycloak
cp dist_keycloak/nexuswst-keycloak-theme.jar /app/dist_keycloak/
# En Linux el contenedor escribe como root: devuelve el .jar al dueño de la carpeta.
chown -R "$(stat -c '%u:%g' /app/package.json)" /app/dist_keycloak 2>/dev/null || true
