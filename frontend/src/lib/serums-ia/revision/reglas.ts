import type { CitaPregunta } from "@/lib/questions/api";

/** Reglas de las citas de una pregunta (docs/api/serums-ia-e3-07-revision-basica.md §2 y §4). El servidor valida igual. */
export const MAX_CITAS = 20;
export const CONSULTA_MIN = 3;
export const CONSULTA_MAX = 300;

/** Consulta lista para enviar: sin espacios en los extremos y de 3 a 300 caracteres; si no, `null`. */
export function consultaValida(texto: string): string | null {
  const consulta = texto.trim();
  const largo = Array.from(consulta).length;
  return largo >= CONSULTA_MIN && largo <= CONSULTA_MAX ? consulta : null;
}

export const idsDeCitas = (citas: readonly CitaPregunta[]): number[] => citas.map((c) => c.pasaje_id);

export const estaCitado = (citas: readonly CitaPregunta[], pasajeId: number): boolean =>
  citas.some((c) => c.pasaje_id === pasajeId);

export const citasLlenas = (citas: readonly CitaPregunta[]): boolean => citas.length >= MAX_CITAS;

/** Lista completa que se envía al añadir una cita (el `PUT` reemplaza el conjunto). */
export const idsTrasAnadir = (citas: readonly CitaPregunta[], pasajeId: number): number[] => [
  ...idsDeCitas(citas),
  pasajeId,
];

/** Lista completa que se envía al quitar una cita. */
export const idsTrasQuitar = (citas: readonly CitaPregunta[], pasajeId: number): number[] =>
  idsDeCitas(citas).filter((id) => id !== pasajeId);

export const fuenteRetirada = (cita: Pick<CitaPregunta, "fuente_estado">): boolean => cita.fuente_estado === "retirada";
