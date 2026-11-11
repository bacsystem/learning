# ADR 0004: despliegue en Railway y alojamiento de la base de datos de producción

- **Estado:** **aceptado con condiciones** (2026-11-11). Todas las decisiones del usuario (U1–U4) están tomadas. Lo que solo se puede confirmar en el panel de Railway o en la restauración de prueba va como **condición previa al lanzamiento** (sección «Condiciones de aceptación»). Si una condición falla, se reabre este ADR.
  - Plataforma decidida por el usuario (vía tech-lead): por ahora el backend, el frontend y Postgres 16 se despliegan en **Railway**. Es el punto de partida, y según la demanda se puede migrar a otra nube. El contrato es portable: imágenes Docker más variables de entorno (Decisión 9).
  - Antes de Railway se valida en una **preproducción local** (`deploy/`), donde prueba qa (Decisión 10).
  - Despliegue por imagen publicada en Docker Hub (Decisión 9). Copia diaria cifrada fuera de Railway en **Cloudflare R2**, aprobada por el usuario (Decisión 11).
  - Plan **Railway Pro** (decisión del usuario, U1 resuelto).
  - Objetivos fijados por el usuario: **RPO 5 min** con PITR, y 24 h si solo queda la copia de R2; **RTO 2 h**; **retención de 30 días** en R2 (U2–U4).
