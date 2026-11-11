#!/bin/sh
# Verifica que una imagen publicable no lleve archivos ni secretos que no debe.
# Las imágenes de Docker Hub son públicas (docs/ops/checklist-release.md §5).
# Uso: deploy/verify-image.sh <imagen:tag>   (sale con 1 si encuentra algo)
set -eu
img="${1:?uso: deploy/verify-image.sh <imagen:tag>}"
status=0

cid=$(docker create "$img")
trap 'docker rm -f "$cid" >/dev/null 2>&1' EXIT

# 1. Sistema de archivos de la app (/app, el WORKDIR de las dos imágenes), sin node_modules.
bad=$(docker export "$cid" | tar -t \
  | grep -E '^app/' | grep -v '^app/node_modules/' \
  | grep -E '(^|/)\.env|(^|/)assets/|(^|/)bin/|_test\.go$|\.test$|(^|/)coverage|(^|/)\.git(/|$)' || true)
if [ -n "$bad" ]; then
  echo "✗ $img contiene archivos que no deben publicarse:"; echo "$bad" | sed 's/^/    /'; status=1
fi

# 2. Secretos en capas (history) o en el ENV de la imagen. NEXT_PUBLIC_* es público.
pat='secret|password|passwd|token|private_key|api_key|encryption_key|jwt|hmac|smtp_pass|database_url'
leaks=$( { docker history --no-trunc --format '{{.CreatedBy}}' "$img"
           docker inspect -f '{{range .Config.Env}}{{println .}}{{end}}' "$img"; } \
  | grep -iE "$pat" || true)
if [ -n "$leaks" ]; then
  echo "✗ $img: posible secreto en capas o ENV:"; echo "$leaks" | cut -c1-200 | sed 's/^/    /'; status=1
fi

pub=$(docker inspect -f '{{range .Config.Env}}{{println .}}{{end}}' "$img" | grep '^NEXT_PUBLIC_' || true)
[ -z "$pub" ] || { echo "ℹ $img: NEXT_PUBLIC_* incrustadas (públicas, revisar que no haya nada sensible):"; echo "$pub" | sed 's/^/    /'; }

[ "$status" -eq 0 ] && echo "✓ $img: sin archivos prohibidos ni secretos en capas/ENV"
exit "$status"
