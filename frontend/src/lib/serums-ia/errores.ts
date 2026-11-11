import { apiErrorCode } from "@/lib/http/api-error";

/** Código de error de la API → clave i18n (namespace `dashboard`) de cada pantalla de SERUMS IA (docs/api/README.md §8). */
export type ClavesDeError = Readonly<Record<string, string>>;

/** Clave i18n del código del error según `claves`, o `null` si no tiene traducción. */
export function claveDeError(err: unknown, claves: ClavesDeError): string | null {
  const code = apiErrorCode(err);
  return code && Object.hasOwn(claves, code) ? claves[code] : null;
}

/**
 * Texto a mostrar para un error de la API. Orden: traducción del código, mensaje del servidor y, si no hubo respuesta
 * del servidor (fallo de red: sin `status`), el texto genérico.
 */
export function mensajeDeError(
  err: unknown,
  claves: ClavesDeError,
  t: (clave: string) => string,
  generico: string,
): string {
  const clave = claveDeError(err, claves);
  if (clave) return t(clave);
  const { status, message } = (err ?? {}) as { status?: unknown; message?: unknown };
  return typeof status === "number" && typeof message === "string" && message ? message : generico;
}
