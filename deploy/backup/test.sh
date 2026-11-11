#!/bin/sh
# Prueba de punta a punta del cron de copia, sin R2 ni credenciales reales:
#   Postgres 16 desechable con TLS (sslmode=require, como en producción),
#   migraciones reales + copias_bd (064) + backup_ro, y versitygw (servidor S3
#   que valida SigV4, solo para esta prueba). Casos:
#     1. pg_dump sin permisos  → fallida y NINGÚN objeto subido
#     2. copia correcta        → objeto cifrado con age + fila correcta
#     3. bucket inexistente, 4. credenciales malas, 5. endpoint colgado → fallida
#     6. restauración cronometrada con la clave privada en una base nueva
# Uso: make backup/test  (BACKUP_TEST_IMAGE=<imagen> prueba una ya construida,
#      p. ej. la de release linux/amd64). Todo se borra al terminar.
set -eu
cd "$(dirname "$0")/../.."

NET=capacitate-backup-test
IMG="${BACKUP_TEST_IMAGE:-capacitate-backup:test}"
S3IMG=versity/versitygw:v1.10.0
TMP=$(mktemp -d)
fail() { echo "✗ $*" >&2; exit 1; }
cleanup() {
  docker rm -f bkt-pg bkt-s3 bkt-restore >/dev/null 2>&1 || true
  docker network rm "$NET" >/dev/null 2>&1 || true
  rm -rf "$TMP"
}
trap cleanup EXIT

[ -n "${BACKUP_TEST_IMAGE:-}" ] || docker build -q -t "$IMG" deploy/backup >/dev/null
docker network create "$NET" >/dev/null

# Postgres con TLS (certificado autofirmado de la prueba).
openssl req -x509 -newkey rsa:2048 -nodes -days 1 -subj "/CN=bkt-pg" \
  -keyout "$TMP/server.key" -out "$TMP/server.crt" 2>/dev/null
docker run -d --name bkt-pg --network "$NET" -e POSTGRES_USER=owner -e POSTGRES_PASSWORD=ownerpw \
  -e POSTGRES_DB=capacitate -p 127.0.0.1:55445:5432 -v "$TMP:/certs:ro" postgres:16-alpine sh -c \
  'install -o postgres -m 0600 /certs/server.key /var/lib/postgresql/server.key &&
   install -o postgres -m 0644 /certs/server.crt /var/lib/postgresql/server.crt &&
   exec docker-entrypoint.sh postgres -c ssl=on -c ssl_cert_file=/var/lib/postgresql/server.crt -c ssl_key_file=/var/lib/postgresql/server.key' >/dev/null
docker run -d --name bkt-s3 --network "$NET" --entrypoint versitygw "$S3IMG" \
  --access testkey --secret testsecret12345 --region auto posix /tmp >/dev/null
until docker exec bkt-pg pg_isready -U owner -d capacitate >/dev/null 2>&1; do sleep 1; done
sleep 2

# Esquema real (goose) + copias_bd del contrato (migración 064) + aprovisionamiento.
make -s migrate/up DATABASE_URL='postgres://owner:ownerpw@127.0.0.1:55445/capacitate?sslmode=require' >/dev/null 2>&1 \
  || fail "migraciones"
docker exec -i bkt-pg psql -U owner -d capacitate -q -v ON_ERROR_STOP=1 >/dev/null <<'SQL'
CREATE TABLE IF NOT EXISTS copias_bd (
    id BIGSERIAL PRIMARY KEY, iniciada_en TIMESTAMPTZ NOT NULL, terminada_en TIMESTAMPTZ NOT NULL,
    resultado VARCHAR(10) NOT NULL CHECK (resultado IN ('correcta', 'fallida')),
    tamano_bytes BIGINT CHECK (tamano_bytes >= 0),
    CONSTRAINT ck_copias_bd_tamano CHECK ((resultado = 'correcta') = (tamano_bytes IS NOT NULL)),
    CONSTRAINT ck_copias_bd_tiempos CHECK (terminada_en >= iniciada_en));
CREATE ROLE backup_ro LOGIN PASSWORD 'backuppw';
GRANT pg_read_all_data TO backup_ro;
GRANT INSERT ON copias_bd TO backup_ro;
GRANT USAGE ON SEQUENCE copias_bd_id_seq TO backup_ro;
-- Para el caso 1: puede registrar, pero no leer las tablas (pg_dump falla).
CREATE ROLE sin_lectura LOGIN PASSWORD 'sinlecturapw';
GRANT INSERT ON copias_bd TO sin_lectura;
GRANT USAGE ON SEQUENCE copias_bd_id_seq TO sin_lectura;
SQL

