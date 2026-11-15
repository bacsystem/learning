import * as yup from "yup";

import { apiErrorCode } from "@/lib/http/api-error";

/** Largo máximo del nombre de una etiqueta (`CreateEtiquetaRequest` y `UpdateEtiquetaRequest` del backend). */
export const NOMBRE_ETIQUETA_MAX = 100;

export interface ValoresEtiqueta {
  /** `0` es «sin elegir». */
  asignatura_id: number;
  nombre: string;
}

export interface MensajesValidacionEtiqueta {
  asignaturaRequerida: string;
  nombreRequerido: string;
  nombreLongitud: string;
}

/** Reglas del cuerpo de las etiquetas. La asignatura solo se elige al crear: al editar no viaja. El servidor las vuelve a aplicar. */
export function esquemaEtiqueta(m: MensajesValidacionEtiqueta, modo: "create" | "edit") {
  return yup.object({
    asignatura_id:
      modo === "create" ? yup.number().min(1, m.asignaturaRequerida).required(m.asignaturaRequerida) : yup.number(),
    nombre: yup.string().trim().required(m.nombreRequerido).max(NOMBRE_ETIQUETA_MAX, m.nombreLongitud),
  });
}

/** Operación de etiquetas que falló; decide el texto genérico. */
export type AccionEtiqueta = "guardar" | "estado" | "eliminar";

const CLAVE_GENERICA: Record<AccionEtiqueta, string> = {
  guardar: "comun-error-guardar",
  estado: "comun-error-estado",
  eliminar: "comun-error-eliminar",
};

const STATUS_CONFLICTO = 409;
const STATUS_NO_ENCONTRADO = 404;

/**
 * Clave i18n (namespace `dashboard`) del error de una operación de etiquetas. Las etiquetas responden en el formato
 * heredado, sin `code` (docs/api/README.md §3), así que se traduce por status y, ante cualquier otro, el genérico de la
 * acción. Nunca el `message` del servidor: viene siempre en español y con nombres internos de campo (§8).
 */
export function claveErrorEtiqueta(err: unknown, accion: AccionEtiqueta): string {
  const { status } = (err ?? {}) as { status?: unknown };
  if (apiErrorCode(err) === undefined) {
    if (status === STATUS_CONFLICTO && accion === "guardar") return "admin-etiquetas-error-duplicada";
    if (status === STATUS_NO_ENCONTRADO) return "admin-etiquetas-error-no-existe";
  }
  return CLAVE_GENERICA[accion];
}
