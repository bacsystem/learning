import type { ConfirmarImportacion, PreguntaBalotario, VistaPreviaBalotario } from "./api";
import * as yup from "yup";

/** Valores del formulario de confirmación: los selects hablan en texto y `""` es «nada elegido». */
export interface ValoresImportacion {
  proceso_id: string;
  especialidad_id: string;
  asignatura_id: string;
}

export const VALORES_INICIALES: ValoresImportacion = { proceso_id: "", especialidad_id: "", asignatura_id: "" };

/** Los tres campos son obligatorios; el servidor valida de nuevo y es la fuente de verdad (§4.1). */
export function esquemaImportacion(requerido: string) {
  return yup.object({
    proceso_id: yup.string().required(requerido),
    especialidad_id: yup.string().required(requerido),
    asignatura_id: yup.string().required(requerido),
  });
}

/** Cuerpo de `POST …/importaciones`: reenvía la versión que el admin vio (D9), no la vigente de ahora. */
export function cuerpoConfirmacion(version: number, v: ValoresImportacion): ConfirmarImportacion {
  return {
    version,
    proceso_id: Number(v.proceso_id),
    especialidad_id: Number(v.especialidad_id),
    asignatura_id: Number(v.asignatura_id),
  };
}

/** «Confirmar» solo se ofrece cuando el servidor dice que se puede. */
export const sePuedeImportar = (vista: Pick<VistaPreviaBalotario, "puede_confirmar">): boolean => vista.puede_confirmar;

/** Letras de las alternativas correctas, en orden («B», «A, C»). */
export const letrasCorrectas = (p: Pick<PreguntaBalotario, "alternativas">): string =>
  p.alternativas
    .filter((a) => a.correcta)
    .map((a) => a.letra)
    .join(", ");

/** Última página de `problemas` (al menos 1, aunque no haya ninguno). */
export const ultimaPaginaProblemas = (p: Pick<VistaPreviaBalotario["problemas"], "total" | "per_page">): number =>
  Math.max(1, Math.ceil(p.total / p.per_page));
