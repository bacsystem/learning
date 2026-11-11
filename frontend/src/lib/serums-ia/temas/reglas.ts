import type { EspecialidadTema, TemaPayload } from "./api";
import * as yup from "yup";

/** Reglas de validación de T3/T4 (docs/api/serums-ia-e1-taxonomia.md §1). El servidor las vuelve a aplicar. */
export const NOMBRE_MIN = 3;
export const NOMBRE_MAX = 120;
export const DESCRIPCION_MAX = 500;
export const ESPECIALIDADES_MAX = 50;

export interface ValoresTema {
  nombre: string;
  descripcion: string;
  especialidad_ids: number[];
}

/** Como el servidor: quita los espacios de los extremos y colapsa los internos. */
export function normalizarNombre(nombre: string): string {
  return nombre.trim().replace(/\s+/g, " ");
}

/** Longitud en code points (runes), no en unidades UTF-16. */
function longitud(texto: string): number {
  return [...texto].length;
}

export interface MensajesValidacion {
  nombreRequerido: string;
  nombreLongitud: string;
  descripcionLongitud: string;
  especialidadesMax: string;
}

export function esquemaTema(m: MensajesValidacion) {
  return yup.object({
    nombre: yup
      .string()
      .required(m.nombreRequerido)
      .test("longitud", m.nombreLongitud, (v) => {
        const n = longitud(normalizarNombre(v ?? ""));
        return n >= NOMBRE_MIN && n <= NOMBRE_MAX;
      }),
    descripcion: yup.string().test("longitud", m.descripcionLongitud, (v) => longitud(v ?? "") <= DESCRIPCION_MAX),
    especialidad_ids: yup.array(yup.number().required()).max(ESPECIALIDADES_MAX, m.especialidadesMax).required(),
  });
}

/** Valores del formulario → cuerpo de T3/T4: nombre normalizado, descripción vacía como `null` e ids sin repetir. */
export function aPayload(v: ValoresTema): TemaPayload {
  const descripcion = v.descripcion.trim();
  return {
    nombre: normalizarNombre(v.nombre),
    descripcion: descripcion === "" ? null : descripcion,
    especialidad_ids: [...new Set(v.especialidad_ids)],
  };
}

export interface OpcionEspecialidad extends EspecialidadTema {
  inactiva: boolean;
}

/**
 * Casillas de especialidades del formulario: las activas y, detrás, las que el tema ya tiene asociadas pero dejaron de
 * estar activas, marcadas como inactivas para poder distinguirlas y quitarlas.
 */
export function opcionesEspecialidad(
  activas: readonly EspecialidadTema[],
  asociadas: readonly EspecialidadTema[],
): OpcionEspecialidad[] {
  const ids = new Set(activas.map((e) => e.id));
  return [
    ...activas.map((e) => ({ ...e, inactiva: false })),
    ...asociadas.filter((e) => !ids.has(e.id)).map((e) => ({ ...e, inactiva: true })),
  ];
}

export type Direccion = "subir" | "bajar";

/** Devuelve una copia con el elemento de `indice` movido una posición; en los extremos la devuelve igual. */
export function mover<T>(lista: readonly T[], indice: number, direccion: Direccion): T[] {
  const destino = direccion === "subir" ? indice - 1 : indice + 1;
  const copia = [...lista];
  if (indice < 0 || indice >= lista.length || destino < 0 || destino >= lista.length) return copia;
  [copia[indice], copia[destino]] = [copia[destino], copia[indice]];
  return copia;
}
