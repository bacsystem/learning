import type { ReactNode } from "react";

import { TrazoAnimado } from "@/components/datos/trazo-animado";
import {
  ChipVariacion,
  TarjetaMetrica,
  type TonoMetrica,
  VisualDeMetrica,
  type VisualMetrica,
} from "@/components/patrones/metricas";
import type { IconoComponente } from "@/lib/iconos";
import { cn } from "@/lib/utils";

/** Sparkline de una serie (sin ejes): línea de la serie principal que se dibuja al aparecer, área tenue y punto final. */
export function Sparkline({
  datos,
  ancho = 120,
  alto = 28,
  className,
}: {
  datos: number[];
  ancho?: number;
  alto?: number;
  className?: string;
}) {
  if (datos.length < 2) return null;
  const min = Math.min(...datos);
  const max = Math.max(...datos);
  const rango = max - min || 1;
  const px = 2;
  const puntos = datos.map(
    (v, i) =>
      [px + (i / (datos.length - 1)) * (ancho - px * 2), alto - px - ((v - min) / rango) * (alto - px * 2)] as const,
  );
  const linea = puntos.map(([x, y], i) => `${i === 0 ? "M" : "L"}${x.toFixed(1)},${y.toFixed(1)}`).join(" ");
  const [ux, uy] = puntos[puntos.length - 1];
  return (
    <svg
      width={ancho}
      height={alto}
      viewBox={`0 0 ${ancho} ${alto}`}
      aria-hidden="true"
      className={cn("shrink-0 overflow-visible", className)}
    >
      <path d={`${linea} L${ux.toFixed(1)},${alto} L${px},${alto} Z`} fill="var(--chart-base)" opacity="0.7" />
      <TrazoAnimado d={linea} stroke="var(--chart-1)" />
      <circle cx={ux} cy={uy} r="3" fill="var(--chart-1)" stroke="var(--card)" strokeWidth="2" />
    </svg>
  );
}

/**
 * KPI con tendencia en su tarjeta: encabezado con icono, valor grande, sparkline (`serie`) o mini visual (`visual`) y
 * chip de variación vs. período anterior. `invertir` cuando bajar es bueno (rechazos, errores). Para que el valor cuente
 * al aparecer, pásale `<NumeroAnimado valor={…} />`.
 */
export function Kpi({
  etiqueta,
  valor,
  variacion,
  periodo = "vs. período anterior",
  invertir,
  serie,
  icono,
  tono,
  visual,
  destacada,
  acciones,
  className,
}: {
  etiqueta: ReactNode;
  valor: ReactNode;
  /** Porcentaje, p. ej. 12.5 o -3; omite para no mostrar tendencia. */
  variacion?: number;
  periodo?: ReactNode;
  invertir?: boolean;
  serie?: number[];
  icono?: IconoComponente;
  tono?: TonoMetrica;
  visual?: VisualMetrica;
  destacada?: boolean;
  acciones?: ReactNode;
  className?: string;
}) {
  return (
    <TarjetaMetrica
      etiqueta={etiqueta}
      icono={icono}
      tono={tono}
      destacada={destacada}
      acciones={acciones}
      className={className}
    >
      <div className={cn("text-metrica tabular-nums", destacada ? "text-current" : "text-foreground")}>{valor}</div>
      {serie ? (
        <Sparkline datos={serie} />
      ) : visual ? (
        <VisualDeMetrica visual={visual} tono={tono} destacada={destacada} />
      ) : null}
      {variacion != null ? (
        <div
          className={cn(
            "text-etiqueta flex min-w-0 items-center gap-1.5",
            destacada ? "opacity-80" : "text-texto-terciario",
          )}
        >
          <ChipVariacion variacion={variacion} invertir={invertir} destacada={destacada} />
          <span className="truncate">{periodo}</span>
        </div>
      ) : null}
    </TarjetaMetrica>
  );
}
