"use client";

import type { ReactNode } from "react";

import { cn } from "@/lib/utils";
import { ScrollArea as ScrollAreaPrimitive } from "@base-ui/react/scroll-area";

const BARRA = "flex touch-none p-0.5 select-none";

/**
 * Contenedor con scroll y scrollbar propios del kit, en vez del nativo del navegador — para listas largas
 * dentro de popups, `PanelLateral` o el propio sidebar. `alto` es obligatorio: sin una altura acotada no hay
 * nada que hacer scroll.
 */
export function ScrollArea({
  children,
  alto,
  orientacion = "vertical",
  className,
}: {
  children: ReactNode;
  alto: string;
  orientacion?: "vertical" | "horizontal" | "ambas";
  className?: string;
}) {
  return (
    <ScrollAreaPrimitive.Root className={cn("overflow-hidden", className)} style={{ height: alto }}>
      <ScrollAreaPrimitive.Viewport className="size-full overscroll-contain outline-none">
        {children}
      </ScrollAreaPrimitive.Viewport>
      {orientacion !== "horizontal" ? (
        <ScrollAreaPrimitive.Scrollbar orientation="vertical" className={cn(BARRA, "h-full w-2.5")}>
          <ScrollAreaPrimitive.Thumb className="bg-border hover:bg-muted-foreground/40 relative flex-1 rounded-full transition-colors" />
        </ScrollAreaPrimitive.Scrollbar>
      ) : null}
      {orientacion !== "vertical" ? (
        <ScrollAreaPrimitive.Scrollbar orientation="horizontal" className={cn(BARRA, "h-2.5 w-full")}>
          <ScrollAreaPrimitive.Thumb className="bg-border hover:bg-muted-foreground/40 relative flex-1 rounded-full transition-colors" />
        </ScrollAreaPrimitive.Scrollbar>
      ) : null}
      {orientacion === "ambas" ? <ScrollAreaPrimitive.Corner className="bg-transparent" /> : null}
    </ScrollAreaPrimitive.Root>
  );
}
