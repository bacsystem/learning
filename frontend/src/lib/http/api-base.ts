/**
 * URL base de la API, un solo lugar para todos los clientes (`src/lib/<dominio>/api.ts`).
 *
 * `NEXT_PUBLIC_API_URL` se incrusta en el bundle al compilar. Antes cada cliente usaba
 * `process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"`: un build de producción sin la variable (o con la cadena
 * vacía, que `??` no cubre) apuntaba en silencio a localhost o a rutas relativas (docs/ops/variables-de-entorno.md, H-4).
 * Ahora, en producción, falta o vacía es un error explícito; `next.config.mjs` además corta el `next build`.
 */
export const API_BASE_DESARROLLO = "http://localhost:8080";

export function resolverApiBase(valor: string | undefined, entorno: string | undefined): string {
  const limpio = valor?.trim() ?? "";
  if (limpio === "") {
    if (entorno === "production") {
      throw new Error(
        "NEXT_PUBLIC_API_URL no está definida o está vacía en un build de producción. Defínela con la URL de la API (p. ej. https://api.ejemplo.com) antes de `npm run build`.",
      );
    }
    return API_BASE_DESARROLLO;
  }
  let url: URL;
  try {
    url = new URL(limpio);
  } catch {
    throw new Error(`NEXT_PUBLIC_API_URL no es una URL válida: «${limpio}».`);
  }
  if (url.protocol !== "http:" && url.protocol !== "https:") {
    throw new Error(`NEXT_PUBLIC_API_URL debe usar http o https: «${limpio}».`);
  }
  // Sin barra final: los clientes concatenan rutas que ya empiezan con «/».
  return limpio.replace(/\/+$/, "");
}

// Acceso literal a `process.env.NEXT_PUBLIC_API_URL`: es lo que Next reemplaza al compilar.
export const API_BASE = resolverApiBase(process.env.NEXT_PUBLIC_API_URL, process.env.NODE_ENV);
