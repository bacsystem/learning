"use client";

import { m, useReducedMotion } from "motion/react";
import type { ReactNode } from "react";

import { MovimientoLocal } from "@/components/movimiento";
import { RESORTE_SUAVE } from "@/lib/movimiento";
import { cn } from "@/lib/utils";
import { Progress as ProgressPrimitive } from "@base-ui/react/progress";

export type TonoProgreso = "primary" | "success" | "warning" | "destructive";

const TONOS: Record<TonoProgreso, string> = {
  primary: "bg-primary",
  success: "bg-success-solid",
  warning: "bg-warning-solid",
  destructive: "bg-destructive",
};

// var(--*) inline, no clases stroke-*, por la misma razón que `datos/grafico.tsx`: colores de trazo SVG fiables
// en cualquier build de Tailwind. Usado por `ProgresoCircular`.
const COLOR_TRAZO: Record<TonoProgreso, string> = {
  primary: "var(--primary)",
  success: "var(--success-solid)",
  warning: "var(--warning-solid)",
  destructive: "var(--destructive)",
};

/**
 * Barra de progreso lineal. `valor` en `0–100`; omítelo (o pásalo `null`) para el estado indeterminado
 * (subida de certificado, envío por lotes) — un tramo que barre el track en vez de un ancho fijo.
 * `etiqueta` + `mostrarValor` arman la fila "Subiendo certificado.p12… 64 %"; sin ninguna de las dos es solo la barra.
 */
export function ProgresoLineal({
  valor,
  tono = "primary",
  etiqueta,
  mostrarValor = true,
  className,
}: {
  valor?: number | null;
  tono?: TonoProgreso;
  etiqueta?: ReactNode;
  mostrarValor?: boolean;
  className?: string;
}) {
  const indeterminado = valor == null;
  return (
    <ProgressPrimitive.Root value={valor ?? null} className={cn("grid gap-1.5", className)}>
      {etiqueta || (mostrarValor && !indeterminado) ? (
        <div className="flex items-center justify-between gap-2">
          {etiqueta ? (
            <ProgressPrimitive.Label className="text-control text-foreground font-medium">
              {etiqueta}
            </ProgressPrimitive.Label>
          ) : (
            <span />
          )}
          {mostrarValor && !indeterminado ? (
            <ProgressPrimitive.Value className="text-etiqueta text-muted-foreground font-mono tabular-nums" />
          ) : null}
        </div>
      ) : null}
      <ProgressPrimitive.Track className="bg-secondary relative h-1.5 overflow-hidden rounded-full">
        <ProgressPrimitive.Indicator
          className={cn(
            "block h-full rounded-full",
            indeterminado
              ? cn("animate-progreso-indeterminado w-1/3", TONOS[tono])
              : cn(
                  "ease-salida transition-[width] duration-(--duracion-lenta) motion-reduce:transition-none",
                  TONOS[tono],
                ),
          )}
        />
      </ProgressPrimitive.Track>
    </ProgressPrimitive.Root>
  );
}

const TAMANOS_CIRCULAR = {
  sm: { caja: 32, grosor: 3 },
  default: { caja: 44, grosor: 4 },
  lg: { caja: 64, grosor: 5 },
} as const;

/**
 * Progreso circular (anillo con `%` al centro por defecto). Sin primitiva propia en base-ui: SVG con
 * `stroke-dasharray`. `valor` en `0–100`; sin él, gira indefinidamente (`animate-spin` sobre el trazo).
 * Útil para cuotas/consumo (plan del sidebar) o como spinner de marca más vistoso que `Spinner`.
 */
export function ProgresoCircular({
  valor,
  tono = "primary",
  tamano = "default",
  contenido,
  className,
}: {
  valor?: number | null;
  tono?: TonoProgreso;
  tamano?: keyof typeof TAMANOS_CIRCULAR;
  /** Reemplaza el `%` central; pásale `null` para dejar el anillo vacío (spinner de marca). */
  contenido?: ReactNode | null;
  className?: string;
}) {
  const { caja, grosor } = TAMANOS_CIRCULAR[tamano];
  const radio = (caja - grosor) / 2;
  const circunferencia = 2 * Math.PI * radio;
  const indeterminado = valor == null;
  const pct = indeterminado ? 0.25 : Math.min(100, Math.max(0, valor)) / 100;
  // strokeDashoffset no es una transformación: con «reducir movimiento» se salta aquí, no en MotionConfig.
  const reducir = useReducedMotion();

  return (
    <div
      role="progressbar"
      aria-valuenow={indeterminado ? undefined : Math.round(valor!)}
      aria-valuemin={0}
      aria-valuemax={100}
      className={cn(
        "relative inline-flex shrink-0 items-center justify-center",
        indeterminado && "animate-spin",
        className,
      )}
      style={{ width: caja, height: caja }}
    >
      <MovimientoLocal>
        <svg width={caja} height={caja} viewBox={`0 0 ${caja} ${caja}`} className="-rotate-90">
          <circle
            cx={caja / 2}
            cy={caja / 2}
            r={radio}
            strokeWidth={grosor}
            fill="none"
            style={{ stroke: "var(--secondary)" }}
          />
          <m.circle
            cx={caja / 2}
            cy={caja / 2}
            r={radio}
            strokeWidth={grosor}
            strokeLinecap="round"
            strokeDasharray={circunferencia}
            fill="none"
            style={{ stroke: COLOR_TRAZO[tono] }}
            // initial={false}: el servidor ya pinta el valor; solo se anima cuando cambia.
            initial={false}
            animate={{ strokeDashoffset: circunferencia * (1 - pct) }}
            transition={reducir ? { duration: 0 } : RESORTE_SUAVE}
          />
        </svg>
      </MovimientoLocal>
      {contenido !== null ? (
        <span className="text-etiqueta text-foreground absolute font-mono font-semibold tabular-nums">
          {contenido ?? (indeterminado ? "" : `${Math.round(valor!)}%`)}
        </span>
      ) : null}
    </div>
  );
}
