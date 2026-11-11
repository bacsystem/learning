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
en Docker Hub, así que no depende de la retención de Railway (72/120 h).

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

## 6. Backups y restauración en Railway

Según [backups](https://docs.railway.com/reference/backups) y
[point-in-time-recovery](https://docs.railway.com/volumes/point-in-time-recovery).
Configuración (ADR 0004):

- **PITR activado** (pgBackRest con archivado de WAL a un bucket de Railway).
  La ventana es de unas 4 semanas. Restaurar a un instante crea un **servicio
  nuevo** sin tocar el original. Con PITR no se puede fijar una versión menor;
  se mantiene el tag mayor (16).
- **Backups de volumen diario y semanal.** Los diarios se guardan 6 días y los
  semanales 27. Los manuales se limitan al 50 % del tamaño del volumen.
  - Restaurar uno crea un volumen nuevo, montado en la misma ruta, **dentro del
    mismo proyecto y entorno**. No se puede restaurar en otro entorno.
  - **Borrar un volumen borra sus backups.**
- **Planes:** la documentación no dice qué plan incluye backups, PITR o HA. La
  imagen para hacer rollback se conserva 72 h en Hobby y 120 h en Pro
  ([plans](https://docs.railway.com/pricing/plans)).

**Dos restauraciones de prueba antes del lanzamiento** (con aprobación del
usuario; cada una se mide y se anota aquí):

1. **PITR a un servicio nuevo**, a un instante reciente.
   - Comprobar la versión de goose y el conteo de filas de las tablas
     principales.
   - Medir el **RPO real**: diferencia entre el instante pedido y la última
     escritura recuperada.
2. **Backup de volumen**, restaurado en el mismo entorno `production`, porque
   Railway no restaura en otro.
   - Se hace **antes** de abrir a usuarios, cuando la base todavía no tiene
     datos reales.
   - Después, el backend tiene que arrancar y `GET /health` responder 200.

En las dos se borra lo restaurado al terminar si contiene datos personales.

Sigue abierto para el usuario (U4): si hace falta además una copia **fuera** de
Railway, porque los backups viven en el mismo proyecto.

## 7. Pendiente de confirmar al aprovisionar (con aprobación del usuario)

- [ ] `SELECT version();` → 16.x
- [ ] `CREATE EXTENSION IF NOT EXISTS pg_trgm;` funciona
- [ ] `SHOW max_connections;` ≥ 2 × 25 × réplicas + margen
- [ ] `sslmode=require` funciona por la URL privada, y cómo se agrega el
      parámetro a la variable de referencia
- [ ] Qué hace el pooling de Railway, si lo hay, con `DATABASE_URL`
- [ ] Región y cifrado en reposo de los backups y del bucket de PITR
- [ ] Cómo correr `cmd/seed/admin` y `cmd/rehash-emails` contra producción:
      las variables selladas no salen en `railway run`. Opción a confirmar:
      un servicio de un solo uso con la imagen del backend y las mismas
      variables por referencia
- [ ] El servicio acepta la imagen pública de Docker Hub como fuente y
      `RAILWAY_HEALTHCHECK_TIMEOUT_SEC=660` se aplica
- [ ] `PORT` inyectado frente al `ENV PORT=3000` de la imagen del frontend
- [ ] El rango de IP del proxy de Railway, para `TRUSTED_PROXIES`
- [ ] Qué plan incluye backups, PITR y HA (en el panel o con soporte)
