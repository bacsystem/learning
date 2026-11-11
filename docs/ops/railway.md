# Despliegue en Railway (runbook)

Decisión del usuario (11-11-2026): por ahora los despliegues van a Railway. La
base de producción se documenta en el ADR 0004 (architect).

Este runbook dice **cómo se configura**. Hasta que el usuario lo apruebe, **no
se crea ni se configura nada en Railway**. Todo lo que se configure en el panel
se anota aquí, en el mismo PR que lo documenta: nada queda configurado a mano
sin dejarlo escrito.

Fuente: solo la documentación oficial de Railway (docs.railway.com),
consultada el 11-11-2026. Cada dato lleva su enlace. Lo que la documentación
**no dice** está marcado como «sin confirmar» y se comprueba en el primer
aprovisionamiento.

## 1. Avisos que condicionan el diseño

- **Config as code (`railway.json`/`railway.toml`) está obsoleto.** Railway lo
  lee solo hasta el **01-12-2026**, y los servicios nuevos ya no pueden
  activarlo. Su reemplazo es `.railway/railway.ts`, que se aplica con la CLI
  (`railway config plan/apply`).
  ([config-as-code](https://docs.railway.com/reference/config-as-code),
  [infrastructure-as-code](https://docs.railway.com/infrastructure-as-code))
  - **Decisión:** no se agrega `railway.json`. La configuración va en el panel
    y queda escrita en este documento (§3).
  - Adoptar `.railway/railway.ts` sería una herramienta nueva: lo tiene que
    aprobar el tech-lead.
- **El healthcheck solo se usa durante el despliegue.** Railway no vigila el
  endpoint una vez que el despliegue está activo.
  ([healthchecks](https://docs.railway.com/guides/healthchecks))
  - Para tener alertas hace falta un monitor externo de `/health`. Elegirlo
    agrega un servicio, así que lo aprueba el tech-lead.
- **«Wait for CI» solo funciona con workflows `on: push: branches`.** Railway
  despliega desde **una rama** conectada. No hay disparo por tag.
  ([github-autodeploys](https://docs.railway.com/deployments/github-autodeploys))
  - Nuestro CI corre solo con tags `v*`. Por eso, si el autodeploy queda activo
    sobre `main`, el merge a `main` despliega **antes** del tag y de su CI.
  - **Propuesta:** autodeploy **desactivado**. Se despliega a mano, con la
    aprobación del usuario, cuando el CI del tag pasa (ver
    [checklist-release.md](checklist-release.md) §5).
- **Variables selladas.** Una variable sellada no se ve en el panel, no se lee
  por la API y no se puede «des-sellar».
  ([variables](https://docs.railway.com/guides/variables))
  - Antes de sellar `ENCRYPTION_KEY`, `JWT_SECRET` o (cuando exista)
    `ANTIFRAUD_HMAC_KEY`, se guarda una copia en el gestor de secretos del
    equipo. Si no, perder el proyecto de Railway hace irrecuperables los datos
    cifrados.

## 2. Servicios del proyecto

Un proyecto, el entorno `production` y tres servicios:

| Servicio | Origen | Root Directory | Build |
|---|---|---|---|
| `Postgres` | Plantilla Postgres de Railway (imagen «SSL-enabled», basada en la oficial `postgres`, con un volumen) | — | — |
| `backend` | Repo GitHub `bacsystem/learning`, rama `main` | `/backend` | `backend/Dockerfile` |
| `frontend` | Repo GitHub `bacsystem/learning`, rama `main` | `/frontend` | `frontend/Dockerfile` |

- **Root Directory**: con él, Railway solo descarga ese directorio para cada
  despliegue ([monorepo](https://docs.railway.com/guides/monorepo)).
- **Dockerfile**: Railway usa el `Dockerfile` de la raíz del origen
  ([dockerfiles](https://docs.railway.com/guides/dockerfiles)). Si con Root
  Directory no lo detecta (sin confirmar), se usa la variable
  `RAILWAY_DOCKERFILE_PATH`.
- **Versión de Postgres**: la documentación no dice qué versión despliega la
  plantilla. La guía de PITR usa `postgres-ssl:16` como ejemplo de tag
  ([point-in-time-recovery](https://docs.railway.com/volumes/point-in-time-recovery)).
  Al aprovisionar se comprueba con `SELECT version();`, que debe dar 16.x
  (mínimo 15).
- **`pg_trgm`**: la documentación no lo menciona
  ([postgresql](https://docs.railway.com/guides/postgresql)). La imagen se
  basa en la oficial, que incluye `contrib`. Se comprueba al aprovisionar con
  `CREATE EXTENSION IF NOT EXISTS pg_trgm;` antes del primer despliegue, porque
  la migración 013 la necesita.

## 3. Configuración por servicio

### `backend`

| Ajuste | Valor | Por qué / fuente |
|---|---|---|
| Pre-deploy Command | `./migrate -dir ./migrations -goose /usr/local/bin/goose up` | Corre entre el build y el deploy, en un contenedor aparte, con las variables del servicio y la red privada. Si falla, el despliegue no sigue y el anterior sigue sirviendo ([pre-deploy-command](https://docs.railway.com/guides/pre-deploy-command)). El advisory lock de `cmd/migrate` sigue protegiendo frente a arranques concurrentes |
| Pre-deploy Timeout | `600` s (igual que `MIGRATE_LOCK_TIMEOUT=10m`) | Sin límite por defecto. Se acepta de 1 a 3600 s |
| Start Command | **vacío** | Con un Start Command, Railway reemplaza el `ENTRYPOINT` de la imagen ([start-command](https://docs.railway.com/guides/start-command)). Vacío, usa el `entrypoint.sh` de la imagen, que vuelve a correr `migrate up`: como el pre-deploy ya migró, es un no-op (`no migrations to run`, comprobado en local) y después arranca `./server` |
| Healthcheck Path | `/health` | Railway espera un 2xx antes de activar el despliegue nuevo. Timeout por defecto: 300 s. `/health` hace ping a la base |
| `RAILWAY_DEPLOYMENT_DRAINING_SECONDS` | `45` | Por defecto Railway manda SIGTERM y mata a los **0 s** ([deployments](https://docs.railway.com/reference/deployments)). El backend necesita hasta 40 s para cerrar HTTP y vaciar el worker de correo (`stop_grace_period: 45s` en compose) |
| Restart policy | `ON_FAILURE` | — |
| Public Networking | dominio generado o propio (CNAME + TXT; TLS automático) | ([public-networking](https://docs.railway.com/guides/public-networking)). El servidor escucha en `:$PORT`; Railway inyecta `PORT` |
| Réplicas | 1 | Ver la nota de conexiones en §4 |

### `frontend`

| Ajuste | Valor | Por qué |
|---|---|---|
| Start Command | vacío (`CMD ["node","server.js"]` de la imagen) | — |
| Healthcheck Path | `/` | — |
| Variables de build | `NEXT_PUBLIC_API_URL` = URL **pública** del backend (`https://…`), y `NEXT_PUBLIC_STORAGE_PREFIX` y `NEXT_PUBLIC_CONTACT_EMAIL` si aplican | Railway pasa las variables al build solo si el Dockerfile las declara con `ARG`, y el nuestro ya lo hace ([dockerfiles](https://docs.railway.com/guides/dockerfiles)). La red privada **no existe durante el build**, y además la URL la usa el navegador, así que siempre es la pública. Si falta, el build falla (guardia del Dockerfile) |
| `PORT` | lo inyecta Railway | La imagen fija `PORT=3000` y `HOSTNAME=0.0.0.0`. Railway inyecta `PORT` al ejecutar (sin confirmar que pise el `ENV` de la imagen). Comprobarlo en el primer despliegue |

### `Postgres`

- **Sin acceso público.** Así no existe `DATABASE_PUBLIC_URL` y no hay
  tráfico de salida facturado.
- El backend se conecta por la **red privada**
  (`postgres.railway.internal`), cifrada con WireGuard
  ([private-networking](https://docs.railway.com/networking/private-networking/how-it-works)).
- Backups y PITR: §5.

## 4. Variables en Railway

Inventario completo: [variables-de-entorno.md](variables-de-entorno.md).

En Railway se cargan como **variables de servicio**. Los secretos van
**sellados**, después de guardar su copia (§1).

| Servicio | Variable | Valor en Railway |
|---|---|---|
| backend | `DATABASE_URL` | `${{Postgres.DATABASE_URL}}`, una variable de referencia. El `sslmode` se decide en el ADR 0004 (§6) |
| backend | `JWT_SECRET`, `ENCRYPTION_KEY` | Sellados. Se generan con `openssl rand -hex 32` y nunca se pegan en mensajes ni en el PR |
| backend | `ENV` | `production` |
| backend | `CORS_ORIGINS`, `FRONTEND_BASE_URL` | El dominio público del frontend |
| backend | `TRUSTED_PROXIES` | El rango del proxy de Railway, sin confirmar: la documentación consultada no lo publica. Mientras no se confirme, el rate limit y la auditoría ven la IP del proxy. **Pendiente**, ver H-3 |
| backend | `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_FROM` | Los del proveedor de correo |
| backend | `SMTP_PASSWORD` | Sellado |
| backend | `MIGRATE_LOCK_TIMEOUT` | `10m` |
| backend | `RAILWAY_DEPLOYMENT_DRAINING_SECONDS` | `45` |
| backend | `LOG_LEVEL` | Vacío (queda en `info`) |
| frontend | `NEXT_PUBLIC_*` | Ver §3 |

**Conexiones.** Cada backend abre hasta **25** conexiones
(`internal/db/db.go`: `SetMaxOpenConns(25)`). Durante el solapamiento de un
despliegue conviven el viejo y el nuevo, y el pre-deploy abre una más. Por
eso `max_connections` tiene que ser ≥ 2 × 25 × réplicas + margen. Se comprueba
con `SHOW max_connections;` al aprovisionar.

## 5. Backups y restauración en Railway

Según [backups](https://docs.railway.com/reference/backups) y
[point-in-time-recovery](https://docs.railway.com/volumes/point-in-time-recovery):

- **Backups del volumen**: diarios (se guardan 6 días), semanales (27 días) y
  mensuales (89 días); se pueden combinar. Los manuales se limitan al 50 % del
  tamaño del volumen.
- **Restaurar un backup** crea un volumen nuevo montado en la misma ruta y deja
  el viejo desmontado. Solo funciona **dentro del mismo proyecto y entorno**.
  **Borrar un volumen borra sus backups.**
- **PITR** (pgBackRest con archivado de WAL): se activa desde la pestaña
  Backups, con una ventana de unas 4 semanas. Restaurar a un instante crea un
  **servicio hermano nuevo** sin tocar el original.
  - Con PITR no se puede fijar una versión menor de Postgres; se mantiene el
    tag mayor (16).
- **Planes**: la documentación no dice qué plan incluye backups o PITR. Las
  imágenes para hacer rollback se conservan 72 h en Hobby y 120 h en Pro
  ([plans](https://docs.railway.com/pricing/plans)). Se confirma con el plan
  que contrate el usuario.

**Hueco frente al ADR 0004:** los backups de Railway viven en el mismo proyecto.
Si se pierde el proyecto o la cuenta, se pierden también los backups.

- **Propuesta:** un `pg_dump -Fc` diario **fuera de Railway**, cifrado. El
  destino es una decisión del usuario y del architect.
- Lo ejecutaría un servicio cron de Railway con la imagen oficial `postgres:16`
  (`deploy.cronSchedule` existe, pero en config as code, que está obsoleto).
  Es un servicio más: lo aprueba el tech-lead.

**Restauración de prueba en Railway**, antes de abrir a usuarios y luego de
forma periódica:

1. Restaurar con PITR a un instante reciente. Esto crea un servicio hermano.
2. Apuntar a ese servicio un backend de prueba temporal, en un entorno aparte
   y con la `ENCRYPTION_KEY` de producción.
3. Comprobar la versión de goose, `GET /health` y que un login descifra.
4. Medir el tiempo, anotarlo y borrar el servicio de prueba, porque contiene
   datos personales.

## 6. Pendiente de confirmar al aprovisionar (con aprobación del usuario)

- [ ] `SELECT version();` → 16.x
- [ ] `CREATE EXTENSION IF NOT EXISTS pg_trgm;` funciona
- [ ] `SHOW max_connections;` ≥ 2 × 25 × réplicas + margen
- [ ] TLS hacia Postgres por la red privada. La documentación no dice qué
      certificado usa la imagen «SSL-enabled» (posiblemente autofirmado, sin
      confirmar). Si es autofirmado, `verify-full` no es posible sin su CA.
      El `sslmode` final lo fija el ADR 0004
- [ ] El Root Directory `/backend` y `/frontend` detecta el `Dockerfile`
- [ ] `PORT` inyectado frente al `ENV PORT=3000` de la imagen del frontend
- [ ] El rango de IP del proxy de Railway, para `TRUSTED_PROXIES`
- [ ] Qué plan incluye backups y PITR
