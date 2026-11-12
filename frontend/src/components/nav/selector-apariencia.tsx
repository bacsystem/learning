"use client";

import { type CSSProperties, type ReactNode, useRef } from "react";

import { useApariencia } from "@/components/apariencia";
import { FOCO, ROTULO } from "@/lib/estilos";
import { cn } from "@/lib/utils";

type Opcion = { valor: string; etiqueta: string; estilo: CSSProperties; contenido?: ReactNode };

/**
 * Grupo de muestras que se elige como un radio: Tab entra a la elegida, las flechas cambian y aplican. Cada muestra lleva
 * su nombre en `aria-label` y en el tooltip nativo. `forma`: círculo (paletas), tarjeta (tonos oscuros: lienzo + tarjeta +
 * acento) o «Aa» (tipografías, cada una en su fuente). Las dos últimas muestran además el nombre de la elegida debajo.
 */
function GrupoMuestras({
  etiqueta,
  opciones,
  valor,
  onCambio,
  forma,
  grande = false,
}: {
  etiqueta: string;
  opciones: Opcion[];
  valor: string;
  onCambio: (valor: string) => void;
  forma: "circulo" | "tarjeta" | "texto";
  /** Muestras más grandes y el nombre de la elegida también en las paletas (panel «Apariencia»). */
  grande?: boolean;
}) {
  const botones = useRef<(HTMLButtonElement | null)[]>([]);

  function mover(desde: number, paso: number) {
    const i = (desde + paso + opciones.length) % opciones.length;
    onCambio(opciones[i].valor);
    botones.current[i]?.focus();
  }

  return (
    <div className={cn("grid", grande ? "gap-2" : "gap-1.5")}>
      <div
        role="radiogroup"
        aria-label={etiqueta}
        // Tarjetas y «Aa» en columnas iguales que encogen (minmax(0, 1fr)): en flex, Safari no achicaba los botones y la fila se salía del panel en un teléfono.
        style={forma === "circulo" ? undefined : { gridTemplateColumns: `repeat(${opciones.length}, minmax(0, 1fr))` }}
        className={cn(forma === "circulo" ? "flex flex-wrap items-center" : "grid", grande ? "gap-2.5" : "gap-1.5")}
      >
        {opciones.map((o, i) => {
          const activa = o.valor === valor;
          return (
            <button
              key={o.valor}
              ref={(el) => {
                botones.current[i] = el;
              }}
              type="button"
              role="radio"
              aria-checked={activa}
              aria-label={o.etiqueta}
              title={o.etiqueta}
              tabIndex={activa ? 0 : -1}
              onClick={() => onCambio(o.valor)}
              onKeyDown={(e) => {
                if (e.key === "ArrowRight" || e.key === "ArrowDown") {
                  e.preventDefault();
                  mover(i, 1);
                } else if (e.key === "ArrowLeft" || e.key === "ArrowUp") {
                  e.preventDefault();
                  mover(i, -1);
                }
              }}
              style={o.estilo}
              className={cn(
                "ease-salida shrink-0 cursor-pointer transition-shadow duration-(--duracion-rapida)",
                forma === "circulo" && (grande ? "size-9 rounded-full" : "size-5 rounded-full"),
                forma === "tarjeta" && (grande ? "h-14 min-w-0 rounded-xl" : "h-7 min-w-0 rounded-lg"),
                forma === "texto" &&
                  cn(
                    "bg-secondary text-foreground grid min-w-0 place-items-center font-semibold",
                    grande ? "text-titulo-pagina h-12 rounded-xl" : "text-cuerpo h-8 rounded-lg",
                  ),
                activa
                  ? cn("ring-foreground ring-offset-popover ring-2 ring-offset-2", forma === "texto" && "bg-popover")
                  : forma !== "texto" && "ring-border ring-1",
                FOCO,
              )}
            >
              {o.contenido}
            </button>
          );
        })}
      </div>
      {forma !== "circulo" || grande ? (
        <span className="text-etiqueta text-texto-terciario text-center">
          {opciones.find((o) => o.valor === valor)?.etiqueta}
        </span>
      ) : null}
    </div>
  );
}

