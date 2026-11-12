# learning/Makefile — `make help` lista los targets.
#
# Grupos: dev/ (desarrollo), test · lint · fmt · build (= CI), migrate/ (base de
# backend/.env), docker/ (imágenes de release), compose/ (stack local), clean.
.DEFAULT_GOAL := help

# ── Versiones de herramientas: las mismas que .github/workflows/ ──────────────
# Si cambias una, cámbiala también en el workflow (docs/ops/paridad-ci-local.md).
GOOSE_VERSION         := v3.21.1
GOVULNCHECK_VERSION   := v1.9.0
GOLANGCI_LINT_VERSION := v2.15.0
GOOSE := go run github.com/pressly/goose/v3/cmd/goose@$(GOOSE_VERSION)

# Base de desarrollo de backend/.env (migrate/* y fix-encrypt-personas).
# Producción (Railway) no se toca desde aquí: su base no tiene acceso público.
DATABASE_URL   ?= $(shell grep -E '^DATABASE_URL=' backend/.env 2>/dev/null | cut -d'=' -f2-)
ENCRYPTION_KEY ?= $(shell grep -E '^ENCRYPTION_KEY=' backend/.env 2>/dev/null | cut -d'=' -f2-)

# Base de tests de integración (la misma que usa el CI). `make test` la exige:
# sin ella los tests de integración se saltan y el resultado no es el del CI.
# OJO: `make test` hace `down-to 0` sobre esta base; usa una base desechable.
TEST_DATABASE_URL ?=

# ── Imágenes de release (Docker Hub, repos públicos) ──────────────────────────
# Las construye y publica el usuario desde un checkout limpio del tag
# (docs/ops/checklist-release.md §5). Ningún agente hace docker login/push.
DOCKER_USER  ?= cbaciliod
# Sin valor por defecto: siempre el tag del release (DOCKER_TAG=v1.0.0).
DOCKER_TAG   ?=
# Versión que reporta el backend (/health): el tag sin la «v».
VERSION      := $(DOCKER_TAG:v%=%)
BACKEND_IMG  := $(DOCKER_USER)/learning-backend:$(DOCKER_TAG)
# Cron de copia diaria a R2 (deploy/backup/, ADR 0004 D11).
BACKUP_IMG   := $(DOCKER_USER)/learning-backup:$(DOCKER_TAG)
# La imagen del frontend es por entorno (NEXT_PUBLIC_API_URL se incrusta al construir):
# producción = vX.Y.Z; otro entorno = vX.Y.Z-<entorno> (docs/ops/railway.md §2).
FRONTEND_ENV ?= prod
FRONTEND_IMG := $(DOCKER_USER)/learning-frontend:$(DOCKER_TAG)$(if $(filter prod,$(FRONTEND_ENV)),,-$(FRONTEND_ENV))
# OJO: no exportes esta variable desde make (`export NEXT_PUBLIC_API_URL`): el
# target build la heredaría y, en silencio, incrustaría la URL de producción en el
# build local. Solo se pasa como build-arg en docker/build/frontend.
NEXT_PUBLIC_API_URL        ?= https://api-capacitate.bacsystemsolutions.com
NEXT_PUBLIC_STORAGE_PREFIX ?=
NEXT_PUBLIC_CONTACT_EMAIL  ?=

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

.PHONY: help dev dev/backend dev/frontend test lint fmt build \
        migrate/up migrate/down migrate/status migrate/create fix-encrypt-personas \
        docker/build docker/build/backend docker/build/frontend docker/verify \
        docker/push docker/release docker/build/backup docker/push/backup backup/test \
        compose/init compose/up compose/down compose/reset compose/restart \
        compose/rebuild compose/logs compose/ps compose/psql \
        compose/migrate/status compose/seed/admin clean \
        check/test-db check/docker-tag check/release-tree compose/check-env

help: ## Lista los targets
	@awk 'BEGIN{FS=":.*## "} /^[a-z][a-zA-Z0-9_\/-]+:.*## /{printf "  \033[36m%-24s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# ── Desarrollo ────────────────────────────────────────────────────────────────
dev: ## Backend (air) y frontend (next dev) en paralelo
	@$(MAKE) -j2 dev/backend dev/frontend

dev/backend: ## Backend con recarga en caliente (air, backend/.air.toml)
	cd backend && air

dev/frontend: ## Frontend en http://localhost:3000 (next dev)
	cd frontend && npm run dev

