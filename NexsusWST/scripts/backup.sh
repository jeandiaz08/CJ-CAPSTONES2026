#!/usr/bin/env bash
# =============================================================================
#  Respaldo de PostgreSQL (H-17-T05 / SCRUM-199, PV-15)
#  Uso:  ./scripts/backup.sh            (o: make backup)
#
#  - Respalda la base completa: esquema de Nexus (Core y Órdenes) y el
#    esquema «keycloak» (usuarios, roles y sesiones del proveedor de identidad).
#  - Formato «custom» de pg_dump (comprimido) en ./backups/nexuswst_<fecha>.dump
#  - Verifica que el archivo se pueda leer antes de darlo por bueno.
#  - Conserva los últimos BACKUP_KEEP respaldos (por defecto 7).
#  - Redis no se respalda: guarda caché (se reconstruye) y colas con AOF.
#
#  Variables opcionales: BACKUP_DIR, BACKUP_KEEP, COMPOSE (por ejemplo
#  COMPOSE="docker compose -f docker-compose.yml -f docker-compose.prod.yml").
# =============================================================================
set -euo pipefail
cd "$(dirname "$0")/.."

COMPOSE=${COMPOSE:-docker compose}
DIR=${BACKUP_DIR:-./backups}
KEEP=${BACKUP_KEEP:-7}
FILE="$DIR/nexuswst_$(date +%Y%m%d-%H%M%S).dump"

mkdir -p "$DIR"
# Si algo falla a mitad de camino, no queda un archivo a medias.
trap 'rm -f "$FILE.tmp"' EXIT
echo "Respaldando PostgreSQL en $FILE ..."
# -T: sin TTY, para que el binario llegue intacto por la salida estándar.
$COMPOSE exec -T postgres sh -c 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --format=custom --compress=6' > "$FILE.tmp"

# Un respaldo que no se puede leer no sirve: se valida con pg_restore --list.
if ! $COMPOSE exec -T postgres pg_restore --list < "$FILE.tmp" > /dev/null; then
  rm -f "$FILE.tmp"
  echo "ERROR: el respaldo quedó corrupto; no se guardó." >&2
  exit 1
fi
mv "$FILE.tmp" "$FILE"

# Rotación: deja solo los últimos $KEEP
ls -1t "$DIR"/nexuswst_*.dump 2>/dev/null | tail -n +$((KEEP + 1)) | while read -r old; do
  rm -f -- "$old" && echo "Eliminado respaldo antiguo: $old"
done

echo "Listo: $FILE ($(du -h "$FILE" | cut -f1))"
