"use client";

import { m, useReducedMotion } from "motion/react";

import { MovimientoLocal } from "@/components/movimiento";
import { CURVA_SALIDA, DURACION } from "@/lib/movimiento";

/**
 * Trazo SVG que se dibuja al montarse (`pathLength` 0 → 1). Es de cliente para que `Sparkline` (Server Component) y los
 * gráficos lo usen. Con «reducir movimiento» aparece completo. `retraso` en segundos, para escalonar varias series.
 */
export function TrazoAnimado({
  d,
  stroke,
  strokeWidth = 2,
  retraso = 0,
}: {
  d: string;
  stroke: string;
  strokeWidth?: number;
  retraso?: number;
}) {
  const reducir = useReducedMotion();
  return (
    <MovimientoLocal>
      <m.path
        d={d}
        fill="none"
        stroke={stroke}
        strokeWidth={strokeWidth}
        strokeLinejoin="round"
        strokeLinecap="round"
        initial={reducir ? false : { pathLength: 0 }}
        animate={{ pathLength: 1 }}
        transition={{ duration: DURACION.trazo, ease: CURVA_SALIDA, delay: retraso }}
      />
    </MovimientoLocal>
  );
}
