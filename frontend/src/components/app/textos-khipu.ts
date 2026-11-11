import type { TextosTablaDatos } from "@/components/datos/tabla-datos";
import type { TextosTablaListado } from "@/components/datos/tabla-listado";
import type { TextosPieTabla } from "@/components/patrones/pie-tabla";

/**
 * Textos de los componentes de khipu traducidos con las claves del namespace `dashboard`. Son funciones puras de `t`
 * (el de `useTranslations("dashboard")`) para poder probar el mapeo; los hooks de `use-textos-khipu` las envuelven.
 */
export type Traductor = (clave: string, valores?: Record<string, string | number>) => string;

export function textosPieTabla(t: Traductor): TextosPieTabla {
  return {
    mostrando: t("tabla-mostrando"),
    de: t("tabla-de"),
    anterior: t("tabla-anterior"),
    siguiente: t("tabla-siguiente"),
    filas: t("tabla-filas"),
    filasPorPagina: t("tabla-filas-por-pagina"),
  };
}

export function textosTablaDatos(t: Traductor): TextosTablaDatos {
  return {
    ...textosPieTabla(t),
    accionesSeleccion: t("tabla-acciones-seleccion"),
    quitarSeleccion: t("tabla-quitar-seleccion"),
    seleccionarTodo: t("tabla-seleccionar-todo"),
    seleccionarFila: (id) => t("tabla-seleccionar-fila", { id }),
    // Dos claves y no un plural ICU: `check-i18n` toma las ramas del plural como placeholders.
    seleccionados: (n) => (n === 1 ? t("tabla-seleccionado") : t("tabla-seleccionados")),
  };
}

export function textosTablaListado(t: Traductor): TextosTablaListado & TextosTablaDatos {
  return {
    ...textosTablaDatos(t),
    buscar: t("tabla-buscar"),
    columnas: t("tabla-columnas"),
    columnasVisibles: t("tabla-columnas-visibles"),
    exportar: t("tabla-exportar"),
    exportarCsv: t("tabla-exportar-csv"),
    imprimir: t("tabla-imprimir"),
  };
}
