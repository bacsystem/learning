import type { ReactNode } from "react";

import { cn } from "@/lib/utils";
import { CheckIcon } from "@phosphor-icons/react/dist/ssr";

export type Paso = { titulo: ReactNode; descripcion?: ReactNode };

/**
 * Indicador de pasos (onboarding, asistentes). `actual` es 0-based; los anteriores se marcan completados.
 * Cada paso ocupa el mismo ancho, con el círculo y el texto centrados en su columna; la línea une los centros de los círculos
 * (primaria hasta el paso actual). En móvil se ocultan las descripciones para que los títulos quepan en una fila.
 */
export function Pasos({ pasos, actual, className }: { pasos: Paso[]; actual: number; className?: string }) {
  return (
    <ol className={cn("grid auto-cols-fr grid-flow-col items-start", className)}>
      {pasos.map((p, i) => {
        const estado = i < actual ? "hecho" : i === actual ? "actual" : "pendiente";
        return (
          <li key={i} className="relative flex min-w-0 flex-col items-center gap-2 px-1 text-center">
            {/* Tramo desde el paso anterior: de centro a centro de los círculos (size-7 = 28 px), con 8 px de aire a cada lado. */}
            {i > 0 ? (
              <span
                aria-hidden="true"
                className={cn(
                  "absolute top-3.5 right-[calc(50%+22px)] left-[calc(-50%+22px)] h-px",
                  i <= actual ? "bg-primary" : "bg-border",
                )}
              />
            ) : null}
            <span
              aria-current={estado === "actual" ? "step" : undefined}
              className={cn(
                "text-etiqueta relative flex size-7 shrink-0 items-center justify-center rounded-full border font-mono font-semibold",
                estado === "hecho" && "border-primary bg-primary text-primary-foreground",
                estado === "actual" && "border-primary bg-accent text-primary ring-ring/20 ring-3",
                estado === "pendiente" && "border-border bg-muted text-muted-foreground",
              )}
            >
              {estado === "hecho" ? <CheckIcon className="size-3.5" weight="bold" /> : i + 1}
            </span>
            <div className="min-w-0">
              <p
                className={cn(
                  "text-control font-medium",
                  estado === "pendiente" ? "text-muted-foreground" : "text-foreground",
                )}
              >
                {p.titulo}
              </p>
              {p.descripcion ? (
                <p className="text-etiqueta text-muted-foreground hidden sm:block">{p.descripcion}</p>
              ) : null}
            </div>
          </li>
        );
      })}
    </ol>
  );
}

/** Tarjeta de paso del asistente: cabecera con "Paso n de N", contenido y pie con Atrás/Continuar. */
export function TarjetaPaso({
  numero,
  total,
  titulo,
  children,
  pie,
  className,
}: {
  numero: number;
  total: number;
  titulo: ReactNode;
  children: ReactNode;
  pie?: ReactNode;
  className?: string;
}) {
  return (
    <section className={cn("border-border bg-card rounded-xl border shadow-xs", className)}>
      <div className="border-border/60 border-b px-5 py-4">
        <span className="text-etiqueta text-muted-foreground">
          Paso {numero} de {total}
        </span>
        <h2 className="text-titulo text-foreground mt-0.5">{titulo}</h2>
      </div>
      <div className="grid gap-4 px-5 py-4">{children}</div>
      {pie ? (
        <div className="border-border/60 flex items-center justify-end gap-2 border-t px-5 py-3">{pie}</div>
      ) : null}
    </section>
  );
}
