import type { ReactNode } from "react";

import { TARJETA_ELEVABLE } from "@/lib/estilos";
import { type IconoComponente, PESO_ICONO, PESO_ICONO_ACTIVO } from "@/lib/iconos";
import { cn } from "@/lib/utils";
import { ArrowDownRightIcon, ArrowUpRightIcon, MinusIcon } from "@phosphor-icons/react/dist/ssr";

export type Tono = "ok" | "aviso" | "error" | "neutro";
/** Tono de una métrica: los de estado más `acento` (la paleta del usuario) para métricas sin juicio (facturado, emitidos). */
export type TonoMetrica = Tono | "acento";
export type VisualMetrica = { tipo: "barra"; valor: number } | { tipo: "puntos"; total: number; marcados: number };

const TEXTO_TONO: Record<TonoMetrica, string> = {
  ok: "text-success-foreground",
  aviso: "text-warning-foreground",
  error: "text-destructive-texto",
  neutro: "text-muted-foreground",
  acento: "text-acento-2-texto dark:text-acento-2",
};
const RELLENO_TONO: Record<TonoMetrica, string> = {
  ok: "bg-success-solid",
  aviso: "bg-warning-solid",
  error: "bg-destructive",
  neutro: "bg-muted-foreground/50",
  acento: "bg-chart-destacado",
};

/** Punto de estado de 6px para ayudas y pills. */
export function Punto({ tono }: { tono: Tono }) {
  return <span className={cn("size-1.5 shrink-0 rounded-full", RELLENO_TONO[tono])} />;
}

/** Ayuda de una métrica con punto y color según tono. */
export function AyudaMetrica({ tono, children }: { tono: Tono; children: ReactNode }) {
  return (
    <span className={cn("flex min-w-0 items-center gap-1", tono !== "neutro" && TEXTO_TONO[tono])}>
      <Punto tono={tono} />
      <span className="truncate">{children}</span>
    </span>
  );
}

/**
 * Tarjeta de métrica: encabezado (icono por tono, etiqueta y `acciones` a la derecha, típicamente un `MenuAcciones`)
 * y el cuerpo que le pases. `destacada` la pinta con la superficie de acento de la paleta (en oscuro, teñida).
 * El icono va sin fondo: el color dice el tono (docs/design-system.md «Iconos»).
 */
export function TarjetaMetrica({
  etiqueta,
  icono: Icono,
  tono = "neutro",
  destacada,
  acciones,
  className,
  children,
}: {
  etiqueta: ReactNode;
  icono?: IconoComponente;
  tono?: TonoMetrica;
  destacada?: boolean;
  acciones?: ReactNode;
  className?: string;
  children: ReactNode;
}) {
  return (
    <div
      className={cn(
        TARJETA_ELEVABLE,
        "flex min-w-0 flex-col gap-2 px-[15px] pt-3.5 pb-[13px]",
        // Dentro de otra tarjeta (galería, paneles) la métrica pasa a la superficie suave: blanco sobre blanco no se distingue.
        "in-[.bg-card]:bg-secondary in-[.bg-card]:shadow-none",
        destacada &&
          "border-acento-borde bg-acento-superficie text-acento-superficie-texto in-[.bg-card]:bg-acento-superficie dark:bg-acento-superficie",
        className,
      )}
    >
      <div className="text-control flex min-h-6 min-w-0 items-center gap-[7px] font-medium">
        {Icono ? (
          <Icono
            aria-hidden
            weight={destacada ? PESO_ICONO_ACTIVO : PESO_ICONO}
            className={cn("size-4 shrink-0", destacada ? "text-current" : TEXTO_TONO[tono])}
          />
        ) : null}
        <span className={cn("min-w-0 flex-1 truncate", destacada ? "text-current" : "text-muted-foreground")}>
          {etiqueta}
        </span>
        {acciones ? <span className="-my-1 shrink-0">{acciones}</span> : null}
      </div>
      {children}
    </div>
  );
}

