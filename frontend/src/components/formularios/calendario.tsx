"use client";

import { type KeyboardEvent, useEffect, useRef, useState } from "react";

import {
  dentroDeRango,
  desdeIso,
  DIAS_SEMANA,
  DIAS_SEMANA_EN,
  hoyIso,
  nombreMes,
  semanasDelMes,
  sumarDias,
  sumarMeses,
} from "@/lib/calendario";
import { OBJETIVO_TACTIL } from "@/lib/estilos";
import { esLocaleFecha, formatearFecha, type LocaleFecha } from "@/lib/formato";
import { cn } from "@/lib/utils";
import { CaretLeftIcon, CaretRightIcon } from "@phosphor-icons/react";

const BOTON_MES = cn(
  OBJETIVO_TACTIL,
  "grid size-8 place-items-center rounded-full text-muted-foreground transition-colors duration-(--duracion-rapida) ease-salida outline-none hover:bg-secondary hover:text-foreground focus-visible:ring-2 focus-visible:ring-acento/55",
);

const TEXTOS: Record<LocaleFecha, { hoy: string; anterior: string; siguiente: string }> = {
  es: { hoy: "Hoy", anterior: "Mes anterior", siguiente: "Mes siguiente" },
  en: { hoy: "Today", anterior: "Previous month", siguiente: "Next month" },
};

/**
 * Calendario de un mes (lunes a domingo) para elegir una fecha. Valor ISO `YYYY-MM-DD`. Teclado como el patrón de
 * grilla de fechas de WAI-ARIA: flechas (día / semana), RePág/AvPág (mes; con Shift, año), Inicio/Fin (semana),
 * Enter o Espacio para elegir. Hoy va marcado con el acento; la fecha elegida, en el color de la acción principal.
 * Lo usa `EntradaFecha` dentro de un popover; también sirve suelto (p. ej. en un panel).
 */
export function Calendario({
  valor,
  onElegir,
  min,
  max,
  conHoy = true,
  locale = "es",
  className,
}: {
  valor?: string;
  onElegir: (iso: string) => void;
  min?: string;
  max?: string;
  /** Botón «Hoy» al pie (si hoy está dentro de `min`/`max`). */
  conHoy?: boolean;
  /** Idioma de los textos y de los nombres de mes y de día (`es` por defecto; otro valor cae en `es`). */
  locale?: LocaleFecha;
  className?: string;
}) {
  const idioma = esLocaleFecha(locale) ? locale : "es";
  const textos = TEXTOS[idioma];
  const diasSemana = idioma === "en" ? DIAS_SEMANA_EN : DIAS_SEMANA;
  const hoy = hoyIso();
  const inicial = desdeIso(valor) ? (valor as string) : hoy;
  const [foco, setFoco] = useState(inicial);
  const [enfocar, setEnfocar] = useState(false);
  const grilla = useRef<HTMLTableElement>(null);
  const fechaFoco = desdeIso(foco) ?? new Date();
  const anio = fechaFoco.getFullYear();
  const mes = fechaFoco.getMonth();

  // Solo después de navegar con el teclado: al abrir, el foco lo pone el popover (en el día elegido o en hoy).
  useEffect(() => {
    if (!enfocar) return;
    grilla.current?.querySelector<HTMLButtonElement>(`[data-iso="${foco}"]`)?.focus();
    setEnfocar(false);
  }, [enfocar, foco]);

  function mover(nuevo: string) {
    setFoco(nuevo);
    setEnfocar(true);
  }

  function teclado(e: KeyboardEvent<HTMLTableElement>) {
    const diaSemana = ((desdeIso(foco)?.getDay() ?? 1) + 6) % 7;
    const destino: Record<string, () => string> = {
      ArrowLeft: () => sumarDias(foco, -1),
      ArrowRight: () => sumarDias(foco, 1),
      ArrowUp: () => sumarDias(foco, -7),
      ArrowDown: () => sumarDias(foco, 7),
      PageUp: () => sumarMeses(foco, e.shiftKey ? -12 : -1),
      PageDown: () => sumarMeses(foco, e.shiftKey ? 12 : 1),
      Home: () => sumarDias(foco, -diaSemana),
      End: () => sumarDias(foco, 6 - diaSemana),
    };
    const calcular = destino[e.key];
    if (!calcular) return;
    e.preventDefault();
    mover(calcular());
  }

  return (
    <div className={cn("grid w-[17.5rem] gap-2", className)}>
      <div className="flex items-center justify-between">
        <button
          type="button"
          aria-label={textos.anterior}
          className={BOTON_MES}
          onClick={() => setFoco(sumarMeses(foco, -1))}
        >
          <CaretLeftIcon weight="bold" className="size-4" />
        </button>
        <span aria-live="polite" className="text-cuerpo text-foreground font-semibold">
          {nombreMes(anio, mes, idioma)}
        </span>
        <button
          type="button"
          aria-label={textos.siguiente}
          className={BOTON_MES}
          onClick={() => setFoco(sumarMeses(foco, 1))}
        >
          <CaretRightIcon weight="bold" className="size-4" />
        </button>
      </div>

      <table
        ref={grilla}
        role="grid"
        aria-label={nombreMes(anio, mes, idioma)}
        onKeyDown={teclado}
        className="w-full border-collapse"
      >
        <thead>
          <tr>
            {diasSemana.map((d) => (
              <th key={d.largo} scope="col" className="text-etiqueta text-texto-terciario pb-1 text-center font-medium">
                <abbr title={d.largo} className="no-underline">
                  {d.corto}
                </abbr>
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {semanasDelMes(anio, mes).map((semana) => (
            <tr key={semana[0].iso}>
              {semana.map((d) => {
                const elegido = d.iso === valor;
                const esHoy = d.iso === hoy;
                const habilitado = dentroDeRango(d.iso, min, max);
                return (
                  <td key={d.iso} role="gridcell" aria-selected={elegido} className="p-0.5 text-center">
                    <button
                      type="button"
                      data-iso={d.iso}
                      tabIndex={d.iso === foco ? 0 : -1}
                      disabled={!habilitado}
                      aria-current={esHoy ? "date" : undefined}
                      aria-label={formatearFecha(d.iso, idioma)}
                      onClick={() => onElegir(d.iso)}
                      onFocus={() => setFoco(d.iso)}
                      className={cn(
                        "text-control ease-salida focus-visible:ring-acento-texto dark:focus-visible:ring-acento mx-auto grid size-9 place-items-center rounded-full tabular-nums transition-colors duration-(--duracion-rapida) outline-none focus-visible:ring-2",
                        d.delMes ? "text-foreground" : "text-texto-terciario",
                        !elegido && "hover:bg-secondary",
                        esHoy && !elegido && "bg-acento-suave text-acento-texto dark:text-acento font-semibold",
                        elegido && "bg-primary text-primary-foreground font-semibold",
                        !habilitado && "pointer-events-none opacity-30",
                      )}
                    >
                      {d.dia}
                    </button>
                  </td>
                );
              })}
            </tr>
          ))}
        </tbody>
      </table>

      {conHoy && dentroDeRango(hoy, min, max) ? (
        <div className="flex justify-end">
          <button
            type="button"
            onClick={() => onElegir(hoy)}
            className="text-control text-foreground hover:bg-secondary focus-visible:ring-acento/55 h-7 rounded-full px-3 font-medium transition-colors focus-visible:ring-2 focus-visible:outline-none"
          >
            {textos.hoy}
          </button>
        </div>
      ) : null}
    </div>
  );
}
