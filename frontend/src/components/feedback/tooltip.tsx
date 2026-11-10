"use client";

import type { ReactNode } from "react";

import { cn } from "@/lib/utils";
import { Tooltip as TooltipPrimitive } from "@base-ui/react/tooltip";

/**
 * Tooltip real (reemplaza al `title` nativo cuando importa la consistencia visual).
 * `<Tooltip texto="Exportar reporte: próximamente"><button …/></Tooltip>` — el hijo debe aceptar ref y props (elementos nativos o componentes con `render`).
 */
export function Tooltip({
  texto,
  children,
  lado = "top",
  className,
}: {
  texto: ReactNode;
  children: React.ReactElement;
  lado?: "top" | "bottom" | "left" | "right";
  className?: string;
}) {
  return (
    <TooltipPrimitive.Root>
      <TooltipPrimitive.Trigger delay={300} render={children} />
      <TooltipPrimitive.Portal>
        <TooltipPrimitive.Positioner side={lado} sideOffset={6} className="isolate z-50">
          <TooltipPrimitive.Popup
            className={cn(
              "bg-foreground text-etiqueta text-background max-w-64 rounded-lg px-2.5 py-1.5 leading-snug shadow-md",
              "origin-(--transform-origin) transition-[opacity,transform] duration-(--duracion-rapida) data-ending-style:scale-95 data-ending-style:opacity-0 data-starting-style:scale-95 data-starting-style:opacity-0",
              className,
            )}
          >
            {texto}
          </TooltipPrimitive.Popup>
        </TooltipPrimitive.Positioner>
      </TooltipPrimitive.Portal>
    </TooltipPrimitive.Root>
  );
}

/** Ponlo una vez en la app para que los tooltips compartan retardo y se abran al instante entre vecinos. */
export const TooltipProvider = TooltipPrimitive.Provider;
