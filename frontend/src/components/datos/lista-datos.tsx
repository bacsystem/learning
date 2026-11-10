import type { ReactNode } from "react";

import { cn } from "@/lib/utils";

export type DatoLista = { etiqueta: ReactNode; valor: ReactNode; mono?: boolean };

/** Lista clave/valor (ficha de un registro): etiqueta a la izquierda, valor a la derecha; `columnas=2` para fichas anchas. */
export function ListaDatos({
  datos,
  columnas = 1,
  className,
}: {
  datos: DatoLista[];
  columnas?: 1 | 2;
  className?: string;
}) {
  return (
    <dl className={cn("grid gap-x-8 gap-y-3", columnas === 2 && "sm:grid-cols-2", className)}>
      {datos.map((d, i) => (
        <div
          key={i}
          className="border-border/60 grid min-w-0 gap-0.5 border-b pb-2 last:border-0 sm:grid-cols-[minmax(120px,40%)_1fr] sm:items-baseline sm:gap-x-4 sm:border-0 sm:pb-0"
        >
          <dt className="text-etiqueta text-muted-foreground font-medium">{d.etiqueta}</dt>
          <dd className={cn("text-control text-foreground min-w-0", d.mono && "font-mono tabular-nums")}>{d.valor}</dd>
        </div>
      ))}
    </dl>
  );
}
