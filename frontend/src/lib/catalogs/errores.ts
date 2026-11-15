import type { OpcionesError } from "@/lib/errores-api";

/**
 * Textos de los errores de los catálogos del admin. Responden en el formato heredado, sin `code`
 * (catalogs/service.go), así que se traducen por status: un 409 al guardar es un duplicado y, al desactivar un grado
 * o una especialidad, que aún hay usuarios activos vinculados.
 */
const CONFLICTO = 409;

export const ERRORES_INSTITUCION = {
  guardar: { accion: "guardar", porStatus: { [CONFLICTO]: "admin-error-institucion-duplicada" } },
  estado: { accion: "estado" },
} satisfies Record<string, OpcionesError>;

export const ERRORES_GRADO = {
  guardar: { accion: "guardar", porStatus: { [CONFLICTO]: "admin-error-grado-duplicado" } },
  estado: { accion: "estado", porStatus: { [CONFLICTO]: "admin-error-grado-con-usuarios" } },
} satisfies Record<string, OpcionesError>;

export const ERRORES_ESPECIALIDAD = {
  guardar: { accion: "guardar", porStatus: { [CONFLICTO]: "admin-error-especialidad-duplicada" } },
  estado: { accion: "estado", porStatus: { [CONFLICTO]: "admin-error-especialidad-con-usuarios" } },
} satisfies Record<string, OpcionesError>;

export const ERRORES_ASIGNATURA = {
  guardar: { accion: "guardar", porStatus: { [CONFLICTO]: "admin-error-asignatura-duplicada" } },
  estado: { accion: "estado" },
} satisfies Record<string, OpcionesError>;

export const ERRORES_PROCESO = {
  guardar: { accion: "guardar", porStatus: { [CONFLICTO]: "admin-error-proceso-duplicado" } },
  estado: { accion: "estado" },
} satisfies Record<string, OpcionesError>;

/** Cambio de correo de un usuario (inscripciones/handler.go): 409 si ya lo usa otra cuenta, 422 si no es válido. */
export const ERRORES_CORREO_USUARIO = {
  generico: "admin-error-actualizar-correo",
  porStatus: { [CONFLICTO]: "admin-error-correo-en-uso", 422: "admin-error-correo-invalido" },
} satisfies OpcionesError;
