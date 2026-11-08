#!/usr/bin/env bash
# =============================================================================
#  Pasos del pipeline de integración continua (H-17-T03 / SCRUM-197).
#  Los mismos pasos corren en GitHub Actions (.github/workflows/ci.yml), en
#  GitLab CI (.gitlab-ci.yml) y en tu equipo, para que «en mi máquina pasa»
#  signifique lo mismo que «en el pipeline pasa».
#
#  Uso:  ./scripts/ci.sh core | orders | frontend | images | all
#
#  core / orders: formato (gofmt), go vet y pruebas con detector de carreras y
#                 cobertura (coverage.out, la lee SonarQube). Las pruebas que
#                 necesitan PostgreSQL se saltan solas sin TEST_DATABASE_URL.
#  frontend:      npm ci, lint, tipos y build de producción de Next.js.
#  images:        construye las imágenes de producción (sin publicarlas).
# =============================================================================
set -euo pipefail
cd "$(dirname "$0")/.."

go_service() {
  local dir=$1
  echo "::: $dir: formato, vet y pruebas"
  (
    cd "$dir"
    unformatted=$(gofmt -l .)
    if [ -n "$unformatted" ]; then
      echo "Archivos sin gofmt (corre: gofmt -w .):"
      echo "$unformatted"
      exit 1
    fi
    go vet ./...
    # Pruebas unitarias: sin base de datos (las de PostgreSQL se saltan).
    TEST_DATABASE_URL="" go test -race -count=1 -covermode=atomic -coverprofile=coverage.out ./...
    go tool cover -func=coverage.out | tail -n 1
  )
}

frontend() {
  echo "::: frontend-web: dependencias, lint, tipos y build"
  (
    cd frontend-web
    npm ci --no-audit --no-fund
    npm run lint
    npm run typecheck
    NEXT_TELEMETRY_DISABLED=1 npm run build
  )
}

images() {
  echo "::: imágenes de producción"
  docker build --target prod -t nexuswst-core:ci backend-core
  docker build --target prod -t nexuswst-orders:ci orders-service
  docker build --target prod -t nexuswst-frontend:ci frontend-web
}

case "${1:-all}" in
  core) go_service backend-core ;;
  orders) go_service orders-service ;;
  frontend) frontend ;;
  images) images ;;
  all) go_service backend-core; go_service orders-service; frontend ;;
  *) echo "uso: $0 core|orders|frontend|images|all" >&2; exit 2 ;;
esac
