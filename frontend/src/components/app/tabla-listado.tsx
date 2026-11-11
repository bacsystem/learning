"use client";

import { useTextosTablaListado } from "./textos-khipu";
import { useTranslations } from "next-intl";
import type { ComponentProps } from "react";

import { TablaListado as TablaListadoKhipu } from "@/components/datos/tabla-listado";
import { useToast } from "@/components/feedback/toast";

export type { ColumnaListado } from "@/components/datos/tabla-listado";

/**
 * `TablaListado` de khipu con todos sus textos traducidos con next-intl (barra, pie, paginador y selección) y un aviso si
 * se bloquea la impresión.
 */
export function TablaListado<T>(props: ComponentProps<typeof TablaListadoKhipu<T>>) {
  const t = useTranslations("dashboard");
  const toast = useToast();
  const textos = useTextosTablaListado();
  return (
    <TablaListadoKhipu<T>
      {...props}
      textos={{ ...textos, ...props.textos }}
      onImpresionBloqueada={props.onImpresionBloqueada ?? (() => toast.error(t("tabla-impresion-bloqueada")))}
    />
  );
}
