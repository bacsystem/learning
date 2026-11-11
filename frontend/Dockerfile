# Imagen base fijada a la versión de parche de .nvmrc (la misma que usa el CI).
# Al subir Node, cambia .nvmrc y las tres líneas FROM juntas.
# ── Stage 1: Dependencies ───────────────────────────────────────────────────
FROM node:22.23.4-alpine3.23 AS deps

RUN apk add --no-cache libc6-compat
WORKDIR /app

COPY package.json package-lock.json ./
RUN --mount=type=cache,target=/root/.npm \
    npm ci

# ── Stage 2: Build ───────────────────────────────────────────────────────────
FROM node:22.23.4-alpine3.23 AS builder

WORKDIR /app

COPY --from=deps /app/node_modules ./node_modules
COPY . .

ARG NEXT_PUBLIC_API_URL
ARG NEXT_PUBLIC_STORAGE_PREFIX=""
ARG NEXT_PUBLIC_CONTACT_EMAIL=""
ENV NEXT_PUBLIC_API_URL=$NEXT_PUBLIC_API_URL
ENV NEXT_PUBLIC_STORAGE_PREFIX=$NEXT_PUBLIC_STORAGE_PREFIX
ENV NEXT_PUBLIC_CONTACT_EMAIL=$NEXT_PUBLIC_CONTACT_EMAIL
ENV NEXT_TELEMETRY_DISABLED=1

# La URL de la API se incrusta en el bundle: si llega vacía, el frontend
# llamaría a rutas relativas sin avisar. Se falla aquí con un error claro.
RUN test -n "$NEXT_PUBLIC_API_URL" || (echo "ERROR: falta el build-arg NEXT_PUBLIC_API_URL (p. ej. https://api.dominio)" >&2 && exit 1)

RUN --mount=type=cache,target=/app/.next/cache \
    npx next build

# ── Stage 3: Runtime ─────────────────────────────────────────────────────────
FROM node:22.23.4-alpine3.23 AS runner

WORKDIR /app

ENV NODE_ENV=production
ENV PORT=3000
ENV HOSTNAME=0.0.0.0

RUN addgroup --system --gid 1001 nodejs && \
    adduser  --system --uid 1001 nextjs

COPY --from=builder --chown=nextjs:nodejs /app/.next/standalone ./
COPY --from=builder --chown=nextjs:nodejs /app/.next/static    ./.next/static
COPY --from=builder --chown=nextjs:nodejs /app/public          ./public

USER nextjs

EXPOSE 3000

CMD ["node", "server.js"]
