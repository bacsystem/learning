import type { OpcionesError } from "@/lib/errores-api";

/**
 * `PATCH /admin/inscripciones/{id}/estado` (docs/api/plataforma-avisos-idioma.md §2.1): rechazar sin motivo es el 422
 * `motivo_requerido`. El 409 `transicion_invalida` (ya la decidió otra persona) no sale como toast: la pantalla avisa y
 * recarga la lista.
 */
export const TRANSICION_INVALIDA = "transicion_invalida";

export const ERROR_ESTADO_INSCRIPCION = {
  generico: "admin-error-actualizar",
  porCodigo: { motivo_requerido: "admin-inscripciones-motivo-requerido" },
} satisfies OpcionesError;
