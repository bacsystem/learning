"use client";

import { textosPieTabla, textosTablaDatos, textosTablaListado } from "./textos-khipu";
import { useTranslations } from "next-intl";
import { useMemo } from "react";

/** Textos del pie de tabla de khipu (`PieTabla`), traducidos con next-intl. */
export function useTextosPieTabla() {
  const t = useTranslations("dashboard");
  return useMemo(() => textosPieTabla(t), [t]);
}

/** Textos de `TablaDatos` (pie y selección), traducidos con next-intl. */
export function useTextosTablaDatos() {
  const t = useTranslations("dashboard");
  return useMemo(() => textosTablaDatos(t), [t]);
}

/** Textos de `TablaListado`: los de su barra y, además, los de la tabla. */
export function useTextosTablaListado() {
  const t = useTranslations("dashboard");
  return useMemo(() => textosTablaListado(t), [t]);
}
