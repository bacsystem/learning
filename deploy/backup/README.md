# Copia diaria a Cloudflare R2 (runbook)

ADR 0004, Decisión 11 · historia PLT-05 · contrato `copias_bd` en
`docs/api/plataforma-salud.md`. Es la copia **fuera de Railway**: RPO de 24 h
si solo queda R2, y retención de **30 días**.

Nada de esto se crea solo. El bucket, el token y las reglas de R2, el servicio
cron de Railway y el rol `backup_ro` los crea **el usuario** (o devops con su
aprobación explícita), y cada paso se anota en
[`docs/ops/condiciones-lanzamiento.md`](../../docs/ops/condiciones-lanzamiento.md)
(C7, C12, C13).

## Cómo funciona

```
pg_dump -Fc (rol backup_ro, sslmode=require)  →  age -r <clave PÚBLICA>  →  /tmp/AAAA-MM-DD.dump.age
→ PUT firmado SigV4 a R2: pg/AAAA-MM-DD.dump.age  →  HEAD (mismo tamaño)
→ INSERT en copias_bd (correcta o fallida, con trap)  →  borra /tmp
```

- **Cifrado asimétrico del lado del cliente** (age v1.3.2, compilada desde el
  código y verificada contra sum.golang.org). El cron solo tiene la **clave
  pública**. La privada la custodia el usuario fuera de Railway y de R2,
  separada de `ENCRYPTION_KEY`. Al disco solo llega el archivo cifrado.
- **Subida con `curl --aws-sigv4`**: sin herramientas extra. Las credenciales
  van por stdin (`-K -`), no en argv.
- **Registro**: un `INSERT` por ejecución en `copias_bd`, correcta o fallida.
  Si la base no responde, no queda fila y el panel la marca atrasada por
  antigüedad.
- **Logs**: solo fecha, tamaño y resultado. Si falla: `http=…` y `curl=…` en la
  subida, o `motivo=` en Postgres, de una lista cerrada (`tiempo_agotado`,
  `autenticacion`, `permisos`, `tls`, `conexion`, `no_existe`, `otro`). El
  texto de libpq o de curl nunca se registra: podría llevar host, IP, usuario
  o el ACCOUNT_ID de R2.
