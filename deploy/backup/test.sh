#!/bin/sh
# Prueba de punta a punta del cron de copia, sin R2 ni credenciales reales:
#   Postgres 16 desechable con TLS (sslmode=require, como en producción),
#   migraciones reales + copias_bd (064) + backup_ro, y versitygw (servidor S3
#   que valida SigV4, solo para esta prueba). Casos:
#     1. pg_dump sin permisos  → fallida y NINGÚN objeto subido
#     2. copia correcta        → objeto cifrado con age + fila correcta
#     3. bucket inexistente, 4. credenciales malas, 5. endpoint colgado → fallida
#     6. restauración cronometrada con la clave privada en una base nueva
#     7. PUT con http 500 (HEAD «correcto»): solo el chequeo http=200 lo detecta
#     8. la contraseña de Postgres no aparece en argv de pg_dump ni de psql
#     9. pg_dump colgado → timeout lo corta (motivo=tiempo_agotado)
#    10. copias_bd no existe (064 sin aplicar) → copia subida, exit 3, motivo=no_existe
# Uso: make backup/test  (BACKUP_TEST_IMAGE=<imagen> prueba una ya construida,
#      p. ej. la de release linux/amd64). Todo se borra al terminar.
set -eu
cd "$(dirname "$0")/../.."

# Nombres y puerto POR CORRIDA (sufijo = PID): dos corridas a la vez (otra sesión,
# otro agente) no se pisan ni se borran entre sí.
RUN="bkt$$"
NET="capacitate-backup-test-$$"
PGC="$RUN-pg"; S3C="$RUN-s3"; RSC="$RUN-restore"
IMG="${BACKUP_TEST_IMAGE:-capacitate-backup:test-$$}"
# Solo para esta prueba (aprobado por el tech-lead), nunca en producción.
S3IMG=versity/versitygw:v1.10.0@sha256:89cad13cc981986ee284cbafea4613f9533ce6fd5f37a2c96c4c4fb8adb3a316
PG=postgres:16.14-alpine3.23@sha256:42b8b8b29c8a4e933d88943e5b03001a78794905cf786e6e7634e9f2abd5a0d3
GO=golang:1.27.4-alpine3.23@sha256:146e3b9c7c1e2ec3d3f5348c8a686151a75e1976e50efac9fc022ebe446a9a82
TMP=$(mktemp -d)
fail() { echo "✗ $*" >&2; exit 1; }
cleanup() {
  docker rm -f "$PGC" "$S3C" "$RSC" >/dev/null 2>&1 || true
  docker network rm "$NET" >/dev/null 2>&1 || true
  [ -n "${BACKUP_TEST_IMAGE:-}" ] || docker rmi "$IMG" >/dev/null 2>&1 || true
  rm -rf "$TMP"
}
trap cleanup EXIT

[ -n "${BACKUP_TEST_IMAGE:-}" ] || docker build -q -t "$IMG" deploy/backup >/dev/null
docker network create "$NET" >/dev/null

# Postgres con TLS (certificado autofirmado de la prueba).
openssl req -x509 -newkey rsa:2048 -nodes -days 1 -subj "/CN=$PGC" \
  -keyout "$TMP/server.key" -out "$TMP/server.crt" 2>/dev/null
docker run -d --name "$PGC" --network "$NET" -e POSTGRES_USER=owner -e POSTGRES_PASSWORD=ownerpw \
  -e POSTGRES_DB=capacitate -p 127.0.0.1::5432 -v "$TMP:/certs:ro" "$PG" sh -c \
  'install -o postgres -m 0600 /certs/server.key /var/lib/postgresql/server.key &&
   install -o postgres -m 0644 /certs/server.crt /var/lib/postgresql/server.crt &&
   exec docker-entrypoint.sh postgres -c ssl=on -c ssl_cert_file=/var/lib/postgresql/server.crt -c ssl_key_file=/var/lib/postgresql/server.key' >/dev/null
