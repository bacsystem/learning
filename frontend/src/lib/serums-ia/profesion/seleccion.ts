import type { OpcionesProfesion, ProcesoOpcion } from "./api";

/** Lo elegido en el selector; `null` = sin elegir. */
export interface Seleccion {
  procesoId: number | null;
  especialidadId: number | null;
}

/**
 * Selección con la que abre el selector (regla 5 de SIA-E1-04 y F1 del contrato):
 * - la `preseleccion` del servidor (el perfil activo o la especialidad de persona). Si trae `proceso_id: null` y hay un
 *   solo proceso, se usa ese;
 * - sin preselección, si hay un solo proceso, se elige ese y la especialidad queda vacía (el servidor no lo sugiere);
 * - en otro caso, vacío.
 */
export function seleccionInicial({ procesos, preseleccion }: OpcionesProfesion): Seleccion {
  const unico = procesos.length === 1 ? procesos[0].id : null;
  if (preseleccion) {
    return { procesoId: preseleccion.proceso_id ?? unico, especialidadId: preseleccion.especialidad_id };
  }
  return { procesoId: unico, especialidadId: null };
}

/** Especialidades del proceso elegido; sin proceso, ninguna. */
export function especialidadesDe(procesos: readonly ProcesoOpcion[], procesoId: number | null) {
  return procesos.find((p) => p.id === procesoId)?.especialidades ?? [];
}

/**
 * Al cambiar de proceso, la especialidad se conserva solo si también existe en el nuevo (cada proceso tiene las de su
 * institución); si no, se vacía para no enviar un par inválido.
 */
export function cambiarProceso(
  procesos: readonly ProcesoOpcion[],
  actual: Seleccion,
  procesoId: number | null,
): Seleccion {
  const sigue = especialidadesDe(procesos, procesoId).some((e) => e.id === actual.especialidadId);
  return { procesoId, especialidadId: sigue ? actual.especialidadId : null };
}

/** Se puede guardar con un par completo que no sea el perfil activo (guardar el mismo par no cambia nada). */
export function puedeGuardar(
  seleccion: Seleccion,
  activo: { procesoId: number; especialidadId: number } | null,
): boolean {
  if (seleccion.procesoId === null || seleccion.especialidadId === null) return false;
  return !activo || activo.procesoId !== seleccion.procesoId || activo.especialidadId !== seleccion.especialidadId;
}