# ── Verificación (= .github/workflows/) ───────────────────────────────────────
check/test-db:
	@test -n "$(TEST_DATABASE_URL)" || (echo "ERROR: set TEST_DATABASE_URL=postgres://... (base desechable: se hace down-to 0)" && exit 1)
	@echo "$(TEST_DATABASE_URL)" | grep -qE '/[^/?]+_test(_[0-9]+)?([?]|$$)' || (echo "ERROR: el nombre de la base de TEST_DATABASE_URL debe terminar en _test o _test_<n> (make test hace down-to 0)" && exit 1)

test: check/test-db ## Tests como el CI (exige TEST_DATABASE_URL a una base *_test o *_test_<n>)
	cd backend && go mod tidy && git diff --exit-code -- go.mod go.sum
	cd backend && go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...
	cd backend && $(GOOSE) -dir migrations postgres "$(TEST_DATABASE_URL)" up
	cd backend && $(GOOSE) -dir migrations postgres "$(TEST_DATABASE_URL)" down-to 0
	cd backend && $(GOOSE) -dir migrations postgres "$(TEST_DATABASE_URL)" up
	cd backend && TEST_DATABASE_URL="$(TEST_DATABASE_URL)" go test -race -count=1 ./...
	cd frontend && npm test

lint: ## Lint como el CI: go vet, golangci-lint, tsc, eslint, i18n, npm audit (no reescribe)
	cd backend && go vet ./...
	cd backend && go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run --timeout=5m ./...
	cd frontend && npx tsc --noEmit
	cd frontend && npm run lint
	cd frontend && node scripts/check-i18n.mjs
	cd frontend && npm audit --omit=dev --audit-level=high

fmt: ## Formatea: go fmt y eslint --fix
	cd backend && go fmt ./...
	cd frontend && npm run lint:fix

# Comprobación de paridad con el CI, no un artefacto de producción: producción
# sale solo de docker/build (build-arg obligatorio). NEXT_PUBLIC_API_URL: la del
# entorno o, como frontend.yml, http://localhost:8080. Por eso la variable de make
# del mismo nombre (arriba, con la URL de producción) NO debe exportarse: si se
# exporta, este build incrusta la URL de producción sin avisar.
build: ## Build como el CI: go build ./... + bin/server y next build
	cd backend && go build ./...
	cd backend && CGO_ENABLED=0 go build -o bin/server ./cmd/server
	cd frontend && NEXT_PUBLIC_API_URL="$${NEXT_PUBLIC_API_URL:-http://localhost:8080}" npm run build

# ── Migraciones (base de backend/.env o DATABASE_URL=...) ─────────────────────
migrate/up: ## Aplica las migraciones pendientes
	cd backend && $(GOOSE) -dir migrations postgres "$(DATABASE_URL)" up

migrate/down: ## Revierte la última migración
	cd backend && $(GOOSE) -dir migrations postgres "$(DATABASE_URL)" down

migrate/status: ## Estado de las migraciones
	cd backend && $(GOOSE) -dir migrations postgres "$(DATABASE_URL)" status

# Numeración secuencial (-s) de 3 dígitos como las existentes (001…). Sin -s,
# goose usa una marca de tiempo y, una vez aplicada, toda migración numerada
# posterior quedaría fuera de orden.
migrate/create: ## Crea la siguiente migración numerada (name=add_algo)
	@test -n "$(name)" || (echo "ERROR: set name=nombre_de_la_migracion" && exit 1)
	cd backend && $(GOOSE) -dir migrations -s create $(name) sql \
	  && f=$$(ls migrations/00[0-9][0-9][0-9]_$(name).sql) \
	  && mv "$$f" "migrations/$$(basename "$$f" | cut -c3-)" \
	  && echo "→ migrations/$$(basename "$$f" | cut -c3-)"

fix-encrypt-personas: ## Genera el SQL para cifrar personas en texto plano (bases antiguas, puesta-en-marcha §5)
	@test -n "$(ENCRYPTION_KEY)" || (echo "ERROR: ENCRYPTION_KEY vacío (backend/.env o ENCRYPTION_KEY=...)" && exit 1)
	@cd backend && DATABASE_URL="$(DATABASE_URL)" ENCRYPTION_KEY="$(ENCRYPTION_KEY)" go run ./cmd/fix-encrypt-personas/

# ── Imágenes de release ───────────────────────────────────────────────────────
check/docker-tag:
	@echo "$(DOCKER_TAG)" | grep -qE '^v[0-9]+\.[0-9]+\.[0-9]+$$' || (echo "ERROR: set DOCKER_TAG=vX.Y.Z (el tag del release)" && exit 1)

