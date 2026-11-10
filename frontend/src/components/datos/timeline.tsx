import type { ReactNode } from "react";

import { Punto, type Tono } from "@/components/patrones/metricas";
import { cn } from "@/lib/utils";

export type EventoTimeline = {
  id: string;
  titulo: ReactNode;
  detalle?: ReactNode;
  fecha: string;
  tono?: Tono;
  actual?: boolean;
};

/** Línea de tiempo vertical (historial de intentos, auditoría). Fecha en mono a la derecha; el evento `actual` se resalta. */
export function Timeline({ eventos, className }: { eventos: EventoTimeline[]; className?: string }) {
  return (
    <ol className={cn("border-border/80 relative grid gap-0 border-l pl-5", className)}>
      {eventos.map((e, i) => (
        <li key={e.id} className={cn("relative pb-4", i === eventos.length - 1 && "pb-0")}>
          <span
            className={cn(
              "bg-card ring-card absolute top-1.5 -left-[25px] flex size-[9px] items-center justify-center rounded-full ring-2",
              e.actual && "ring-accent ring-4",
            )}
          >
            <Punto tono={e.tono ?? "neutro"} />
          </span>
          <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-0.5">
            <p className={cn("text-control font-medium", e.actual ? "text-foreground" : "text-foreground/90")}>
              {e.titulo}
            </p>
            <time className="text-etiqueta text-muted-foreground font-mono">{e.fecha}</time>
          </div>
          {e.detalle ? <p className="text-control text-muted-foreground mt-0.5 leading-relaxed">{e.detalle}</p> : null}
        </li>
      ))}
    </ol>
  );
}
