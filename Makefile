# learning/Makefile
DB_LOCAL := $(shell grep -E '^DATABASE_URL=' backend/.env 2>/dev/null | cut -d'=' -f2-)
DB_PROD  ?= $(PROD_DATABASE_URL)
DB       ?= local
DATABASE_URL := $(if $(filter prod,$(DB)),$(DB_PROD),$(DB_LOCAL))

ENC_KEY_LOCAL := $(shell grep -E '^ENCRYPTION_KEY=' backend/.env 2>/dev/null | cut -d'=' -f2-)
ENCRYPTION_KEY ?= $(ENC_KEY_LOCAL)

# ── Docker Hub ────────────────────────────────────────────────────────────────
DOCKER_USER  ?= cbaciliod
DOCKER_TAG   ?= v1.0.0
BACKEND_IMG  := $(DOCKER_USER)/learning-backend:$(DOCKER_TAG)
FRONTEND_IMG := $(DOCKER_USER)/learning-frontend:$(DOCKER_TAG)

# Frontend public vars (baked at build time)
NEXT_PUBLIC_API_URL ?= https://api-capacitate.bacsystemsolutions.com
# Sin valor por defecto: se toman del entorno o de la línea de make (H117).
NEXT_PUBLIC_STORAGE_PREFIX    ?=
NEXT_PUBLIC_CONTACT_EMAIL     ?=

# ── Docker Compose (local) ────────────────────────────────────────────────────
# Lee la configuración del .env de la raíz (ver .env.example).
COMPOSE    := docker compose
root_env    = $(shell grep -E '^$(1)=' .env 2>/dev/null | cut -d'=' -f2-)
C_PG_USER  := $(or $(call root_env,POSTGRES_USER),learning)
C_PG_PASS  := $(call root_env,POSTGRES_PASSWORD)
C_PG_DB    := $(or $(call root_env,POSTGRES_DB),learning)
C_DB_PORT  := $(or $(call root_env,DB_PORT),5433)
C_DB_URL   := postgres://$(C_PG_USER):$(C_PG_PASS)@localhost:$(C_DB_PORT)/$(C_PG_DB)?sslmode=disable
C_ENC_KEY  := $(call root_env,ENCRYPTION_KEY)
s          ?=

.PHONY: dev dev/backend dev/frontend \
        test test/backend test/frontend \
        lint build \
        migrate/up migrate/down migrate/status migrate/create \
        fix-encrypt-personas \
        docker/build docker/push docker/release \
        docker/build/backend docker/build/frontend \
        compose/init compose/check-env compose/up compose/down compose/reset \
        compose/restart compose/rebuild compose/logs compose/ps compose/psql \
        compose/migrate/status compose/seed/admin \
        clean

# ── Desarrollo ────────────────────────────────────────────────────────────────
dev:
	@$(MAKE) -j2 dev/backend dev/frontend

dev/backend:
	cd backend && air

dev/frontend:
	cd frontend && npm run dev

# ── Tests ─────────────────────────────────────────────────────────────────────
test:
	cd backend && go test ./... -race
	cd frontend && npm run test:coverage

test/backend:
	cd backend && go test ./... -v -race -coverprofile=coverage.out

test/frontend:
	cd frontend && npm run test:coverage

# ── Lint ──────────────────────────────────────────────────────────────────────
lint:
	cd backend && go vet ./... && go fmt ./...
	cd frontend && npm run lint

# ── Build ─────────────────────────────────────────────────────────────────────
build:
	cd backend && CGO_ENABLED=0 go build -o bin/server ./cmd/server
	cd frontend && npm run build

# ── Migraciones (DB de backend/.env, o DB=prod con PROD_DATABASE_URL) ────────
migrate/up:
	cd backend && goose -dir migrations postgres "$(DATABASE_URL)" up

migrate/down:
	cd backend && goose -dir migrations postgres "$(DATABASE_URL)" down

migrate/status:
	cd backend && goose -dir migrations postgres "$(DATABASE_URL)" status

# Uso: make migrate/create name=add_algo
migrate/create:
	@test -n "$(name)" || (echo "ERROR: set name=nombre_de_la_migracion" && exit 1)
	cd backend && goose -dir migrations create $(name) sql

# Genera los UPDATE para cifrar personas con DNI en texto plano. Uso: make fix-encrypt-personas [DB=prod]
fix-encrypt-personas:
	@test -n "$(ENCRYPTION_KEY)" || (echo "ERROR: ENCRYPTION_KEY vacío (backend/.env o ENCRYPTION_KEY=...)" && exit 1)
	@echo "==> Generando SQL para cifrar personas con DNI en texto plano (DB=$(DB))..."
	@cd backend && DATABASE_URL="$(DATABASE_URL)" ENCRYPTION_KEY="$(ENCRYPTION_KEY)" go run ./cmd/fix-encrypt-personas/

# ── Docker (imágenes para Docker Hub) ─────────────────────────────────────────
docker/build/backend:
	docker build --platform linux/amd64 --no-cache -t $(BACKEND_IMG) backend/

docker/build/frontend:
	@test -n "$(NEXT_PUBLIC_API_URL)" || (echo "ERROR: set NEXT_PUBLIC_API_URL" && exit 1)
	docker build --platform linux/amd64 \
		--build-arg "NEXT_PUBLIC_API_URL=$(NEXT_PUBLIC_API_URL)" \
		--build-arg "NEXT_PUBLIC_STORAGE_PREFIX=$(NEXT_PUBLIC_STORAGE_PREFIX)" \
		--build-arg "NEXT_PUBLIC_CONTACT_EMAIL=$(NEXT_PUBLIC_CONTACT_EMAIL)" \
		-t $(FRONTEND_IMG) frontend/