docker run -d --name "$S3C" --network "$NET" --entrypoint versitygw "$S3IMG" \
  --access testkey --secret testsecret12345 --region auto posix /tmp >/dev/null
# Listo por TCP (-h 127.0.0.1), no por socket: durante la inicialización la imagen
# oficial levanta un servidor temporal que solo escucha en el socket Unix, dice
# «ready», se apaga y recién entonces arranca el definitivo con TCP. pg_isready
# por socket pasa dentro de esa ventana y la migración, por TCP, fallaba a veces.
until docker exec "$PGC" pg_isready -h 127.0.0.1 -U owner -d capacitate >/dev/null 2>&1; do sleep 1; done

# Esquema real (goose) + copias_bd del contrato (migración 064) + aprovisionamiento.
# Puerto del host asignado por Docker (127.0.0.1::5432): nunca choca con otra corrida.
PGPORT_HOST=$(docker port "$PGC" 5432/tcp | head -1 | sed 's/.*://')
make -s migrate/up DATABASE_URL="postgres://owner:ownerpw@127.0.0.1:${PGPORT_HOST}/capacitate?sslmode=require" >"$TMP/migrate.log" 2>&1 \
  || fail "migraciones: $(tail -3 "$TMP/migrate.log")"
# copias_bd sale de la migración real (064) cuando ya está en el repo. Mientras
# no exista, se crea con el DDL del contrato (docs/api/plataforma-salud.md):
# es el único caso en que el test copia el esquema.
if ls backend/migrations/064_*.sql >/dev/null 2>&1; then
  echo "· copias_bd: migración 064 del repo"
else
  echo "· copias_bd: DDL del contrato (la 064 todavía no está en esta rama)"
  docker exec -i "$PGC" psql -U owner -d capacitate -q -v ON_ERROR_STOP=1 >/dev/null <<'SQL'
CREATE TABLE copias_bd (
    id BIGSERIAL PRIMARY KEY, iniciada_en TIMESTAMPTZ NOT NULL, terminada_en TIMESTAMPTZ NOT NULL,
    resultado VARCHAR(10) NOT NULL CHECK (resultado IN ('correcta', 'fallida')),
    tamano_bytes BIGINT CHECK (tamano_bytes >= 0),
    CONSTRAINT ck_copias_bd_tamano CHECK ((resultado = 'correcta') = (tamano_bytes IS NOT NULL)),
    CONSTRAINT ck_copias_bd_tiempos CHECK (terminada_en >= iniciada_en));
SQL
fi
docker exec -i "$PGC" psql -U owner -d capacitate -q -v ON_ERROR_STOP=1 >/dev/null <<'SQL'
CREATE ROLE backup_ro LOGIN PASSWORD 'backuppw';
GRANT pg_read_all_data TO backup_ro;
GRANT INSERT ON copias_bd TO backup_ro;
GRANT USAGE ON SEQUENCE copias_bd_id_seq TO backup_ro;
CREATE DATABASE sin_tabla;
-- Para el caso 1: puede registrar, pero no leer las tablas (pg_dump falla).
CREATE ROLE sin_lectura LOGIN PASSWORD 'sinlecturapw';
GRANT INSERT ON copias_bd TO sin_lectura;
GRANT USAGE ON SEQUENCE copias_bd_id_seq TO sin_lectura;
SQL

BUCKET_URL="http://$S3C:7070/capacitate-backups"
s3() { docker run --rm --network "$NET" -v "$TMP:/k" --entrypoint curl "$IMG" -s --aws-sigv4 'aws:amz:auto:s3' --user testkey:testsecret12345 "$@"; }
s3 -o /dev/null -X PUT --data '<CreateBucketConfiguration xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><LocationConstraint>auto</LocationConstraint></CreateBucketConfiguration>' "$BUCKET_URL"
# Claves bajo pg/ (ListObjectsV2), una por línea.
claves() { s3 "$BUCKET_URL?list-type=2&prefix=pg/" | grep -o '<Key>[^<]*</Key>' | sed -E 's#</?Key>##g'; }

