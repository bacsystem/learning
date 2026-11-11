import { type ClavesDeError, mensajeDeError } from "@/lib/serums-ia/errores";

/**
 * Códigos de error de PE1–PE3 con traducción propia. `no_encontrado` aquí es el proceso, así que tiene su clave en
 * `serumsIa.pesos.errores` en vez de la de temas.
 */
const CLAVES_PESOS: ClavesDeError = {
  validacion: "serumsIa.pesos.errores.validacion",
  referencia_invalida: "serumsIa.pesos.errores.referencia_invalida",
  peso_duplicado: "serumsIa.pesos.errores.peso_duplicado",
  limite_preguntas: "serumsIa.pesos.errores.limite_preguntas",
  pesos_no_suman_100: "serumsIa.pesos.errores.pesos_no_suman_100",
  no_encontrado: "serumsIa.pesos.errores.no_encontrado",
  error_interno: "serumsIa.errores.error_interno",
};

/** Texto a mostrar para un error de la API de pesos (ver `mensajeDeError`). */
export function mensajeErrorPesos(err: unknown, t: (clave: string) => string, generico: string): string {
  return mensajeDeError(err, CLAVES_PESOS, t, generico);
}
