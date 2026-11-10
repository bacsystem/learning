"use client";

import Link from "next/link";
import type { ReactNode } from "react";

import { Skeleton } from "@/components/feedback/skeleton";
import { CabeceraSeccion } from "@/components/patrones/cabecera-seccion";
import { TARJETA } from "@/lib/estilos";
import { type IconoComponente, PESO_ICONO, PESO_ICONO_FUNCIONAL } from "@/lib/iconos";
import { cn } from "@/lib/utils";
import { CaretRightIcon } from "@phosphor-icons/react";

/** Tarjeta de una sección del panel: cabecera con icono y título, y el cuerpo con el padding de la tarjeta. */
export function Seccion({
  icon,
  titulo,
  subtitulo,
  derecha,
  children,
  className,
  cuerpoClassName,
}: {
  icon: IconoComponente;
  titulo: ReactNode;
  subtitulo?: ReactNode;
  derecha?: ReactNode;
  children: ReactNode;
  className?: string;
  cuerpoClassName?: string;
}) {
  return (
    <section className={cn(TARJETA, "flex min-w-0 flex-col", className)}>
      <CabeceraSeccion icon={icon} titulo={titulo} subtitulo={subtitulo} derecha={derecha} />
      <div className={cn("min-w-0 flex-1 px-4 pt-1 pb-4", cuerpoClassName)}>{children}</div>
    </section>
  );
}

/** Estado vacío compacto dentro de una sección: icono neutro y mensaje, con acción opcional. */
export function VacioSeccion({
  icon: Icono,
  children,
  accion,
  className,
}: {
  icon: IconoComponente;
  children: ReactNode;
  accion?: ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("flex min-h-40 flex-col items-center justify-center gap-2 py-6 text-center", className)}>
      <Icono aria-hidden weight={PESO_ICONO} className="text-texto-terciario size-6" />
      <p className="text-control text-muted-foreground">{children}</p>
      {accion ? <div className="mt-1">{accion}</div> : null}
    </div>
  );
}

/** Bloque de carga con la altura del gráfico que reemplaza. */
export function CargandoGrafico({ alto = 220 }: { alto?: number }) {
  return (
    <div aria-busy="true" className="flex flex-col gap-2">
      <Skeleton className="h-3 w-32" />
      <div aria-hidden style={{ height: alto }} className="bg-secondary w-full animate-pulse rounded-lg" />
    </div>
  );
}

/** Enlace «Ver todos» de la cabecera de una sección. */
export function EnlaceVerTodos({ href, children }: { href: string; children: ReactNode }) {
  return (
    <Link
      href={href}
      className="h-control text-control text-muted-foreground hover:bg-secondary hover:text-foreground inline-flex items-center gap-1 rounded-full px-2.5 font-medium transition-colors"
    >
      {children}
      <CaretRightIcon aria-hidden weight={PESO_ICONO_FUNCIONAL} className="size-3.5 rtl:rotate-180" />
    </Link>
  );
}

/** Tono de rendimiento según el porcentaje de aciertos (≥ 60 aprobado, ≥ 40 en riesgo). */
export function tonoRendimiento(porcentaje: number): "ok" | "aviso" | "error" {
  return porcentaje >= 60 ? "ok" : porcentaje >= 40 ? "aviso" : "error";
}

export const TONO_PROGRESO = { ok: "success", aviso: "warning", error: "destructive" } as const;