# La imagen sale del tag, nunca del árbol de trabajo.
check/release-tree: check/docker-tag
	@test -z "$$(git status --porcelain)" || (echo "ERROR: hay cambios sin commit o archivos sin seguimiento; construye desde un checkout limpio del tag" && exit 1)
	@git tag --points-at HEAD | grep -qxF "$(DOCKER_TAG)" || (echo "ERROR: HEAD no es el tag $(DOCKER_TAG) (git switch --detach $(DOCKER_TAG))" && exit 1)
	@v=$$(sed -nE 's/^  "version": "([^"]+)".*/\1/p' frontend/package.json); \
	  test "$$v" = "$(VERSION)" || (echo "ERROR: frontend/package.json dice $$v y el tag es $(DOCKER_TAG); alinéalo en release/$(VERSION)" && exit 1)

docker/build/backend: check/release-tree ## Imagen del backend (linux/amd64) desde el tag DOCKER_TAG
	docker build --platform linux/amd64 --no-cache --build-arg "VERSION=$(VERSION)" -t $(BACKEND_IMG) backend/

docker/build/frontend: check/release-tree ## Imagen del frontend para FRONTEND_ENV (prod por defecto) con NEXT_PUBLIC_API_URL
	@test -n "$(NEXT_PUBLIC_API_URL)" || (echo "ERROR: set NEXT_PUBLIC_API_URL" && exit 1)
	docker build --platform linux/amd64 --no-cache \
		--build-arg "NEXT_PUBLIC_API_URL=$(NEXT_PUBLIC_API_URL)" \
		--build-arg "NEXT_PUBLIC_STORAGE_PREFIX=$(NEXT_PUBLIC_STORAGE_PREFIX)" \
		--build-arg "NEXT_PUBLIC_CONTACT_EMAIL=$(NEXT_PUBLIC_CONTACT_EMAIL)" \
		-t $(FRONTEND_IMG) frontend/

docker/build: docker/build/backend docker/build/frontend ## Las dos imágenes desde el tag DOCKER_TAG

docker/verify: check/docker-tag ## Las imágenes no llevan .env*, assets/, bin/, tests ni secretos (repos públicos)
	deploy/verify-image.sh $(BACKEND_IMG)
	deploy/verify-image.sh $(FRONTEND_IMG)

docker/push: check/release-tree docker/verify ## Publica en Docker Hub (solo el usuario, con su sesión)
	docker push $(BACKEND_IMG)
	docker push $(FRONTEND_IMG)

docker/release: docker/build docker/push ## build + verify + push (solo el usuario)
	@echo "✓ $(BACKEND_IMG) y $(FRONTEND_IMG) publicados en Docker Hub"

# El cron de copia va aparte: solo se publica cuando cambia deploy/backup/.
docker/build/backup: check/release-tree ## Imagen del cron de copia a R2 (linux/amd64) desde el tag
	docker build --platform linux/amd64 --no-cache -t $(BACKUP_IMG) deploy/backup/

docker/push/backup: check/release-tree ## Verifica y publica la imagen del cron (solo el usuario)
	deploy/verify-image.sh $(BACKUP_IMG)
	docker push $(BACKUP_IMG)

backup/test: ## Prueba de punta a punta del cron de copia (Postgres y S3 desechables, sin R2)
	deploy/backup/test.sh

# ── Docker Compose (local) ────────────────────────────────────────────────────
# Uso: make compose/init → make compose/up → http://localhost:3000
# s=<servicio> limita logs/restart/rebuild a db | backend | frontend

# Crea .env desde .env.example: JWT_SECRET y POSTGRES_PASSWORD aleatorios y ENCRYPTION_KEY
# de backend/.env (si es válida, para que los datos cifrados sean compatibles) o aleatoria.
# No pisa un .env existente. ENV_EXAMPLE/ENV_OUT permiten probarlo sobre otros archivos.
ENV_EXAMPLE ?= .env.example
ENV_OUT     ?= .env
compose/init: ## Crea .env desde .env.example con secretos aleatorios (no pisa uno existente)
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

compose/up: compose/check-env ## Levanta db + backend (migra) + frontend
	$(COMPOSE) up -d --build
	@$(COMPOSE) ps

compose/down: ## Detiene el stack (conserva los datos)
	$(COMPOSE) down

compose/reset: ## Detiene el stack y BORRA el volumen de Postgres
	$(COMPOSE) down -v

compose/restart: ## Reinicia servicios (s=<servicio>)
	$(COMPOSE) restart $(s)

compose/rebuild: compose/check-env ## Reconstruye sin caché (necesario si cambian las NEXT_PUBLIC_*; s=frontend)
	$(COMPOSE) build --no-cache $(s)
	$(COMPOSE) up -d $(s)

