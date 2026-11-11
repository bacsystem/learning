"use client";

import { useTextosTablaDatos } from "./use-textos-khipu";
import type { ComponentProps } from "react";

import { TablaDatos as TablaDatosKhipu } from "@/components/datos/tabla-datos";

export type { Columna } from "@/components/datos/tabla-datos";

/** `TablaDatos` de khipu con los textos del pie, del paginador y de la selección traducidos con next-intl. */
export function TablaDatos<T>(props: ComponentProps<typeof TablaDatosKhipu<T>>) {
  const textos = useTextosTablaDatos();
  return <TablaDatosKhipu<T> {...props} textos={{ ...textos, ...props.textos }} />;
}
