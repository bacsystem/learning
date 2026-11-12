"use client";

import type { ReactNode } from "react";

import { IconoSeccion } from "@/components/patrones/cabecera-seccion";
import { Sheet, SheetClose, SheetContent, SheetDescription, SheetTitle, SheetTrigger } from "@/components/ui/sheet";
import { OBJETIVO_TACTIL } from "@/lib/estilos";
import type { IconoComponente } from "@/lib/iconos";
import { cn } from "@/lib/utils";
import { XIcon } from "@phosphor-icons/react";

/**
 * Panel lateral de detalle (derecha): cabecera con icono y título, cuerpo con scroll y pie de acciones.
 * Úsalo para ver un registro sin salir de la tabla; para crear/editar prefiere un diálogo.
 */
export function PanelLateral({
  trigger,
  open,
  onOpenChange,
  icon,
  titulo,
  descripcion,
  children,
  pie,
  ancho = "sm:max-w-lg",
  etiquetaCerrar = "Cerrar",
}: {
  trigger?: ReactNode;
  open?: boolean;
  onOpenChange?: (v: boolean) => void;
  icon: IconoComponente;
  titulo: ReactNode;
  descripcion?: ReactNode;
  children: ReactNode;
  pie?: ReactNode;
  ancho?: string;
  /** Nombre del botón de cerrar para el lector de pantalla, para traducirlo (por defecto «Cerrar»). */
  etiquetaCerrar?: string;
}) {
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      {trigger ? <SheetTrigger render={<span />}>{trigger}</SheetTrigger> : null}
      {/* En un teléfono el 75 % del cajón dejaba ~280 px y el contenido no entraba: el panel ocupa el ancho menos los márgenes; desde `sm`, su `ancho`. */}
      <SheetContent
        side="right"
        showCloseButton={false}
        className={cn("w-full gap-0 p-0 data-[side=right]:w-[calc(100%-1.5rem)]", ancho)}
      >
        <div className="flex items-start gap-2.5 px-5 pt-5 pb-3">
          <IconoSeccion icon={icon} />
          <div className="min-w-0 flex-1">
            <SheetTitle className="text-titulo text-foreground">{titulo}</SheetTitle>
            {descripcion ? (
              <SheetDescription className="text-control text-muted-foreground">{descripcion}</SheetDescription>
            ) : null}
          </div>
          <SheetClose
            aria-label={etiquetaCerrar}
            className={cn(
              OBJETIVO_TACTIL,
              "text-texto-terciario hover:bg-secondary hover:text-foreground -mt-1 -mr-1 grid size-8 place-items-center rounded-full transition-colors",
            )}
          >
            <XIcon weight="bold" className="size-4" />
          </SheetClose>
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto px-5 py-2">{children}</div>
        {pie ? <div className="flex items-center justify-end gap-2 px-5 pt-3 pb-5">{pie}</div> : null}
      </SheetContent>
    </Sheet>
  );
}
