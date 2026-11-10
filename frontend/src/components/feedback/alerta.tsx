import type { ReactNode } from "react";

import { type IconoComponente, PESO_ICONO } from "@/lib/iconos";
import { cn } from "@/lib/utils";
import { CheckCircleIcon, InfoIcon, WarningCircleIcon, WarningIcon } from "@phosphor-icons/react/dist/ssr";

type Tono = "info" | "ok" | "aviso" | "error";

const ESTILOS: Record<Tono, { caja: string; icono: IconoComponente }> = {
  info: { caja: "bg-accent text-accent-foreground", icono: InfoIcon },
  ok: { caja: "bg-success text-success-foreground", icono: CheckCircleIcon },
  aviso: { caja: "bg-warning text-warning-foreground", icono: WarningIcon },
  error: { caja: "bg-destructive-suave text-destructive-texto", icono: WarningCircleIcon },
};

/**
 * Alerta de página o de sección: banda con borde, icono, título opcional, texto y acción a la derecha.
 * Para avisos dentro de un diálogo usa `Aviso` (patrones/cabecera-dialogo).
 */
export function Alerta({
  tono = "info",
  titulo,
  children,
  accion,
  icon,
  className,
}: {
  tono?: Tono;
  titulo?: ReactNode;
  children: ReactNode;
  accion?: ReactNode;
  icon?: IconoComponente;
  className?: string;
}) {
  const Icono = icon ?? ESTILOS[tono].icono;
  return (
    <div
      role={tono === "error" ? "alert" : "status"}
      className={cn("flex flex-wrap items-start gap-3 rounded-[14px] px-4 py-3", ESTILOS[tono].caja, className)}
    >
      <Icono weight={PESO_ICONO} className="mt-0.5 size-4 shrink-0" />
      <div className="text-control min-w-0 flex-1 basis-56 leading-relaxed">
        {titulo ? <p className="font-semibold">{titulo}</p> : null}
        <div className={cn(titulo && "mt-0.5 opacity-90")}>{children}</div>
      </div>
      {accion ? <div className="shrink-0 self-center">{accion}</div> : null}
    </div>
  );
}
