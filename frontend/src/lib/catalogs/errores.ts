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

/**
 * Procesos y sus asignaturas (catalogs/handler.go, repository.go). Activar con pesos que no suman 100 % es un 400;
 * asignar o quitar una asignatura de un proceso activo cuya suma dejaría de ser 100 % es un 409 sin código;
 * asignar una ya asignada es el 409 con código `peso_duplicado` (en esta pantalla no pasa por `serums-ia/pesos`).
 */
export const ERRORES_PROCESO = {
  guardar: { accion: "guardar", porStatus: { [CONFLICTO]: "admin-error-proceso-duplicado" } },
  estado: { accion: "estado", porStatus: { 400: "admin-error-proceso-pesos-no-suman" } },
  asignar: {
    generico: "admin-error-asignar-asignatura",
    porCodigo: { peso_duplicado: "admin-error-asignatura-ya-asignada" },
    porStatus: { [CONFLICTO]: "admin-error-proceso-activo-pesos" },
  },
  quitar: {
    generico: "admin-error-eliminar-asignatura",
    porStatus: { [CONFLICTO]: "admin-error-proceso-activo-pesos" },
  },
} satisfies Record<string, OpcionesError>;
