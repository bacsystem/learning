# syntax=docker/dockerfile:1
# Imágenes base fijadas a versión de parche (H64). Para fijar también por
# digest: golang:1.27.3-alpine3.23@sha256:... / alpine:3.23.4@sha256:...

# ── Stage 1: Build ──────────────────────────────────────────────────────────
# El builder corre en la plataforma nativa y compila en cruzado para
# TARGETOS/TARGETARCH, que BuildKit rellena desde --platform (por defecto la
# plataforma del host). H70: los ARG se declaran SIN valor por defecto (un
# default fijo pisaba el valor de BuildKit y la imagen arm64 llevaba binarios
# x86-64); con el builder clásico, que no los rellena, los RUN caen a
# linux/amd64. `make docker/build/backend` sigue pasando --platform
# linux/amd64 para releases.
FROM --platform=$BUILDPLATFORM golang:1.27.3-alpine3.23 AS builder

ARG TARGETOS
ARG TARGETARCH
ARG GOOSE_VERSION=v3.21.1

WORKDIR /app

# Cache deps before copying source
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# goose compilado desde el código fuente, versión fijada y verificada contra
# sum.golang.org por el toolchain (H45; antes: binario x86_64 bajado con wget
# sin checksum). Con compilación cruzada `go install` deja el binario en
# /go/bin/<os>_<arch>/, en nativo en /go/bin/.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    mkdir -p /out \
 && os="${TARGETOS:-linux}" arch="${TARGETARCH:-amd64}" \
 && CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
    go install -ldflags="-s -w" github.com/pressly/goose/v3/cmd/goose@${GOOSE_VERSION} \
 && { mv "/go/bin/${os}_${arch}/goose" /out/goose 2>/dev/null || mv /go/bin/goose /out/goose; }

# Server + wrapper de migraciones con advisory lock (H44)
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    export CGO_ENABLED=0 GOOS="${TARGETOS:-linux}" GOARCH="${TARGETARCH:-amd64}" \
 && go build -ldflags="-s -w" -o /out/server ./cmd/server \
 && go build -ldflags="-s -w" -o /out/migrate ./cmd/migrate

# ── Stage 2: Runtime ────────────────────────────────────────────────────────
FROM alpine:3.23.4

# wget para el HEALTHCHECK lo aporta busybox (incluido en alpine base).
RUN apk add --no-cache ca-certificates tzdata \
 && addgroup -S -g 10001 app \
 && adduser -S -D -H -u 10001 -G app -s /sbin/nologin app

WORKDIR /app

# Archivos propiedad de root y solo lectura/ejecución para el usuario app.
COPY --from=builder --chmod=0755 /out/goose /usr/local/bin/goose
COPY --from=builder --chmod=0755 /out/server /out/migrate ./
COPY migrations/ ./migrations/
COPY --chmod=0755 entrypoint.sh .

USER 10001:10001

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- http://localhost:8080/health || exit 1

ENTRYPOINT ["./entrypoint.sh"]
