"use client";

import { cn } from "@/lib/utils";
import { Separator as SeparatorPrimitive } from "@base-ui/react/separator";

/**
 * Línea divisoria entre bloques o dentro de una fila (menú, toolbar, cabecera). `orientacion="vertical"`
 * reemplaza al separador que hasta ahora se armaba a mano en sitios como `TopBar` (`h-4 w-px bg-border`);
 * en vertical necesita una altura explícita en `className` (el separador no tiene tamaño propio en ese eje).
 */
export function Separator({
  orientacion = "horizontal",
  className,
}: {
  orientacion?: "horizontal" | "vertical";
  className?: string;
}) {
  return (
    <SeparatorPrimitive
      orientation={orientacion}
      className={cn("bg-border shrink-0", orientacion === "horizontal" ? "h-px w-full" : "w-px", className)}
    />
  );
}
