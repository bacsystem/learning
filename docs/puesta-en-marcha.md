# Puesta en marcha de un entorno nuevo

Pasos para levantar el sistema desde cero (local con Docker Compose o un servidor
nuevo). Todavía no existe un entorno productivo: no hay datos que migrar.

## 0. Archivos `.env` e historial de git

`backend/.env` y `frontend/.env` **no se versionan** (están en `.gitignore`).
Para desarrollo sin Docker, copiarlos desde las plantillas y completarlos:

```bash
cp backend/env.example backend/.env
cp frontend/env.example frontend/.env
```

El historial de git **ya se limpió** con
[git filter-repo](https://github.com/newren/git-filter-repo): se quitaron los
`.env` versionados, la carpeta `assets/` y los binarios compilados, y después se
creó el remoto (GitHub, privado). No volver a correr `filter-repo` sobre este
repositorio: reescribe todos los commits y obliga a re-clonar a quien ya tenga
una copia. Para que no haga falta, no commitear nunca un `.env`, `assets/` ni
binarios (están en `.gitignore`; revisar `git status` antes de `git add`).

Un `Makefile` antiguo del historial menciona la URL de una base de datos en
Railway que ya no existe (no hay entorno productivo ni base en Railway): no
requiere ninguna acción.

## 1. Configuración (Docker Compose)

```bash
make compose/init     # crea .env desde env.example
```

`make compose/init` no pisa un `.env` existente. Genera `JWT_SECRET`,
`POSTGRES_PASSWORD` y `ANTIFRAUD_HMAC_KEY` aleatorios y toma `ENCRYPTION_KEY` de `backend/.env` si es
válida (para que los datos cifrados sean compatibles) o genera una nueva. Sin
`make`: `cp env.example .env` y completar los obligatorios (`JWT_SECRET`,
`ENCRYPTION_KEY`, `ANTIFRAUD_HMAC_KEY`, `POSTGRES_PASSWORD`); `docker compose` falla si falta alguno.

`POSTGRES_PASSWORD` no tiene valor por defecto. Postgres solo lo aplica al
**crear** el volumen `pgdata`: un volumen existente conserva la contraseña con
la que se creó. Para cambiarla, `make compose/reset` (borra los datos) o
`ALTER USER learning PASSWORD '...'` desde `make compose/psql`, y después
actualizar `.env`.

**Si ya tenías un `.env`** (creado antes de `ANTIFRAUD_HMAC_KEY`), `make
compose/up` se detiene con un mensaje claro. Añade la clave a mano:

```bash
echo "ANTIFRAUD_HMAC_KEY=$(openssl rand -hex 32)" >> .env
```

Los puertos (Postgres `5433`, backend `8080`, frontend `3000`; configurables con
`DB_PORT`, `BACKEND_PORT`, `FRONTEND_PORT`) se publican solo en `127.0.0.1`.
Para acceder desde otra máquina, crear un `docker-compose.override.yml` local
que **reemplace** la lista `ports` del servicio con `!override` y publique el
puerto en `0.0.0.0` (o en la IP de la interfaz), y ajustar `CORS_ORIGINS` y
`NEXT_PUBLIC_API_URL`:

```yaml
services:
  backend:
    ports: !override
      - "0.0.0.0:${BACKEND_PORT:-8080}:8080"
```

Sin `!override` compose concatena las listas: el puerto queda publicado en
`127.0.0.1` y en `0.0.0.0` a la vez y el contenedor no arranca ("failed to
bind host port ...: address already in use"). Para comprobarlo,
`docker compose config` debe mostrar una sola publicación por puerto.

Variables que el backend valida al arrancar:

| Variable | Regla |
|---|---|
| `DATABASE_URL` | obligatoria (en compose se arma con `POSTGRES_*`) |
| `JWT_SECRET` | obligatoria, al menos 32 bytes |
| `ENCRYPTION_KEY` | obligatoria, 64 caracteres hex (AES-256-GCM y HMAC de DNI/email). **No se puede cambiar después de cargar personas**: los datos cifrados y los HMAC dependen de ella |
| `ANTIFRAUD_HMAC_KEY` | obligatoria en **todos** los entornos, 64 caracteres hex ([ADR 0002](adr/0002-secreto-hmac-contadores-antifraude.md)). Distinta de `ENCRYPTION_KEY` y de `JWT_SECRET`: **no copies** `ENCRYPTION_KEY` aquí. Cambiarla sin rotar pierde los contadores de cuota y de espera (no hay datos personales). Rotación: mover el valor vigente a `ANTIFRAUD_HMAC_KEY_PREVIOUS`, poner uno nuevo (`openssl rand -hex 32`) y reiniciar; a los 31 días vaciar `ANTIFRAUD_HMAC_KEY_PREVIOUS` |
| `CORS_ORIGINS` | obligatoria salvo con `ENV=development` o `ENV=test` (un `ENV` vacío o mal escrito también la exige) |
| `FRONTEND_BASE_URL` | URL válida; se usa en los enlaces de los correos. Obligatoria salvo con `ENV=development`/`test` (allí, vacía, usa `http://localhost:3000`), igual que `CORS_ORIGINS`, `SMTP_HOST`, `SMTP_PORT` y `SMTP_FROM` |
| `TRUSTED_PROXIES` | CIDRs de los proxies de confianza; sin ella se ignora `X-Forwarded-For` |
| `LOG_LEVEL` | opcional (`trace`, `debug`, `info`, `warn`, `error`); vacío = `debug` con `ENV=development`, `info` en el resto |
| `MIGRATE_LOCK_TIMEOUT` | opcional (por defecto `10m`) |

Sin `SMTP_*` los correos quedan en `email_queue` y no se envían; en ese caso
los registros no se pueden completar (el enlace de verificación llega por
correo).

## 2. Levantar

```bash
make compose/up       # db + backend + frontend (docker compose up -d --build)
make compose/ps       # estado; make compose/logs [s=backend] para los logs
```

El backend aplica las migraciones al arrancar con `cmd/migrate`, que toma un
advisory lock de Postgres: con varias réplicas solo una migra y las demás esperan.
En Linux (la imagen Docker y el CI) el kernel mata a goose si `migrate` muere,
incluso con `kill -9` (Pdeathsig), así que el advisory lock no queda libre con
goose vivo. Quedan dos riesgos residuales:

- En cualquier sistema, la sentencia que goose tenía en curso **sigue
  ejecutándose en el servidor** hasta terminar (Postgres no detecta el cliente
  caído sin `client_connection_check_interval`). Una migración normal es una
  transacción: al terminar no puede confirmar y se revierte, pero mientras
  tanto conserva sus locks y una segunda ejecución espera por ellos.
- **Excepción: las migraciones `-- +goose NO TRANSACTION`** (054 y 058, que
  usan `CREATE INDEX CONCURRENTLY`) **no se revierten**. Si se mata `migrate`
  durante una de ellas, los índices ya creados quedan creados y el que estaba
  en construcción puede quedar `INVALID` (o seguir construyéndose en el
  servidor). Al volver a migrar, el chequeo final de la migración aborta si
  encuentra un índice `INVALID`. Antes de borrarlo, confirma que ya no hay
  ninguna construcción en curso, porque un índice que todavía se está
  construyendo también aparece como no válido:

  ```sql
  -- Construcciones de índices en curso (debe salir vacío):
  SELECT pid, relid::regclass AS tabla, index_relid::regclass AS indice, phase
  FROM pg_stat_progress_create_index;

  -- Índices INVALID que quedan:
  SELECT c.relname FROM pg_index i JOIN pg_class c ON c.oid = i.indexrelid
  WHERE NOT (i.indisvalid AND i.indisready);
  ```

  Si la primera consulta muestra una construcción, espera a que termine (o
  cancélala con `pg_cancel_backend(pid)`). Con la primera vacía, ejecuta
  `DROP INDEX CONCURRENTLY <nombre>;` por cada índice `INVALID` y vuelve a
  migrar.
- Fuera de Linux (`go run ./cmd/migrate` en macOS o Windows) no hay Pdeathsig:
  si `migrate` muere con `kill -9`, goose sigue corriendo sin el advisory lock
  y otra instancia podría migrar en paralelo. No matar `migrate` con `-9` a
  mitad de una migración.

`make compose/migrate/status` muestra la versión aplicada.

La imagen del backend se construye para la arquitectura del host (en Apple
Silicon corre nativa en arm64); `make docker/build/backend` genera la de
release para `linux/amd64`.

## 3. Datos iniciales

1. **Catálogos y personas**: cargar instituciones, grados, especialidades,
   asignaturas y el padrón de personas. Las personas se cargan siempre cifradas
   con `cmd/seed` (`go run ./cmd/seed <archivo.sql>` con `DATABASE_URL` y
   `ENCRYPTION_KEY`); el comando termina con código distinto de 0 si alguna
   línea no se pudo procesar.
2. **Administrador**: se crea ligado a una persona dedicada, nunca a una persona
   real del padrón, porque el email de la persona es el canal de recuperación
   de la contraseña.

   ```bash
   make compose/seed/admin ADMIN_EMAIL=admin@tu-dominio [ADMIN_USERNAME=admin]
   ```

   Corre `go run ./cmd/seed/admin` en el host (requiere Go) contra el Postgres
   del compose. La contraseña (12 a 72 bytes) se pide por teclado. El usuario
   queda siempre con `rol=admin` y `estado=activo`. Si ya existe un usuario con
   ese nombre ligado a otra persona (bases creadas con el seed anterior) o que
   no es admin activo, el comando falla y hay que agregar `ADMIN_REBIND=1`: lo
   re-liga a la persona dedicada, fuerza `admin`/`activo`, rota la contraseña,
   cierra sus sesiones e invalida sus tokens de recuperación.
3. **Precio de inscripción**: configurar `configuracion_pago` de cada año desde
   Ingresos → Configuración **antes de abrir inscripciones**. Sin precio,
   `POST /inscripciones` responde 409.
4. **Procesos**: un proceso solo se puede activar si los pesos de sus
   asignaturas suman 100. Con el proceso activo no se aceptan cambios que
   rompan esa suma: hay que desactivarlo, editar y reactivar.

## 4. Registro de usuarios

1. `POST /auth/register` recibe solo `{dni, email}` y responde **siempre 202**
   con el mismo mensaje y una duración mínima fija, exista o no el DNI y
   coincida o no el email (no revela qué rama se ejecutó).
2. Si el DNI está en el padrón, aún no tiene usuario y el email coincide con el
   registrado, se envía un enlace al email **del padrón** (si la persona no
   tenía email, al enviado, que no puede pertenecer a otra persona y se guarda
   solo al verificar). El enlace abre `/auth/verify-email` en el frontend. Ahí la persona elige usuario y contraseña
   (`POST /auth/verify-email` con `{token, username, password,
   confirm_password}`); el token es de un solo uso.
3. La cuenta queda en estado `pendiente` hasta que un administrador la activa
   (Usuarios → estado, `PATCH /admin/usuarios/{id}/estado`).

Los intentos fallidos de registro se limitan por (DNI, IP) — 5 cada 15 min — y
por DNI — 30 por hora —, y los intentos de verificación por token. Los
contadores se guardan en Postgres (tabla `auth_intentos_fallidos`): son
compartidos entre réplicas y sobreviven a reinicios. Si el dueño real de un DNI
queda bloqueado, un administrador puede reiniciar sus límites con
`DELETE /admin/registro-limites` y el cuerpo JSON `{"dni": "12345678"}` (8
dígitos). El DNI va en el cuerpo y no en la ruta para que no quede en los logs
de acceso. Responde 404 si el DNI no está en el padrón.

## 5. Bases creadas con versiones anteriores

Solo aplica a bases locales que ya tenían datos antes de la rama
`fix/backend-review`:

- `go run ./cmd/rehash-emails` (prueba) y luego `-apply`: recalcula
  `email_hmac` con el email normalizado (minúsculas y sin espacios). Mientras
  no se corra, las búsquedas aceptan también el HMAC antiguo.
- `go run ./cmd/fix-encrypt-personas` (o `make fix-encrypt-personas`): genera
  el SQL para cifrar personas que hayan quedado en texto plano y recalcular
  `dni_hmac`/`email_hmac`. También es necesario después de un
  `goose down-to 8` + `up` con personas cargadas: la migración 009 les pone un
  `dni_hmac` provisional `legacy:<id>`.
- Si la migración 047 se aplicó con su versión original, los coeficientes por
  asignatura quedaron en 1.0: revisarlos en la pantalla de procesos.
- La migración 049 cierra como `completado` (sin nota) los simulacros con un
  estado desconocido (p.ej. `abandonado`) antes de crear su CHECK.
- Si la migración 054 falla con "índice(s) INVALID", un `CREATE INDEX
  CONCURRENTLY` anterior quedó a medias: `DROP INDEX CONCURRENTLY <nombre>` y
  volver a arrancar el backend.

## 6. Verificación

Lo mismo que el CI, con las versiones fijadas, desde la raíz:
`make lint`, `make test TEST_DATABASE_URL=...` (base desechable: hace
`down-to 0`) y `make build`. Ver `docs/ops/paridad-ci-local.md`; la
operación de producción está en `docs/ops/`. Paso a paso:

```bash
cd backend
go build ./... && go vet ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...
TEST_DATABASE_URL=postgres://...?sslmode=disable go test -race -count=1 ./...
```

`TEST_DATABASE_URL` debe apuntar a una base migrada a la última versión
(`goose -dir migrations postgres "$TEST_DATABASE_URL" up`, goose v3.21.1).

El CI (`.github/workflows/backend.yml`) corre solo al pushear un tag de
producción (`v*`, p. ej. `v1.2.0`); no corre en pull requests ni en pushes a
ramas, así que antes de fusionar hay que correr estos comandos en local. Corre
lo mismo contra Postgres 16 (más `go mod tidy` limpio y el ciclo de migraciones `up` →
`down-to 0` → `up`), y golangci-lint v2.15.0 en un job aparte sobre todo el
módulo, con la configuración de `backend/.golangci.yml`. El módulo está en 0
issues, así que cualquier hallazgo nuevo hace fallar el job, y también falla si
el linter no puede ejecutarse. Para correrlo en local:
`cd backend && go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 run ./...`.
