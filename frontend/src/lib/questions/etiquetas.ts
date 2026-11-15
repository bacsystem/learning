import * as yup from "yup";

import type { OpcionesError } from "@/lib/errores-api";

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

const NO_EXISTE = { 404: "admin-etiquetas-error-no-existe" } as const;

/**
 * Las etiquetas responden en el formato heredado, sin `code`: se traducen por status. Un 409 solo ocurre al
 * guardar (nombre repetido en la asignatura); un 404, en cualquier operación (la etiqueta ya no existe).
 */
export const ERRORES_ETIQUETA = {
  guardar: { accion: "guardar", porStatus: { ...NO_EXISTE, 409: "admin-etiquetas-error-duplicada" } },
  estado: { accion: "estado", porStatus: NO_EXISTE },
  eliminar: { accion: "eliminar", porStatus: NO_EXISTE },
} satisfies Record<string, OpcionesError>;