compose/logs: ## Logs en vivo (s=<servicio>)
	$(COMPOSE) logs -f --tail=100 $(s)

compose/ps: ## Estado de los contenedores
	$(COMPOSE) ps

compose/psql: ## Consola psql dentro del contenedor de Postgres
	$(COMPOSE) exec db sh -c 'psql -U "$$POSTGRES_USER" -d "$$POSTGRES_DB"'

compose/migrate/status: ## Estado de las migraciones dentro del backend
	$(COMPOSE) exec backend sh -c 'goose -dir ./migrations postgres "$$DATABASE_URL" status'

# Crea el usuario admin con una persona dedicada (no una persona real del padrón).
# La contraseña (12-72 bytes) se pide por teclado y se pasa por stdin: no queda
# en el historial ni en `ps`. ADMIN_REBIND=1 re-liga un admin existente.
compose/seed/admin: compose/check-env ## Crea el admin (ADMIN_EMAIL=..., contraseña por teclado)
	@test -n "$(ADMIN_EMAIL)" || (echo "ERROR: set ADMIN_EMAIL=..." && exit 1)
	@test -n "$(C_ENC_KEY)" || (echo "ERROR: ENCRYPTION_KEY vacío en .env" && exit 1)
	@printf "Contraseña del admin: "; stty -echo; read -r pw; stty echo; echo; \
	cd backend && printf '%s\n' "$$pw" | DATABASE_URL="$(C_DB_URL)" ENCRYPTION_KEY="$(C_ENC_KEY)" \
		ADMIN_EMAIL="$(ADMIN_EMAIL)" ADMIN_USERNAME="$(ADMIN_USERNAME)" ADMIN_REBIND="$(ADMIN_REBIND)" \
		go run ./cmd/seed/admin

# ── Preproducción local para qa (deploy/local/) ───────────────────────────────
# Las MISMAS imágenes que van a Railway: TAG=vX.Y.Z usa las publicadas
# (backend vX.Y.Z, frontend vX.Y.Z-preprod); BUILD=1 las construye con los
# mismos Dockerfiles. Variables falsas en deploy/local/preprod.vars.
# Uso: make local/up TAG=v1.0.0 | make local/up BUILD=1 → http://localhost:3110
LOCAL_DIR  := deploy/local
TAG        ?=
BUILD      ?=
local_var   = $(shell grep -E '^$(1)=' $(LOCAL_DIR)/preprod.vars | cut -d'=' -f2-)
L_PG_USER  := $(call local_var,PREPROD_PG_USER)
L_PG_PASS  := $(call local_var,PREPROD_PG_PASSWORD)
L_PG_DB    := $(call local_var,PREPROD_PG_DB)
L_ENC_KEY  := $(call local_var,PREPROD_ENCRYPTION_KEY)
L_DEMO_ADM := $(call local_var,PREPROD_DEMO_ADMIN_PASSWORD)
L_DEMO_USR := $(call local_var,PREPROD_DEMO_POSTULANTE_PASSWORD)
L_BACKEND  := $(if $(BUILD),capacitate-preprod/backend:local,$(DOCKER_USER)/learning-backend:$(TAG))
L_FRONTEND := $(if $(BUILD),capacitate-preprod/frontend:local,$(DOCKER_USER)/learning-frontend:$(TAG)-preprod)
# Para los comandos que no arrancan servicios, compose igual exige las imágenes.
L_IMAGES    = BACKEND_IMAGE="$(or $(1),unused)" FRONTEND_IMAGE="$(or $(2),unused)"
LOCAL_COMPOSE := docker compose --env-file $(LOCAL_DIR)/preprod.vars -f $(LOCAL_DIR)/compose.yml \
                 $(if $(BUILD),-f $(LOCAL_DIR)/compose.build.yml)

.PHONY: local/up local/down local/reset local/seed local/test local/logs local/ps local/psql \
        local/certs check/local-images check/local-owner

# Hay UNA sola preproducción por máquina (proyecto capacitate-preprod, puertos
# fijos). Si ya corre desde OTRO checkout, up/down/reset/seed abortan: así nadie
# pisa ni borra la de otro (qa, otro agente). Para usarla desde aquí, primero
# `make local/down` en el checkout que la levantó (o ver deploy/local/README.md).
L_MY_COMPOSE := $(CURDIR)/$(LOCAL_DIR)/compose.yml
check/local-owner:
	@cfg=$$(docker compose ls --all --filter name=capacitate-preprod 2>/dev/null | awk '$$1=="capacitate-preprod"{print $$NF}'); \
	if [ -n "$$cfg" ] && ! echo "$$cfg" | tr ',' '\n' | grep -qxF "$(L_MY_COMPOSE)"; then \
	  echo "ERROR: la preproducción ya corre desde otro checkout:"; echo "  $$cfg" | tr ',' '\n' | sed -n 1p; \
	  echo "  Usa ese checkout, o pide a su dueño un 'make local/down' allí. No se tocó nada."; exit 1; fi

