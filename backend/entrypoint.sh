#!/bin/sh
set -e

# Migraciones bajo un advisory lock de Postgres: si arrancan varias réplicas a
# la vez, solo una migra y las demás esperan (H44). DATABASE_URL viene del
# entorno y se pasa a goose por GOOSE_DBSTRING (no queda en argv).
# MIGRATE_LOCK_TIMEOUT (p.ej. 10m) limita la espera por el lock.
echo "Running database migrations..."
./migrate -dir ./migrations -goose /usr/local/bin/goose -lock-timeout "${MIGRATE_LOCK_TIMEOUT:-10m}" up

echo "Starting server..."
exec ./server