s3() { docker run --rm --network "$NET" -v "$TMP:/k" --entrypoint curl "$IMG" -s --aws-sigv4 'aws:amz:auto:s3' --user testkey:testsecret12345 "$@"; }
s3 -o /dev/null -X PUT --data '<CreateBucketConfiguration xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><LocationConstraint>auto</LocationConstraint></CreateBucketConfiguration>' http://bkt-s3:7070/capacitate-backups
OBJ="http://bkt-s3:7070/capacitate-backups/pg/$(date -u +%Y-%m-%d).dump.age"

docker run --rm -v "$TMP:/k" --entrypoint sh golang:1.27.4-alpine3.23 -c \
  'go run filippo.io/age/cmd/age-keygen@v1.3.2 -o /k/key.txt 2>/dev/null; chmod 644 /k/key.txt'
PUB=$(awk '/public key/{print $4}' "$TMP/key.txt")

copia() { # $1=usuario:clave de Postgres $2=endpoint $3=bucket $4=secret de S3
  docker run --rm --network "$NET" \
    -e BACKUP_DATABASE_URL="postgres://$1@bkt-pg:5432/capacitate?sslmode=require" \
    -e AGE_RECIPIENT="$PUB" -e R2_ENDPOINT="$2" -e R2_BUCKET="$3" -e R2_ACCESS_KEY_ID=testkey \
    -e R2_SECRET_ACCESS_KEY="$4" -e BACKUP_TIMEOUT=5 "$IMG"
}
OK_DB=backup_ro:backuppw; S3=http://bkt-s3:7070; B=capacitate-backups; SEC=testsecret12345

copia sin_lectura:sinlecturapw "$S3" "$B" "$SEC" && fail "pg_dump sin permisos no falló" || true
[ "$(s3 -o /dev/null -w '%{http_code}' --head "$OBJ")" = 404 ] || fail "se subió un objeto tras un pg_dump fallido"
echo "✓ 1. pg_dump sin permisos → fallida y sin objeto en el bucket"
copia "$OK_DB" "$S3" "$B" "$SEC" && echo "✓ 2. copia correcta (sslmode=require)" || fail "la copia correcta falló"
copia "$OK_DB" "$S3" no-existe "$SEC" && fail "bucket inexistente no falló" || echo "✓ 3. bucket inexistente → fallida"
copia "$OK_DB" "$S3" "$B" mala && fail "credenciales malas no fallaron" || echo "✓ 4. credenciales malas → fallida"
copia "$OK_DB" http://10.255.255.1 "$B" "$SEC" && fail "endpoint colgado no falló" || echo "✓ 5. endpoint colgado → fallida por timeout"

filas=$(docker exec bkt-pg psql -U owner -d capacitate -Atc "select string_agg(resultado, ',' order by id) from copias_bd")
[ "$filas" = "fallida,correcta,fallida,fallida,fallida" ] || fail "copias_bd: $filas"
echo "✓ copias_bd: $filas"

# 6. Restauración (RTO de prueba): bajar, descifrar con la privada, pg_restore en una base nueva.
t0=$(date +%s)
s3 -f -o /k/r.dump.age "$OBJ"
[ "$(head -c 21 "$TMP/r.dump.age")" = "age-encryption.org/v1" ] || fail "el objeto no está cifrado con age"
docker run -d --name bkt-restore -e POSTGRES_PASSWORD=r -e POSTGRES_DB=restaurada postgres:16-alpine >/dev/null
until docker exec bkt-restore pg_isready -U postgres -d restaurada >/dev/null 2>&1; do sleep 1; done
sleep 2
docker run --rm -v "$TMP:/k" --entrypoint age "$IMG" -d -i /k/key.txt -o /k/r.dump /k/r.dump.age
docker cp "$TMP/r.dump" bkt-restore:/tmp/r.dump >/dev/null
docker exec bkt-restore pg_restore -U postgres -d restaurada --no-owner --no-privileges --exit-on-error /tmp/r.dump
v=$(docker exec bkt-restore psql -U postgres -d restaurada -Atc "select max(version_id) from goose_db_version")
echo "✓ 6. restaurada en una base nueva: goose ${v}, en $(( $(date +%s) - t0 )) s"
