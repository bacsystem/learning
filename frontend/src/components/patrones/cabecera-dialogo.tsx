import type { ReactNode } from "react";

import { DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { type IconoComponente, PESO_ICONO } from "@/lib/iconos";
import { cn } from "@/lib/utils";

/** Color del círculo del icono de un diálogo. `peligro` para acciones irreversibles. */
export type TonoDialogo = "neutro" | "peligro" | "aviso" | "ok";

const CIRCULO: Record<TonoDialogo, string> = {
  neutro: "bg-accent text-accent-foreground",
  peligro: "bg-destructive-suave text-destructive-texto",
  aviso: "bg-warning text-warning-foreground",
  ok: "bg-success text-success-foreground",
};

/**
 * Cabecera estándar de diálogo: icono en un círculo del tono (la única excepción a «icono sin fondo»: es la cara del
 * diálogo), título y descripción. Sin línea debajo: el espacio separa. Úsala dentro de `<DialogContent className="gap-0 p-0">`;
 * el cuerpo va en `px-5 pb-2` y el pie en `PieDialogo`.
 */
export function CabeceraDialogo({
  icon: Icon,
  titulo,
  descripcion,
  tono = "neutro",
}: {
  icon: IconoComponente;
  titulo: ReactNode;
  descripcion?: ReactNode;
  tono?: TonoDialogo;
}) {
  return (
    <DialogHeader className="px-5 pt-5 pr-14 pb-3">
      <div className="flex items-start gap-3">
        <span className={cn("grid size-10 shrink-0 place-items-center rounded-full", CIRCULO[tono])}>
          <Icon aria-hidden weight={PESO_ICONO} className="size-[22px]" />
        </span>
        <div className="min-w-0 pt-0.5">
          <DialogTitle>{titulo}</DialogTitle>
          {descripcion ? <DialogDescription className="mt-0.5">{descripcion}</DialogDescription> : null}
        </div>
      </div>
    </DialogHeader>
  );
}

/** Pie de diálogo; con `izquierda` (p. ej. un enlace) reparte los extremos. Sin línea encima. */
export function PieDialogo({
  children,
  izquierda,
  className,
}: {
  children: ReactNode;
  izquierda?: ReactNode;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "flex flex-wrap items-center gap-2 px-5 pt-3 pb-5",
        izquierda ? "justify-between" : "justify-end",
        className,
      )}
    >
      {izquierda}
      <div className="flex items-center gap-2">{children}</div>
    </div>
  );
}

/**
 * Aviso dentro de un diálogo o formulario: ámbar para advertencias, verde para confirmaciones, `peligro` para lo que una acción destructiva va a hacer
 * (dar de baja, anular; el tinte de error de `Alerta`) y `neutro` para un efecto que no alarma (la vista previa de un cambio).
 */
export function Aviso({
  tono,
  icon: Icon,
  children,
}: {
  tono: "aviso" | "ok" | "peligro" | "neutro";
  icon: IconoComponente;
  children: ReactNode;
}) {
  return (
    <div
      className={cn(
        "text-control flex items-start gap-2.5 rounded-[14px] px-3 py-2.5 leading-relaxed",
        tono === "aviso" && "bg-warning text-warning-foreground",
        tono === "ok" && "bg-success text-success-foreground",
        tono === "peligro" && "bg-destructive-suave text-destructive-texto",
        tono === "neutro" && "bg-secondary text-foreground",
      )}
    >
      <Icon aria-hidden weight={PESO_ICONO} className="mt-0.5 size-4 shrink-0" />
      <div>{children}</div>
    </div>
  );
}
