# Atajos (opcionales). En Windows sin make, copia el comando de la derecha.
DC=docker compose
DCPROD=docker compose -f docker-compose.yml -f docker-compose.prod.yml
THEME_DIR=infrastructure/keycloak/theme
THEME_JAR=$(THEME_DIR)/dist_keycloak/nexuswst-keycloak-theme.jar

.PHONY: theme up down logs ps restart rebuild psql redis-cli test-core test-orders tidy migrate-new seed-dev backup restore prod-up prod-down nuke

theme:         ## Compila el .jar del tema de Keycloak (solo requiere Docker)
	docker run --rm -v "$(PWD)/$(THEME_DIR):/app" -w /app node:22-bookworm-slim sh docker-build.sh
$(THEME_JAR):
	$(MAKE) theme
up: $(THEME_JAR) ## Levanta todo en modo desarrollo
	$(DC) up -d --build
down:          ## Detiene todo (conserva datos)
	$(DC) down
logs:          ## Logs en vivo de todos los servicios
	$(DC) logs -f --tail=100
ps:
	$(DC) ps
rebuild:       ## Reconstruye imágenes sin caché
	$(DC) build --no-cache
psql:          ## Consola SQL
	$(DC) exec postgres sh -c 'psql -U $$POSTGRES_USER -d $$POSTGRES_DB'
redis-cli:
	$(DC) exec redis sh -c 'redis-cli -a $$REDIS_PASSWORD --no-auth-warning'
tidy:          ## go mod tidy en ambos servicios Go (genera go.sum)
	$(DC) run --rm --no-deps core go mod tidy
	$(DC) run --rm --no-deps orders go mod tidy
test-core:
	$(DC) run --rm --no-deps core go test ./...
test-orders:
	$(DC) run --rm --no-deps orders go test ./...
migrate-new:   ## make migrate-new svc=backend-core name=crear_tabla_x
	docker run --rm -v "$(PWD)/$(svc)/migrations:/migrations" migrate/migrate:v4.18.3 create -ext sql -dir /migrations -seq $(name)
seed-dev:      ## Vuelve a cargar los datos ficticios de desarrollo (también corre con make up)
	$(DC) run --rm seed-dev
backup:        ## Respaldo de PostgreSQL en ./backups (conserva los últimos 7)
	./scripts/backup.sh
restore:       ## make restore file=backups/nexuswst_AAAAMMDD-HHMMSS.dump (pide confirmación)
	./scripts/restore.sh $(file)
prod-up: $(THEME_JAR)
	$(DCPROD) up -d --build
prod-down:
	$(DCPROD) down
nuke:          ## ¡BORRA volúmenes (BD y Redis)! Solo desarrollo
	$(DC) down -v