- **Timeouts**: `BACKUP_TIMEOUT` (1800 s por defecto) para el dump y para la
  subida, más 30 s de conexión a Postgres y a R2. Railway **no** corta un cron
  colgado ([cron-jobs](https://docs.railway.com/reference/cron-jobs)).
- **Códigos de salida**: 0 si sale bien; 1 si falla; 2 si falta configuración;
  3 si la copia salió bien pero no se pudo registrar.

## Variables del servicio cron

| Variable | Valor | Sellada |
|---|---|---|
| `BACKUP_DATABASE_URL` | `postgres://backup_ro:<pw>@postgres.railway.internal:5432/<db>?sslmode=require` | **sí** |
| `AGE_RECIPIENT` | clave pública `age1…` (no es secreta) | no |
| `R2_ENDPOINT` | `https://<ACCOUNT_ID>.r2.cloudflarestorage.com` (o el endpoint jurisdiccional, C12) | **sí** |
| `R2_BUCKET` | el bucket dedicado | no |
| `R2_ACCESS_KEY_ID`, `R2_SECRET_ACCESS_KEY` | token **Object Read & Write limitado a ese bucket** (R2 no ofrece permiso solo de escritura) | **sí** |
| `BACKUP_TIMEOUT` | `1800` | no |

## Aprovisionamiento (una vez, con aprobación del usuario)

1. **R2** (el usuario, en Cloudflare):
   - bucket **privado** y dedicado, con la jurisdicción que decida el usuario
     (C12);
   - regla de ciclo de vida que borra a los **30 días**;
   - **bucket lock de 30 días**, que prevalece sobre el ciclo de vida: un token
     comprometido no puede borrar copias;
   - token de API Object Read & Write limitado a ese bucket.
2. **Clave age**: `age-keygen -o capacitate-backup.key` en la máquina del
   usuario.
   - La línea `# public key: age1…` va a `AGE_RECIPIENT`.
   - El archivo de la clave privada se guarda **fuera** de Railway y de R2 (en
     el gestor de secretos del usuario) y nunca se sube.
3. **Rol `backup_ro`** en la base de producción, como dueño de la base, **después
   de que la migración 064 (`copias_bd`) esté en producción**:

   ```sql
   CREATE ROLE backup_ro LOGIN PASSWORD '<openssl rand -hex 24>';
   GRANT pg_read_all_data TO backup_ro;
   GRANT INSERT ON copias_bd TO backup_ro;
   GRANT USAGE ON SEQUENCE copias_bd_id_seq TO backup_ro;
   ```

   No va en goose: el rol no existe en CI ni en compose.
4. **Imagen**, desde el checkout limpio del tag:
   `make docker/build/backup DOCKER_TAG=vX.Y.Z` y, el usuario,
   `make docker/push/backup DOCKER_TAG=vX.Y.Z`.
5. **Servicio cron en Railway**, en el mismo proyecto:
   - fuente: la imagen `cbaciliod/learning-backup:vX.Y.Z`;
   - Cron Schedule: `0 7 * * *`, en UTC (02:00 en Perú). El intervalo mínimo
     es de 5 min;
   - las variables de arriba, selladas donde corresponde;
   - restart policy `NEVER`, para que un fallo no reintente en bucle.
6. Primera ejecución manual (`railway cron run`) y comprobar la fila
   `correcta` en `copias_bd` y el objeto en R2 (C7).

**Alerta (C13):** la documentación de Railway no dice si una ejecución fallida
de un cron avisa. Se comprueba provocando un fallo a propósito, por ejemplo
con un `R2_BUCKET` inexistente durante una ejecución manual. Si Railway no
avisa, el panel de salud (PLT-05) lo muestra como degradado o caído, y se le
lleva al usuario la decisión de otro canal.

**Un solo objeto por día:** `pg/AAAA-MM-DD.dump.age`. Con el bucket lock, otra
ejecución **correcta** el mismo día no puede sobrescribirlo y queda registrada
como fallida. Para repetir una copia el mismo día, primero hay que comprobar
que la anterior falló.

## Restauración desde R2 (prueba mensual y antes del lanzamiento: C6, C7)

En un Postgres 16 **efímero y aislado**, nunca en `deploy/` ni en producción.
La clave privada se carga solo durante la prueba.

```bash
# 1. Bajar el objeto. Las credenciales se leen sin eco a un archivo 0600:
#    no quedan en el historial ni en `ps`.
umask 077; read -r -p 'R2 key: ' K; read -rs -p 'R2 secret: ' S; echo
printf 'user = "%s:%s"\n' "$K" "$S" > r2.curl; unset K S
curl -K r2.curl --fail --aws-sigv4 'aws:amz:auto:s3' -o copia.dump.age \
  "https://<ACCOUNT_ID>.r2.cloudflarestorage.com/<bucket>/pg/AAAA-MM-DD.dump.age"
rm -f r2.curl
# 2. Descifrar con la clave privada:
age -d -i capacitate-backup.key -o copia.dump copia.dump.age
# 3. Restaurar en una base nueva (sin dueños ni GRANT: backup_ro no existe ahí):
pg_restore -d <base nueva> --no-owner --no-privileges --exit-on-error copia.dump
# 4. Comprobar: SELECT max(version_id) FROM goose_db_version; conteos; un backend
#    de prueba con la ENCRYPTION_KEY de producción que haga login (descifra).
# 5. Cronometrar todo (criterio: ≤ 2 h, C6), anotarlo y borrar la base y los archivos.
```

## Probarlo sin R2

```bash
make backup/test
```

Levanta un Postgres 16 desechable con las migraciones reales, `copias_bd` y
`backup_ro`, y un servidor S3 local que valida SigV4 (`versity/versitygw`,
solo para la prueba). Después corre:

- una copia correcta;
- tres fallos: bucket inexistente, credenciales malas y endpoint colgado;
- la comprobación de las filas de `copias_bd`;
- una restauración cronometrada con la clave privada.

Al terminar borra todo.
