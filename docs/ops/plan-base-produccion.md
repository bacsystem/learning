# Plan de la base de producción (propuesta)

Estado: **propuesta**. Dónde se aloja la base (proveedor y arquitectura) no lo
decide devops: se pidió un ADR al architect (`docs/adr/`). Este documento fija
los requisitos que salen del código, las opciones y los procedimientos de
backup y restauración que valen para cualquiera de ellas.

Decisión del usuario: **Postgres 16** en producción (mínimo 15).

## 1. Requisitos que salen del código

| Requisito | Por qué |
|---|---|
| Postgres ≥ 15, objetivo 16 | La migración 060 usa `UNIQUE NULLS NOT DISTINCT`. CI y compose usan `postgres:16-alpine` |
| Extensión `pg_trgm` | La migración 013 hace `CREATE EXTENSION IF NOT EXISTS pg_trgm`. El usuario que migra necesita permiso para crearla, o hay que crearla una vez por adelantado |
| Conexión **directa** (sin pooler en modo transaction) para migrar | `cmd/migrate` toma `pg_advisory_lock` de **sesión** para que solo una réplica migre. Un PgBouncer en modo transaction lo rompe. El servidor puede ir por un pooler en modo session |
| `CREATE INDEX CONCURRENTLY` | Las migraciones 054 y 058 van sin transacción. Si un proveedor limita la duración de las sentencias, hay que tenerlo en cuenta |
| TLS en `DATABASE_URL` | `lib/pq` v1.12.3 usa `sslmode=require` por defecto (cifra pero no verifica el certificado). En producción, `verify-full`: con el CA del proveedor (`sslrootcert=<ruta>`) o con `sslrootcert=system` |
| Usuarios separados | Un **propietario** del esquema, para migrar, y un usuario de la **aplicación**. Hoy `entrypoint.sh` migra con la misma `DATABASE_URL` que usa el servidor: separarlos exige un cambio en el arranque, que se decide en el ADR |
| `ENCRYPTION_KEY` custodiada **aparte** del backup | Los datos personales van cifrados con AES-GCM. Un backup sin su clave no sirve, y la clave junto al backup anula el cifrado |

## 2. Opciones para el ADR

| | A. Postgres gestionado con PITR | B. Postgres 16 propio en una VM | C. Postgres en el mismo host que la app |
|---|---|---|---|
| Ejemplos | AWS RDS, Google Cloud SQL, DigitalOcean Managed, Neon… | VM dedicada, con wal-g o pgBackRest hacia almacenamiento de objetos | Un `docker-compose` de producción con un servicio `db` |
| Backups y PITR | Los da el proveedor (diarios + WAL) | Se montan y operan a mano | Solo `pg_dump` (RPO = intervalo entre dumps) |
| Restauración | Desde la consola o la API, a una instancia nueva | Script propio | `pg_restore` |
| `pg_trgm` | Disponible en los principales proveedores (hay que confirmarlo en el elegido) | Sí | Sí |
| Operación (parches, discos, alertas) | Del proveedor | Nuestra | Nuestra, y compite con la app por CPU y disco |
| Coste | Más alto | Medio | Bajo |
| Riesgo principal | Dependencia del proveedor | Error operativo en el WAL o en los backups | Se pierde la base y la app juntas si cae el host |

Preguntas que el ADR necesita del usuario o del po:

- **RPO**: cuántos datos se pueden perder.
- **RTO**: cuánto tiempo puede estar caída la plataforma.
- **Retención legal**: cuánto tiempo se guardan backups con datos personales y
  cómo se cumple una solicitud de supresión en los backups. Esto depende de la
  normativa aplicable y lo decide el usuario.

No se proponen cifras hasta tener esas respuestas.

## 3. Aprovisionamiento (cualquier opción)

Cada paso queda registrado en el PR de infraestructura que lo haga. Nada se
configura a mano sin dejarlo escrito.

1. Instancia Postgres 16 en la región de la app, con TLS y sin acceso público,
   o con lista de IPs permitidas.
2. Base `capacitate` (nombre a confirmar) y los roles:
   - propietario: `CREATE` en el esquema y `pg_trgm` creada;
   - aplicación: `SELECT`, `INSERT`, `UPDATE` y `DELETE`, si el ADR separa los
     usuarios.
3. `DATABASE_URL` en el gestor de secretos del entorno, con el `sslmode`
   acordado. Nunca en el repositorio ni en mensajes.
4. Primer arranque del backend: `entrypoint.sh` aplica 001…N en orden. Se
   comprueba con `goose status`, o con
   `SELECT max(version_id) FROM goose_db_version`.
5. Admin inicial con `cmd/seed/admin`. La contraseña se pide por stdin.
6. Backups automáticos activos y **una restauración de prueba** (sección 5)
   **antes** de abrir la plataforma a usuarios.

## 4. Backups

- **Automáticos**: los del proveedor (A) o wal-g/pgBackRest (B), más un
  `pg_dump -Fc` lógico diario, independiente del proveedor. El dump sirve para
  migrar de proveedor y para restaurar una tabla suelta.
- **Antes de cada despliegue**: un `pg_dump -Fc` o un snapshot manual, porque
  el arranque del backend migra (ver
  [checklist-release.md](checklist-release.md) §5).
- **Cifrado en reposo** del almacenamiento de backups, con acceso restringido.
- **Retención**: pendiente de la respuesta sobre retención legal (§2).
- **`ENCRYPTION_KEY`, `JWT_SECRET`** y, cuando exista, `ANTIFRAUD_HMAC_KEY` se
  respaldan en el gestor de secretos, **separados** de los backups de datos.

## 5. Restauración probada

Procedimiento ensayado en local (Postgres 16, base migrada hasta la 061):

```bash
pg_dump -U <usuario> -d <base> -Fc -f backup.dump
createdb -U <usuario> restore_test
pg_restore -U <usuario> -d restore_test --no-owner --exit-on-error backup.dump
psql -U <usuario> -d restore_test -Atc "select max(version_id) from goose_db_version"   # → 61
psql -U <usuario> -d restore_test -Atc "select count(*) from pg_extension where extname='pg_trgm'"  # → 1
```

Resultado del ensayo (11-11-2026): restauración sin errores, versión 61,
`pg_trgm` presente.

Prueba de restauración en producción, una vez antes del lanzamiento y luego de
forma periódica (frecuencia según el RTO):

1. Restaurar el último backup en una base **nueva y aislada**, nunca sobre la
   de producción.
2. Comprobar la versión de goose y el número de filas de las tablas
   principales.
3. Arrancar un backend de prueba contra la copia, **con la `ENCRYPTION_KEY` de
   producción tomada del gestor de secretos**. Comprobar `GET /health` y que un
   login descifra los datos. Esto prueba que el backup y la clave sirven
   juntos.
4. Medir el tiempo total (esa es la cifra real de RTO) y dejarlo anotado en un
   runbook de `docs/ops/`.
5. Borrar la copia, porque contiene datos personales.

## 6. Observabilidad mínima de la base

Alertas a configurar en la opción elegida:

- Backup fallido o más antiguo de lo esperado.
- Disco por encima del 80 %.
- Conexiones cerca de `max_connections`.
- `GET /health` del backend respondiendo 503: el health hace `PingContext` a la
  base.
