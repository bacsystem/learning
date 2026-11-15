import { apiErrorCode } from "@/lib/http/api-error";

/** Operación que falló; su texto genérico es `comun-error-<acción>` (namespace `dashboard`). */
export type AccionApi = "cargar" | "guardar" | "estado" | "eliminar";

export interface MapasDeError {
  /** Código estable del error (docs/api/README.md §3) → clave i18n. */
  porCodigo?: Readonly<Record<string, string>>;
  /** Status HTTP → clave i18n, para las respuestas en formato heredado, sin `code`. */
  porStatus?: Readonly<Record<number, string>>;
}

/**
 * Clave i18n del error de una llamada a la API. Orden: traducción del código, traducción del status y, si no hay
 * ninguna (5xx, red, código desconocido), el texto genérico de la acción. Nunca el `message` del servidor: viene
 * siempre en español y a veces con nombres internos de campo (docs/api/README.md §8).
 *
 * `claveErrorAuth` (lib/auth) no se apoya en este helper: tiene claves propias por status y distingue el error de red.
 */
export function claveErrorApi(err: unknown, accion: AccionApi, { porCodigo, porStatus }: MapasDeError = {}): string {
  const code = apiErrorCode(err);
  if (code && porCodigo && Object.hasOwn(porCodigo, code)) return porCodigo[code];
  const { status } = (err ?? {}) as { status?: unknown };
  if (typeof status === "number" && porStatus?.[status]) return porStatus[status];
  return `comun-error-${accion}`;
}
