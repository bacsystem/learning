"use client";

import { useTranslations } from "next-intl";
import type { ComponentProps } from "react";

import { TablaListado as TablaListadoKhipu } from "@/components/datos/tabla-listado";
import { useToast } from "@/components/feedback/toast";

export type { ColumnaListado } from "@/components/datos/tabla-listado";

/** `TablaListado` de khipu con los textos de su barra traducidos con next-intl y un aviso si se bloquea la impresión. */
export function TablaListado<T>(props: ComponentProps<typeof TablaListadoKhipu<T>>) {
  const t = useTranslations("dashboard");
  const toast = useToast();
  return (
    <TablaListadoKhipu<T>
      {...props}
      textos={{
        buscar: t("tabla-buscar"),
        columnas: t("tabla-columnas"),
        columnasVisibles: t("tabla-columnas-visibles"),
        exportar: t("tabla-exportar"),
        exportarCsv: t("tabla-exportar-csv"),
        imprimir: t("tabla-imprimir"),
        ...props.textos,
      }}
      onImpresionBloqueada={props.onImpresionBloqueada ?? (() => toast.error(t("tabla-impresion-bloqueada")))}
    />
  );
}
