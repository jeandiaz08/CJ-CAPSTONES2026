#!/usr/bin/env bash
# =============================================================================
#  Revisa que los rangos de IP de Cloudflare configurados en Traefik sigan al
#  día con los publicados en https://www.cloudflare.com/ips (H-17-T08).
#  Cloudflare los cambia muy rara vez; conviene correrlo antes de desplegar.
#  Uso: ./scripts/cloudflare-ips.sh   (sale con 1 si hay diferencias)
# =============================================================================
set -euo pipefail
cd "$(dirname "$0")/.."

publicados=$( { curl -fsS https://www.cloudflare.com/ips-v4; echo; curl -fsS https://www.cloudflare.com/ips-v6; } | grep -v '^$' | sort -u)
for f in infrastructure/traefik/traefik.cloudflare.yml infrastructure/traefik/dynamic/cloudflare.yml; do
  configurados=$(grep -oE '"[0-9a-f:.]+/[0-9]+"' "$f" | tr -d '"' | sort -u)
  if [ "$publicados" != "$configurados" ]; then
    echo "DIFERENCIAS en $f (< publicados por Cloudflare, > configurados):"
    diff <(echo "$publicados") <(echo "$configurados") || true
    falla=1
  else
    echo "OK $f: $(echo "$configurados" | wc -l) rangos al día"
  fi
done
exit ${falla:-0}
