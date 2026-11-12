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
export const claveEstadoPregunta = (estado: EstadoPregunta) => `serumsIa.estadosPregunta.${estado}`;
export const claveOrigenPregunta = (origen: OrigenPregunta) => `serumsIa.revision.listado.origenes.${origen}`;

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
    .map((r) => `${r.es_correcta ? 1 : 0}${r.texto}`)
    .sort()
    .join("\n");

/**
 * Solo para avisar antes de guardar: el servidor compara con lo guardado y decide el estado (E3-02 §4.5).
 * Compara exacto, sin recortar, igual que el servidor y como se envía el formulario. Reordenar alternativas no es
 * cambiar el contenido.
 */
export function cambiaElContenido(guardado: ContenidoPregunta, editado: ContenidoPregunta): boolean {
  return (
    guardado.enunciado !== editado.enunciado ||
    guardado.tipo !== editado.tipo ||
    guardado.explicacion !== editado.explicacion ||
    guardado.referencia !== editado.referencia ||
    huellaRespuestas(guardado) !== huellaRespuestas(editado)
  );
}

/** Un mapa por operación: `estado_invalido` significa otra cosa al eliminar que al citar o cambiar de estado. */
const CLAVES_EDICION: Record<string, string> = { clave_oficial: "serumsIa.errores.clave_oficial" };
const CLAVES_ELIMINACION: Record<string, string> = { estado_invalido: "serumsIa.preguntas.errores.estado_invalido" };

/** Un código sin traducción propia no muestra el `message` del servidor (siempre en español): usa este texto neutro. */
const CLAVE_ERROR_GENERICO = "serumsIa.errores.generico";

/** Con código: su clave o la genérica. Sin código (formato heredado de la API): `undefined`, y manda el `message`. */
const claveDe = (claves: Record<string, string>, err: unknown): string | undefined => {
  const codigo = apiErrorCode(err);
  if (!codigo) return undefined;
  return Object.hasOwn(claves, codigo) ? claves[codigo] : CLAVE_ERROR_GENERICO;
};

/** Clave i18n del error de `PUT /preguntas/{id}`, la genérica si el código no tiene traducción, o `undefined` si el error no trae código. */
export const claveErrorEdicion = (err: unknown) => claveDe(CLAVES_EDICION, err);

/** Clave i18n del error de `DELETE /preguntas/{id}`, la genérica si el código no tiene traducción, o `undefined` si el error no trae código. */
export const claveErrorEliminacion = (err: unknown) => claveDe(CLAVES_ELIMINACION, err);
