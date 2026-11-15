import { apiErrorCode } from "@/lib/http/api-error";

/** Operación que falló; su texto genérico es `comun-error-<acción>` (namespace `dashboard`). */
export type AccionApi = "cargar" | "guardar" | "estado" | "eliminar";

export interface MapasDeError {
  /** Código estable del error (docs/api/README.md §3) → clave i18n. */
  porCodigo?: Readonly<Record<string, string>>;
  /** Status HTTP → clave i18n, solo para las respuestas en formato heredado, sin `code`. */
  porStatus?: Readonly<Record<number, string>>;
}

/**
 * Qué falló y cómo se traduce. Exactamente una de dos: la `accion` (texto genérico `comun-error-<acción>`) o un
 * `generico` propio de la pantalla (p. ej. `admin-ingresos-error-tarifa`); el tipo no admite las dos.
 */
export type OpcionesError = MapasDeError &
  ({ accion: AccionApi; generico?: never } | { accion?: never; generico: string });

/**
 * Clave i18n del error de una llamada a la API (docs/api/README.md §3 y §8):
 * 1. con `code`: su traducción en `porCodigo` o, si no la tiene, el genérico; el status no se mira, porque un código
 *    desconocido no significa lo que significaría su status;
 * 2. sin `code` (formato heredado): la traducción del status en `porStatus` o el genérico.
 * Nunca el `message` del servidor: viene siempre en español y a veces con nombres internos de campo.
 *
 * `claveErrorAuth` (lib/auth) no se apoya en este helper: tiene claves propias por status y distingue el error de red.
 */
export function claveErrorApi(err: unknown, { accion, generico, porCodigo, porStatus }: OpcionesError): string {
  const claveGenerica = generico ?? `comun-error-${accion}`;
  const code = apiErrorCode(err);
  if (code) return porCodigo && Object.hasOwn(porCodigo, code) ? porCodigo[code] : claveGenerica;
  const { status } = (err ?? {}) as { status?: unknown };
  return (typeof status === "number" && porStatus?.[status]) || claveGenerica;
}