check/local-images:
	@test -n "$(BUILD)" || echo "$(TAG)" | grep -qE '^v[0-9]+\.[0-9]+\.[0-9]+$$' || \
	  (echo "ERROR: set TAG=vX.Y.Z (imágenes publicadas) o BUILD=1 (build local)" && exit 1)

local/certs:
	@test -f $(LOCAL_DIR)/certs/server.crt || { mkdir -p $(LOCAL_DIR)/certs && \
	  openssl req -x509 -newkey rsa:2048 -nodes -days 825 -subj "/CN=db" \
	    -keyout $(LOCAL_DIR)/certs/server.key -out $(LOCAL_DIR)/certs/server.crt 2>/dev/null && \
	  echo "✓ certificado autofirmado de Postgres en $(LOCAL_DIR)/certs/"; }

local/up: check/local-images check/local-owner local/certs ## Levanta la preproducción (TAG=vX.Y.Z o BUILD=1): db TLS, mailpit, backend (migra), frontend
	$(call L_IMAGES,$(L_BACKEND),$(L_FRONTEND)) $(LOCAL_COMPOSE) up -d --wait $(if $(BUILD),--build)
	@$(call L_IMAGES) $(LOCAL_COMPOSE) ps

local/down: check/local-owner ## Detiene la preproducción (conserva los datos)
	$(call L_IMAGES) $(LOCAL_COMPOSE) down

local/seed: check/local-owner ## Carga los datos sintéticos de qa (cmd/seed/demo)
	@test -d backend/cmd/seed/demo || (echo "PENDIENTE: falta backend/cmd/seed/demo (backend-dev); crea catálogos, admin y usuarios sintéticos." && exit 1)
	docker run --rm --network capacitate-preprod_default \
	  -v "$(CURDIR)/backend:/src:ro" -v capacitate-preprod-gomod:/go/pkg/mod -w /src \
	  -e GOFLAGS=-buildvcs=false -e ENV=preprod \
	  -e DATABASE_URL="postgres://$(L_PG_USER):$(L_PG_PASS)@db:5432/$(L_PG_DB)?sslmode=require" \
	  -e ENCRYPTION_KEY="$(L_ENC_KEY)" \
	  -e DEMO_ADMIN_PASSWORD="$(L_DEMO_ADM)" -e DEMO_POSTULANTE_PASSWORD="$(L_DEMO_USR)" \
	  golang:1.27.4-alpine3.23 go run ./cmd/seed/demo

# Pide confirmación (escribir «borrar»); sin terminal: CONFIRM=borrar.
local/reset: check/local-images check/local-owner ## BORRA la base de preproducción (pide confirmación o CONFIRM=borrar), levanta, migra y siembra
	@c="$(CONFIRM)"; if [ -z "$$c" ]; then printf "Se BORRAN los datos de la preproducción local. Escribe «borrar» para seguir: "; read -r c; fi; \
	  [ "$$c" = borrar ] || { echo "Cancelado: no se borró nada."; exit 1; }
	$(call L_IMAGES) $(LOCAL_COMPOSE) down -v
	$(MAKE) local/up TAG=$(TAG) BUILD=$(BUILD)
	$(MAKE) local/seed

local/test: ## make test contra capacitate_preprod_test (con la preproducción levantada)
	$(MAKE) test TEST_DATABASE_URL="postgres://$(L_PG_USER):$(L_PG_PASS)@localhost:5441/capacitate_preprod_test?sslmode=require"

local/logs: ## Logs en vivo de la preproducción (s=db|backend|frontend|mailpit)
	$(call L_IMAGES) $(LOCAL_COMPOSE) logs -f --tail=100 $(s)

local/ps: ## Estado de los contenedores de la preproducción
	$(call L_IMAGES) $(LOCAL_COMPOSE) ps

local/psql: ## Consola psql en la base de preproducción
	$(call L_IMAGES) $(LOCAL_COMPOSE) exec db psql -U "$(L_PG_USER)" -d "$(L_PG_DB)"

clean: ## Borra artefactos de build y cobertura
	cd backend && rm -rf bin/ tmp/ coverage.out
	cd frontend && rm -rf .next/ coverage/