docker run --rm -v "$TMP:/k" --entrypoint sh "$GO" -c \
  'go run filippo.io/age/cmd/age-keygen@v1.3.2 -o /k/key.txt 2>/dev/null; chmod 644 /k/key.txt'
PUB=$(awk '/public key/{print $4}' "$TMP/key.txt")

# Stubs que van delante en el PATH (/usr/local/sbin) solo en los casos 7–9.
mkdir -p "$TMP/stubs" "$TMP/out"; chmod 777 "$TMP/out"
cat > "$TMP/stubs/curl" <<'STUB'
#!/bin/sh
case "${STUB:-}:$*" in
  put500:*--upload-file*) printf 500; exit 0 ;;
  put500:*--head*) printf 'HTTP/1.1 200 OK\r\ncontent-length: %s\r\n\r\n' "$(stat -c %s /tmp/copia.dump.age)"; exit 0 ;;
esac
exec /usr/bin/curl "$@"
STUB
cat > "$TMP/stubs/pg_dump" <<'STUB'
#!/bin/sh
[ "${STUB:-}" = lento ] && exec sleep 60
[ "${STUB:-}" = args ] && echo "pg_dump $*" >> /stubs-out/args
exec /usr/local/bin/pg_dump "$@"
STUB
cat > "$TMP/stubs/psql" <<'STUB'
#!/bin/sh
[ "${STUB:-}" = args ] && echo "psql $*" >> /stubs-out/args
exec /usr/local/bin/psql "$@"
STUB
chmod 755 "$TMP/stubs/"*

copia() { # $1=usuario:clave $2=endpoint $3=bucket $4=secret de S3 [$5=stub] [$6=base] [$7=timeout]
  docker run --rm --network "$NET" -v "$TMP/stubs:/usr/local/sbin:ro" -v "$TMP/out:/stubs-out" \
    -e STUB="${5:-}" \
    -e BACKUP_DATABASE_URL="postgres://$1@$PGC:5432/${6:-capacitate}?sslmode=require" \
    -e AGE_RECIPIENT="$PUB" -e R2_ENDPOINT="$2" -e R2_BUCKET="$3" -e R2_ACCESS_KEY_ID=testkey \
    -e R2_SECRET_ACCESS_KEY="$4" -e BACKUP_TIMEOUT="${7:-5}" "$IMG"
}
OK_DB=backup_ro:backuppw; S3="http://$S3C:7070"; B=capacitate-backups; SEC=testsecret12345

copia sin_lectura:sinlecturapw "$S3" "$B" "$SEC" && fail "pg_dump sin permisos no falló" || true
[ -z "$(claves)" ] || fail "se subió un objeto tras un pg_dump fallido"
echo "✓ 1. pg_dump sin permisos → fallida y sin objeto en el bucket"
copia "$OK_DB" "$S3" "$B" "$SEC" && echo "✓ 2. copia correcta (sslmode=require)" || fail "la copia correcta falló"
sleep 1
copia "$OK_DB" "$S3" "$B" "$SEC" || fail "repetir la copia el mismo día falló"
[ "$(claves | wc -l | tr -d ' ')" = 2 ] || fail "esperaba 2 objetos: $(claves)"
claves | grep -qE '^pg/[0-9]{4}-[0-9]{2}-[0-9]{2}/[0-9]{6}Z\.dump\.age$' || fail "clave con formato inesperado: $(claves)"
echo "✓ 2b. repetida el mismo día → otro objeto, sin chocar ($(claves | tail -1))"
copia "$OK_DB" "$S3" no-existe "$SEC" && fail "bucket inexistente no falló" || echo "✓ 3. bucket inexistente → fallida"
copia "$OK_DB" "$S3" "$B" mala && fail "credenciales malas no fallaron" || echo "✓ 4. credenciales malas → fallida"
copia "$OK_DB" http://10.255.255.1 "$B" "$SEC" && fail "endpoint colgado no falló" || echo "✓ 5. endpoint colgado → fallida por timeout"

