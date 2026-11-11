"use client";

import { useTextosPieTabla } from "./use-textos-khipu";
import type { ComponentProps } from "react";

import { PieTabla as PieTablaKhipu } from "@/components/patrones/pie-tabla";

/** `PieTabla` de khipu (listados paginados en el servidor) con sus textos y los del selector de filas traducidos. */
export function PieTabla(props: ComponentProps<typeof PieTablaKhipu>) {
  const textos = useTextosPieTabla();
  return <PieTablaKhipu {...props} textos={{ ...textos, ...props.textos }} />;
}
