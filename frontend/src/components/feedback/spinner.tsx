import type { SVGProps } from "react";

import { cn } from "@/lib/utils";
import { CircleNotchIcon } from "@phosphor-icons/react/dist/ssr";

const TAMANOS = {
  xs: "size-3",
  sm: "size-3.5",
  default: "size-4",
  lg: "size-5",
  xl: "size-6",
} as const;

/**
 * Indicador de carga inline (icono + `animate-spin`). Hereda el color del texto: envuélvelo en
 * `text-muted-foreground`, `text-primary-foreground`… según dónde viva. Úsalo dentro de `BotonAsync`,
 * celdas en actualización o cualquier hueco donde `ProgresoLineal`/`ProgresoCircular` sean demasiado.
 */
export function Spinner({
  tamano = "default",
  className,
  ...props
}: {
  tamano?: keyof typeof TAMANOS;
  className?: string;
} & Omit<SVGProps<SVGSVGElement>, "className">) {
  return (
    <CircleNotchIcon
      weight="bold"
      role="status"
      aria-label="Cargando"
      className={cn("animate-spin text-current", TAMANOS[tamano], className)}
      {...props}
    />
  );
}
