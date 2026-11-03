#!/usr/bin/env bash
# =============================================================================
#  Restauración de PostgreSQL (H-17-T05 / SCRUM-199, PV-15)
#  Uso:  ./scripts/restore.sh backups/nexuswst_<fecha>.dump     (o: make restore file=...)
#        Agrega --yes para no pedir confirmación (por ejemplo en una prueba).
#
#  REEMPLAZA los datos actuales por los del respaldo. Pasos:
#   1. Detiene Keycloak, Core y Órdenes (nadie escribe durante la restauración).
#   2. Restaura en una sola transacción: si algo falla, la base queda como
#      estaba y los servicios se vuelven a levantar.
#   3. Levanta de nuevo Keycloak, Core y Órdenes.
#  Conserva los dueños de cada esquema (la base de Nexus y el esquema
#  «keycloak», que pertenece a su propio usuario).
# =============================================================================
set -euo pipefail
cd "$(dirname "$0")/.."

COMPOSE=${COMPOSE:-docker compose}
FILE=${1:-}
YES=${2:-}

if [ -z "$FILE" ] || [ ! -f "$FILE" ]; then
  echo "Uso: $0 <archivo .dump> [--yes]" >&2
  echo "Respaldos disponibles:" >&2
  ls -1t ./backups/nexuswst_*.dump 2>/dev/null | head -10 >&2 || echo "  (ninguno en ./backups)" >&2
  exit 1
fi

# El archivo debe ser un respaldo válido antes de tocar nada.
if ! $COMPOSE exec -T postgres pg_restore --list < "$FILE" > /dev/null; then
  echo "ERROR: $FILE no es un respaldo válido de pg_dump." >&2
  exit 1
fi

if [ "$YES" != "--yes" ]; then
  echo "ATENCIÓN: se reemplazarán TODOS los datos actuales por los de $FILE."
  read -r -p "Escribe RESTAURAR para continuar: " ans
  [ "$ans" = "RESTAURAR" ] || { echo "Cancelado."; exit 1; }
fi

APPS="keycloak core orders"
echo "Deteniendo $APPS ..."
$COMPOSE stop $APPS
# Pase lo que pase, los servicios vuelven a levantarse.
trap 'echo "Levantando $APPS ..."; $COMPOSE start $APPS' EXIT

echo "Restaurando $FILE ..."
$COMPOSE exec -T postgres sh -c 'pg_restore -U "$POSTGRES_USER" -d "$POSTGRES_DB" --clean --if-exists --single-transaction --exit-on-error' < "$FILE"
echo "Restauración terminada."
