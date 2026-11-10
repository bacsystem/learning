"use client";

import { useId } from "react";

import { EntradaFecha } from "@/components/formularios/entrada-fecha";
import { isoHoy, sumarDias } from "@/lib/formato";
import { cn } from "@/lib/utils";

export type RangoFechas = { desde?: string; hasta?: string };

const PRESETS: { etiqueta: string; rango: () => RangoFechas }[] = [
  { etiqueta: "Hoy", rango: () => ({ desde: isoHoy(), hasta: isoHoy() }) },
  { etiqueta: "7 días", rango: () => ({ desde: sumarDias(isoHoy(), -6), hasta: isoHoy() }) },
  { etiqueta: "30 días", rango: () => ({ desde: sumarDias(isoHoy(), -29), hasta: isoHoy() }) },
  { etiqueta: "Este mes", rango: () => ({ desde: `${isoHoy().slice(0, 8)}01`, hasta: isoHoy() }) },
];

/**
 * Rango de fechas para filtros de listados: dos `EntradaFecha` (desde/hasta, cada una acota a la otra con
 * `min`/`max`) más atajos comunes (Hoy, 7/30 días, este mes). `onCambio` recibe el rango completo en cada cambio.
 * `id` es opcional: sin él se genera uno propio con `useId()`, así que dos instancias en la misma página nunca
 * chocan (antes los dos `EntradaFecha` internos usaban ids fijos — "rango-desde"/"rango-hasta" — que se
 * duplicaban en el DOM apenas se renderizaba un segundo `EntradaRangoFechas`).
 */
export function EntradaRangoFechas({
  id,
  valor,
  onCambio,
  disabled,
  className,
}: {
  id?: string;
  valor: RangoFechas;
  onCambio: (v: RangoFechas) => void;
  disabled?: boolean;
  className?: string;
}) {
  const autoId = useId();
  const baseId = id ?? autoId;
  return (
    <div className={cn("grid gap-2", className)}>
      <div className="flex flex-wrap items-center gap-2">
        <EntradaFecha
          id={`${baseId}-desde`}
          valor={valor.desde}
          max={valor.hasta}
          disabled={disabled}
          onCambio={(iso) => onCambio({ ...valor, desde: iso })}
          className="w-40"
        />
        <span className="text-control text-muted-foreground">–</span>
        <EntradaFecha
          id={`${baseId}-hasta`}
          valor={valor.hasta}
          min={valor.desde}
          disabled={disabled}
          onCambio={(iso) => onCambio({ ...valor, hasta: iso })}
          className="w-40"
        />
      </div>
      <div className="flex flex-wrap gap-1.5">
        {PRESETS.map((p) => (
          <button
            key={p.etiqueta}
            type="button"
            disabled={disabled}
            onClick={() => onCambio(p.rango())}
            className="border-border/60 bg-secondary/80 text-etiqueta text-muted-foreground hover:bg-secondary hover:text-foreground rounded-md border px-2 py-1 font-medium transition-colors disabled:pointer-events-none disabled:opacity-60"
          >
            {p.etiqueta}
          </button>
        ))}
      </div>
    </div>
  );
}
