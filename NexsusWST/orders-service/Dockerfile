# syntax=docker/dockerfile:1
# =============================================================================
#  Orders Service (Go hexagonal) - Dockerfile multi-etapa
#    target "dev"  -> Go + Air (recarga en caliente), código montado por volumen
#    target "prod" -> binario estático en imagen distroless, usuario sin root
# =============================================================================

ARG GO_VERSION=1.26

# ---------- Base común --------------------------------------------------------
FROM golang:${GO_VERSION}-alpine AS base
WORKDIR /app
RUN apk add --no-cache git ca-certificates tzdata
ENV CGO_ENABLED=0 \
    GOTOOLCHAIN=local

# ---------- Desarrollo (hot reload) -------------------------------------------
FROM base AS dev
RUN go install github.com/air-verse/air@v1.61.7
# El código llega por bind mount (docker-compose.override.yml)
EXPOSE 8081
CMD ["air", "-c", ".air.toml"]

# ---------- Compilación -------------------------------------------------------
FROM base AS build
# 1) Primero solo go.mod/go.sum -> la descarga de dependencias queda en caché.
#    go.sum se genera la primera vez que corres en dev (ver README) y se sube a Git.
COPY go.mod go.sum* ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
# 2) Luego el código
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -mod=mod -trimpath -ldflags="-s -w" -o /out/api ./cmd/api
# Carpeta de adjuntos: el volumen "attachments" hereda este dueño (nonroot = 65532)
RUN mkdir -p /out/data/attachments && chown -R 65532:65532 /out/data

# ---------- Producción --------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot AS prod
WORKDIR /app
COPY --from=build /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=build /out/api /app/api
COPY --from=build --chown=65532:65532 /out/data /data
USER nonroot:nonroot
EXPOSE 8081
ENTRYPOINT ["/app/api"]
