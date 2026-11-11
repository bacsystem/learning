import { type ClavesDeError, mensajeDeError } from "@/lib/serums-ia/errores";

/** Códigos de error de F1–F3 con traducción propia (docs/api/serums-ia-e1-taxonomia.md §3). */
const CLAVES_PROFESION: ClavesDeError = {
  inscripcion_requerida: "serumsIa.profesion.errores.inscripcion_requerida",
  especialidad_invalida: "serumsIa.profesion.errores.especialidad_invalida",
  estado_invalido: "serumsIa.profesion.errores.estado_invalido",
  validacion: "serumsIa.profesion.errores.validacion",
  cuerpo_invalido: "serumsIa.profesion.errores.validacion",
  error_interno: "serumsIa.profesion.errores.error_interno",
  cuerpo_demasiado_grande: "serumsIa.errores.cuerpo_demasiado_grande",
};

export function mensajeErrorProfesion(err: unknown, t: (clave: string) => string, generico: string): string {
  return mensajeDeError(err, CLAVES_PROFESION, t, generico);
}