/** Mini visual de la métrica: barra de progreso (0–100) o puntos (n de total marcados en el color del tono). */
export function VisualDeMetrica({
  visual,
  tono = "neutro",
  destacada,
}: {
  visual: VisualMetrica;
  tono?: TonoMetrica;
  destacada?: boolean;
}) {
  if (visual.tipo === "barra") {
    const valor = Math.max(0, Math.min(100, visual.valor));
    return (
      <div
        role="progressbar"
        aria-valuenow={valor}
        aria-valuemin={0}
        aria-valuemax={100}
        className={cn("h-1.5 overflow-hidden rounded-full", destacada ? "bg-current/15" : "bg-secondary")}
      >
        <div
          className={cn("h-full rounded-full", destacada ? "bg-current" : RELLENO_TONO[tono])}
          style={{ width: `${valor}%` }}
        />
      </div>
    );
  }
  const marcados = Math.max(0, Math.min(visual.total, visual.marcados));
  return (
    <div
      aria-hidden
      className="grid h-1.5 gap-1"
      style={{ gridTemplateColumns: `repeat(${visual.total}, minmax(0, 1fr))` }}
    >
      {Array.from({ length: visual.total }, (_, i) => (
        <span
          key={i}
          className={cn(
            "rounded-full",
            i < marcados
              ? destacada
                ? "bg-current"
                : RELLENO_TONO[tono]
              : destacada
                ? "bg-current/15"
                : "bg-secondary",
          )}
        />
      ))}
    </div>
  );
}

/** Chip de variación: verde si es buena, rojo si es mala, neutro si es cero. `invertir` cuando bajar es bueno. */
export function ChipVariacion({
  variacion,
  invertir,
  destacada,
}: {
  variacion: number;
  invertir?: boolean;
  destacada?: boolean;
}) {
  const positiva = variacion > 0;
  const negativa = variacion < 0;
  const buena = invertir ? negativa : positiva;
  const mala = invertir ? positiva : negativa;
  const Flecha = positiva ? ArrowUpRightIcon : negativa ? ArrowDownRightIcon : MinusIcon;
  return (
    <span
      className={cn(
        "text-etiqueta inline-flex shrink-0 items-center gap-0.5 rounded-full px-1.5 py-0.5 font-semibold tabular-nums",
        destacada
          ? "bg-current/12 text-current"
          : buena
            ? "bg-success text-success-foreground"
            : mala
              ? "bg-destructive-suave text-destructive-texto"
              : "bg-secondary text-foreground",
      )}
    >
      <Flecha weight="bold" className="size-3" />
      {Math.abs(variacion).toFixed(1)}%
    </span>
  );
}

/**
 * Una métrica en su tarjeta: etiqueta, valor grande (con sufijo opcional), mini visual opcional y ayuda. Para que el
 * valor cuente al aparecer, pásale `<NumeroAnimado valor={…} />` como hijo; un identificador va envuelto en `font-mono`.
 * `texto` cuando el valor es una palabra (un estado, un entorno, un código) y no un número: va en `text-metrica-texto` (18 px),
 * porque a 22 px una palabra larga domina la tarjeta y la parte en dos líneas.
 */
export function Metrica({
  etiqueta,
  children,
  sufijo,
  ayuda,
  icono,
  tono,
  visual,
  destacada,
  acciones,
  texto,
}: {
  etiqueta: string;
  children: ReactNode;
  sufijo?: ReactNode;
  ayuda: ReactNode;
  icono?: IconoComponente;
  tono?: TonoMetrica;
  visual?: VisualMetrica;
  destacada?: boolean;
  acciones?: ReactNode;
  texto?: boolean;
}) {
  return (
    <TarjetaMetrica etiqueta={etiqueta} icono={icono} tono={tono} destacada={destacada} acciones={acciones}>
      <div
        className={cn(
          "flex min-w-0 flex-wrap items-baseline gap-x-1.5",
          texto ? "text-metrica-texto" : "text-metrica tabular-nums",
          destacada ? "text-current" : "text-foreground",
        )}
      >
        {children}
        {sufijo ? (
          <span className={cn("text-control font-normal", destacada ? "opacity-70" : "text-texto-terciario")}>
            {sufijo}
          </span>
        ) : null}
      </div>
      {visual ? <VisualDeMetrica visual={visual} tono={tono} destacada={destacada} /> : null}
      <div
        className={cn(
          "text-etiqueta flex min-w-0 items-center gap-1.5 truncate",
          destacada ? "opacity-80" : "text-texto-terciario",
        )}
      >
        {ayuda}
      </div>
    </TarjetaMetrica>
  );
}

/** Fila de métricas: tarjetas separadas por 12px (4 columnas en desktop, 2 en tablet, 1 en móvil; `columnas` de 2 a 5). */
export function FilaMetricas({ children, columnas = 4 }: { children: ReactNode; columnas?: 2 | 3 | 4 | 5 }) {
  return (
    <section
      className={cn(
        "grid grid-cols-1 gap-3 sm:grid-cols-2",
        columnas === 5 && "lg:grid-cols-5",
        columnas === 4 && "lg:grid-cols-4",
        columnas === 3 && "lg:grid-cols-3",
        columnas === 2 && "lg:grid-cols-2",
      )}
    >
      {children}
    </section>
  );
}
