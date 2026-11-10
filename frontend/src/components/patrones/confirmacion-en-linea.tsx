"use client";

import { AnimatePresence, m } from "motion/react";
import { useState, useTransition } from "react";

import { MovimientoLocal } from "@/components/movimiento";
import type { IconoComponente } from "@/lib/iconos";
import { CURVA_SALIDA, DURACION } from "@/lib/movimiento";
import { cn } from "@/lib/utils";
import { ArrowClockwiseIcon, ProhibitIcon } from "@phosphor-icons/react";

const ACCION =
  "inline-flex h-control items-center gap-1 rounded-lg px-2.5 text-control font-medium whitespace-nowrap transition-colors duration-(--duracion-rapida) ease-salida disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-60";

// La acción sale hacia la izquierda y la confirmación entra desde la derecha (y al revés al cancelar).
const CAMBIO = {
  initial: { opacity: 0, x: 8 },
  animate: { opacity: 1, x: 0 },
  exit: { opacity: 0, x: -8 },
  transition: { duration: DURACION.rapida, ease: CURVA_SALIDA },
};

/**
 * Acción irreversible confirmada en línea (sin diálogo): "Revocar" → "¿Revocar de forma permanente? [Sí, revocar] [Cancelar]".
 * `onConfirmar` puede devolver un mensaje de error para mostrarlo debajo.
 */
export function ConfirmacionEnLinea({
  etiqueta,
  pregunta,
  confirmar,
  icon: Icon = ProhibitIcon,
  onConfirmar,
}: {
  etiqueta: string;
  pregunta: string;
  confirmar: string;
  icon?: IconoComponente;
  onConfirmar: () => Promise<string | null | void>;
}) {
  const [abierto, setAbierto] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [pendiente, startTransition] = useTransition();

  function ejecutar() {
    setError(null);
    startTransition(async () => {
      const resultado = await onConfirmar();
      if (resultado) {
        setError(resultado);
        return;
      }
      setAbierto(false);
    });
  }

  return (
    <div className="flex flex-col items-end gap-1">
      {/* `relative`: `popLayout` saca de flujo el bloque que sale y lo posiciona contra este contenedor. */}
      <div className="relative flex items-center justify-end">
        <MovimientoLocal>
          <AnimatePresence mode="popLayout" initial={false}>
            {abierto ? (
              <m.div key="confirmar" {...CAMBIO} className="flex items-center justify-end gap-1">
                <span className="text-etiqueta text-muted-foreground mr-1">{pregunta}</span>
                <button
                  type="button"
                  disabled={pendiente}
                  onClick={ejecutar}
                  className={cn(ACCION, "bg-destructive text-destructive-foreground hover:bg-destructive/90 shadow-xs")}
                >
                  {pendiente ? (
                    <ArrowClockwiseIcon weight="bold" className="size-3.5 animate-spin" />
                  ) : (
                    <Icon className="size-3.5" />
                  )}
                  {confirmar}
                </button>
                <button
                  type="button"
                  disabled={pendiente}
                  onClick={() => setAbierto(false)}
                  className={cn(ACCION, "border-border bg-card text-foreground hover:bg-secondary border shadow-xs")}
                >
                  Cancelar
                </button>
              </m.div>
            ) : (
              <m.div key="accion" {...CAMBIO}>
                <button
                  type="button"
                  onClick={() => setAbierto(true)}
                  className={cn(ACCION, "text-destructive-texto hover:bg-destructive-suave")}
                >
                  <Icon className="size-3.5" />
                  {etiqueta}
                </button>
              </m.div>
            )}
          </AnimatePresence>
        </MovimientoLocal>
      </div>
      {error ? <span className="text-etiqueta text-destructive-texto">{error}</span> : null}
    </div>
  );
}
