# Atajos (opcionales). En Windows sin make, copia el comando de la derecha.
DC=docker compose
DCPROD=docker compose -f docker-compose.yml -f docker-compose.prod.yml

.PHONY: up down logs ps restart rebuild psql redis-cli test-core test-orders tidy migrate-new prod-up prod-down nuke

up:            ## Levanta todo en modo desarrollo
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
prod-up:
	$(DCPROD) up -d --build
prod-down:
	$(DCPROD) down
nuke:          ## ¡BORRA volúmenes (BD y Redis)! Solo desarrollo
	$(DC) down -v
