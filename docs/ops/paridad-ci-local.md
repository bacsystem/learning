# Paridad entre el CI y local

Objetivo: lo que corre el CI (`.github/workflows/backend.yml` y `frontend.yml`,
solo con tags `v*`) es lo mismo que corre un developer con `make lint`,
`make test` y `make build`, con las mismas versiones.

## Comparación (antes → después de este cambio)

### Backend

| Paso del CI | Antes, en make | Ahora |
|---|---|---|
| `go mod tidy` + `git diff --exit-code go.mod go.sum` | no estaba | `make test` |
| `go build ./...` | `make build` solo compilaba `cmd/server` | `make build` (`./...` + `bin/server`) |
| `go vet ./...` | `make lint` | `make lint` |
| govulncheck v1.9.0 | no estaba | `make test` (misma versión, con `go run`) |
| goose v3.21.1: `up` → `down-to 0` → `up` | no estaba (y `migrate/*` usaba el goose instalado, en cualquier versión) | `make test`. `migrate/*` usa ahora goose v3.21.1 |
| `go test -race -count=1 ./...` con `TEST_DATABASE_URL` sobre Postgres 16 | `go test -race` **sin** `TEST_DATABASE_URL`: los tests de integración se saltaban y el resultado salía en verde | `make test` exige `TEST_DATABASE_URL`, por argumento o exportada en el entorno, y que el nombre de la base termine en `_test` o `_test_<n>` (p. ej. `learning_test_2`), porque hace `down-to 0` |
| golangci-lint v2.15.0 | no estaba. `make lint` corría `go fmt`, que **reescribe** archivos en vez de fallar | `make lint` (misma versión, con `go run`). `go fmt` pasa a `make fmt` |

### Frontend

| Paso del CI | Antes, en make | Ahora |
|---|---|---|
| `npm ci` | — (lo corre el developer) | — |
| `npx tsc --noEmit` | no estaba | `make lint` |
| `npm run lint` | `make lint` | `make lint` |
| `npm test` | `npm run test:coverage` (lo mismo, con cobertura) | `make test` (`npm test`). Para la cobertura, `cd frontend && npm run test:coverage` (se quitaron `test/backend` y `test/frontend`, que duplicaban `test`) |
| `node scripts/check-i18n.mjs` | no estaba | `make lint` |
| `npm run build` | `make build` | `make build` |

## Versiones fijadas

| Herramienta | CI | Local (`make`) | Docker |
|---|---|---|---|
| Go | `go-version-file: backend/go.mod` → toolchain go1.27.4 | `GOTOOLCHAIN=auto` (por defecto) descarga go1.27.4 según `go.mod` | `golang:1.27.4-alpine3.23` |
| Node | `frontend/.nvmrc` = 22.23.5 (PR #102) | `nvm use` en `frontend/` (no lo fija make) | `node:22.23.5-alpine3.23` (antes `node:22-alpine`, flotante) |
| Postgres | `postgres:16-alpine` | la base de `TEST_DATABASE_URL` (Postgres 16) | `postgres:16-alpine` (compose) |
| goose | v3.21.1 | v3.21.1 (`GOOSE_VERSION` del Makefile) | v3.21.1 (`ARG GOOSE_VERSION`) |
| govulncheck | v1.9.0 | v1.9.0 | — |
| golangci-lint | v2.15.0 | v2.15.0 | — |

Las versiones de goose, govulncheck y golangci-lint están en dos sitios: el
`env:` del workflow y el principio del `Makefile`. Si cambias una, cambia las
dos.

## Cómo correr lo mismo que el CI

```bash
# Postgres 16 desechable (make test hace down-to 0 sobre esta base)
docker run -d --rm --name pg-test -e POSTGRES_USER=learning -e POSTGRES_PASSWORD=learning \
  -e POSTGRES_DB=learning_test -p 127.0.0.1:55432:5432 postgres:16-alpine

(cd frontend && nvm use && npm ci)
make lint
make test TEST_DATABASE_URL='postgres://learning:learning@127.0.0.1:55432/learning_test?sslmode=disable'
make build
docker stop pg-test
```

## Diferencias que quedan

- **El CI no construye las imágenes Docker.** Construirlas es manual
  (`make docker/release DOCKER_TAG=vX.Y.Z`). Que el CI las construya y publique
  es una decisión de despliegue (registro, credenciales): va al ADR de
  despliegue y no se agrega aquí.
- **`npm audit` no está en el pipeline.** Hoy falla: next 16.2.4 tiene una
  vulnerabilidad crítica (RCE) y next-intl, postcss y sharp tienen avisos
  altos o moderados. Se agrega como paso del workflow cuando el frontend suba
  esas dependencias, porque si no, bloquearía todos los tags. Mientras tanto
  está en el checklist de release como bloqueo manual.
- **Node local no lo fija make**: hay que usar `nvm use` (o equivalente) con
  `frontend/.nvmrc`.
- **Node**: 22.23.5, el último parche de la 22 (de seguridad). `.nvmrc` sube en el
  PR #102 y el Dockerfile en este PR: se fusionan juntos.
- **`make migrate/create`** usaba la marca de tiempo de goose. Ahora numera en
  secuencia (`-s`, renombrado a 3 dígitos). Comprobado sobre una copia de
  `migrations/`: crea `062_probe.sql`.

## Verificación de este cambio

Ejecutado el 11-11-2026 en macOS arm64: Go go1.27.4 (por `GOTOOLCHAIN`),
Node 22.14.0 (nvm), Postgres `postgres:16-alpine` desechable. Detalle en el
PR que introdujo este documento.