docker/build: docker/build/backend docker/build/frontend

docker/push:
	docker push $(BACKEND_IMG)
	docker push $(FRONTEND_IMG)

# build + push en un solo paso
docker/release: docker/build docker/push
	@echo "✓ $(BACKEND_IMG) y $(FRONTEND_IMG) publicados en Docker Hub"

# ── Docker Compose (local) ────────────────────────────────────────────────────
## Uso: make compose/init → make compose/up → http://localhost:3000
## s=<servicio> limita logs/restart/rebuild a db | backend | frontend

# Crea .env desde .env.example: JWT_SECRET y POSTGRES_PASSWORD aleatorios y ENCRYPTION_KEY
# de backend/.env (si es válida, para que los datos cifrados sean compatibles) o aleatoria.
# No pisa un .env existente. ENV_EXAMPLE/ENV_OUT permiten probarlo sobre otros archivos.
ENV_EXAMPLE ?= .env.example
ENV_OUT     ?= .env
compose/init:
	@if [ -f "$(ENV_OUT)" ]; then echo "$(ENV_OUT) ya existe — no se modifica"; exit 0; fi; \
	key=$$(grep -E '^ENCRYPTION_KEY=[0-9a-fA-F]{64}$$' backend/.env 2>/dev/null | cut -d= -f2-); src="backend/.env"; \
	[ -n "$$key" ] || { key=$$(openssl rand -hex 32); src="generada"; }; \
	jwt=$$(openssl rand -hex 32); pgpw=$$(openssl rand -hex 24); \
	[ -n "$$jwt" ] && [ -n "$$pgpw" ] || { echo "ERROR: openssl rand falló"; exit 1; }; \
	sed -e "s/^JWT_SECRET=$$/JWT_SECRET=$$jwt/" -e "s/^ENCRYPTION_KEY=$$/ENCRYPTION_KEY=$$key/" \
	    -e "s/^POSTGRES_PASSWORD=$$/POSTGRES_PASSWORD=$$pgpw/" "$(ENV_EXAMPLE)" > "$(ENV_OUT)"; \
	echo "✓ $(ENV_OUT) creado (JWT_SECRET y POSTGRES_PASSWORD generados, ENCRYPTION_KEY: $$src)"

compose/check-env:
	@test -f .env || (echo "ERROR: falta .env — ejecuta: make compose/init" && exit 1)
	@test -n "$(C_PG_PASS)" || (echo "ERROR: POSTGRES_PASSWORD vacío en .env (genera uno con: openssl rand -hex 24)" && exit 1)

# Construye (si hace falta) y levanta db + backend (corre migraciones) + frontend
compose/up: compose/check-env
	$(COMPOSE) up -d --build
	@$(COMPOSE) ps

# Detiene y elimina los contenedores (conserva los datos de Postgres)
compose/down:
	$(COMPOSE) down

# Detiene todo y BORRA el volumen de Postgres (base de datos vacía al volver a levantar)
compose/reset:
	$(COMPOSE) down -v

compose/restart:
	$(COMPOSE) restart $(s)

# Reconstruye sin caché; necesario si cambian las NEXT_PUBLIC_* del .env (s=frontend)
compose/rebuild: compose/check-env
	$(COMPOSE) build --no-cache $(s)
	$(COMPOSE) up -d $(s)

compose/logs:
	$(COMPOSE) logs -f --tail=100 $(s)

compose/ps:
	$(COMPOSE) ps

# Consola psql interactiva dentro del contenedor de Postgres
compose/psql:
	$(COMPOSE) exec db sh -c 'psql -U "$$POSTGRES_USER" -d "$$POSTGRES_DB"'

compose/migrate/status:
	$(COMPOSE) exec backend sh -c 'goose -dir ./migrations postgres "$$DATABASE_URL" status'

# Crea el usuario admin con una persona dedicada (no una persona real del padrón).
# La contraseña (12-72 bytes) se pide por teclado y se pasa por stdin: no queda
# en el historial ni en `ps`. ADMIN_REBIND=1 re-liga un admin existente.
# Uso: make compose/seed/admin ADMIN_EMAIL=admin@tu-dominio [ADMIN_USERNAME=admin] [ADMIN_REBIND=1]
compose/seed/admin: compose/check-env
	@test -n "$(ADMIN_EMAIL)" || (echo "ERROR: set ADMIN_EMAIL=..." && exit 1)
	@test -n "$(C_ENC_KEY)" || (echo "ERROR: ENCRYPTION_KEY vacío en .env" && exit 1)
	@printf "Contraseña del admin: "; stty -echo; read -r pw; stty echo; echo; \
	cd backend && printf '%s\n' "$$pw" | DATABASE_URL="$(C_DB_URL)" ENCRYPTION_KEY="$(C_ENC_KEY)" \
		ADMIN_EMAIL="$(ADMIN_EMAIL)" ADMIN_USERNAME="$(ADMIN_USERNAME)" ADMIN_REBIND="$(ADMIN_REBIND)" \
		go run ./cmd/seed/admin

# ── Limpieza ──────────────────────────────────────────────────────────────────
clean:
	cd backend && rm -rf bin/ tmp/ coverage.out
	cd frontend && rm -rf .next/ coverage/
