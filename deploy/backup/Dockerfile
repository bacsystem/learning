# syntax=docker/dockerfile:1
# Copia diaria de la base a Cloudflare R2 (ADR 0004, Decisión 11; PLT-05).
# Imagen pública cbaciliod/learning-backup:vX.Y.Z: no lleva secretos (todo
# entra por variables selladas del servicio cron de Railway).

# age compilado desde el código fuente, versión fijada y verificada contra
# sum.golang.org por el toolchain (mismo criterio que goose en backend/Dockerfile).
FROM --platform=$BUILDPLATFORM golang:1.27.4-alpine3.23 AS age
ARG TARGETOS
ARG TARGETARCH
ARG AGE_VERSION=v1.3.2
# Con compilación cruzada `go install` deja el binario en /go/bin/<os>_<arch>/.
RUN --mount=type=cache,target=/go/pkg/mod \
    mkdir -p /out \
 && os="${TARGETOS:-linux}" arch="${TARGETARCH:-amd64}" \
 && CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
    go install -ldflags="-s -w" filippo.io/age/cmd/age@${AGE_VERSION} \
 && { mv "/go/bin/${os}_${arch}/age" /out/age 2>/dev/null || mv /go/bin/age /out/age; }

# pg_dump/psql de la misma versión mayor que el servidor (Postgres 16).
FROM postgres:16.14-alpine3.23

# curl ≥ 7.75 firma con --aws-sigv4 (API S3 de R2): no hace falta otra herramienta.
RUN apk add --no-cache curl ca-certificates

COPY --from=age --chmod=0755 /out/age /usr/local/bin/age
COPY --chmod=0755 backup.sh /usr/local/bin/backup.sh

# Usuario sin privilegios de la imagen oficial (uid 70); escribe solo en /tmp.
USER postgres
WORKDIR /tmp
ENTRYPOINT ["/usr/local/bin/backup.sh"]
