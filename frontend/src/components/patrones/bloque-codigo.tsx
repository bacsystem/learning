"use client";

import type { ReactNode } from "react";

import { BotonCopiar } from "@/components/patrones/boton-copiar";
import { cn } from "@/lib/utils";

/**
 * Bloque de código con barra superior (método + URL + slot para pestañas) y cuerpo en `codigo` (casi negro en los dos modos).
 * `tono="claro"` para respuestas/JSON sobre muted.
 */
export function BloqueCodigo({
  codigo,
  metodo,
  url,
  barra,
  pie,
  tono = "oscuro",
  alturaMax = "52vh",
  className,
}: {
  codigo: string;
  metodo?: string;
  url?: string;
  barra?: ReactNode;
  pie?: ReactNode;
  tono?: "oscuro" | "claro";
  alturaMax?: string;
  className?: string;
}) {
  return (
    <div className={cn("border-border bg-card min-w-0 overflow-hidden rounded-xl border shadow-sm", className)}>
      {metodo || url || barra ? (
        <div className="border-border bg-muted/60 flex flex-wrap items-center gap-2 border-b px-4 py-2">
          {/* El verbo HTTP es un literal: se escribe en mayúsculas con toUpperCase, no con una clase CSS de transformación. */}
          {metodo ? (
            <span className="bg-accent text-etiqueta text-accent-foreground rounded-md px-1.5 py-0.5 font-mono font-semibold">
              {metodo.toUpperCase()}
            </span>
          ) : null}
          {url ? <code className="text-control text-foreground min-w-0 flex-1 truncate font-mono">{url}</code> : null}
          {barra}
          <BotonCopiar texto={codigo} className="size-control rounded-lg" />
        </div>
      ) : null}
      <pre
        style={{ maxHeight: alturaMax }}
        className={cn(
          "text-control overflow-auto p-4 font-mono leading-relaxed",
          tono === "oscuro" ? "bg-codigo text-codigo-foreground" : "bg-muted/50 text-foreground",
        )}
      >
        <code>{codigo}</code>
      </pre>
      {pie ? <div className="border-border text-etiqueta text-muted-foreground border-t px-4 py-2">{pie}</div> : null}
    </div>
  );
}

/** Control segmentado pequeño (pestañas de lenguaje, filtros por tipo). */
export function Segmentado<T extends string>({
  opciones,
  valor,
  onCambio,
  className,
}: {
  opciones: { id: T; etiqueta: string }[];
  valor: T;
  onCambio: (v: T) => void;
  className?: string;
}) {
  return (
    <div className={cn("h-control bg-secondary inline-flex shrink-0 items-center gap-0.5 rounded-lg p-0.5", className)}>
      {opciones.map((o) => (
        <button
          key={o.id}
          type="button"
          onClick={() => onCambio(o.id)}
          className={cn(
            "text-control ease-salida h-7 rounded-md px-2.5 font-medium transition-colors duration-(--duracion-rapida)",
            valor === o.id
              ? "bg-card text-foreground dark:bg-input shadow-sm"
              : "text-muted-foreground hover:text-foreground",
          )}
        >
          {o.etiqueta}
        </button>
      ))}
    </div>
  );
}
