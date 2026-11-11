import type { Tono } from "./metricas";
import type { ReactNode } from "react";

import { type IconoComponente, PESO_ICONO } from "@/lib/iconos";
import { cn } from "@/lib/utils";

const ESTILOS: Record<Tono, string> = {
  ok: "bg-success text-success-foreground",
  aviso: "bg-warning text-warning-foreground",
  error: "bg-destructive-suave text-destructive-texto",
  neutro: "bg-secondary text-muted-foreground",
};
const PUNTOS: Record<Tono, string> = {
  ok: "bg-success-solid",
  aviso: "bg-warning-solid",
  error: "bg-destructive",
  neutro: "bg-muted-foreground/50",
};

/**
 * Pill de estado: relleno tenue + punto sólido, sin borde. Verde = OK, ámbar = provisional, rojo = error, neutro = en proceso/inactivo.
 * `icon` cambia el punto por un icono (p. ej. el estado de un servicio): toma el color del texto del tono y es decorativo
 * (`aria-hidden`), así que el estado lo sigue diciendo el texto.
 */
export function PillEstado({
  tono,
  icon: Icono,
  children,
  className,
}: {
  tono: Tono;
  icon?: IconoComponente;
  children: ReactNode;
  className?: string;
}) {
  return (
    <span
      className={cn(
        "text-etiqueta inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 font-medium whitespace-nowrap",
        ESTILOS[tono],
        className,
      )}
    >
      {Icono ? (
        <Icono aria-hidden weight={PESO_ICONO} className="size-3.5 shrink-0" />
      ) : (
        <span className={cn("size-1.5 shrink-0 rounded-full", PUNTOS[tono])} />
      )}
      {children}
    </span>
  );
}

/** Chip informativo (contadores, versiones). Los ids van en `ChipCodigo` (mono). */
export function Chip({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <span
      className={cn(
        "bg-secondary text-etiqueta text-muted-foreground inline-flex items-center gap-1 rounded-full px-2.5 py-0.5 font-medium whitespace-nowrap tabular-nums",
        className,
      )}
    >
      {children}
    </span>
  );
}

/** Chip de identificador (código de serie, prefijo de llave): mono sobre acento; `inactivo` lo tacha. */
export function ChipCodigo({
  children,
  inactivo,
  className,
}: {
  children: ReactNode;
  inactivo?: boolean;
  className?: string;
}) {
  return (
    <span
      className={cn(
        "text-etiqueta rounded-full px-2.5 py-0.5 font-mono font-medium",
        inactivo ? "bg-secondary text-muted-foreground line-through" : "bg-accent text-accent-foreground",
        className,
      )}
    >
      {children}
    </span>
  );
}

/** Etiqueta "Próximamente". */
export function ChipPronto({ children = "Próximamente" }: { children?: ReactNode }) {
  return (
    <span className="bg-secondary text-etiqueta text-muted-foreground rounded-full px-2 py-0.5 font-medium whitespace-nowrap">
      {children}
    </span>
  );
}
