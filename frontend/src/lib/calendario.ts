/**
 * Aritmética de fechas para el calendario de `EntradaFecha`. Todo es ISO `YYYY-MM-DD` en hora local: una fecha de
 * calendario no tiene zona horaria, y construirla con `new Date("2026-09-01")` (UTC) la corría un día en Lima.
 * Sin dependencias: lo usan los componentes y los tests de node.
 */

export type DiaCalendario = { iso: string; dia: number; delMes: boolean };

/** Nombres de mes del calendario (en Perú se escribe «Setiembre»). */
const MESES = [
  "Enero",
  "Febrero",
  "Marzo",
  "Abril",
  "Mayo",
  "Junio",
  "Julio",
  "Agosto",
  "Setiembre",
  "Octubre",
  "Noviembre",
  "Diciembre",
];

/** Encabezados de columna, semana de lunes a domingo. */
export const DIAS_SEMANA = [
  { corto: "L", largo: "lunes" },
  { corto: "M", largo: "martes" },
  { corto: "M", largo: "miércoles" },
  { corto: "J", largo: "jueves" },
  { corto: "V", largo: "viernes" },
  { corto: "S", largo: "sábado" },
  { corto: "D", largo: "domingo" },
];

const dosCifras = (n: number) => String(n).padStart(2, "0");

/** `Date` local (mediodía, lejos de cualquier cambio de horario) a ISO `YYYY-MM-DD`. */
export function aIso(fecha: Date): string {
  return `${fecha.getFullYear()}-${dosCifras(fecha.getMonth() + 1)}-${dosCifras(fecha.getDate())}`;
}

/** ISO `YYYY-MM-DD` a `Date` local al mediodía, o `null` si no es una fecha válida. */
export function desdeIso(iso: string | undefined): Date | null {
  const m = iso?.match(/^(\d{4})-(\d{2})-(\d{2})$/);
  if (!m) return null;
  const fecha = new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3]), 12);
  return fecha.getMonth() === Number(m[2]) - 1 ? fecha : null;
}

export function hoyIso(): string {
  return aIso(new Date());
}

export function sumarDias(iso: string, dias: number): string {
  const fecha = desdeIso(iso) ?? new Date();
  fecha.setDate(fecha.getDate() + dias);
  return aIso(fecha);
}

/** Suma meses; si el día no existe en el mes de destino (31 → febrero) queda en el último día. */
export function sumarMeses(iso: string, meses: number): string {
  const fecha = desdeIso(iso) ?? new Date();
  const dia = fecha.getDate();
  fecha.setDate(1);
  fecha.setMonth(fecha.getMonth() + meses);
  const ultimo = new Date(fecha.getFullYear(), fecha.getMonth() + 1, 0).getDate();
  fecha.setDate(Math.min(dia, ultimo));
  return aIso(fecha);
}

/** Si `iso` está entre `min` y `max` (inclusive). Las ISO se comparan como texto. */
export function dentroDeRango(iso: string, min?: string, max?: string): boolean {
  return (!min || iso >= min) && (!max || iso <= max);
}

/** «Setiembre 2026». `mes` empieza en 0. */
export function nombreMes(anio: number, mes: number): string {
  return `${MESES[mes]} ${anio}`;
}

/** Seis semanas (lunes a domingo) que contienen el mes; los días de los meses vecinos van con `delMes: false`. */
export function semanasDelMes(anio: number, mes: number): DiaCalendario[][] {
  const primero = new Date(anio, mes, 1, 12);
  const desfase = (primero.getDay() + 6) % 7; // lunes = 0
  const inicio = new Date(anio, mes, 1 - desfase, 12);
  const semanas: DiaCalendario[][] = [];
  for (let s = 0; s < 6; s++) {
    const semana: DiaCalendario[] = [];
    for (let d = 0; d < 7; d++) {
      const fecha = new Date(inicio.getFullYear(), inicio.getMonth(), inicio.getDate() + s * 7 + d, 12);
      semana.push({ iso: aIso(fecha), dia: fecha.getDate(), delMes: fecha.getMonth() === mes });
    }
    semanas.push(semana);
  }
  return semanas;
}
