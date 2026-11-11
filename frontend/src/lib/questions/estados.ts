import type { EstadoPregunta, OrigenPregunta, Pregunta } from "./api";

import { apiErrorCode } from "@/lib/http/api-error";

/**
 * Reglas de la UI sobre el ciclo de vida de una pregunta (SIA-E3-02, docs/api/serums-ia-e3-02-ciclo-preguntas.md).
 * Las transiciones las valida el servidor; aquí solo se decide qué ofrecer y qué avisar.
 */

export const ESTADOS_PREGUNTA: readonly EstadoPregunta[] = [
  "borrador",
  "en_revision",
  "aprobada",
  "rechazada",
  "activo",
  "eliminada",
];

/** Valores del filtro `estado` de `GET /preguntas` (§4.3): todos salvo `eliminada`, que no se lista. */
export const ESTADOS_FILTRABLES = ESTADOS_PREGUNTA.filter(
  (e): e is Exclude<EstadoPregunta, "eliminada"> => e !== "eliminada",
);

export const ORIGENES_PREGUNTA: readonly OrigenPregunta[] = ["manual", "balotario", "ia"];

/** Claves i18n (`dashboard.*`) del nombre de un estado y de un origen. */
export const claveEstadoPregunta = (estado: EstadoPregunta) => `admin-preguntas-estado-${estado.replace("_", "-")}`;
export const claveOrigenPregunta = (origen: OrigenPregunta) => `admin-preguntas-origen-${origen}`;

/** P2 (decisión del PO): `DELETE` solo desde estos estados; un borrador o una en revisión se rechazan primero. */
export const ESTADOS_ELIMINABLES: readonly EstadoPregunta[] = ["aprobada", "rechazada", "activo"];

export function puedeEliminar(estado: EstadoPregunta): boolean {
  return ESTADOS_ELIMINABLES.includes(estado);
}

/** P3 (§4.5): editar el contenido de una pregunta publicada la pasa a `en_revision` y deja de servirse. */
export function editarContenidoLaRetira(estado: EstadoPregunta): boolean {
  return estado === "aprobada" || estado === "activo";
}

/** Lo que el §4.5 llama «contenido»: cambiarlo en una publicada la pasa a revisión; la clasificación no. */
export type ContenidoPregunta = Pick<Pregunta, "enunciado" | "tipo"> & {
  respuestas: { texto: string; es_correcta: boolean }[];
  explicacion: string;
  referencia: string;
};

export function contenidoDePregunta(p: Pregunta): ContenidoPregunta {
  return {
    enunciado: p.enunciado,
    tipo: p.tipo,
    respuestas: p.respuestas,
    explicacion: p.retroalimentacion?.explicacion ?? "",
    referencia: p.retroalimentacion?.referencia ?? "",
  };
}

const huellaRespuestas = (c: ContenidoPregunta) =>
  c.respuestas
    .map((r) => `${r.es_correcta ? 1 : 0}${r.texto.trim()}`)
    .sort()
    .join("\n");

/**
 * Solo para avisar antes de guardar: el servidor compara con lo guardado y decide el estado (E3-02 §4.5).
 * Reordenar alternativas no es cambiar el contenido.
 */
export function cambiaElContenido(guardado: ContenidoPregunta, editado: ContenidoPregunta): boolean {
  return (
    guardado.enunciado.trim() !== editado.enunciado.trim() ||
    guardado.tipo !== editado.tipo ||
    guardado.explicacion.trim() !== editado.explicacion.trim() ||
    guardado.referencia.trim() !== editado.referencia.trim() ||
    huellaRespuestas(guardado) !== huellaRespuestas(editado)
  );
}

/** Claves i18n (`dashboard.*`) de los errores con código de E3-02 (§7) que llegan a estas pantallas. */
const CLAVE_ERROR: Record<string, string> = {
  estado_invalido: "serumsIa.avisosPregunta.eliminarAyuda",
  clave_oficial: "serumsIa.errores.clave_oficial",
};

/** Clave i18n del error, si tiene una propia; si no, `undefined` y la pantalla usa su mensaje habitual. */
export function claveErrorPregunta(err: unknown): string | undefined {
  const codigo = apiErrorCode(err);
  return codigo ? CLAVE_ERROR[codigo] : undefined;
}
