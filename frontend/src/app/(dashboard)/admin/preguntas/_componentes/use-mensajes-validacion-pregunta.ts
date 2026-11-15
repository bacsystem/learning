import { useTranslations } from "next-intl";
import { useMemo } from "react";

import { LIMITES } from "@/lib/admin/limites";
import {
  ENUNCIADO_MAX,
  ENUNCIADO_MIN,
  type MensajesValidacionPregunta,
  RESPUESTAS_MIN,
  RETRO_MAX,
  RETRO_MIN,
} from "@/lib/questions/pregunta";

/** Mensajes traducidos de la validación del formulario de pregunta (nueva y edición). */
export function useMensajesValidacionPregunta(): MensajesValidacionPregunta {
  const t = useTranslations("dashboard");
  return useMemo(
    () => ({
      procesoRequerido: t("admin-preguntas-selecciona-proceso"),
      asignaturaRequerida: t("admin-preguntas-selecciona-asignatura"),
      enunciadoRequerido: t("admin-preguntas-validacion-enunciado"),
      enunciadoLongitud: t("admin-preguntas-validacion-enunciado-longitud", { min: ENUNCIADO_MIN, max: ENUNCIADO_MAX }),
      respuestaRequerida: t("admin-preguntas-validacion-respuesta"),
      respuestaMaximo: t("admin-preguntas-validacion-respuesta-maximo", { max: LIMITES.respuesta.texto }),
      respuestasMinimo: t("admin-preguntas-validacion-respuestas-minimo", { min: RESPUESTAS_MIN }),
      retroLongitud: t("admin-preguntas-validacion-retro", { min: RETRO_MIN, max: RETRO_MAX }),
    }),
    [t],
  );
}
