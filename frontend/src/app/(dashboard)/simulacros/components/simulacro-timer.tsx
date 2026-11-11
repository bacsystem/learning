"use client";

import { useTranslations } from "next-intl";
import { useEffect, useRef, useState } from "react";

import { DialogContent } from "@/components/app/dialogo";
import { CabeceraDialogo, PieDialogo } from "@/components/patrones/cabecera-dialogo";
import { Dialog } from "@/components/ui/dialog";
import { BOTON_DESTRUCTIVO } from "@/lib/estilos";
import { PESO_ICONO } from "@/lib/iconos";
import { cn } from "@/lib/utils";
import { TimerIcon } from "@phosphor-icons/react";

interface SimulacroTimerProps {
  tiempoRestanteInicial: number; // from server
  onExpired: () => void;
}

function formatTime(seconds: number): string {
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = seconds % 60;
  if (h > 0) {
    return `${String(h).padStart(2, "0")}:${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")}`;
  }
  return `${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")}`;
}

export function SimulacroTimer({ tiempoRestanteInicial, onExpired }: SimulacroTimerProps) {
  const t = useTranslations("dashboard");
  const [remaining, setRemaining] = useState(tiempoRestanteInicial);
  const [expiredDialogOpen, setExpiredDialogOpen] = useState(false);
  const expiredRef = useRef(false);
  const deadlineRef = useRef(Date.now() + tiempoRestanteInicial * 1000);

  useEffect(() => {
    deadlineRef.current = Date.now() + tiempoRestanteInicial * 1000;
    setRemaining(tiempoRestanteInicial);
    expiredRef.current = false;
  }, [tiempoRestanteInicial]);

  useEffect(() => {
    const interval = setInterval(() => {
      const rem = Math.max(0, Math.round((deadlineRef.current - Date.now()) / 1000));
      setRemaining(rem);
      if (rem === 0 && !expiredRef.current) {
        expiredRef.current = true;
        clearInterval(interval);
        setExpiredDialogOpen(true);
      }
    }, 1000);
    return () => clearInterval(interval);
  }, []);

  const isUrgent = remaining > 0 && remaining <= 300; // 5 minutes

  return (
    <>
      <div
        role="timer"
        aria-live="off"
        className={cn(
          "flex items-center gap-3 rounded-xl px-3.5 py-2 transition-colors",
          isUrgent ? "bg-destructive-suave text-destructive-texto" : "bg-card text-foreground ring-border ring-1",
        )}
      >
        <TimerIcon
          aria-hidden
          weight={PESO_ICONO}
          className={cn("size-5 shrink-0", !isUrgent && "text-muted-foreground")}
        />
        <div className="text-right">
          <span className={cn("text-etiqueta block", isUrgent ? "text-current" : "text-muted-foreground")}>
            {t("simulacros-tiempo-restante")}
          </span>
          <span className="text-metrica block tabular-nums">{formatTime(remaining)}</span>
        </div>
      </div>

      {/* Sin cierre: el tiempo terminó y la única salida es enviar. */}
      <Dialog open={expiredDialogOpen} onOpenChange={() => {}}>
        <DialogContent className="gap-0 p-0 sm:max-w-md" showCloseButton={false}>
          <CabeceraDialogo
            icon={TimerIcon}
            tono="aviso"
            titulo={t("simulacros-tiempo-agotado")}
            descripcion={t("simulacros-envio-automatico")}
          />
          <PieDialogo>
            <button
              type="button"
              className={BOTON_DESTRUCTIVO}
              onClick={() => {
                setExpiredDialogOpen(false);
                onExpired();
              }}
            >
              {t("simulacros-enviar-todo")}
            </button>
          </PieDialogo>
        </DialogContent>
      </Dialog>
    </>
  );
}
