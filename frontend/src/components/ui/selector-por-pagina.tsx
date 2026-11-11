"use client";

import { OPCIONES_POR_PAGINA } from "@/lib/paginacion";
import { cn } from "@/lib/utils";

/** `etiqueta` es el nombre del grupo para el lector de pantalla y `textoFilas` el rótulo visible; por defecto en castellano. */
export function SelectorPorPagina({
  valor,
  onCambio,
  etiqueta = "Filas por página",
  textoFilas = "Filas",
}: {
  valor: number;
  onCambio: (n: number) => void;
  etiqueta?: string;
  textoFilas?: string;
}) {
  return (
    <div
      className="text-etiqueta text-muted-foreground inline-flex items-center gap-2"
      role="group"
      aria-label={etiqueta}
    >
      <span>{textoFilas}</span>
      <div className="h-control border-border bg-card inline-flex items-center rounded-lg border p-0.5 shadow-xs">
        {OPCIONES_POR_PAGINA.map((n) => (
          <button
            key={n}
            type="button"
            aria-pressed={valor === n}
            onClick={() => onCambio(n)}
            className={cn(
              "text-etiqueta flex h-7 min-w-8 items-center justify-center rounded-md px-1.5 font-mono font-medium transition-colors",
              valor === n
                ? "bg-foreground text-background shadow-xs"
                : "text-foreground/70 hover:bg-secondary hover:text-foreground",
            )}
          >
            {n}
          </button>
        ))}
      </div>
    </div>
  );
}
