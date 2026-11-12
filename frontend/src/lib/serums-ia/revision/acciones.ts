import type { CambiarEstadoPayload, EstadoPregunta, OrigenPregunta } from "@/lib/questions/api";

/** Acciones de revisión según el estado y el origen (docs/api/serums-ia-e3-07-revision-basica.md §3). El servidor valida igual. */
export const MOTIVO_MIN = 5;
export const MOTIVO_MAX = 500;
export const NOTA_MIN = 10;
export const NOTA_MAX = 500;

export interface AccionesDisponibles {
  /** `borrador` manual → `aprobada`. */
  aprobar: boolean;
  /** `en_revision` (cualquier origen) → `aprobada`. */
  revalidar: boolean;
  rechazar: boolean;
  /** `rechazada` → `borrador`. */
  reabrir: boolean;
  /** Borrador de balotario o IA: se aprueba en su cola, no aquí. */
  seRevisaEnCola: boolean;
}

export function accionesDisponibles(estado: EstadoPregunta, origen?: OrigenPregunta): AccionesDisponibles {
  const borrador = estado === "borrador";
  const enRevision = estado === "en_revision";
  return {
    aprobar: borrador && origen === "manual",
    revalidar: enRevision && origen !== undefined,
    rechazar: borrador || enRevision,
    reabrir: estado === "rechazada",
    seRevisaEnCola: borrador && (origen === "balotario" || origen === "ia"),
  };
}

/** Balotario e IA exigen una nota al revalidar; en una manual es opcional. */
export const notaObligatoria = (origen: OrigenPregunta): boolean => origen === "balotario" || origen === "ia";

const largo = (texto: string) => Array.from(texto.trim()).length;

/** Motivo de rechazo listo para enviar (5–500 caracteres), o `null` si no es válido. */
export function motivoValido(texto: string): string | null {
  const n = largo(texto);
  return n >= MOTIVO_MIN && n <= MOTIVO_MAX ? texto.trim() : null;
}

/**
 * Nota de una aprobación o revalidación. Obligatoria (10–500) para balotario e IA; opcional (≤ 500) en una manual,
 * donde vacía se envía como `null`. `undefined` si no es válida.
 */
export function notaParaAprobar(origen: OrigenPregunta, texto: string): string | null | undefined {
  const n = largo(texto);
  if (n === 0) return notaObligatoria(origen) ? undefined : null;
  if (n > NOTA_MAX || (notaObligatoria(origen) && n < NOTA_MIN)) return undefined;
  return texto.trim();
}

/** Cuerpo de `PATCH …/estado` para aprobar o revalidar, o `null` si falta la casilla o la nota no vale. */
export function cuerpoAprobar(origen: OrigenPregunta, verificado: boolean, nota: string): CambiarEstadoPayload | null {
  const notaFinal = notaParaAprobar(origen, nota);
  if (!verificado || notaFinal === undefined) return null;
  return { estado: "aprobada", verificado: true, nota: notaFinal };
}

export const cuerpoRechazar = (motivo: string): CambiarEstadoPayload | null => {
  const valido = motivoValido(motivo);
  return valido === null ? null : { estado: "rechazada", motivo: valido };
};

export const CUERPO_REABRIR: CambiarEstadoPayload = { estado: "borrador" };
