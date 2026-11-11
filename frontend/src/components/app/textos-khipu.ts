"use client";

import { useTranslations } from "next-intl";
import { useMemo } from "react";

import type { TextosTablaDatos } from "@/components/datos/tabla-datos";
import type { TextosTablaListado } from "@/components/datos/tabla-listado";
import type { TextosPieTabla } from "@/components/patrones/pie-tabla";

/** Textos del pie de tabla de khipu (`PieTabla`), traducidos con next-intl. */
export function useTextosPieTabla(): TextosPieTabla {
  const t = useTranslations("dashboard");
  return useMemo(
    () => ({
      mostrando: t("tabla-mostrando"),
      de: t("tabla-de"),
      anterior: t("tabla-anterior"),
      siguiente: t("tabla-siguiente"),
      filas: t("tabla-filas"),
      filasPorPagina: t("tabla-filas-por-pagina"),
    }),
    [t],
  );
}

/** Textos de `TablaDatos` (pie y selección), traducidos con next-intl. */
export function useTextosTablaDatos(): TextosTablaDatos {
  const t = useTranslations("dashboard");
  const pie = useTextosPieTabla();
  return useMemo(
    () => ({
      ...pie,
      accionesSeleccion: t("tabla-acciones-seleccion"),
      quitarSeleccion: t("tabla-quitar-seleccion"),
      seleccionarTodo: t("tabla-seleccionar-todo"),
      seleccionarFila: (id: string) => t("tabla-seleccionar-fila", { id }),
      seleccionados: (n: number) => (n === 1 ? t("tabla-seleccionado") : t("tabla-seleccionados")),
    }),
    [t, pie],
  );
}

/** Textos de `TablaListado`: los de su barra y, además, los de la tabla. */
export function useTextosTablaListado(): TextosTablaListado & TextosTablaDatos {
  const t = useTranslations("dashboard");
  const tabla = useTextosTablaDatos();
  return useMemo(
    () => ({
      ...tabla,
      buscar: t("tabla-buscar"),
      columnas: t("tabla-columnas"),
      columnasVisibles: t("tabla-columnas-visibles"),
      exportar: t("tabla-exportar"),
      exportarCsv: t("tabla-exportar-csv"),
      imprimir: t("tabla-imprimir"),
    }),
    [t, tabla],
  );
}