/** Muestras de las paletas (acento, acento 2 y acento 3 en un círculo). No pinta nada sin `AparienciaProvider`. */
export function SelectorPaleta({
  className,
  grande,
  etiqueta = "Paleta de color",
}: {
  className?: string;
  grande?: boolean;
  etiqueta?: string;
}) {
  const { paleta, setPaleta, paletas } = useApariencia();
  if (paletas.length < 2) return null;
  const opciones = paletas.map((p) => ({
    valor: p.valor,
    etiqueta: p.etiqueta,
    estilo: { background: `conic-gradient(${p.muestra[0]} 0 50%, ${p.muestra[1]} 0 75%, ${p.muestra[2]} 0)` },
  }));
  return (
    <div className={className}>
      <GrupoMuestras
        grande={grande}
        forma="circulo"
        etiqueta={etiqueta}
        opciones={opciones}
        valor={paleta}
        onCambio={setPaleta}
      />
    </div>
  );
}

/**
 * Muestras de los tonos del modo oscuro: lienzo, tarjeta y acento de la paleta en diagonal, para que se distingan (tres
 * círculos casi negros no se diferenciaban). Se puede elegir estando en claro: se aplica al pasar a oscuro.
 */
export function SelectorTonoOscuro({
  className,
  grande,
  etiqueta = "Tono del modo oscuro",
}: {
  className?: string;
  grande?: boolean;
  etiqueta?: string;
}) {
  const { tonoOscuro, setTonoOscuro, tonosOscuro } = useApariencia();
  if (tonosOscuro.length < 2) return null;
  const opciones = tonosOscuro.map((t) => ({
    valor: t.valor,
    etiqueta: t.etiqueta,
    estilo: {
      background: `linear-gradient(135deg, ${t.lienzo ?? t.muestra} 0 45%, ${t.muestra} 45% 75%, var(--acento) 75%)`,
    },
  }));
  return (
    <div className={className}>
      <GrupoMuestras
        grande={grande}
        forma="tarjeta"
        etiqueta={etiqueta}
        opciones={opciones}
        valor={tonoOscuro}
        onCambio={setTonoOscuro}
      />
    </div>
  );
}

/** Tipografías de interfaz: un «Aa» dibujado en cada fuente. Los datos siguen en JetBrains Mono. */
export function SelectorTipografia({
  className,
  grande,
  etiqueta = "Tipografía",
}: {
  className?: string;
  grande?: boolean;
  etiqueta?: string;
}) {
  const { fuente, setFuente, fuentes } = useApariencia();
  if (fuentes.length < 2) return null;
  const opciones = fuentes.map((f) => ({
    valor: f.valor,
    etiqueta: f.etiqueta,
    estilo: { fontFamily: f.familia },
    contenido: "Aa",
  }));
  return (
    <div className={className}>
      <GrupoMuestras
        grande={grande}
        forma="texto"
        etiqueta={etiqueta}
        opciones={opciones}
        valor={fuente}
        onCambio={setFuente}
      />
    </div>
  );
}

/** Paleta, tono oscuro y tipografía con su rótulo. Va en el menú de usuario bajo el tema. */
export function SelectorApariencia({ className }: { className?: string }) {
  const { paletas } = useApariencia();
  if (paletas.length < 2) return null;
  return (
    <div className={cn("grid gap-3", className)}>
      <div className="flex items-center justify-between gap-3">
        <span className={ROTULO}>Paleta</span>
        <SelectorPaleta />
      </div>
      <div className="grid gap-1.5">
        <span className={ROTULO}>Tono oscuro</span>
        <SelectorTonoOscuro />
      </div>
      <div className="grid gap-1.5">
        <span className={ROTULO}>Tipografía</span>
        <SelectorTipografia />
      </div>
    </div>
  );
}
