import * as yup from "yup";

import { LIMITES } from "@/lib/admin/limites";

/** Reglas de `CreatePreguntaRequest` del backend. El servidor las vuelve a aplicar. */
export const ENUNCIADO_MIN = 10;
export const ENUNCIADO_MAX = 2000;
export const RETRO_MIN = 10;
export const RETRO_MAX = 2000;
export const RESPUESTAS_MIN = 2;

export interface MensajesValidacionPregunta {
  procesoRequerido: string;
  asignaturaRequerida: string;
  enunciadoRequerido: string;
  enunciadoLongitud: string;
  respuestaRequerida: string;
  respuestaMaximo: string;
  respuestasMinimo: string;
  retroLongitud: string;
}

/** Campos que valida el formulario; el resto (grado, etiquetas…) son opcionales y no tienen regla. */
export interface ValoresValidablesPregunta {
  proceso_id: number;
  asignatura_id: number;
  enunciado: string;
  respuestas: { texto: string }[];
  retro_explicacion: string;
}

/** `elegirProceso` es falso en edición, donde el proceso es fijo y no se valida. */
export function esquemaPregunta(m: MensajesValidacionPregunta, elegirProceso: boolean) {
  return yup.object({
    proceso_id: elegirProceso ? yup.number().min(1, m.procesoRequerido).required(m.procesoRequerido) : yup.number(),
    asignatura_id: yup.number().min(1, m.asignaturaRequerida).required(m.asignaturaRequerida),
    enunciado: yup
      .string()
      .trim()
      .required(m.enunciadoRequerido)
      .min(ENUNCIADO_MIN, m.enunciadoLongitud)
      .max(ENUNCIADO_MAX, m.enunciadoLongitud),
    respuestas: yup
      .array(
        yup.object({
          texto: yup.string().trim().required(m.respuestaRequerida).max(LIMITES.respuesta.texto, m.respuestaMaximo),
        }),
      )
      .min(RESPUESTAS_MIN, m.respuestasMinimo)
      .required(),
    // La explicación es opcional; si se escribe, el backend exige entre 10 y 2000 caracteres.
    retro_explicacion: yup
      .string()
      .trim()
      .test("longitud", m.retroLongitud, (v) => !v || (v.length >= RETRO_MIN && v.length <= RETRO_MAX)),
  });
}

/** Errores por campo, un mensaje por clave. Las alternativas llevan `claveErrorRespuesta(índice)`. */
export type ErroresPregunta = Record<string, string>;

export const claveErrorRespuesta = (indice: number) => `respuesta-${indice}`;

const CAMPOS_VALIDABLES = new Set(["proceso_id", "asignatura_id", "enunciado", "respuestas", "retro_explicacion"]);

/** Valida de forma síncrona y devuelve el primer error de cada campo. Vacío si todo es válido. */
export function validarPregunta(
  valores: ValoresValidablesPregunta,
  m: MensajesValidacionPregunta,
  elegirProceso: boolean,
): ErroresPregunta {
  try {
    esquemaPregunta(m, elegirProceso).validateSync(valores, { abortEarly: false });
    return {};
  } catch (e) {
    if (!(e instanceof yup.ValidationError)) throw e;
    const errores: ErroresPregunta = {};
    for (const { path = "", message } of e.inner) {
      const alternativa = /^respuestas\[(\d+)\]/.exec(path);
      const clave = alternativa ? claveErrorRespuesta(Number(alternativa[1])) : path;
      if (alternativa || CAMPOS_VALIDABLES.has(clave)) errores[clave] ??= message;
    }
    return errores;
  }
}

/**
 * Retroalimentación del cuerpo de la pregunta: explicación y referencia sin espacios en los extremos, y `null` si la
 * explicación queda vacía (el backend rechaza una explicación de solo espacios).
 */
export function armarRetroalimentacion(
  explicacion: string,
  referencia: string,
): { explicacion: string; referencia: string | null } | null {
  const texto = explicacion.trim();
  return texto ? { explicacion: texto, referencia: referencia.trim() || null } : null;
}
