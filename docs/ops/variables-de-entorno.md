# Variables de entorno

Inventario sacado del código de `develop` (613b7a9). Fuentes:

- Backend: `backend/internal/config/config.go`, `backend/cmd/*`, `backend/entrypoint.sh`.
- Frontend: `frontend/src`.
- Orquestación: `docker-compose.yml`, `Makefile` y los Dockerfiles.

El valor por defecto se indica **por capa** porque difiere entre ellas. Por
ejemplo, `SMTP_PORT` vale 0 en el código y 587 en compose.

> Pendiente: no se cruzó con `.env.example` porque el archivo está bloqueado
> para el rol devops por una regla de denegación (ver la sección
> [Hallazgos](#hallazgos-para-el-tech-lead)).

En producción (Railway) cada variable se carga como variable de servicio, y
los secretos van sellados. Ver [railway.md](railway.md) §5. Los secretos van **solo** en el gestor de secretos del entorno. Nunca en el
repositorio, en la imagen, en `docker-compose.yml` ni en mensajes.

## Backend: servidor (`cmd/server`, `config.Load`)

> **Cambios en camino** ([PR #100](https://github.com/bacsystem/learning/pull/100), backend-dev-2; aplican
> cuando se fusione):
> - **H-2:** `ENCRYPTION_KEY` que no sea de 64 hex falla en `config.Load`, antes
>   de conectar, con `ENCRYPTION_KEY must be 64 hex characters (a 32-byte key)`.
> - **H-3:** un `LOG_LEVEL` fuera de `trace|debug|info|warn|error` (sin
>   distinguir mayúsculas; vacío = valor por defecto) no arranca. Un
>   `TRUSTED_PROXIES` con una entrada que no sea IP ni CIDR tampoco. Vacío o con
>   una coma al final se acepta.
> - **H-5:** fuera de `development`/`test`, el arranque falla si el `sslmode`
>   efectivo (el del DSN, o `PGSSLMODE` si el DSN no lo trae) no es `require`,
>   `verify-ca` o `verify-full`. El compose (`sslmode=disable`) solo arranca con
>   `ENV=development`/`test`. En Railway: `${{Postgres.DATABASE_URL}}?sslmode=require`.
> - **H-6:** `var version = "0.0.0-dev"` (`main.version`). El Dockerfile la
>   inyecta con `-X main.version=$VERSION` (este PR), y `make docker/build` pasa
>   el tag sin la «v».

`config.Load` lee el entorno y, si existe, un archivo `.env` del directorio de
trabajo (viper). En la imagen no hay `.env`: `backend/.dockerignore` lo excluye.

«Entorno no local» significa `ENV` distinto de `development` y `test`. Incluye
`ENV` vacío, `production`, `staging`, etc.

| Variable | Para qué sirve | Por defecto (código / compose) | Secreto | Si falta o es inválida |
|---|---|---|---|---|
| `DATABASE_URL` | DSN de Postgres (driver `lib/pq`) | — / se arma con `POSTGRES_*` y `sslmode=disable` | **Sí** (lleva la contraseña) | No arranca: `DATABASE_URL is required`. Si el `sslmode` es inválido o el host no responde, falla en `db connect failed` |
| `JWT_SECRET` | Firma de los JWT | — / obligatoria (`:?`) | **Sí** | No arranca: `JWT_SECRET is required`. Con menos de 32 bytes: `JWT_SECRET must be at least 32 bytes` |
| `ENCRYPTION_KEY` | Clave AES-256-GCM de los datos personales; también es la clave HMAC de `dni_hmac` y `email_hmac` | — / obligatoria (`:?`) | **Sí**. Si se pierde, los datos cifrados no se pueden recuperar | Vacía: `ENCRYPTION_KEY is required`. Con formato inválido (no son 64 caracteres hex) falla en `crypto init failed: encryption key must be 32 bytes (64 hex chars)`, ya conectado a la base (ver H-2) |
| `ENV` | Entorno: `development`, `test`, `production`… | vacío / `development` | No | Vacío o desconocido cuenta como **no local**: se exigen CORS, SMTP y `FRONTEND_BASE_URL`. Es el valor por defecto seguro |
| `PORT` | Puerto HTTP | `8080` / `8080` | No | Usa 8080 |
| `CORS_ORIGINS` | Orígenes permitidos, separados por comas | vacío. En entornos locales, vacío equivale a `http://localhost:3000` / vacío | No | En entorno no local, vacío no arranca: `CORS_ORIGINS is required unless ENV is development or test` |
| `TRUSTED_PROXIES` | CIDR o IP de los proxies de confianza, para tomar la IP del cliente (auth, auditoría, rate limit) | vacío (no confía en ningún proxy) / vacío | No | Las entradas inválidas **se ignoran sin avisar** (H-3). Detrás de un balanceador, si no se configura, todas las peticiones parecen venir de la IP del balanceador y el rate limit se vuelve global |
| `FRONTEND_BASE_URL` | Base de los enlaces en los correos (verificación, recuperación) | vacío; en entornos locales usa `http://localhost:3000` / vacío | No | En entorno no local, vacío no arranca. Si es inválida: `FRONTEND_BASE_URL must be a valid URL` |
| `SMTP_HOST` | Servidor SMTP del worker de correo | vacío / vacío | No | En entorno no local, vacío no arranca: `SMTP_HOST is required…` |
| `SMTP_PORT` | Puerto SMTP | `0` / `587` | No | En entorno no local, 0, negativo o no numérico no arranca: `SMTP_PORT must be a positive port…` |
| `SMTP_USER` | Usuario SMTP | vacío / vacío | No | Opcional (SMTP sin autenticación) |
| `SMTP_PASSWORD` | Contraseña SMTP | vacío / vacío | **Sí** | Opcional. Si el servidor la exige, el fallo aparece al enviar, en los logs del worker |
| `SMTP_FROM` | Remitente de los correos | vacío / vacío | No | En entorno no local, vacío no arranca |
| `LOG_LEVEL` | Nivel de log (`debug`, `info`, `warn`…) | `debug` en development, `info` en el resto / vacío | No | Un valor inválido **se ignora sin avisar** y se usa el nivel por defecto (H-3) |

## Backend: arranque del contenedor y migraciones

| Variable | Dónde se usa | Por defecto | Secreto | Si falta o es inválida |
|---|---|---|---|---|
| `MIGRATE_LOCK_TIMEOUT` | `entrypoint.sh` la pasa a `migrate -lock-timeout`: espera máxima por el advisory lock de migraciones | `10m` (entrypoint y compose) | No | Si no es una duración válida, `migrate` sale con error de flag y el contenedor no arranca. El mensaje es de flag de Go, comprensible |
| `GOOSE_BIN` | `cmd/migrate`: binario de goose | `goose`. La imagen pasa `-goose /usr/local/bin/goose` | No | — |
| `DATABASE_URL` | `cmd/migrate`. Se pasa a goose por `GOOSE_DBSTRING`, así no queda en argv | — | **Sí** | `DATABASE_URL requerida` |

Las migraciones corren **en cada arranque** del contenedor, antes del servidor
(`entrypoint.sh`, con `set -e`). Si fallan, el servidor no arranca.

## Backend: comandos de operación (`cmd/seed`, `cmd/seed/admin`, `cmd/rehash-emails`, `cmd/fix-encrypt-personas`)

| Variable | Comandos | Secreto | Si falta |
|---|---|---|---|
| `DATABASE_URL`, `ENCRYPTION_KEY` | todos | **Sí** | Mensaje claro y salida. `cmd/seed`: `env X is required`. `rehash-emails`, `fix-encrypt-personas` y `seed/admin`: `DATABASE_URL y ENCRYPTION_KEY son obligatorios/requeridas`. Con formato inválido: `ENCRYPTION_KEY inválida: …` |
| `ADMIN_EMAIL` | `cmd/seed/admin` (obligatoria) | No (dato personal: no va en logs ni en mensajes) | Error y salida |
| `ADMIN_USERNAME`, `ADMIN_DNI`, `ADMIN_REBIND`, `ADMIN_INSTITUCION_ID`, `ADMIN_GRADO_ID`, `ADMIN_ESPECIALIDAD_ID` | `cmd/seed/admin` (opcionales) | No | Valores por defecto documentados en la cabecera de `cmd/seed/admin/main.go` |
| `ADMIN_PASSWORD` | `cmd/seed/admin` (opcional) | **Sí** | Si está vacía, se lee una línea de stdin. Es lo recomendado: `make compose/seed/admin` la pide por teclado |

## Backend: tests

| Variable | Uso | Si falta |
|---|---|---|
| `TEST_DATABASE_URL` | Tests de integración (`internal/db/dbtest`, `internal/questions/questionstest`). `make test` la exige | Los tests de integración **se saltan** (`t.Skip`) y `go test` sale en verde. Por eso `make test` falla antes si no está definida |

## Cron de copia a R2 (`deploy/backup/backup.sh`)

Solo en el servicio cron de Railway. Detalle y aprovisionamiento en
[deploy/backup/README.md](../../deploy/backup/README.md).

| Variable | Para qué sirve | Por defecto | Secreto | Si falta |
|---|---|---|---|---|
| `BACKUP_DATABASE_URL` | Rol `backup_ro` por la URL privada, con `sslmode=require` | — | **Sí** | Sale con 2: `falta la variable …` |
| `AGE_RECIPIENT` | Clave **pública** age (`age1…`) con la que se cifra | — | No | Sale con 2. También si no empieza por `age1` |
| `R2_ENDPOINT` | `https://<ACCOUNT_ID>.r2.cloudflarestorage.com` | — | **Sí** (lleva el ID de cuenta) | Sale con 2 |
| `R2_BUCKET` | Bucket privado y dedicado | — | No | Sale con 2 |
| `R2_ACCESS_KEY_ID`, `R2_SECRET_ACCESS_KEY` | Token Object Read & Write limitado al bucket | — | **Sí** | Sale con 2 |
| `BACKUP_TIMEOUT` | Segundos máximos para el dump y para la subida | `1800` | No | Usa 1800 |
| `BACKUP_PUSH_URL` | URL del monitor Push de Uptime Kuma (`http://uptime-kuma.railway.internal:3001/api/push/<token>`). Opcional | vacía | **Sí** (lleva el token) | No avisa a Kuma: la copia no se ve afectada. Si el aviso falla, solo deja una línea en el log |

## Decidida pero sin implementar

| Variable | Estado |
|---|---|
| `ANTIFRAUD_HMAC_KEY`, `ANTIFRAUD_HMAC_KEY_PREVIOUS` | Las define el [ADR 0002](../adr/0002-secreto-hmac-contadores-antifraude.md): secreto obligatorio en todos los entornos, 64 caracteres hex, distinto de `ENCRYPTION_KEY` y de `JWT_SECRET`. **No hay código que la lea en `develop`** ni en ninguna rama. Cuando llegue el PR que la implementa, devops agrega en el mismo cambio: `ANTIFRAUD_HMAC_KEY: ${ANTIFRAUD_HMAC_KEY:?…}` en compose, su generación en `make compose/init`, la línea en `.env.example` y esta tabla. Si compose la exige antes de que `compose/init` la genere, `make compose/up` se rompe |

## Frontend (Next.js)

Las `NEXT_PUBLIC_*` se **incrustan en el bundle al construir** (build-args del
Dockerfile). Cambiarlas en tiempo de ejecución no tiene efecto, así que hace
falta una imagen de frontend por entorno.

| Variable | Para qué sirve | Por defecto (código / compose / Makefile / Dockerfile) | Secreto | Si falta |
|---|---|---|---|---|
| `NEXT_PUBLIC_API_URL` | URL base de la API | `http://localhost:8080` si no está definida (`??`) / `http://localhost:8080` / `https://api-capacitate.bacsystemsolutions.com` / sin valor | No (es pública) | **Arreglado en este PR:** el Dockerfile falla si llega vacía. Antes, `ARG` sin valor → `ENV` vacío, y `??` no cubre la cadena vacía, así que el bundle llamaba a rutas relativas sin avisar. Fuera de Docker (`npm run build` sin la variable) se incrusta `http://localhost:8080` (H-4) |
| `NEXT_PUBLIC_STORAGE_PREFIX` | Prefijo de la cookie de idioma (`<prefijo>-locale`) | `capacitate` / vacío / vacío / vacío | No | Usa `capacitate` |
| `NEXT_PUBLIC_CONTACT_EMAIL` | Correo de contacto público | `contact@bacsystemsolutions.com` / vacío / vacío / vacío | No | Usa el de la empresa |
| `NODE_ENV`, `PORT`, `HOSTNAME` | Runtime del servidor standalone | `production`, `3000`, `0.0.0.0` (Dockerfile) | No | — |
| `NEXT_TELEMETRY_DISABLED` | Desactiva la telemetría de Next | `1` (Dockerfile y CI) | No | — |

## Compose: solo desarrollo

`POSTGRES_USER` (`learning`), `POSTGRES_PASSWORD` (obligatoria, la genera
`make compose/init`), `POSTGRES_DB` (`learning`), `DB_PORT` (5433),
`BACKEND_PORT` (8080) y `FRONTEND_PORT` (3000). Compose publica los puertos
solo en `127.0.0.1`.

## Hallazgos para el tech-lead

Todos son de código de la app, así que no se cambian en este PR:

- **H-1. `.env.example` sin verificar.** El rol devops no puede leerlo por una
  regla de denegación, aunque es el dueño de la plantilla. El usuario aprobó
  la lectura; el tech-lead cambia el permiso y entonces se cruza. Hace falta permitir
  la lectura de `.env.example` en `.claude/settings.json` (vía
  `setup-team.sh`), o que otro rol cruce este inventario con la plantilla.
- **H-2 (en curso, backend-dev-2). `ENCRYPTION_KEY` se valida tarde.** `config.Load` solo comprueba que
  no esté vacía. Que sean 64 caracteres hex se comprueba después de conectar a
  la base, y el error no nombra la variable
  (`encryption key must be 32 bytes…`). Propuesta: validarla en `config.Load`
  con un mensaje que diga `ENCRYPTION_KEY`.
- **H-3 (en curso, backend-dev-2). Valores inválidos que se ignoran sin avisar.** `TRUSTED_PROXIES` con
  una entrada inválida la descarta, y `LOG_LEVEL` inválido cae al nivel por
  defecto. Propuesta: fallar en `config.Load` con un error que nombre la
  variable. En producción, un `TRUSTED_PROXIES` mal escrito deja el rate limit
  y la auditoría con la IP del balanceador.
- **H-4. `NEXT_PUBLIC_API_URL` y la cadena vacía.** El frontend usa `??`, que
  no cubre la cadena vacía. El Dockerfile ya lo impide en este PR, pero
  `npm run build` fuera de Docker con la variable vacía repite el problema.
  Propuesta: usar `||` o fallar en el build si falta en producción.
- **H-5 (en curso, backend-dev-2). `DATABASE_URL` sin política de `sslmode`.** `lib/pq` v1.12.3 usa
  `require` por defecto: cifra, pero no verifica el certificado (comprobado en
  `connector.go`). Con `sslrootcert=system` pasa a `verify-full`. `lib/pq`
  también lee las variables `PG*` estándar (`PGSSLMODE`, `PGHOST`…), que no
  deben quedar definidas por accidente en el entorno del contenedor. El nivel de producción
  (`require` o `verify-full` con `sslrootcert`) depende del ADR de la base.
  Ver [plan-base-produccion.md](plan-base-produccion.md). Decidido en el ADR 0004:
  en producción, `sslmode=require` explícito, y el backend lo exigirá
  (backend-dev-2).
- **H-6 (en curso, backend-dev-2; Docker hecho en este PR). La versión de `/health` está fija.** `GET /health` devuelve
  `"version":"1.0.0"` desde `const version` en `cmd/server/main.go`, mientras
  que `frontend/package.json` dice 1.1.0 y todavía no hay tags. Propuesta:
  `var version = "dev"` y que el Dockerfile la inyecte con
  `-ldflags "-X main.version=$VERSION"` desde el tag. Requiere cambiar el
  código; el lado Docker lo hace devops. Decidido: `-X main.version=1.2.0`, sin
  la «v» del tag; fuera de un release, `0.0.0-dev`. Asignado a backend-dev-2
  (#100, ya en develop).
  Valores de `/health` según cómo se construyó la imagen:
  - release (`make docker/build DOCKER_TAG=vX.Y.Z`): `X.Y.Z`;
  - preproducción local con `make local/up BUILD=1`: `0.0.0-preprod`, que
    `deploy/local/compose.build.yml` pasa a propósito para distinguirla;
  - cualquier otro build sin `VERSION` (`docker compose` de desarrollo, `go run`):
    `0.0.0-dev`.
