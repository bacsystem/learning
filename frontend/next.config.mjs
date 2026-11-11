import { PHASE_PRODUCTION_BUILD } from "next/constants.js";
import createNextIntlPlugin from "next-intl/plugin";
import process from "node:process";

const withNextIntl = createNextIntlPlugin();

const securityHeaders = [
  { key: "X-Frame-Options", value: "DENY" },
  { key: "X-Content-Type-Options", value: "nosniff" },
  { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
  { key: "Permissions-Policy", value: "camera=(), microphone=(), geolocation=()" },
  // HSTS: browsers will enforce HTTPS for 1 year (only active over HTTPS)
  { key: "Strict-Transport-Security", value: "max-age=31536000; includeSubDomains" },
];

/** @type {import('next').NextConfig} */
const nextConfig = {
  output: "standalone",
  reactStrictMode: false,
  images: {
    formats: ["image/webp", "image/avif"],
    qualities: [90],
  },
  async headers() {
    return [{ source: "/:path*", headers: securityHeaders }];
  },
};

/**
 * `NEXT_PUBLIC_API_URL` se incrusta al compilar: un build de producción sin ella (o vacía) apuntaría en silencio a
 * localhost o a rutas relativas (docs/ops/variables-de-entorno.md, H-4). Se corta aquí, antes de compilar; la URL la
 * resuelve y valida `src/lib/http/api-base.ts`.
 */
export default function config(phase) {
  if (phase === PHASE_PRODUCTION_BUILD && !process.env.NEXT_PUBLIC_API_URL?.trim()) {
    throw new Error(
      "NEXT_PUBLIC_API_URL no está definida o está vacía: el build de producción la necesita (p. ej. NEXT_PUBLIC_API_URL=https://api.ejemplo.com npm run build).",
    );
  }
  return withNextIntl(nextConfig);
}
