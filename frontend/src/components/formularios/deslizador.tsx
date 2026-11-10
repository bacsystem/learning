"use client";

import { cn } from "@/lib/utils";
import { Slider as SliderPrimitive } from "@base-ui/react/slider";

const PULGAR =
  "block size-[18px] rounded-full bg-white shadow-[0_0_0_1.5px_var(--primary),0_2px_6px_rgba(0,0,0,0.15)] outline-none transition-transform duration-(--duracion-rapida) ease-salida focus-visible:ring-4 focus-visible:ring-acento/55 data-dragging:scale-110";

/**
 * Control deslizante para una magnitud continua (monto, cantidad, porcentaje) o un rango de dos manijas
 * (`valor` como tupla). Para un rango de fechas usa `EntradaRangoFechas`, no esto.
 */
export function Deslizador({
  etiqueta,
  valor,
  onCambio,
  min = 0,
  max = 100,
  paso = 1,
  formato,
  disabled,
  className,
}: {
  /** Nombre accesible del control (o de cada manija, como "Monto" → "Monto mínimo"/"Monto máximo" en un rango);
   * el `<input type="range">` interno no tiene texto visible propio, así que esto es obligatorio para lectores
   * de pantalla — omítelo solo si ya envuelves el deslizador en un `<label>` que lo describe. */
  etiqueta: string;
  valor: number | readonly [number, number];
  onCambio: (v: number | [number, number]) => void;
  min?: number;
  max?: number;
  paso?: number;
  /** Formatea el valor mostrado a la derecha; por defecto el número tal cual. */
  formato?: (v: number) => string;
  disabled?: boolean;
  className?: string;
}) {
  const rango = Array.isArray(valor);
  const f = formato ?? ((v: number) => String(v));
  const etiquetaManija = (i: number) => (rango ? `${etiqueta} ${i === 0 ? "mínimo" : "máximo"}` : etiqueta);

  return (
    <SliderPrimitive.Root
      value={valor}
      onValueChange={(v) => onCambio(v as number | [number, number])}
      min={min}
      max={max}
      step={paso}
      disabled={disabled}
      className={cn("grid gap-1.5", className)}
    >
      <span className="text-control text-foreground font-semibold tabular-nums">
        {rango ? `${f(valor[0])} – ${f(valor[1])}` : f(valor as number)}
      </span>
      <SliderPrimitive.Control className="flex w-full items-center py-2">
        <SliderPrimitive.Track className="bg-secondary relative h-1 w-full rounded-full">
          <SliderPrimitive.Indicator className="bg-primary absolute h-full rounded-full" />
          <SliderPrimitive.Thumb className={PULGAR} getAriaLabel={etiquetaManija} />
          {rango ? <SliderPrimitive.Thumb className={PULGAR} getAriaLabel={etiquetaManija} /> : null}
        </SliderPrimitive.Track>
      </SliderPrimitive.Control>
    </SliderPrimitive.Root>
  );
}