- **Fecha:** 2026-11-11
- **Pedido por:** devops, para el primer despliegue. Su plan operativo está en `docs/ops/plan-base-produccion.md` (PR #97).
- **Relacionado:** [ADR 0001](0001-unicidad-proceso-asignaturas-por-especialidad.md) § Versión de Postgres en producción (Postgres 16, mínimo 15).
- **Fuentes:** documentación oficial de Railway y de Cloudflare R2, consultada el 2026-11-11. Lo que la documentación **no** dice se marca como «no documentado» y se verifica en el panel al aprovisionar, sin suponerlo.

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
| Backups de volumen | La página indica que la función está «still under development» (contrastado por el tech-lead en la revisión del PR #98). Diario (retención 6 días), semanal (27) y mensual (89), más backups manuales (≤ 50 % del tamaño del volumen). La restauración monta un volumen nuevo y redespliega. «Backups can only be restored into the same project + environment». «Wiping a volume deletes all backups». Se cobran como volumen | docs.railway.com/volumes/backups |
| PITR | Archiva el WAL en cada commit (`archive_timeout=60`) a un bucket de Railway, con base backups de pgBackRest. «The last 4 full backups are retained, giving you a restore window of roughly 4 weeks». La restauración crea un **servicio Postgres nuevo** y el cambio a ese servicio es manual. La ventana empieza en el primer base backup después de activarlo. No admite fijar la versión menor. Se cobra el almacenamiento del bucket y el egress | docs.railway.com/volumes/point-in-time-recovery |
| Alta disponibilidad | Conversión opcional a un clúster con failover automático (Patroni, etcd y HAProxy) | guía de PostgreSQL |
| Planes | Hobby 5 USD/mes (volumen ≤ 5 GB, 6 réplicas) y Pro 20 USD/mes (≤ 1 TB, 42 réplicas). **No documentado:** qué plan incluye backups, PITR y alta disponibilidad | docs.railway.com/reference/pricing/plans; railway.com/pricing |
| Región de backups y PITR | **No documentado**: no dice si se guardan fuera de la región del servicio | backups y PITR |
| Cron jobs | Un servicio con horario cron ejecuta su start command y debe terminar. El horario es en UTC, con un intervalo mínimo de 5 minutos y la hora exacta «can vary by a few minutes». Si la ejecución anterior sigue en curso, la nueva se salta. Railway no termina automáticamente una ejecución que no sale | docs.railway.com/reference/cron-jobs |

### Lo que ofrece Cloudflare R2 (copia externa aprobada por el usuario)

Fuente: documentación oficial de Cloudflare R2 (repositorio `cloudflare/cloudflare-docs`, `src/content/docs/r2/…`), consultada el 2026-11-11.

| Tema | Lo que dice Cloudflare | Página |
|---|---|---|
| API S3 | Soporta `PutObject` y multipart upload. Endpoint `https://<ACCOUNT_ID>.r2.cloudflarestorage.com`, con región `auto` | `r2/api/s3/api`, `r2/api/tokens` |
| Tokens | Hay cuatro niveles: Admin Read & Write, Admin Read only, Object Read & Write y Object Read only. Los dos de objetos «you can scope your token to a set of buckets». **No existe un permiso solo de escritura**: Object Read & Write también lee, lista y borra objetos. Las credenciales S3 son el `id` del token (Access Key ID) y el SHA-256 de su `value` (Secret) | `r2/api/tokens` |
| Ciclo de vida | Borra objetos tras N días, por prefijo, y aborta multipart incompletos. «Objects will typically be removed … within 24 hours» del vencimiento. Se configura en el dashboard, con wrangler o con la API S3 | `r2/buckets/object-lifecycles` |
| Bucket locks | «prevent the deletion and overwriting of objects … for a specified period», por prefijo, y no impiden crear objetos nuevos. «Bucket lock rules take precedence over lifecycle rules» | `r2/buckets/bucket-locks` |
| Cifrado en reposo | La página de la API S3 **no lo documenta**; las cabeceras SSE de KMS no están implementadas | `r2/api/s3/api` |

### Cumplimiento de R1–R7 con Railway

| # | ¿Cumple? | Cómo | Si no se confirma |
|---|---|---|---|
| R1 | Sí, a verificar | `:latest` de la imagen es 16; se comprueba con `SELECT version()` | Alinear CI y compose con la versión real (≥ 15), o elegir A |
| R2 | Sí | `pg_trgm` es *contrib* y viene en la imagen | A o B |
| R3 | Sí | Sin pooler por defecto; la URL privada es directa | Si se activa el pooler, migrar por la URL directa |
| R4 | A verificar | `SHOW max_connections` frente a réplicas × 25 | Limitar las réplicas |
| R5 | Parcial | `sslmode=require` sobre la red privada WireGuard; `verify-full` no es posible | Se acepta por la red privada; si el usuario exige `verify-full`, A |
| R6 | Sí | Sealed variables más una copia de `ENCRYPTION_KEY` fuera de Railway | — |
| R7 | Parcial → cubierto con R2 | Backups de volumen (la función figura «still under development») y PITR, ambos dentro de Railway y del mismo proyecto; el plan, la región y el cifrado en reposo no están documentados. Se completa con la **copia diaria cifrada en Cloudflare R2** (Decisión 11) | A |

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
4. **Conexiones:** **2 × 25 × réplicas** del backend + margen (migración, cron de backup, administración) ≤ `max_connections`. El factor 2 cubre el solapamiento del despliegue, cuando conviven el backend viejo y el nuevo (dato de devops). Devops anota el valor real de `SHOW max_connections;` y limita las réplicas en consecuencia.
5. **Backups:**
   - Se activan **PITR** y los backups de volumen **diario y semanal**.
   - Antes de abrir el servicio a usuarios se hacen **restauraciones de prueba**, y después periódicamente:
     - de PITR, a un servicio nuevo, sin tocar el original;
     - de un backup de volumen, **en el mismo entorno `production`**, porque Railway solo restaura en el mismo proyecto y entorno. Se hace antes del lanzamiento, cuando la base todavía no tiene datos reales.
   - En la prueba de PITR, devops mide el RPO real: la diferencia entre el último commit y el punto restaurable.
   - Objetivos (U2–U4):
     - **RPO** de 5 min con PITR, y de 24 h en el escenario de pérdida de Railway, con solo la copia de R2.
     - **RTO** de 2 h.
     - **Retención** de 30 días en R2.
   - **No** se activa el backup de volumen **mensual**, que conserva 89 días. Con diario (6 días), semanal (27), PITR (unas 4 semanas) y R2 (30), ninguna copia supera los ~30 días. Así, un dato personal borrado desaparece de todas las copias en ese plazo.
6. **Secretos:**
   - `JWT_SECRET`, `ENCRYPTION_KEY`, la clave HMAC (ADR 0002) y las credenciales SMTP van como **sealed variables** en el servicio backend.
   - `ENCRYPTION_KEY` tiene además una copia custodiada por el usuario **fuera de Railway** (R6). Si se pierde, los datos personales cifrados son irrecuperables. Si la clave solo existe dentro de Railway, perder el proyecto se lleva la clave y los datos a la vez.
7. **Logs:** sin datos personales ni parámetros de consultas. No se activa `log_statement = all` ni la configuración de pgaudit que registra parámetros (README de contratos §9).
8. **Frontend:** servicio propio con `NEXT_PUBLIC_API_URL` apuntando al dominio público del backend. El backend expone solo su dominio público; la base no expone ninguno.
9. **Portabilidad (contrato con la plataforma).** Railway es el punto de partida: según la demanda se puede migrar a otra nube. Por eso el contrato entre la app y la plataforma es **una imagen Docker por servicio más su configuración por variables de entorno**, y nada más:
   - **Despliegue por imagen publicada** (decisión del usuario): Railway despliega imágenes de un registro, hoy Docker Hub (`cbaciliod/learning-*`). No compila desde el repo.
     - Se despliega por tag inmutable `vX.Y.Z` o por digest, **nunca** `:latest`.
     - El backend se **promueve**: la misma imagen, con el mismo digest, que validó qa en `deploy/` es la que va a Railway.
     - **Quién publica:** el usuario construye y publica las imágenes **desde su máquina** con la cuenta `cbaciliod`, en repositorios **públicos** de Docker Hub. Las credenciales de Docker Hub quedan en esa máquina y nunca van al repo. Hoy no hay publicación desde el CI.
       - **Orden del release** (diseño de devops, aprobado por el tech-lead; revisado el 2026-11-11):
         1. **Validación completa de qa ANTES del tag**: en `deploy/local`, con las imágenes construidas en local desde la rama `release/*` (Decisión 10).
         2. Tag `vX.Y.Z` (GitFlow: `release/*` → `main`).
         3. CI del tag en verde.
         4. El usuario construye, desde un **checkout limpio del tag**, tres imágenes:
            - backend `vX.Y.Z`;
            - frontend `vX.Y.Z`, con la URL de producción;
            - frontend `vX.Y.Z-preprod`, con `NEXT_PUBLIC_API_URL=http://localhost:8180`.

            Pasan por `make docker/verify` y el usuario las publica. El Makefile falla si hay cambios sin commit o si HEAD no es el tag.
         5. **Guardia de versión:** el `version` de `frontend/package.json` debe coincidir con el tag sin la «v», igual que `main.version` del backend. Si no coincide, el build falla.
         6. **Smoke sobre las imágenes publicadas**, en `deploy/local` con `make local/up TAG=vX.Y.Z`: arranque, migraciones, login y `/health`. Se anota el **digest** del backend probado.
         7. Cambio en Railway al tag `vX.Y.Z`, o a ese digest. El backend de producción es **la misma imagen**, con el mismo digest, que pasó el smoke.
       - **Si el smoke falla, no se despliega.** La corrección sale como `vX.Y.(Z+1)` por el mismo flujo: los tags publicados **nunca se reescriben**, ni en git ni en Docker Hub.
       - Ningún agente hace `docker login` ni `docker push`.
       - Rollback: volver al tag anterior en Railway.
     - **Consecuencia de que sean públicas:** cualquiera puede descargarlas. Por eso **ninguna imagen lleva secretos, datos ni `.env`**:
       - toda la configuración sensible entra por variables en runtime;
       - en el frontend solo van los `NEXT_PUBLIC_*`, que de todos modos son públicos en el navegador;
       - `assets/` y los `.env` quedan fuera del contexto de build (`.dockerignore`), y devops lo verifica antes de la primera publicación.
     - **Frontend (opción (a), decidida):** una imagen por entorno, porque `NEXT_PUBLIC_API_URL` se fija en el build (ver abajo).
   - **Imágenes:** se construyen con los mismos `Dockerfile` de backend y frontend que usa compose.
   - **Migraciones:** el backend migra en su arranque (`entrypoint.sh:9`, bajo advisory lock). **No** se usa el Pre-deploy Command de Railway ni ningún otro gancho de la plataforma, por dos motivos:
     - así `deploy/`, Railway y cualquier otra nube se comportan igual;
     - un Pre-deploy duplicaría la migración.

     Usar un gancho exige un ADR.
   - **Configuración:** solo variables de entorno estándar: `DATABASE_URL`, `JWT_SECRET`, `ENCRYPTION_KEY`, `CORS_ORIGINS`, `TRUSTED_PROXIES`, SMTP y las demás de `config.go` y `docker-compose.yml:55-69`. Las referencias `${{…}}` de Railway se resuelven **en el panel**, nunca en el código.
   - **Sin dependencias exclusivas de Railway en el código:** el código no lee variables `RAILWAY_*`, no usa SDKs ni APIs de Railway y no depende de sus dominios `*.railway.internal`, que solo aparecen como valor de `DATABASE_URL`. Toda excepción necesita un ADR.
   - **Base:** Postgres estándar. Migrar de nube significa `pg_dump` / `pg_restore`, o una réplica lógica, hacia otro Postgres 16 con `pg_trgm`, y cambiar `DATABASE_URL`. La recuperación a un punto en el tiempo y los backups de Railway no se migran: se reconfiguran en el destino.
   - Hoy se cumple: no hay ninguna referencia a `RAILWAY` en `backend/` ni en `frontend/src` (búsqueda del 2026-11-11).
   - **Frontend:** `NEXT_PUBLIC_*` se fija en el **build** de Next.js (`ARG NEXT_PUBLIC_API_URL` en `frontend/Dockerfile:19-22`), así que la imagen del frontend se construye por entorno, con su `NEXT_PUBLIC_API_URL`, y se publica con un tag por entorno (p. ej. `vX.Y.Z-preprod` y `vX.Y.Z`). Es un límite del framework, no de la plataforma, y aplica igual en cualquier nube. Que qa valide la misma imagen del frontend que va a producción exigiría leer la URL en runtime: es un cambio de código del frontend que queda fuera de este ADR.
10. **Entornos.**
    - **Preproducción local:** un directorio `deploy/` del repo levanta el stack con las mismas imágenes y la misma forma de configuración que producción. qa valida ahí antes de cada despliegue a Railway, en dos momentos (Decisión 9):
      - **antes del tag:** validación completa contra los criterios de aceptación, con las imágenes construidas en local desde la rama `release/*`;
      - **después de publicar:** un smoke sobre las imágenes publicadas (`make local/up TAG=vX.Y.Z`), anotando el digest del backend.

      Estructura propuesta por devops y revisada por el architect:
      - `deploy/local/`: compose propio (proyecto `capacitate-preprod`, puertos solo en `127.0.0.1`, red propia), un TLS autofirmado generado por `make` (en `.gitignore`), una semilla sintética y la guía para qa;
      - `deploy/railway/`: guía y lista de variables sin secretos, sin archivos de configuración de Railway;
      - una carpeta por destino, `deploy/<nube>/`, todas con las mismas imágenes.
      - La base es `postgres:16-alpine` oficial con `ssl=on`, no la imagen de Railway. `ENV=preprod`.
      - Implementación: [PR #115](https://github.com/bacsystem/learning/pull/115) (`deploy/`, de devops).
    - Para que la validación de qa valga para producción, la preproducción usa **Postgres 16**, el mismo `entrypoint.sh` con migraciones y `ENV` distinto de development/test. Con eso aplica la validación de `sslmode` de producción (hallazgo H-5 del PR #97), así que la base local de preproducción se levanta con TLS.
    - Los datos de preproducción son sintéticos o semilla, **nunca** copias de producción ni de `assets/`, porque contienen datos personales.
    - **Producción:** Railway, entorno `production`.
11. **Copia diaria fuera de Railway, en Cloudflare R2** (aprobado por el usuario). Cubre el riesgo de que datos y backups estén en el mismo proveedor (R7).
    - **Qué se copia:** un `pg_dump` diario de la base de producción en formato custom (`-Fc`), con un `pg_dump` de la **misma versión mayor** que el servidor (16). La imagen del cron parte de una `postgres:16.x-alpine` fijada (diseño de devops).
    - **Rol de solo lectura:** el cron conecta con un rol propio `backup_ro` (`pg_read_all_data`), no con el usuario dueño de la base. Su URL (`BACKUP_DATABASE_URL`, privada y con `sslmode=require`) va como sealed variable del cron.
      - Devops crea el rol una sola vez al aprovisionar y lo documenta en `docs/ops`.
      - No va en las migraciones de goose: los roles son de la instancia, no del esquema.
      - **Excepción (enmienda del 2026-11-11, [ADR 0005](0005-estado-operativo-panel-salud.md)):** `backup_ro` tiene además **un único** permiso de escritura: `INSERT` sobre `copias_bd` y `USAGE` sobre `copias_bd_id_seq`, sin UPDATE ni DELETE. Es un paso de aprovisionamiento que hace devops, después de la migración que crea la tabla.
    - **Registro de cada ejecución (PLT-05):**
      - Al terminar, correcta o fallida (con `trap` y el timeout propio), el cron inserta **una** fila en `copias_bd` con `iniciada_en`, `terminada_en`, `resultado` y `tamano_bytes`. Esquema en `docs/api/plataforma-salud.md`.
      - Sin bucket, ruta ni error. El panel de salud lo lee de ahí.
      - Si el INSERT falla después de una subida correcta, el cron sale con error y alerta.
      - **Restauraciones de prueba (C6, C7):** el dump incluye el GRANT a `backup_ro`. En un Postgres sin ese rol se restaura con `pg_restore --no-owner --no-privileges`.
    - **Cifrado del lado del cliente, antes de subir.** El dump contiene datos personales, así que nunca sale de Railway sin cifrar y no se depende del cifrado en reposo de R2, que no está documentado.
      - Se usa **cifrado asimétrico**: devops propone `age`, una herramienta nueva dentro de la imagen del cron que aprueba el tech-lead. El cron solo tiene la **clave pública** (`AGE_RECIPIENT`): aunque se comprometa, no puede descifrar los dumps.
      - El dump se cifra en tubería (`pg_dump | age`). Al disco temporal solo llega el archivo cifrado, que se sube por la API S3 (`curl --aws-sigv4`, PUT simple más un HEAD de verificación) y después se borra.
      - La **clave privada** la custodia el usuario fuera de Railway y fuera de R2. Está **separada de `ENCRYPTION_KEY`**: son claves distintas, guardadas en lugares distintos. Una restauración completa necesita las dos.
    - **Bucket:** privado, sin acceso público ni dominio público, dedicado solo a estos dumps. Nombre de objeto con fecha **y hora** UTC: `pg/AAAA-MM-DD/HHMMSSZ.dump.age` (PR #122).
      - Con la hora, repetir el cron el mismo día nunca choca con el bucket lock, que impide sobrescribir. Tampoco deja una ejecución `fallida` falsa en `copias_bd` ni pone el panel en degradado.
      - Cada objeto queda protegido sus 30 días.
      - Para restaurar se toma la clave más reciente bajo `pg/`.
    - **Credenciales:**
      - Un token de R2 **Object Read & Write limitado a ese bucket**. R2 no tiene un permiso solo de escritura, así que es el mínimo posible.
      - El Access Key ID, el Secret y el endpoint van como **sealed variables** del servicio de cron. El backend **no** las recibe.
      - Como el token también puede borrar, se activa una **bucket lock rule** con la misma retención. El lock prevalece sobre el ciclo de vida, así que un token comprometido no puede borrar copias antes de tiempo.
    - **Retención:** una regla de **ciclo de vida** del bucket borra los objetos tras N días; R2 los elimina normalmente dentro de las 24 h siguientes al vencimiento. **N = 30 días** (decisión del usuario, U4). El bucket lock usa los mismos 30 días.
    - **Dónde corre:** lo propone devops. Lo previsto es un **cron job de Railway** en el mismo proyecto y entorno `production`: una imagen propia, por ejemplo `cbaciliod/learning-backup`, que conecta por la red privada con `sslmode=require` y sube a R2 por la API S3.
      - El horario va en UTC y el intervalo mínimo es de 5 minutos.
      - El proceso debe terminar siempre: Railway no lo corta y salta la siguiente ejecución si la anterior sigue en curso. Hace falta un timeout propio.
      - El script no deja el dump sin cifrar en disco ni en los logs, y registra solo la fecha, el tamaño y el resultado.
    - **Verificación:**
      - Si una ejecución falla o no corre, se alerta. Primera opción: la notificación de Railway cuando falla un cron, sin servicio nuevo. Un *dead-man's switch* externo sería un servicio nuevo y necesita la aprobación del usuario.
      - **Restauración de prueba periódica desde R2**, al menos mensual y antes del lanzamiento: descargar, descifrar con la clave privada, `pg_restore` en un Postgres 16 **efímero y aislado** y comprobar conteos de tablas.
      - Esa base efímera **no** es la preproducción de `deploy/`, porque ahí nunca van datos de producción (Decisión 10). Se destruye al terminar, y la clave privada se carga solo durante la prueba.
    - Es un servicio externo nuevo **aprobado por el usuario**. El costo es el almacenamiento de R2.

## Decisiones del usuario

| # | Pregunta | Decisión (2026-11-11) | Cómo se cumple |
|---|---|---|---|
| U1 | Proveedor y plan | **Railway, plan Pro** | Volumen de hasta 1 TB y 42 réplicas. Backups y PITR se cobran como almacenamiento y egress |
| U2 | **RPO**: cuántos datos se pueden perder como máximo | **5 min** con PITR. **24 h** si se pierde Railway y solo queda la copia de R2 | Railway no publica un RPO para Postgres. Por lo que documenta (WAL en cada commit, `archive_timeout=60`) se infiere del orden de 1 minuto, dentro de los 5. El cron diario de R2 da 24 h |
| U3 | **RTO**: en cuánto tiempo hay que volver a dar servicio | **2 h** | PITR a un servicio nuevo y cambio manual de `DATABASE_URL`, o restauración de un backup de volumen y redeploy. En el escenario de pérdida de Railway: Postgres nuevo, descarga y descifrado del dump de R2, `pg_restore` y cambio de `DATABASE_URL`. Se mide en las pruebas |
| U4 | Retención | **30 días** en R2 (ciclo de vida y bucket lock) | Sin backup mensual de volumen, así que ninguna copia supera unos 30 días (Decisión 5). Un borrado de datos personales desaparece de todas las copias en ese plazo. El plazo legal (Ley 29733 de Perú, si aplica) es responsabilidad del usuario |

## Condiciones de aceptación (antes del lanzamiento)

Lo verifica devops en el panel, con soporte o con pruebas, y registra el resultado en su plan de `docs/ops/`. **Si alguna condición falla, se reabre este ADR** (A y B quedan como alternativas).

| # | Condición | Cómo se verifica |
|---|---|---|
| C1 | El plan Pro incluye backups de volumen, PITR y cron jobs | Panel o soporte de Railway |
| C2 | La plantilla despliega Postgres 16 con `pg_trgm` | `SELECT version();` y `pg_available_extensions` |
| C3 | `sslmode=require` conecta por la URL privada; se define cómo agregar el parámetro a `${{Postgres.DATABASE_URL}}` | Arranque del backend, que lo exige por H-5 |
| C4 | `max_connections` ≥ 2 × 25 × réplicas + margen | `SHOW max_connections;` |
| C5 | **RPO ≤ 5 min con PITR** | Restauración de prueba de PITR: diferencia entre el último commit y el punto restaurable |
| C6 | **RTO ≤ 2 h**, por PITR y desde R2 | Restauraciones de prueba cronometradas, con el procedimiento escrito en `docs/ops/` |
| C7 | La copia de R2 corre a diario, cifrada, con ciclo de vida y bucket lock de 30 días, y se puede restaurar | Primera ejecución del cron más una restauración de prueba desde R2 |
| C8 | Región y cifrado en reposo de los backups de Railway y del bucket de PITR | Panel o soporte. Si no está cifrado, R2 cifrado del lado del cliente sigue cubriendo la copia externa |
| C9 | Qué pasa con `DATABASE_URL` si se activa el pooling de Railway | Panel o soporte. Mientras no se active, no aplica (Decisión 3) |
| C10 | Cómo se ejecutan `cmd/seed/admin` y `cmd/rehash-emails` con la base privada y las variables selladas | Procedimiento en el plan de devops |
| C11 | Las imágenes públicas no llevan secretos, datos ni `.env` | `make docker/verify` (`deploy/verify-image.sh`): revisa `/app`, `docker history` y ENV, y `docker/push` no publica si falla |
| C12 | **Ubicación o jurisdicción del bucket de R2** para los datos personales. R2 ofrece buckets jurisdiccionales (endpoints `.eu.`, `.fedramp.`) | **La decide el usuario** antes de crear el bucket |
| C13 | El cron alerta si falla o no corre | Ejecución fallida provocada a propósito |

## Consecuencias

- **Positivas:**
  - Una sola plataforma y red privada: la base no queda expuesta a internet.
  - El primer despliegue no necesita cambios de código. `cmd/migrate` funciona con la conexión directa privada.
  - Hay PITR sin operar WAL a mano, con un RPO objetivo de 5 min (C5).
  - La migración a otra nube queda acotada: cambiar dónde corren las imágenes, recrear las variables y mover la base con `pg_dump` / `pg_restore`.
- **Negativas y riesgos:**
  - La imagen del frontend no es la misma entre entornos, porque `NEXT_PUBLIC_API_URL` se fija en el build. Lo que qa valida en preproducción es el mismo **código** y el mismo Dockerfile, no el mismo binario del frontend.
  - Los backups de volumen y el PITR quedan en el mismo proveedor, y la función de backups de volumen figura como «still under development». Una pérdida de la cuenta o del proyecto se llevaría datos y backups juntos. Se mitiga con la copia diaria cifrada en R2 (Decisión 11), con RPO de hasta 24 h para ese escenario.
  - Hay más claves que custodiar: `ENCRYPTION_KEY` y la clave privada de los dumps, separadas. Si se pierde cualquiera de las dos, la copia de R2 no sirve.
  - El token de R2 no puede ser solo de escritura (límite de R2). Se compensa con el bucket lock.
  - Solo hay TLS `require`, sin verificar el certificado. Se acepta porque el tráfico va por la red privada cifrada.
  - La versión de la plantilla no se fija sin perder la gestión de Railway. Hay que vigilar que CI y producción sigan en la misma versión mayor.
  - Sin pooler, las conexiones crecen con las réplicas (25 por réplica).
  - En el MVP hay un solo rol con permisos de esquema, porque servidor y migración comparten `DATABASE_URL`.
  - Comandos puntuales que necesitan `DATABASE_URL` y `ENCRYPTION_KEY` (`cmd/seed/admin`, `cmd/rehash-emails`): con la base solo privada y las variables selladas, que `railway run` no entrega, cómo se ejecutan lo resuelve el plan de devops (condición C10; p. ej. como servicio o tarea de un solo uso dentro del proyecto).
  - El bucket lock de 30 días impide borrar antes de plazo una copia de R2, incluso a pedido. Un dato personal borrado puede seguir hasta 30 días en las copias; se acepta como la retención decidida (U4).