out=$(copia "$OK_DB" "$S3" "$B" "$SEC" put500 2>&1) && fail "PUT con http 500 no falló" || true
echo "$out" | grep -q 'http=500' || fail "PUT 500: $out"
echo "✓ 7. PUT con http 500 y HEAD «correcto» → fallida (http=500)"

copia "$OK_DB" "$S3" "$B" "$SEC" args >/dev/null 2>&1 || fail "la copia con stubs de argv falló"
grep -q "backup_ro@$PGC" "$TMP/out/args" || fail "los stubs no registraron argv"
! grep -q 'backuppw' "$TMP/out/args" || fail "la contraseña aparece en argv: $(cat "$TMP/out/args")"
echo "✓ 8. la contraseña no está en argv de pg_dump ni de psql (va por PGPASSWORD)"
# La más reciente de capacitate (la del caso 10 es de otra base, vacía).
ultima=$(claves | sort | tail -1)

t0=$(date +%s)
out=$(copia "$OK_DB" "$S3" "$B" "$SEC" lento capacitate 3 2>&1) && fail "pg_dump colgado no falló" || true
dt=$(( $(date +%s) - t0 ))
echo "$out" | grep -q 'motivo=tiempo_agotado' || fail "pg_dump colgado: $out"
[ "$dt" -lt 20 ] || fail "el timeout no cortó a tiempo (${dt}s)"
echo "✓ 9. pg_dump colgado → cortado por timeout en ${dt}s (motivo=tiempo_agotado)"

rc=0; out=$(copia "$OK_DB" "$S3" "$B" "$SEC" "" sin_tabla 2>&1) || rc=$?
[ "$rc" = 3 ] && echo "$out" | grep -q 'motivo=no_existe' || fail "sin copias_bd: rc=$rc $out"
echo "✓ 10. sin copias_bd (064 sin aplicar) → copia subida, exit 3, motivo=no_existe"

filas=$(docker exec "$PGC" psql -U owner -d capacitate -Atc "select string_agg(resultado, ',' order by id) from copias_bd")
[ "$filas" = "fallida,correcta,correcta,fallida,fallida,fallida,fallida,correcta,fallida" ] || fail "copias_bd: $filas"
echo "✓ copias_bd: $filas"

# 6. Restauración (RTO de prueba): bajar, descifrar con la privada, pg_restore en una base nueva.
t0=$(date +%s)
s3 -f -o /k/r.dump.age "$BUCKET_URL/$ultima"
[ "$(head -c 21 "$TMP/r.dump.age")" = "age-encryption.org/v1" ] || fail "el objeto no está cifrado con age"
docker run -d --name "$RSC" -e POSTGRES_PASSWORD=r -e POSTGRES_DB=restaurada "$PG" >/dev/null
until docker exec "$RSC" pg_isready -h 127.0.0.1 -U postgres -d restaurada >/dev/null 2>&1; do sleep 1; done
docker run --rm -v "$TMP:/k" --entrypoint age "$IMG" -d -i /k/key.txt -o /k/r.dump /k/r.dump.age
docker cp "$TMP/r.dump" "$RSC":/tmp/r.dump >/dev/null
docker exec "$RSC" pg_restore -U postgres -d restaurada --no-owner --no-privileges --exit-on-error /tmp/r.dump
v=$(docker exec "$RSC" psql -U postgres -d restaurada -Atc "select max(version_id) from goose_db_version" 2>/dev/null) \
  && [ -n "$v" ] || fail "la base restaurada no tiene goose_db_version"
echo "✓ 6. restaurada en una base nueva: goose ${v}, en $(( $(date +%s) - t0 )) s"
