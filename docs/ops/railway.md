# Despliegue en Railway (runbook)

Decisión del usuario (11-11-2026): por ahora los despliegues van a Railway. La
base de producción se documenta en el
[ADR 0004](../adr/0004-alojamiento-base-de-datos-produccion.md) (architect,
propuesto). Railway es el punto de partida y la app no depende de él: la
configuración va solo por variables de entorno, y las referencias `${{…}}`
viven solo en el panel de Railway.

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
    y queda escrita en este documento (§4).
  - Adoptar `.railway/railway.ts` sería una herramienta nueva: lo tiene que
    aprobar el tech-lead.
- **El healthcheck solo se usa durante el despliegue.** Railway no vigila el
  endpoint una vez que el despliegue está activo.
  ([healthchecks](https://docs.railway.com/guides/healthchecks))
  - Para tener alertas hace falta un monitor externo de `/health`. Es un
    servicio nuevo: **pendiente de decisión del usuario**.
- **Despliegue por imagen** (decisión del usuario). `backend` y `frontend`
  usan como fuente una **imagen de Docker Hub**, no un build desde el repo. Con
  una imagen como fuente, Railway se salta el build
  ([deployments](https://docs.railway.com/reference/deployments)).
  - Desplegar es cambiar el **tag de la imagen** en el servicio, con la
    aprobación del usuario.
  - Sin repo conectado no hay autodeploy, así que nada despliega solo al hacer
    merge a `main`.
- **Variables selladas.** Una variable sellada no se ve en el panel, no se lee
  por la API y no se puede «des-sellar».
  ([variables](https://docs.railway.com/guides/variables))
  - Antes de sellar `ENCRYPTION_KEY`, `JWT_SECRET` o (cuando exista)
    `ANTIFRAUD_HMAC_KEY`, se guarda una copia en el gestor de secretos del
    equipo. Si no, perder el proyecto de Railway hace irrecuperables los datos
    cifrados.
- **Las migraciones corren en el arranque del backend** (`entrypoint.sh`, con
  advisory lock), **no** en el Pre-deploy Command de Railway. Lo fija la
  Decisión 9 del ADR 0004: el despliegue funciona igual en `deploy/local` y en
  cualquier otra nube.
  - Consecuencia: si una migración falla, el despliegue nuevo no pasa el
    healthcheck. Railway no lo activa y el anterior sigue sirviendo, porque
    solo cambia de despliegue tras un 2xx en `/health`
    ([healthchecks](https://docs.railway.com/guides/healthchecks)).
  - **Ojo:** la migración que falla a medias pudo haber cambiado el esquema
    que usa el despliegue anterior. Por eso las migraciones son aditivas
    (expand/contract) y hay un backup antes de cada despliegue.

## 2. Imágenes y tags

Repositorios **públicos** en Docker Hub (decisión del usuario):
`cbaciliod/learning-backend` y `cbaciliod/learning-frontend`. Cualquiera
puede bajarlas, así que **nunca llevan secretos**: ni en capas, ni en
build-args, ni en `ENV`. Los `NEXT_PUBLIC_*` son públicos por definición y no
llevan nada sensible. Antes de publicar se ejecuta la verificación de
contenido ([checklist-release.md](checklist-release.md) §5).

| Imagen | Tag | Cómo se construye |
|---|---|---|
| backend | `vX.Y.Z` | `make docker/release DOCKER_TAG=vX.Y.Z`. Una sola imagen para todos los entornos: la configuración entra por variables |
| frontend, producción | `vX.Y.Z` | `make docker/release DOCKER_TAG=vX.Y.Z`, con `NEXT_PUBLIC_API_URL` = API de producción |
| frontend, otro entorno | `vX.Y.Z-<entorno>` (p. ej. `v1.0.0-preprod`) | `make docker/build/frontend DOCKER_TAG=vX.Y.Z FRONTEND_ENV=<entorno> NEXT_PUBLIC_API_URL=<api del entorno>` |

La imagen del frontend es **por entorno** porque `NEXT_PUBLIC_API_URL` se
incrusta en el build. La alternativa sería que el frontend lea la URL en
ejecución, lo que cambia el código de la app y se evalúa con el architect.
Los tags publicados no se reescriben: si un tag necesita una corrección, sale
uno nuevo.

**Rollback**: volver a poner en el servicio el tag anterior. Las imágenes viven
en Docker Hub, así que no depende de la retención de Railway (120 h en Pro).

## 3. Servicios del proyecto

Un proyecto, el entorno `production` y tres servicios:

| Servicio | Fuente |
|---|---|
| `Postgres` | Plantilla **oficial** de PostgreSQL de Railway, **sin cambiar la imagen ni el start command**: si se cambian, Railway deja de gestionarla ([point-in-time-recovery](https://docs.railway.com/volumes/point-in-time-recovery)). La imagen es «SSL-enabled», basada en la oficial `postgres`, con un volumen |
| `backend` | Imagen `cbaciliod/learning-backend:vX.Y.Z` |
| `frontend` | Imagen `cbaciliod/learning-frontend:vX.Y.Z` |

- **Versión de Postgres**: la documentación de Railway no la dice. El README de
  `railwayapp-templates/postgres-ssl` indica que `:latest` es 16 y que incluye
  `contrib` (fuente: architect, ADR 0004). Al aprovisionar se comprueba con
  `SELECT version();`, que debe dar 16.x (mínimo 15), y se anota aquí. Si no
  es 16, se avisa al architect para alinear el CI.
- **`pg_trgm`**: se comprueba al aprovisionar con
  `CREATE EXTENSION IF NOT EXISTS pg_trgm;`, antes del primer despliegue,
  porque la migración 013 la necesita.

## 4. Configuración por servicio

### `backend`

| Ajuste | Valor | Por qué / fuente |
|---|---|---|
| Fuente | Imagen `cbaciliod/learning-backend:vX.Y.Z` | §2 |
| Pre-deploy Command | **vacío** | Las migraciones las hace `entrypoint.sh` (§1) |
| Start Command | **vacío** | Con un Start Command, Railway reemplaza el `ENTRYPOINT` de la imagen ([start-command](https://docs.railway.com/guides/start-command)), y se saltaría `entrypoint.sh` y la migración |
| Healthcheck Path | `/health` | Railway espera un 2xx antes de activar el despliegue nuevo. `/health` hace ping a la base |
| `RAILWAY_HEALTHCHECK_TIMEOUT_SEC` | `660` | Por defecto son 300 s. Tiene que cubrir la espera del lock (`MIGRATE_LOCK_TIMEOUT=10m`) más la migración y el arranque |
| `RAILWAY_DEPLOYMENT_DRAINING_SECONDS` | `45` | Por defecto Railway manda SIGTERM y mata a los **0 s** ([deployments](https://docs.railway.com/reference/deployments)). El backend necesita hasta 40 s para cerrar HTTP y vaciar el worker de correo (`stop_grace_period: 45s` en compose) |
| Restart policy | `ON_FAILURE` | — |
| Public Networking | dominio generado o propio (CNAME + TXT; TLS automático) | ([public-networking](https://docs.railway.com/guides/public-networking)). El servidor escucha en `:$PORT`; Railway inyecta `PORT` |
| Réplicas | 1 | Ver la nota de conexiones en §5 |

### `frontend`

| Ajuste | Valor | Por qué |
|---|---|---|
| Fuente | Imagen `cbaciliod/learning-frontend:vX.Y.Z` | Construida con el `NEXT_PUBLIC_API_URL` **público** de producción (§2). En el panel de Railway no se cargan `NEXT_PUBLIC_*`: no tendrían efecto |
| Start Command | vacío (`CMD ["node","server.js"]` de la imagen) | — |
| Healthcheck Path | `/` | — |
| `PORT` | lo inyecta Railway | La imagen fija `PORT=3000` y `HOSTNAME=0.0.0.0`. Hay que comprobar al aprovisionar que el `PORT` de Railway pisa el de la imagen (sin confirmar) |

### `Postgres`

- **Sin acceso público.** Así no existe `DATABASE_PUBLIC_URL` y no hay
  tráfico de salida facturado.
- El backend se conecta por la **red privada**
  (`postgres.railway.internal`), cifrada con WireGuard
  ([private-networking](https://docs.railway.com/networking/private-networking/how-it-works)),
  con `sslmode=require`. `verify-full` no es posible: el certificado lo
  genera la propia imagen (ADR 0004, R5 «parcial»).
- Sin TCP Proxy (acceso público) y sin pooler. Así `cmd/migrate` y su
  advisory lock de sesión funcionan sin cambios.
- Backups y PITR: §6.

## 5. Variables en Railway

Inventario completo: [variables-de-entorno.md](variables-de-entorno.md).

En Railway se cargan como **variables de servicio**. Los secretos van
**sellados**, después de guardar su copia (§1).

| Servicio | Variable | Valor en Railway |
|---|---|---|
| backend | `DATABASE_URL` | `${{Postgres.DATABASE_URL}}?sslmode=require`: la URL privada, por referencia. Con H-5 (backend-dev-2), el backend exigirá un `sslmode` explícito en producción. Comprobar que la URL de referencia no traiga ya parámetros (si trae `?`, hay que agregar `&sslmode=require`) |
| backend | `JWT_SECRET`, `ENCRYPTION_KEY`, `ANTIFRAUD_HMAC_KEY` (cuando exista) | Sellados. Se generan con `openssl rand -hex 32` y nunca se pegan en mensajes ni en el PR. **El usuario** guarda una copia de `ENCRYPTION_KEY` fuera de Railway antes de sellarla |
| backend | `ENV` | `production` |
| backend | `CORS_ORIGINS`, `FRONTEND_BASE_URL` | El dominio público del frontend |
| backend | `TRUSTED_PROXIES` | El rango del proxy de Railway, sin confirmar: la documentación consultada no lo publica. Mientras no se confirme, el rate limit y la auditoría ven la IP del proxy. **Pendiente**, ver H-3 |
| backend | `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_FROM` | Los del proveedor de correo |
| backend | `SMTP_PASSWORD` | Sellado |
| backend | `MIGRATE_LOCK_TIMEOUT` | `10m` |
| backend | `RAILWAY_DEPLOYMENT_DRAINING_SECONDS` | `45` |
| backend | `LOG_LEVEL` | Vacío (queda en `info`) |
| frontend | — | Los `NEXT_PUBLIC_*` van en la imagen, no en Railway (§2) |

**Conexiones.** Cada backend abre hasta **25** conexiones
(`internal/db/db.go`: `SetMaxOpenConns(25)`). Durante el solapamiento de un
despliegue conviven el viejo y el nuevo, y `cmd/migrate` abre una más al arrancar. Por
eso `max_connections` tiene que ser ≥ 2 × 25 × réplicas + margen. Se comprueba
con `SHOW max_connections;` al aprovisionar.

## 6. Backups, restauración y objetivos

**Objetivos (decisión del usuario):**

- **RPO 5 min con PITR.** Si solo queda la copia de R2, el RPO es de 24 h.
- **RTO 2 h.**
- **Retención en R2: 30 días.**

Plan **Pro** de Railway. La verificación de estas condiciones antes del
lanzamiento se registra en
[condiciones-lanzamiento.md](condiciones-lanzamiento.md) (C1–C13 del
ADR 0004).

Según [backups](https://docs.railway.com/reference/backups) y
[point-in-time-recovery](https://docs.railway.com/volumes/point-in-time-recovery):

- **PITR activado** (pgBackRest con archivado de WAL a un bucket de Railway).
  La ventana es de unas 4 semanas. Restaurar a un instante crea un **servicio
  nuevo** sin tocar el original. Con PITR no se puede fijar una versión menor;
  se mantiene el tag mayor (16).
- **Backups de volumen: solo diario y semanal.** Los diarios se guardan 6 días
  y los semanales 27.
  - **El mensual NO se activa**: guarda 89 días, y ninguna copia debe pasar de
    unos 30 días.
  - Los backups manuales se limitan al 50 % del tamaño del volumen.
  - Restaurar uno crea un volumen nuevo, montado en la misma ruta, **dentro del
    mismo proyecto y entorno**. No se puede restaurar en otro entorno.
  - **Borrar un volumen borra sus backups.**
- **Copia diaria fuera de Railway, en Cloudflare R2** (ADR 0004, D11). La crea
  un cron con `pg_dump` 16, cifrada con age, y el bucket lleva ciclo de vida y
  bucket lock de **30 días**.
  - El bucket, el token y las reglas los crea el usuario cuando el diseño esté
    aprobado. La implementación va en un PR aparte (`deploy/backup/`).
  - La jurisdicción del bucket la decide el usuario (C12).
- **Plan Pro:** la documentación no dice si backups, PITR, HA y cron jobs
  dependen del plan. **A confirmar en el panel** (C1).
  - La imagen para hacer rollback se conserva **120 h** en Pro
    ([plans](https://docs.railway.com/pricing/plans)), aunque el rollback
    principal es volver al tag anterior de la imagen (§2).

**Restauraciones de prueba antes del lanzamiento** (con aprobación del
usuario). Cada una se cronometra y el resultado se anota en
[condiciones-lanzamiento.md](condiciones-lanzamiento.md):

| Prueba | Procedimiento | Criterio de éxito |
|---|---|---|
| PITR a un servicio nuevo | 1. Anotar la hora T0. 2. Escribir una fila marcador. 3. Restaurar a T0 + 1 min, lo que crea un servicio nuevo. 4. Apuntar a él un backend temporal. 5. Comprobar la versión de goose, el conteo de filas, `GET /health` y un login que descifre | **RPO medido ≤ 5 min** (la diferencia entre el último commit y el punto restaurado) y **tiempo total ≤ 2 h** (C5, C6) |
| Backup de volumen | Restaurarlo en el mismo entorno `production`, **antes** de abrir a usuarios, cuando la base todavía no tiene datos reales. Después, el backend arranca y `GET /health` responde 200 | Arranca y responde 200 |
| Copia de R2 | Bajar con curl, descifrar con `age -d -i <clave>` (la clave privada se carga solo durante la prueba) y hacer `pg_restore` en un Postgres 16 efímero y aislado, **no** en `deploy/`, que se destruye al terminar | **Tiempo total ≤ 2 h** (C6, C7) |

En todas se borra lo restaurado al terminar si contiene datos personales. La
prueba de R2 se repite **cada mes**.

## 7. Monitoreo y alertas (plan Pro, sin servicio externo)

El usuario decidió habilitar lo que ofrece Railway, sin servicios externos.

| Qué | Cómo | Fuente |
|---|---|---|
| CPU, RAM, disco y salida de red | **Monitors** en el Observability Dashboard: umbral por encima o por debajo y aviso por email, in-app o webhook. Son de Pro y se agregan por entorno | [observability](https://docs.railway.com/guides/observability) |
| Caída del proceso | Restart policy `ON_FAILURE`. Al agotar los reintentos el deploy pasa a `Crashed` y Railway avisa por email a los miembros del proyecto y por webhook | [deployment-actions](https://docs.railway.com/guides/deployment-actions), [restart-policy](https://docs.railway.com/deployments/restart-policy) |
| Deploy fallido y volumen casi lleno | Webhooks del proyecto (Settings → Webhooks), con formato nativo para Slack y Discord. No van firmados: se protegen con una cabecera secreta | [webhooks](https://docs.railway.com/observability/webhooks) |

**Qué no cubre** (el usuario lo sabe):

- No hay chequeo HTTP continuo: `/health` solo se consulta al desplegar
  ([healthchecks](https://docs.railway.com/guides/healthchecks)).
- Si el proceso está vivo pero `/health` responde 503 (por ejemplo, con la
  base caída), el deploy sigue `Active` y **no salta ninguna alerta**.
- Railway no recoge latencia ni tasa de errores HTTP
  ([metrics](https://docs.railway.com/reference/metrics)).
- No está documentado si una ejecución fallida de un cron avisa. Railway no
  corta un cron colgado
  ([cron-jobs](https://docs.railway.com/reference/cron-jobs)).

Por eso el cron de R2 lleva su propio `timeout` y su propio aviso (C13). Las
alternativas para el hueco de `/health` (la plantilla de Uptime Kuma dentro del
proyecto, o que el backend salga con error si pierde la base) están
pendientes de decisión del usuario.

## 8. Pendiente de confirmar al aprovisionar (con aprobación del usuario)

Se registra en [condiciones-lanzamiento.md](condiciones-lanzamiento.md). Además
de C1–C13:

- [ ] El servicio acepta la imagen pública de Docker Hub como fuente y
      `RAILWAY_HEALTHCHECK_TIMEOUT_SEC=660` se aplica
- [ ] `PORT` inyectado frente al `ENV PORT=3000` de la imagen del frontend
- [ ] El rango de IP del proxy de Railway, para `TRUSTED_PROXIES`
