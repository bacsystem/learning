"use client";

import { type ReactNode, useState } from "react";

import { CabeceraDialogo, PieDialogo } from "@/components/patrones/cabecera-dialogo";
import { Dialog, DialogContent } from "@/components/ui/dialog";
import { BOTON_DESTRUCTIVO, BOTON_PRIMARIO, BOTON_SECUNDARIO } from "@/lib/estilos";
import type { IconoComponente } from "@/lib/iconos";
import { WarningIcon } from "@phosphor-icons/react";

/**
 * Confirmación modal para acciones irreversibles que NO caben en línea (afectan a varios registros o necesitan explicación).
 * `onConfirmar` puede devolver un mensaje de error para mostrarlo sin cerrar.
 */
export function DialogoConfirmacion({
  open,
  onOpenChange,
  icon = WarningIcon,
  titulo,
  descripcion,
  children,
  textoConfirmar = "Confirmar",
  textoCancelar = "Cancelar",
  textoPendiente = "Procesando…",
  destructiva = true,
  onConfirmar,
}: {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  icon?: IconoComponente;
  titulo: ReactNode;
  descripcion?: ReactNode;
  children?: ReactNode;
  textoConfirmar?: string;
  /** Textos del botón de cancelar y del estado pendiente, para traducirlos (por defecto en castellano). */
  textoCancelar?: string;
  textoPendiente?: string;
  destructiva?: boolean;
  onConfirmar: () => Promise<string | null | void> | string | null | void;
}) {
  const [pendiente, setPendiente] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function confirmar() {
    setPendiente(true);
    setError(null);
    try {
      const r = await onConfirmar();
      if (r) {
        setError(r);
        return;
      }
      onOpenChange(false);
    } finally {
      setPendiente(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="gap-0 p-0 sm:max-w-md">
        <CabeceraDialogo
          icon={icon}
          titulo={titulo}
          descripcion={descripcion}
          tono={destructiva ? "peligro" : "neutro"}
        />
        {children || error ? (
          <div className="text-control text-muted-foreground grid gap-3 px-5 pb-2 leading-relaxed">
            {children}
            {error ? <p className="text-destructive-texto">{error}</p> : null}
          </div>
        ) : null}
        <PieDialogo>
          <button type="button" disabled={pendiente} onClick={() => onOpenChange(false)} className={BOTON_SECUNDARIO}>
            {textoCancelar}
          </button>
          <button
            type="button"
            disabled={pendiente}
            onClick={confirmar}
            className={destructiva ? BOTON_DESTRUCTIVO : BOTON_PRIMARIO}
          >
            {pendiente ? textoPendiente : textoConfirmar}
          </button>
        </PieDialogo>
      </DialogContent>
    </Dialog>
  );
}
