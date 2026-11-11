import type { AreaPeso, AreaPesoPayload } from "./api";
import * as yup from "yup";

/** Reglas de PE3 (docs/api/serums-ia-e1-taxonomia.md §2) y de `catalogs/dto.go`. El servidor las vuelve a aplicar. */
export const AREAS_MAX = 50;
export const PREGUNTAS_AREA_MIN = 1;
export const PREGUNTAS_AREA_MAX = 200;
export const PREGUNTAS_TOTAL_MAX = 200;
export const PESO_MIN = 0.01;
export const PESO_MAX = 100;
export const COEFICIENTE_MIN = 0.01;
export const COEFICIENTE_MAX = 999.99;
const COEFICIENTE_DEFECTO = 1;
/** Σ pesos = 100 ± 0,01 (`pesoValido` del backend), en centésimos para no depender de la coma flotante. */
const SUMA_OBJETIVO_CENTESIMOS = 10000;
const TOLERANCIA_CENTESIMOS = 1;

/** Una fila del formulario: los números van como texto (lo que escribe el admin) y se convierten aquí. */
export interface FilaPeso {
  asignatura_id: number;
  nombre: string;
  peso_porcentual: string;
  numero_preguntas: string;
  coeficiente: string;
}

export interface ValoresPesos {
  areas: FilaPeso[];
}

/** Texto → número, aceptando coma decimal. `null` si está vacío o no es un número. */
export function aNumero(texto: string): number | null {
  const limpio = texto.trim().replace(",", ".");
  if (limpio === "") return null;
  const n = Number(limpio);
  return Number.isFinite(n) ? n : null;
}

function centesimos(n: number): number {
  return Math.round(n * 100);
}

function conDosDecimales(n: number): boolean {
  return Math.abs(n * 100 - Math.round(n * 100)) < 1e-6;
}

export interface Totales {
  /** Suma de pesos en %, redondeada a centésimos. */
  sumaPesos: number;
  sumaPreguntas: number;
  sumaValida: boolean;
  preguntasValidas: boolean;
  /** Ids de asignatura repetidos. */
  repetidas: number[];
}

/** Lo que `totales` lee de una fila (también la forma parcial que da yup al validar). */
type FilaParaTotales = Pick<FilaPeso, "asignatura_id"> &
  Partial<Pick<FilaPeso, "peso_porcentual" | "numero_preguntas">>;

/** Totales en vivo de la tabla (lo que se ve debajo y lo que habilita «Guardar»). Los vacíos cuentan como 0. */
export function totales(areas: readonly FilaParaTotales[]): Totales {
  const sumaCentesimos = areas.reduce((s, a) => s + centesimos(aNumero(a.peso_porcentual ?? "") ?? 0), 0);
  const sumaPreguntas = areas.reduce((s, a) => s + (aNumero(a.numero_preguntas ?? "") ?? 0), 0);
  const vistas = new Set<number>();
  const repetidas = new Set<number>();
  for (const a of areas) (vistas.has(a.asignatura_id) ? repetidas : vistas).add(a.asignatura_id);
  return {
    sumaPesos: sumaCentesimos / 100,
    sumaPreguntas,
    sumaValida: Math.abs(sumaCentesimos - SUMA_OBJETIVO_CENTESIMOS) <= TOLERANCIA_CENTESIMOS,
    preguntasValidas: sumaPreguntas <= PREGUNTAS_TOTAL_MAX,
    repetidas: [...repetidas],
  };
}

export interface MensajesValidacion {
  areasMin: string;
  areasMax: string;
  peso: string;
  preguntas: string;
  coeficiente: string;
  repetidas: string;
  limitePreguntas: string;
  sumaPesos: string;
}

function numeroEnRango(min: number, max: number, { entero = false, dosDecimales = false } = {}) {
  return (v: string | undefined) => {
    const n = aNumero(v ?? "");
    if (n === null || n < min || n > max) return false;
    if (entero && !Number.isInteger(n)) return false;
    return !dosDecimales || conDosDecimales(n);
  };
}

/** Por fila (errores al salir del campo) y del conjunto (repetidas, total ≤ 200 y suma = 100), en el orden de PE3. */
export function esquemaPesos(m: MensajesValidacion) {
  const fila = yup.object({
    asignatura_id: yup.number().required(),
    nombre: yup.string().required(),
    peso_porcentual: yup.string().test("peso", m.peso, numeroEnRango(PESO_MIN, PESO_MAX, { dosDecimales: true })),
    numero_preguntas: yup
      .string()
      .test("preguntas", m.preguntas, numeroEnRango(PREGUNTAS_AREA_MIN, PREGUNTAS_AREA_MAX, { entero: true })),
    coeficiente: yup
      .string()
      .test("coeficiente", m.coeficiente, numeroEnRango(COEFICIENTE_MIN, COEFICIENTE_MAX, { dosDecimales: true })),
  });
  return yup.object({
    areas: yup
      .array(fila.required())
      .required()
      .min(1, m.areasMin)
      .max(AREAS_MAX, m.areasMax)
      .test("repetidas", m.repetidas, (areas) => totales(areas ?? []).repetidas.length === 0)
      .test("limite", m.limitePreguntas, (areas) => totales(areas ?? []).preguntasValidas)
      .test("suma", m.sumaPesos, (areas) => totales(areas ?? []).sumaValida),
  });
}

/** Área del servidor → fila editable. */
export function aFila(
  a: Pick<AreaPeso, "asignatura_id" | "nombre" | "peso_porcentual" | "numero_preguntas" | "coeficiente">,
): FilaPeso {
  return {
    asignatura_id: a.asignatura_id,
    nombre: a.nombre,
    peso_porcentual: String(a.peso_porcentual),
    numero_preguntas: String(a.numero_preguntas),
    coeficiente: String(a.coeficiente),
  };
}

/** Fila nueva al añadir un área: sin peso ni preguntas, coeficiente 1. */
export function filaNueva(asignaturaId: number, nombre: string): FilaPeso {
  return {
    asignatura_id: asignaturaId,
    nombre,
    peso_porcentual: "",
    numero_preguntas: "",
    coeficiente: String(COEFICIENTE_DEFECTO),
  };
}

/** Filas ya validadas → cuerpo de PE3. */
export function aPayload(areas: readonly FilaPeso[]): AreaPesoPayload[] {
  return areas.map((a) => ({
    asignatura_id: a.asignatura_id,
    peso_porcentual: aNumero(a.peso_porcentual) ?? 0,
    numero_preguntas: aNumero(a.numero_preguntas) ?? 0,
    coeficiente: aNumero(a.coeficiente) ?? 0,
  }));
}
