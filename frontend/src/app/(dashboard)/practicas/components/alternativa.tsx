"use client";

import type { ReactNode } from "react";

import { FOCO, PRESION } from "@/lib/estilos";
import { cn } from "@/lib/utils";
import { CheckCircleIcon, XCircleIcon } from "@phosphor-icons/react";

/** Estado visual de una alternativa: sin marcar, marcada por el alumno, la correcta o la marcada que resultó incorrecta. */
export type EstadoAlternativa = "none" | "selected" | "correct" | "wrong";

const CAJA: Record<EstadoAlternativa, string> = {
  none: "bg-card ring-1 ring-border",
  selected: "bg-accent text-accent-foreground ring-2 ring-acento-texto dark:ring-acento",
  correct: "bg-success text-success-foreground ring-1 ring-success-border",
  wrong: "bg-destructive-suave text-destructive-texto ring-1 ring-destructive-border",
};

const LETRA: Record<EstadoAlternativa, string> = {
  none: "bg-secondary text-muted-foreground",
  selected: "bg-primary text-primary-foreground",
  correct: "bg-success-foreground text-card",
  wrong: "bg-destructive text-destructive-foreground",
};

/**
 * Alternativa de una pregunta: letra en círculo + texto. Con `onSeleccionar` es un botón conmutable (`aria-pressed`);
 * sin él (validada o en revisión) es solo lectura. Correcto/incorrecto también van en icono, no solo en color.
 */
export function Alternativa({
  letra,
  texto,
  estado,
  onSeleccionar,
  compacta = false,
  elegida,
  estadoSr,
}: {
  letra: ReactNode;
  texto: ReactNode;
  estado: EstadoAlternativa;
  onSeleccionar?: () => void;
  /** Revisión: menos alto y texto de cuerpo. */
  compacta?: boolean;
  /**
   * Revisión: si el alumno eligió esta alternativa. Elegida lleva borde doble; la correcta que no eligió muestra su check atenuado.
   * Sin definir (examen en curso) no cambia nada.
   */
  elegida?: boolean;
  /** Revisión: el estado en texto para lectores de pantalla (no depende solo del color y del icono). */
  estadoSr?: string;
}) {
  const interactiva = !!onSeleccionar;
  const contenido = (
    <>
      <span
        aria-hidden
        className={cn(
          "ease-salida grid shrink-0 place-items-center rounded-full font-semibold transition-colors duration-(--duracion-rapida)",
          compacta ? "text-etiqueta size-7" : "text-control size-9",
          LETRA[estado],
        )}
      >
        {letra}
      </span>
      <span
        className={cn(
          "min-w-0 flex-1 text-left whitespace-pre-line",
          compacta ? "text-cuerpo" : "text-titulo font-normal",
          estado !== "none" && "font-medium",
          estado === "none" && (compacta ? "text-muted-foreground" : "text-foreground"),
        )}
      >
        {texto}
        {estadoSr ? <span className="sr-only">{` (${estadoSr})`}</span> : null}
      </span>
      {estado === "correct" ? (
        <CheckCircleIcon
          aria-hidden
          weight="fill"
          className={cn("shrink-0", compacta ? "size-4" : "size-5", elegida === false && "opacity-50")}
        />
      ) : estado === "wrong" ? (
        <XCircleIcon aria-hidden weight="fill" className={cn("shrink-0", compacta ? "size-4" : "size-5")} />
      ) : null}
    </>
  );

  const base = cn(
    "flex w-full items-center rounded-xl transition-[background-color,box-shadow,color] duration-(--duracion-rapida) ease-salida",
    compacta ? "gap-3 px-3 py-2.5" : "min-h-14 gap-3.5 px-4 py-3",
    CAJA[estado],
    elegida && "ring-2",
  );

  if (!interactiva) {
    return (
      <div className={base} data-estado={estado}>
        {contenido}
      </div>
    );
  }

  return (
    <button
      type="button"
      aria-pressed={estado === "selected"}
      onClick={onSeleccionar}
      data-estado={estado}
      className={cn(base, "cursor-pointer", FOCO, PRESION, estado === "none" && "hover:bg-secondary hover:ring-input")}
    >
      {contenido}
    </button>
  );
}
