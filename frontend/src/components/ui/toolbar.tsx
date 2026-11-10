"use client";

import { cn } from "@/lib/utils";
import { Toolbar as ToolbarPrimitive } from "@base-ui/react/toolbar";

/**
 * Barra de acciones agrupadas (edición de tabla, cabecera de editor) con navegación por flechas entre
 * los controles — a diferencia de un `<div className="flex">` de botones sueltos, aquí el foco se mueve
 * con el teclado como en una sola fila de widgets.
 */
function Toolbar({ className, orientation = "horizontal", ...props }: ToolbarPrimitive.Root.Props) {
  return (
    <ToolbarPrimitive.Root
      data-slot="toolbar"
      orientation={orientation}
      className={cn(
        "h-control border-border/60 bg-secondary/80 flex items-center gap-1 rounded-lg border p-0.5",
        orientation === "vertical" && "h-fit w-9 flex-col",
        className,
      )}
      {...props}
    />
  );
}

function ToolbarGroup({ className, ...props }: ToolbarPrimitive.Group.Props) {
  return (
    <ToolbarPrimitive.Group data-slot="toolbar-group" className={cn("flex items-center gap-1", className)} {...props} />
  );
}

function ToolbarButton({ className, ...props }: ToolbarPrimitive.Button.Props) {
  return (
    <ToolbarPrimitive.Button
      data-slot="toolbar-button"
      className={cn(
        "text-muted-foreground hover:bg-card hover:text-foreground focus-visible:ring-ring/50 aria-pressed:bg-card aria-pressed:text-foreground inline-flex size-7 items-center justify-center gap-1.5 rounded-md transition-colors outline-none focus-visible:ring-2 disabled:pointer-events-none disabled:opacity-50 aria-pressed:shadow-xs [&_svg]:size-3.5 [&_svg]:shrink-0",
        className,
      )}
      {...props}
    />
  );
}

function ToolbarLink({ className, ...props }: ToolbarPrimitive.Link.Props) {
  return (
    <ToolbarPrimitive.Link
      data-slot="toolbar-link"
      className={cn(
        "text-control text-muted-foreground hover:bg-card hover:text-foreground focus-visible:ring-ring/50 inline-flex h-7 items-center rounded-md px-2 font-medium outline-none focus-visible:ring-2",
        className,
      )}
      {...props}
    />
  );
}

function ToolbarSeparator({ className, ...props }: ToolbarPrimitive.Separator.Props) {
  return (
    <ToolbarPrimitive.Separator
      data-slot="toolbar-separator"
      className={cn("bg-border mx-0.5 h-5 w-px", className)}
      {...props}
    />
  );
}

function ToolbarInput({ className, ...props }: ToolbarPrimitive.Input.Props) {
  return (
    <ToolbarPrimitive.Input
      data-slot="toolbar-input"
      className={cn(
        "bg-card text-control text-foreground placeholder:text-muted-foreground/70 focus-visible:ring-ring/50 h-7 w-24 rounded-md border-0 px-2 outline-none focus-visible:ring-2",
        className,
      )}
      {...props}
    />
  );
}

export { Toolbar, ToolbarGroup, ToolbarButton, ToolbarLink, ToolbarSeparator, ToolbarInput };
