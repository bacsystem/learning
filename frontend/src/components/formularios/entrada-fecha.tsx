"use client";

import { useRef, useState } from "react";

import { Calendario } from "@/components/formularios/calendario";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { CAMPO, CONTROL_FILTRO } from "@/lib/estilos";
import { formatearFecha } from "@/lib/formato";
import { PESO_ICONO } from "@/lib/iconos";
import { cn } from "@/lib/utils";
import { CalendarBlankIcon, CaretDownIcon, XIcon } from "@phosphor-icons/react";

/**
 * Fecha con calendario propio en un popover (el selector nativo del navegador no se puede estilizar y cambiaba con
 * cada sistema; en iOS un campo vacío no mostraba nada). Muestra «15 Set 2026»; valor ISO `YYYY-MM-DD`. Para rangos, usa dos
 * `EntradaFecha` (desde/hasta). `id` va en el disparador, así un `<label htmlFor>` lo enfoca.
 *
 * - `limpiable`: con una fecha elegida, una ✕ la quita (`onCambio("")`). Es lo que necesita un filtro opcional.
 * - `nombre`: además pinta un `<input type="hidden">` con el ISO, para que un `<form>` nativo lo envíe.
 * - `enBarra`: la píldora de las barras de filtro (`CONTROL_FILTRO`), junto a los `Select` de la barra; sin ella, la de un formulario (`CAMPO`).
 */
export function EntradaFecha({
  id,
  valor,
  onCambio,
  min,
  max,
  disabled,
  placeholder = "Elegir fecha",
  limpiable = false,
  etiquetaLimpiar = "Quitar la fecha",
  nombre,
  enBarra = false,
  invalido,
  describidoPor,
  className,
}: {
  id: string;
  valor?: string;
  onCambio?: (iso: string) => void;
  min?: string;
  max?: string;
  disabled?: boolean;
  placeholder?: string;
  /** Con una fecha elegida, una ✕ la quita: `onCambio("")`. */
  limpiable?: boolean;
  /** Nombre accesible de la ✕. */
  etiquetaLimpiar?: string;
  /** Nombre del `<input type="hidden">` que lleva el ISO en un `<form>` nativo. */
  nombre?: string;
  /** Sin efecto desde 0.5.0: el disparador ya muestra la fecha formateada. Se conserva por compatibilidad. */
  conVistaPrevia?: boolean;
  /** Las dos variantes miden h-control (altura única de control); se conserva por compatibilidad. Igual que StepperNumerico. */
  variante?: "filtro" | "campo";
  /** Píldora de barra de filtros (`CONTROL_FILTRO`) en vez del campo de formulario (`CAMPO`). */
  enBarra?: boolean;
  /** Marca el campo con error (`aria-invalid`, borde rojo de `CAMPO`) y lo une a su mensaje (`aria-describedby`). */
  invalido?: boolean;
  describidoPor?: string;
  className?: string;
}) {
  const [abierto, setAbierto] = useState(false);
  const caja = useRef<HTMLDivElement>(null);
  const conValor = !!valor;
  const conLimpiar = limpiable && conValor && !disabled;

  return (
    <div className={cn("relative inline-flex min-w-0", className)}>
      {nombre ? <input type="hidden" name={nombre} value={valor ?? ""} /> : null}
      <Popover open={abierto} onOpenChange={setAbierto}>
        <PopoverTrigger
          id={id}
          disabled={disabled}
          aria-invalid={invalido || undefined}
          aria-describedby={describidoPor}
          render={<button type="button" />}
          className={cn(
            enBarra ? cn(CONTROL_FILTRO, "px-3") : CAMPO,
            "group flex w-full items-center gap-2 text-left",
            // La ✕ va encima del disparador (un botón no puede ir dentro de otro): se le deja su lugar a la derecha.
            conLimpiar && "pr-8",
          )}
        >
          <CalendarBlankIcon weight={PESO_ICONO} className="text-muted-foreground size-4 shrink-0" />
          <span className={cn("min-w-0 flex-1 truncate tabular-nums", !conValor && "text-texto-terciario")}>
            {conValor ? formatearFecha(valor) : placeholder}
          </span>
          {conLimpiar ? null : (
            <CaretDownIcon
              weight="bold"
              className="text-texto-terciario ease-salida size-3.5 shrink-0 transition-transform duration-(--duracion-rapida) group-data-popup-open:rotate-180"
            />
          )}
        </PopoverTrigger>
        {/* Al abrir, el foco va al día elegido (o a hoy), no al botón «Mes anterior»: así las flechas funcionan de inmediato. */}
        <PopoverContent
          className="w-auto p-3"
          initialFocus={() => caja.current?.querySelector<HTMLElement>("button[data-iso][tabindex='0']") ?? true}
        >
          <div ref={caja}>
            <Calendario
              valor={valor}
              min={min}
              max={max}
              onElegir={(iso) => {
                onCambio?.(iso);
                setAbierto(false);
              }}
            />
          </div>
        </PopoverContent>
      </Popover>
      {conLimpiar ? (
        <button
          type="button"
          aria-label={etiquetaLimpiar}
          title={etiquetaLimpiar}
          onClick={() => onCambio?.("")}
          className="text-texto-terciario hover:bg-secondary hover:text-foreground focus-visible:ring-ring/50 absolute top-1/2 right-2 grid size-5 -translate-y-1/2 place-items-center rounded-full transition-colors focus-visible:ring-2 focus-visible:outline-none"
        >
          <XIcon weight="bold" className="size-3" />
        </button>
      ) : null}
    </div>
  );
}
