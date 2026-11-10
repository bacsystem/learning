import type { ReactNode } from "react";

import type { IconoComponente } from "@/lib/iconos";
import { cn } from "@/lib/utils";

/**
 * Estado vacío de página o de sección: icono en caja accent, título, explicación y acción principal.
 * Para el cuerpo de una tabla usa `TablaVacia` (patrones/pie-tabla).
 */
export function EstadoVacio({
  icon: Icon,
  titulo,
  children,
  accion,
  className,
}: {
  icon: IconoComponente;
  titulo: ReactNode;
  children?: ReactNode;
  accion?: ReactNode;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "border-border bg-card flex flex-col items-center justify-center gap-3 rounded-xl border border-dashed px-6 py-14 text-center",
        className,
      )}
    >
      <div className="bg-accent text-primary flex size-10 items-center justify-center rounded-lg">
        <Icon className="size-5" />
      </div>
      <div className="max-w-sm">
        <h3 className="text-cuerpo text-foreground font-semibold">{titulo}</h3>
        {children ? <p className="text-control text-muted-foreground mt-1 leading-relaxed">{children}</p> : null}
      </div>
      {accion ? <div className="mt-1">{accion}</div> : null}
    </div>
  );
}
