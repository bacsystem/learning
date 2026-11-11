import { apiErrorCode } from "@/lib/http/api-error";

/** Código de error de la API → clave i18n (namespace `dashboard`) de cada pantalla de SERUMS IA (docs/api/README.md §8). */
export type ClavesDeError = Readonly<Record<string, string>>;

/** Clave i18n del código del error según `claves`, o `null` si no tiene traducción. */
export function claveDeError(err: unknown, claves: ClavesDeError): string | null {
  const code = apiErrorCode(err);
  return code && Object.hasOwn(claves, code) ? claves[code] : null;
}

/** Códigos que cambian de sentido según el recurso: tienen texto propio de cada recurso y uno neutro del módulo. */
const CODIGOS_COMUNES = new Set(["estado_invalido", "no_encontrado", "validacion", "referencia_invalida"]);

/**
 * Clave i18n de un error del módulo SERUMS IA (docs/api/README.md §8). Orden: `serumsIa.<recurso>.errores.<codigo>`
 * (solo los códigos comunes), `serumsIa.errores.<codigo>` y, si no hay ninguna, `serumsIa.errores.generico`.
 * Nunca el `message` del servidor, que viene siempre en español.
 */
export function claveErrorModulo(err: unknown, recurso: string, tiene: (clave: string) => boolean): string {
  const code = apiErrorCode(err);
  const candidatas = code
    ? [...(CODIGOS_COMUNES.has(code) ? [`serumsIa.${recurso}.errores.${code}`] : []), `serumsIa.errores.${code}`]
    : [];
  return candidatas.find(tiene) ?? "serumsIa.errores.generico";
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
