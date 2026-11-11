import { apiErrorCode } from "@/lib/http/api-error";

/** Errores de `PUT …/citas` con texto propio (E3-07 §4.2). Un código ajeno al mapa usa el genérico del módulo. */
const CLAVES_CITAS: Readonly<Record<string, string>> = {
  fuente_no_activa: "serumsIa.errores.fuente_no_activa",
  limite_citas: "serumsIa.errores.limite_citas",
  ultima_cita_activa: "serumsIa.errores.ultima_cita_activa",
};

export const CLAVE_ERROR_GENERICO = "serumsIa.errores.generico";

/** Clave i18n del error de citas o de la búsqueda. Nunca se muestra el `message` del servidor (README §8). */
export function claveErrorRevision(err: unknown): string {
  const code = apiErrorCode(err);
  return code && Object.hasOwn(CLAVES_CITAS, code) ? CLAVES_CITAS[code] : CLAVE_ERROR_GENERICO;
}
