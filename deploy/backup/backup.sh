#!/bin/sh
# Copia diaria de Postgres a Cloudflare R2 (ADR 0004 D11, PLT-05).
#
#   pg_dump -Fc | age -r <clave pública>  →  /tmp (solo cifrado)  →  PUT a R2
#   → HEAD de verificación → INSERT en copias_bd (correcta o fallida) → borra /tmp
#
# Variables (todas obligatorias salvo BACKUP_TIMEOUT):
#   BACKUP_DATABASE_URL   rol backup_ro, URL privada con sslmode=require (sellada)
#   AGE_RECIPIENT         clave PÚBLICA age (age1…); la privada la guarda el usuario
#   R2_ENDPOINT           https://<ACCOUNT_ID>.r2.cloudflarestorage.com (sellada)
#   R2_BUCKET             bucket privado y dedicado
#   R2_ACCESS_KEY_ID      token Object Read & Write limitado al bucket (sellada)
#   R2_SECRET_ACCESS_KEY  (sellada)
#   BACKUP_TIMEOUT        segundos para dump+cifrado y para la subida (por defecto 1800):
#                         Railway no corta un cron colgado.
#
# Los logs solo dicen fecha, tamaño y resultado: nunca URL, bucket ni credenciales.
set -eu

for v in BACKUP_DATABASE_URL AGE_RECIPIENT R2_ENDPOINT R2_BUCKET R2_ACCESS_KEY_ID R2_SECRET_ACCESS_KEY; do
  eval "val=\${$v:-}"
  [ -n "$val" ] || { echo "backup: falta la variable $v" >&2; exit 2; }
done
case "$AGE_RECIPIENT" in age1*) ;; *) echo "backup: AGE_RECIPIENT no es una clave pública age (age1…)" >&2; exit 2 ;; esac
TIMEOUT="${BACKUP_TIMEOUT:-1800}"
# Ninguna conexión espera indefinidamente (tampoco el INSERT final).
export PGCONNECT_TIMEOUT=30

iniciada=$(date -u +%Y-%m-%dT%H:%M:%SZ)
# Un objeto por ejecución (UTC): repetir el cron nunca choca con el bucket lock.
sello=$(date -u +%Y-%m-%d/%H%M%S)
objeto="pg/${sello}Z.dump.age"
archivo="/tmp/copia.dump.age"
resultado=fallida
bytes=""

# El texto de libpq puede llevar host, IP, usuario o base: nunca se registra.
# Solo una categoría de una lista cerrada (allowlist).
categoria() {
  f="$1"
  if   grep -qi 'timeout expired'                                   "$f"; then echo tiempo_agotado
  elif grep -qiE 'password authentication failed|pg_hba'            "$f"; then echo autenticacion
  elif grep -qi 'permission denied'                                 "$f"; then echo permisos
  elif grep -qiE 'SSL|certificate'                                  "$f"; then echo tls
  elif grep -qiE 'could not translate|could not connect|refused|no route|unreachable' "$f"; then echo conexion
  elif grep -qi 'does not exist'                                    "$f"; then echo no_existe
  elif [ -s "$f" ]; then echo otro
  else echo sin_detalle
  fi
}

# Al salir, pase lo que pase: registra la ejecución y borra el archivo cifrado.
registrar() {
  rc=$?
  rm -f "$archivo" /tmp/pg_dump.err /tmp/psql.err /tmp/curl.err
  terminada=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  if ! psql "$BACKUP_DATABASE_URL" -X -q -v ON_ERROR_STOP=1 \
       -v ini="$iniciada" -v fin="$terminada" -v res="$resultado" -v bytes="$bytes" >/dev/null 2>/tmp/psql.err <<'SQL'
INSERT INTO copias_bd (iniciada_en, terminada_en, resultado, tamano_bytes)
VALUES (:'ini', :'fin', :'res', NULLIF(:'bytes', '')::bigint);
SQL
  then
    echo "backup: no se pudo registrar la ejecución en copias_bd (motivo=$(categoria /tmp/psql.err))" >&2
    [ "$rc" -ne 0 ] || rc=3
  fi
  echo "backup: ${terminada} resultado=${resultado}${bytes:+ tamano_bytes=${bytes}}"
  exit "$rc"
}
trap registrar EXIT
trap 'exit 143' TERM INT

: > /tmp/pg_dump.err; : > /tmp/psql.err
echo "backup: ${iniciada} inicio"

# 1. Dump cifrado en tubería: al disco solo llega el archivo cifrado.
export BACKUP_DATABASE_URL AGE_RECIPIENT archivo
# Si pg_dump falla (permisos, conexión), pipefail hace fallar la tubería aunque
# age haya escrito un archivo: no se sube nada y queda la fila «fallida».
timeout "$TIMEOUT" sh -c 'set -o pipefail; pg_dump --format=custom --no-password "$BACKUP_DATABASE_URL" 2>/tmp/pg_dump.err | age -r "$AGE_RECIPIENT" -o "$archivo"' 2>/dev/null \
  || { echo "backup: falló o superó ${TIMEOUT}s el dump/cifrado (motivo=$(categoria /tmp/pg_dump.err))" >&2; exit 1; }
tamano=$(stat -c %s "$archivo")
[ "$tamano" -gt 0 ] || { echo "backup: el archivo cifrado está vacío" >&2; exit 1; }

# 2. Subida firmada (SigV4, región auto). Las credenciales van por stdin (-K -),
#    no en argv, para que no aparezcan en la lista de procesos.
url="${R2_ENDPOINT%/}/${R2_BUCKET}/${objeto}"
credenciales() { printf 'user = "%s:%s"\n' "$R2_ACCESS_KEY_ID" "$R2_SECRET_ACCESS_KEY"; }
#    Del error solo se registra el código HTTP y el de curl: su mensaje incluiría
#    el host (el ACCOUNT_ID de R2).
rc=0
http=$(credenciales | curl -K - --silent --connect-timeout 30 --max-time "$TIMEOUT" \
  --aws-sigv4 "aws:amz:auto:s3" --upload-file "$archivo" -o /dev/null -w '%{http_code}' "$url") || rc=$?
[ "$rc" -eq 0 ] && [ "$http" = 200 ] \
  || { echo "backup: falló la subida a R2 (http=${http:-000} curl=${rc})" >&2; exit 1; }

# 3. Verificación: el objeto existe y mide lo mismo que el archivo local.
remoto=$(credenciales | curl -K - --fail --silent --show-error --connect-timeout 30 --max-time 60 \
  --aws-sigv4 "aws:amz:auto:s3" --head "$url" 2>/dev/null \
  | tr -d '\r' | awk 'tolower($1)=="content-length:"{print $2}')
[ "$remoto" = "$tamano" ] || { echo "backup: la verificación no coincide (local ${tamano}, R2 ${remoto:-sin respuesta})" >&2; exit 1; }

resultado=correcta
bytes="$tamano"
