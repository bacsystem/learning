# ADR 0004: despliegue en Railway y alojamiento de la base de datos de producción

- **Estado:** propuesto (2026-11-11).
  - Plataforma decidida por el usuario (vía tech-lead): por ahora el backend, el frontend y Postgres 16 se despliegan en **Railway**. Es el punto de partida, y según la demanda se puede migrar a otra nube. El contrato es portable: imágenes Docker más variables de entorno (Decisión 9).
  - Antes de Railway se valida en una **preproducción local** (`deploy/`), donde prueba qa (Decisión 10).
  - Pendientes del usuario: el plan de Railway (U1), RPO (U2), RTO (U3), retención y copia fuera de Railway (U4). Con esas respuestas, el ADR pasa a «aceptado».
- **Fecha:** 2026-11-11
- **Pedido por:** devops, para el primer despliegue. Su plan operativo está en `docs/ops/plan-base-produccion.md` (PR #97).
- **Relacionado:** [ADR 0001](0001-unicidad-proceso-asignaturas-por-especialidad.md) § Versión de Postgres en producción (Postgres 16, mínimo 15).
- **Fuentes:** documentación oficial de Railway, consultada el 2026-11-11. Lo que la documentación **no** dice se marca como «no documentado» y se verifica en el panel al aprovisionar, sin suponerlo.

## Contexto

Todavía no existe base de datos de producción (ADR 0001). Hoy la base solo existe en compose (`docker-compose.yml:24`, `postgres:16-alpine` con el volumen `pgdata`, `:32-33`) y en el CI (`.github/workflows/backend.yml:31`).

### Requisitos que salen del código

| # | Requisito | Evidencia |
|---|---|---|
| R1 | Postgres 16 (mínimo 15), la misma versión que el CI | `UNIQUE NULLS NOT DISTINCT` en `backend/migrations/060_add_especialidad_to_proceso_asignaturas.sql:25`; ADR 0001 |
| R2 | Extensión `pg_trgm` | `backend/migrations/013_create_etiquetas_pregunta.sql:2` |
| R3 | Las migraciones necesitan una **conexión directa de sesión**. Un pooler en modo *transaction* rompe el advisory lock de sesión | `backend/cmd/migrate/main.go:99-132`. Corre en cada arranque del contenedor con el **mismo** `DATABASE_URL` que el servidor (`backend/entrypoint.sh:9-12`) |
| R4 | Cada réplica del backend abre hasta 25 conexiones | `backend/internal/db/db.go:17` |
| R5 | TLS hacia la base: compose usa `sslmode=disable`, y producción no puede | `docker-compose.yml:56` |
| R6 | `ENCRYPTION_KEY` (cifrado de datos personales) es obligatoria y no vive en la base. Sin esa clave un backup no sirve | `backend/internal/config/config.go:51,76-77`, `docker-compose.yml:58` |
| R7 | La base guarda datos personales: backups, acceso restringido y recuperación ante un borrado accidental | CLAUDE.md (`assets/`); README de contratos §9 |

### Lo que ofrece Railway (verificado en la documentación oficial)

| Tema | Lo que dice Railway | Fuente |
|---|---|---|
| Responsabilidades | Railway se ocupa del aprovisionamiento, el almacenamiento, la red y los parches de seguridad de la imagen. Los backups, el PITR, la alta disponibilidad, el pooling, el tamaño del disco y las actualizaciones mayores los **activa el usuario** | docs.railway.com/databases § «What Railway manages» |
| Imagen y versión | Imagen «SSL-enabled Postgres», basada en la oficial (`postgres:<versión>-trixie`). Hay imágenes de la 13 a la 18, y `:latest` «currently points to PostgreSQL 16». La guía no dice qué versión despliega la plantilla | docs.railway.com/guides/postgresql; github.com/railwayapp-templates/postgres-ssl (README) |
| Cambiar la imagen | «Changing the template as shipped, such as its image, start command or variables, makes the database yours to manage» | docs.railway.com/guides/postgresql |
| Extensiones | La imagen trae los módulos *contrib* más pgvector y pgaudit. `pg_trgm` es *contrib*. Railway dice que no agregará extensiones a la plantilla | postgres-ssl (README); guía de PostgreSQL |
| Red | «Databases are deployed private by default». El tráfico entre servicios va por la red privada (`SERVICE.railway.internal`, «encrypted Wireguard tunnels»). El acceso público es opcional (TCP Proxy, con egress facturado) | guía de PostgreSQL; docs.railway.com/reference/private-networking |
| Variables | La base expone `DATABASE_URL`, `PGHOST`, `PGPORT`, `PGUSER`, `PGPASSWORD` y `PGDATABASE`, y `DATABASE_PUBLIC_URL` si hay acceso público. Otro servicio la referencia con `${{Postgres.DATABASE_URL}}`. Existen las *sealed variables*: no se ven en la UI ni en la API, no se pueden des-sellar y no se copian a entornos de PR | guía de PostgreSQL; docs.railway.com/variables |
| TLS | Certificado generado por `init-ssl.sh` al inicializar y renovado al redeploy o reinicio si vence en ≤ 30 días (820 días por defecto). No documenta una CA verificable ni el `sslmode` del cliente | postgres-ssl (README) |
| Pooler | No viene uno por defecto. PgBouncer es opcional y lo activa el usuario | guía de PostgreSQL; § «What Railway manages» |
| Backups de volumen | Diario (retención 6 días), semanal (27) y mensual (89), más backups manuales (≤ 50 % del tamaño del volumen). La restauración monta un volumen nuevo y redespliega. «Backups can only be restored into the same project + environment». «Wiping a volume deletes all backups». Se cobran como volumen | docs.railway.com/volumes/backups |
| PITR | Archiva el WAL en cada commit (`archive_timeout=60`) a un bucket de Railway, con base backups de pgBackRest. «The last 4 full backups are retained, giving you a restore window of roughly 4 weeks». La restauración crea un **servicio Postgres nuevo** y el cambio a ese servicio es manual. La ventana empieza en el primer base backup después de activarlo. No admite fijar la versión menor. Se cobra el almacenamiento del bucket y el egress | docs.railway.com/volumes/point-in-time-recovery |
| Alta disponibilidad | Conversión opcional a un clúster con failover automático (Patroni, etcd y HAProxy) | guía de PostgreSQL |
| Planes | Hobby 5 USD/mes (volumen ≤ 5 GB, 6 réplicas) y Pro 20 USD/mes (≤ 1 TB, 42 réplicas). **No documentado:** qué plan incluye backups, PITR y alta disponibilidad | docs.railway.com/reference/pricing/plans; railway.com/pricing |
| Región de backups y PITR | **No documentado**: no dice si se guardan fuera de la región del servicio | backups y PITR |

### Cumplimiento de R1–R7 con Railway

| # | ¿Cumple? | Cómo | Si no se confirma |
|---|---|---|---|
| R1 | Sí, a verificar | `:latest` de la imagen es 16; se comprueba con `SELECT version()` | Alinear CI y compose con la versión real (≥ 15), o elegir A |
| R2 | Sí | `pg_trgm` es *contrib* y viene en la imagen | A o B |
| R3 | Sí | Sin pooler por defecto; la URL privada es directa | Si se activa el pooler, migrar por la URL directa |
| R4 | A verificar | `SHOW max_connections` frente a réplicas × 25 | Limitar las réplicas |
| R5 | Parcial | `sslmode=require` sobre la red privada WireGuard; `verify-full` no es posible | Se acepta por la red privada; si el usuario exige `verify-full`, A |
| R6 | Sí | Sealed variables más una copia de `ENCRYPTION_KEY` fuera de Railway | — |
| R7 | Parcial | Backups de volumen y PITR, pero dentro de Railway y del mismo proyecto; el plan, la región y el cifrado en reposo no están documentados | Copia externa (U4) o A |

**A y B quedan como alternativas** si Railway no cumple algún punto marcado «a verificar» o «parcial» que el usuario considere obligatorio.

## Opciones consideradas para la base

Se mantienen las letras A, B y C del plan de devops. Railway se suma como opción D.

### D. Postgres de Railway, en el mismo proyecto que el backend y el frontend (elegida)

- **R1:** la plantilla oficial usa la imagen `postgres-ssl`, cuyo `:latest` es 16. Al aprovisionar se comprueba con `SELECT version()`. **No** se cambia la imagen para fijar la versión, porque así la base deja de ser gestionada por Railway.
- **R2:** `pg_trgm` es *contrib* y viene en la imagen.
- **R3:** sin pooler por defecto. El backend usa el `DATABASE_URL` privado, que es una conexión directa: `cmd/migrate` funciona sin cambios.
- **R5:** cifrado en la red privada (Wireguard) y TLS del servidor.
- **R7:** backups de volumen y PITR, ambos dentro de Railway.
- **Ventajas:** una sola plataforma, red privada sin exponer la base y sin servicios nuevos fuera de Railway.
- **Límites:**
  - Los backups y el PITR viven en el mismo proveedor, y los backups de volumen además en el mismo proyecto y entorno: borrar el volumen borra sus backups.
  - El certificado TLS no permite `verify-full`.
  - El plan que incluye backups y PITR no está documentado.

### A. Postgres gestionado externo con PITR (RDS, Cloud SQL, DO Managed, Neon…)

- PITR con retención configurable, backups en otra región o cuenta, y `verify-full` contra la CA del proveedor.
- La app de Railway llegaría a la base por internet: la documentación consultada no describe una red privada entre Railway y otros proveedores (no verificado). Habría que exponer la base con allowlist de IPs y pagar egress.
- Es un servicio externo nuevo, con costo y contrato propios.
- **Descartada por ahora:** el usuario eligió Railway. Se reabre con un ADR nuevo si U1–U4 exigen algo que Railway no da, por ejemplo backups fuera del proveedor por razón legal.

### B. Postgres 16 propio en una VM, con archivado de WAL

Descartada: el usuario eligió Railway. Además exige operar a mano los backups, los parches y el TLS.

### C. Postgres en el mismo host que la app, solo con `pg_dump`

Descartada: el usuario eligió Railway. Además, sin PITR se pierden hasta 24 h de datos, lo que no es aceptable con datos personales (R7).

## Decisión (propuesta)

**Backend, frontend y Postgres en un solo proyecto de Railway**, con un entorno `production` separado de cualquier entorno de pruebas o PR.

1. **Base:** la plantilla oficial de PostgreSQL de Railway, sin cambiar la imagen ni el start command.
   - Al aprovisionar, devops registra la salida de `SELECT version();` y de `SELECT * FROM pg_available_extensions WHERE name = 'pg_trgm';`.
   - Si la versión no es la 16, se avisa al architect antes de migrar. Hay que alinear el CI (`backend.yml:31`) y `docker-compose.yml:24` con la versión real, porque producción y CI deben coincidir (ADR 0001).
2. **Conexión del backend:**
   - `DATABASE_URL = ${{Postgres.DATABASE_URL}}`, la URL **privada** (`*.railway.internal`), con `sslmode=require` explícito. `verify-full` no es posible, porque el certificado lo genera la propia imagen y Railway no documenta una CA verificable. La red privada ya va cifrada con Wireguard.
   - **Sin** acceso público a la base (sin TCP Proxy). Si un admin necesita entrar, habilita el TCP Proxy de forma temporal y lo quita al terminar.
3. **Sin pooler** (R3). Si se activa el PgBouncer de Railway, el backend sigue usando la URL **directa** para migrar. Separar `MIGRATE_DATABASE_URL` es un cambio de código con ADR propio, fuera del MVP.
4. **Conexiones:** réplicas del backend × 25 (R4) + margen ≤ `max_connections`. Devops anota el valor real de `SHOW max_connections;` y limita las réplicas en consecuencia.
5. **Backups:**
   - Se activan **PITR** y los backups de volumen **diario y semanal**.
   - Antes de abrir el servicio a usuarios se hacen **restauraciones de prueba**, y después periódicamente:
     - de PITR, a un servicio nuevo, sin tocar el original;
     - de un backup de volumen, **en el mismo entorno `production`**, porque Railway solo restaura en el mismo proyecto y entorno. Se hace antes del lanzamiento, cuando la base todavía no tiene datos reales.
   - En la prueba de PITR, devops mide el RPO real: la diferencia entre el último commit y el punto restaurable.
   - El RPO, el RTO y la retención finales dependen de U2–U4.
6. **Secretos:**
   - `JWT_SECRET`, `ENCRYPTION_KEY`, la clave HMAC (ADR 0002) y las credenciales SMTP van como **sealed variables** en el servicio backend.
   - `ENCRYPTION_KEY` tiene además una copia custodiada por el usuario **fuera de Railway** (R6). Si se pierde, los datos personales cifrados son irrecuperables. Si la clave solo existe dentro de Railway, perder el proyecto se lleva la clave y los datos a la vez.
7. **Logs:** sin datos personales ni parámetros de consultas. No se activa `log_statement = all` ni la configuración de pgaudit que registra parámetros (README de contratos §9).
8. **Frontend:** servicio propio con `NEXT_PUBLIC_API_URL` apuntando al dominio público del backend. El backend expone solo su dominio público; la base no expone ninguno.
9. **Portabilidad (contrato con la plataforma).** Railway es el punto de partida: según la demanda se puede migrar a otra nube. Por eso el contrato entre la app y la plataforma es **una imagen Docker por servicio más su configuración por variables de entorno**, y nada más:
   - **Imágenes:** las mismas `Dockerfile` de backend y frontend que usa compose. El backend migra en su arranque (`entrypoint.sh`), así que no depende de ganchos de deploy de la plataforma.
   - **Configuración:** solo variables de entorno estándar: `DATABASE_URL`, `JWT_SECRET`, `ENCRYPTION_KEY`, `CORS_ORIGINS`, `TRUSTED_PROXIES`, SMTP y las demás de `config.go` y `docker-compose.yml:55-69`. Las referencias `${{…}}` de Railway se resuelven **en el panel**, nunca en el código.
   - **Sin dependencias exclusivas de Railway en el código:** el código no lee variables `RAILWAY_*`, no usa SDKs ni APIs de Railway y no depende de sus dominios `*.railway.internal`, que solo aparecen como valor de `DATABASE_URL`. Toda excepción necesita un ADR.
   - **Base:** Postgres estándar. Migrar de nube significa `pg_dump` / `pg_restore`, o una réplica lógica, hacia otro Postgres 16 con `pg_trgm`, y cambiar `DATABASE_URL`. La recuperación a un punto en el tiempo y los backups de Railway no se migran: se reconfiguran en el destino.
   - Hoy se cumple: no hay ninguna referencia a `RAILWAY` en `backend/` ni en `frontend/src` (búsqueda del 2026-11-11).
   - **Frontend:** `NEXT_PUBLIC_*` se fija en el **build** de Next.js (`ARG NEXT_PUBLIC_API_URL` en `frontend/Dockerfile:19-22`), así que la imagen del frontend se construye por entorno, con su `NEXT_PUBLIC_API_URL`. Es un límite del framework, no de la plataforma, y aplica igual en cualquier nube.
10. **Entornos.**
    - **Preproducción local:** un directorio `deploy/` del repo levanta el stack con las mismas imágenes y la misma forma de configuración que producción. qa valida ahí antes de cada despliegue a Railway. Devops define su estructura en su plan (`docs/ops/`) y en `deploy/`.
    - Para que la validación de qa valga para producción, la preproducción usa **Postgres 16**, el mismo `entrypoint.sh` con migraciones y `ENV` distinto de development/test. Con eso aplica la validación de `sslmode` de producción (hallazgo H-5 del PR #97), así que la base local de preproducción se levanta con TLS.
    - Los datos de preproducción son sintéticos o semilla, **nunca** copias de producción ni de `assets/`, porque contienen datos personales.
    - **Producción:** Railway, entorno `production`.

**Antes de contratar, devops verifica en el panel o con soporte** lo que Railway no documenta:

- qué plan incluye backups, PITR y alta disponibilidad;
- en qué región se guardan los backups y el bucket de PITR, y si los volúmenes, los backups y el bucket están **cifrados en reposo**;
- qué versión de Postgres despliega la plantilla;
- que `sslmode=require` conecta por la URL privada, y cómo se agrega ese parámetro a `${{Postgres.DATABASE_URL}}` (p. ej. `${{Postgres.DATABASE_URL}}?sslmode=require`, si la URL no trae ya parámetros);
- qué pasa con `DATABASE_URL` si se activa el pooling de Railway, si reemplaza la URL directa o crea otra. Esto no quedó verificado: la página de pooling no estaba disponible.

## Pendiente de decisión del usuario

| # | Pregunta | Lo que Railway permite |
|---|---|---|
| U1 | Proveedor: **resuelto, Railway**. Queda elegir el plan (Hobby o Pro) | Hobby: volumen ≤ 5 GB. Pro: hasta 1 TB. Los backups y el PITR se cobran como almacenamiento y egress. El plan requerido no está documentado (lo verifica devops) |
| U2 | **RPO**: cuántos datos se pueden perder como máximo | Railway no publica un RPO para Postgres. **Inferencia** a partir de lo que sí documenta (WAL en cada commit, `archive_timeout=60`): del orden de 1 minuto con PITR, a confirmar en la restauración de prueba. Solo con backups de volumen, hasta 24 h (diario) |
| U3 | **RTO**: en cuánto tiempo hay que volver a dar servicio | Restaurar un backup de volumen tarda «seconds to minutes» más el redeploy. El PITR crea un servicio nuevo y el cambio a él es manual. La alta disponibilidad da failover automático ante la caída del nodo, pero no protege de un borrado |
| U4 | **Retención** de backups y de datos personales (plazo legal; Ley 29733 de Perú si aplica), y si hace falta una **copia fuera de Railway** | Volumen: 6, 27 y 89 días (fijos por programa). PITR: unas 4 semanas (últimos 4 base backups). Ninguno se guarda fuera de Railway según la documentación. Una copia externa (p. ej. `pg_dump` cifrado a otro proveedor) sería un servicio nuevo: requiere tu aprobación y un ADR |

## Consecuencias

- **Positivas:**
  - Una sola plataforma y red privada: la base no queda expuesta a internet.
  - El primer despliegue no necesita cambios de código. `cmd/migrate` funciona con la conexión directa privada.
  - Hay PITR sin operar WAL a mano. El RPO estimado es de aproximadamente un minuto, a confirmar en la prueba.
  - La migración a otra nube queda acotada: cambiar dónde corren las imágenes, recrear las variables y mover la base con `pg_dump` / `pg_restore`.
- **Negativas y riesgos:**
  - La imagen del frontend no es la misma entre entornos, porque `NEXT_PUBLIC_API_URL` se fija en el build. Lo que qa valida en preproducción es el mismo **código** y el mismo Dockerfile, no el mismo binario del frontend.
  - Los backups y el PITR quedan en el mismo proveedor: una pérdida de la cuenta o del proyecto se lleva datos y backups juntos. Se mitiga con la copia externa de U4 si el usuario la aprueba.
  - Solo hay TLS `require`, sin verificar el certificado. Se acepta porque el tráfico va por la red privada cifrada.
  - La versión de la plantilla no se fija sin perder la gestión de Railway. Hay que vigilar que CI y producción sigan en la misma versión mayor.
  - Sin pooler, las conexiones crecen con las réplicas (25 por réplica).
  - En el MVP hay un solo rol con permisos de esquema, porque servidor y migración comparten `DATABASE_URL`.
  - Comandos puntuales que necesitan `DATABASE_URL` y `ENCRYPTION_KEY` (`cmd/seed/admin`, `cmd/rehash-emails`): con la base solo privada y las variables selladas, que `railway run` no entrega, cómo se ejecutan queda **abierto** para el plan de devops (p. ej. como servicio o tarea de un solo uso dentro del proyecto).
