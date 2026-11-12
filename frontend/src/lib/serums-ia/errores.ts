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
  return candidatas.find(tiene) ?? CLAVE_ERROR_GENERICO;
}

/** Texto neutro traducido de un código que la UI no conoce: la API responde `message` siempre en español. */
export const CLAVE_ERROR_GENERICO = "serumsIa.errores.generico";

/**
 * Texto a mostrar para un error de la API (docs/api/README.md §8). Orden:
 * 1. la traducción del código;
 * 2. con código pero sin traducción, el genérico traducido: nunca el `message`, que viene en español;
 * 3. sin código (formato heredado o respuesta del middleware), el texto del servidor;
 * 4. sin respuesta (fallo de red: sin `status`), el texto genérico de la pantalla.
 */
export function mensajeDeError(
  err: unknown,
  claves: ClavesDeError,
  t: (clave: string) => string,
  generico: string,
): string {
  const clave = claveDeError(err, claves);
  if (clave) return t(clave);
  if (apiErrorCode(err)) return t(CLAVE_ERROR_GENERICO);
  const { status, message } = (err ?? {}) as { status?: unknown; message?: unknown };
  return typeof status === "number" && typeof message === "string" && message ? message : generico;
}
