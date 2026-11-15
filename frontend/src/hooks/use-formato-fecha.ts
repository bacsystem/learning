"use client";

import { useLocale } from "next-intl";
import { useMemo } from "react";

import { comoLocale } from "@/constants";
import { formatearFecha, formatearFechaHora, type LocaleFecha } from "@/lib/formato";

/** Fechas con la receta de khipu en el idioma dado (`es` por defecto): «15 Nov 2026» / «Nov 15, 2026». */
export function formatoFecha(locale: LocaleFecha) {
  return {
    /** ISO `YYYY-MM-DD` a texto. */
    fecha: (iso: string) => formatearFecha(iso, locale),
    /** Instante ISO a fecha y hora de Lima. */
    fechaHora: (iso: string) => formatearFechaHora(iso, locale),
    /** «2026-03» a mes y año: «Mar 2026» / «Mar 2026». */
    mesAnio: (mes: string) => formatearFecha(`${mes}-01`, locale).replace(/^\d+ /, "").replace(/ \d+,/, ""),
    /** Día y mes sin el año: «5 Nov» / «Nov 5». */
    fechaCorta: (iso: string) => formatearFecha(iso, locale).replace(/,? \d{4}$/, ""),
  };
}

/** Las fechas de la pantalla en el idioma activo de la app, con la misma receta en todas. */
export function useFormatoFecha() {
  const locale = comoLocale(useLocale()) ?? "es";
  return useMemo(() => ({ ...formatoFecha(locale), locale }), [locale]);
}
