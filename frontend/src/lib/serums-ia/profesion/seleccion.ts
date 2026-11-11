import type { OpcionesProfesion, ProcesoOpcion } from "./api";

import { apiErrorCode } from "@/lib/http/api-error";

/** Lo elegido en el selector; `null` = sin elegir. */
export interface Seleccion {
  procesoId: number | null;
  especialidadId: number | null;
}

/**
 * Selección con la que abre el selector (regla 5 de SIA-E1-04 y F1 del contrato):
 * - la `preseleccion` del servidor (el perfil activo o la especialidad de persona). Si trae `proceso_id: null` y hay un
 *   solo proceso, se usa ese;
 * - con varios procesos y `proceso_id: null`, la especialidad sugerida **no** entra al selector, porque sin proceso no
 *   tiene opciones donde mostrarse (qa E1-04 H1): se muestra aparte (`especialidadSugerida`) y se aplica al elegir un
 *   proceso que la tenga (`cambiarProceso`);
 * - sin preselección, si hay un solo proceso, se elige ese y la especialidad queda vacía (el servidor no lo sugiere);
 * - en otro caso, vacío.
 */
export function seleccionInicial({ procesos, preseleccion }: OpcionesProfesion): Seleccion {
  const unico = procesos.length === 1 ? procesos[0].id : null;
  const procesoId = preseleccion?.proceso_id ?? unico;
  if (preseleccion && procesoId !== null) return { procesoId, especialidadId: preseleccion.especialidad_id };
  return { procesoId, especialidadId: null };
}

/** Especialidad sugerida por el servidor, con su nombre, para mostrarla mientras no hay proceso elegido. */
export function especialidadSugerida({
  procesos,
  preseleccion,
}: OpcionesProfesion): { id: number; nombre: string } | null {
  if (!preseleccion) return null;
  for (const p of procesos) {
    const e = p.especialidades.find((x) => x.id === preseleccion.especialidad_id);
    if (e) return { id: e.id, nombre: e.nombre };
  }
  return null;
}

/** Especialidades del proceso elegido; sin proceso, ninguna. */
export function especialidadesDe(procesos: readonly ProcesoOpcion[], procesoId: number | null) {
  return procesos.find((p) => p.id === procesoId)?.especialidades ?? [];
}

/**
 * Al cambiar de proceso, la especialidad se conserva solo si también existe en el nuevo (cada proceso tiene las de su
 * institución); si no, se vacía para no enviar un par inválido. Sin especialidad elegida, se aplica la sugerida si el
 * nuevo proceso la tiene.
 */
export function cambiarProceso(
  procesos: readonly ProcesoOpcion[],
  actual: Seleccion,
  procesoId: number | null,
  /** Sugerencia del servidor: se aplica si aún no hay especialidad elegida y el nuevo proceso la tiene. */
  sugeridaId: number | null = null,
): Seleccion {
  const disponibles = especialidadesDe(procesos, procesoId);
  if (disponibles.some((e) => e.id === actual.especialidadId))
    return { procesoId, especialidadId: actual.especialidadId };
  const sugerida = actual.especialidadId === null && disponibles.some((e) => e.id === sugeridaId);
  return { procesoId, especialidadId: sugerida ? sugeridaId : null };
}

/** Se puede guardar con un par completo que no sea el perfil activo (guardar el mismo par no cambia nada). */
export function puedeGuardar(
  seleccion: Seleccion,
  activo: { procesoId: number; especialidadId: number } | null,
): boolean {
  if (seleccion.procesoId === null || seleccion.especialidadId === null) return false;
  return !activo || activo.procesoId !== seleccion.procesoId || activo.especialidadId !== seleccion.especialidadId;
}

/** Resultado de un intento de guardar (F3): guardado, hay que pedir confirmación del reinicio, o error. */
export type ResultadoGuardado = { tipo: "guardado" } | { tipo: "confirmar" } | { tipo: "error"; error: unknown };

/**
 * Un intento de F3. Si ya había otra profesión y aún no se confirmó, el servidor responde 409
 * `confirmacion_requerida`: se pide confirmar al usuario (el diálogo) y **no** se reenvía solo. Tras confirmar, se llama
 * de nuevo con `confirmarReinicio = true`; un 409 a esa altura es un error, no otra confirmación.
 */
export async function intentarGuardar(
  guardar: (confirmarReinicio: boolean) => Promise<void>,
  confirmarReinicio: boolean,
): Promise<ResultadoGuardado> {
  try {
    await guardar(confirmarReinicio);
    return { tipo: "guardado" };
  } catch (error) {
    if (!confirmarReinicio && apiErrorCode(error) === "confirmacion_requerida") return { tipo: "confirmar" };
    return { tipo: "error", error };
  }
}
