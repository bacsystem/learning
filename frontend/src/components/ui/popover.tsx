"use client";

import type { ReactNode } from "react";

import { ANIMACION_POPUP, OBJETIVO_TACTIL } from "@/lib/estilos";
import { cn } from "@/lib/utils";
import { Popover as PopoverPrimitive } from "@base-ui/react/popover";
import { XIcon } from "@phosphor-icons/react";

const Popover = PopoverPrimitive.Root;
const PopoverTrigger = PopoverPrimitive.Trigger;
const PopoverClose = PopoverPrimitive.Close;

/**
 * Popup flotante para filtros avanzados, ayuda contextual con acciones o cualquier contenido rico que no
 * quepa en un `Tooltip` (ese es solo texto). Misma familia visual que `SelectContent`/`ComboboxPopup`.
 * `<Popover><PopoverTrigger render={<button/>}>Filtros</PopoverTrigger><PopoverContent>…</PopoverContent></Popover>`
 */
function PopoverContent({
  children,
  side = "bottom",
  sideOffset = 8,
  align = "start",
  alignOffset = 0,
  className,
  ...props
}: PopoverPrimitive.Popup.Props &
  Pick<PopoverPrimitive.Positioner.Props, "side" | "sideOffset" | "align" | "alignOffset">) {
  return (
    <PopoverPrimitive.Portal>
      <PopoverPrimitive.Positioner
        side={side}
        sideOffset={sideOffset}
        align={align}
        alignOffset={alignOffset}
        className="isolate z-50"
      >
        <PopoverPrimitive.Popup
          data-slot="popover-content"
          className={cn(
            "bg-popover text-popover-foreground ring-border w-72 rounded-xl p-3.5 shadow-md ring-1 outline-none",
            ANIMACION_POPUP,
            className,
          )}
          {...props}
        >
          {children}
        </PopoverPrimitive.Popup>
      </PopoverPrimitive.Positioner>
    </PopoverPrimitive.Portal>
  );
}

/** Cabecera opcional del popover: título + descripción a la izquierda, cerrar a la derecha. */
function PopoverHeader({
  titulo,
  descripcion,
  conCierre = true,
  className,
}: {
  titulo: ReactNode;
  descripcion?: ReactNode;
  conCierre?: boolean;
  className?: string;
}) {
  return (
    <div className={cn("mb-2.5 flex items-start justify-between gap-2", className)}>
      <div className="min-w-0">
        <PopoverPrimitive.Title className="text-cuerpo text-foreground font-semibold">{titulo}</PopoverPrimitive.Title>
        {descripcion ? (
          <PopoverPrimitive.Description className="text-control text-muted-foreground mt-0.5">
            {descripcion}
          </PopoverPrimitive.Description>
        ) : null}
      </div>
      {conCierre ? (
        <PopoverClose
          aria-label="Cerrar"
          className={cn(
            OBJETIVO_TACTIL,
            "text-muted-foreground/70 hover:bg-secondary hover:text-foreground shrink-0 rounded p-0.5 transition-colors",
          )}
        >
          <XIcon weight="bold" className="size-3.5" />
        </PopoverClose>
      ) : null}
    </div>
  );
}

export { Popover, PopoverTrigger, PopoverContent, PopoverHeader, PopoverClose };
