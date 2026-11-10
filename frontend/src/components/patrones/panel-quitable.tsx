"use client";

import type { ReactNode } from "react";

import { Button } from "@/components/ui/button";
import { ETIQUETA_CAMPO } from "@/lib/estilos";
import { cn } from "@/lib/utils";
import { XIcon } from "@phosphor-icons/react";

/**
 * Un bloque opcional de un formulario que el usuario agregó con un botón «+» (crédito, detracción, referencias…) y puede quitar con la ✕.
 * Lo de adentro va sin borde propio: el recuadro es este. Quitar es cosa de quien lo usa (`onQuitar` apaga la opción y limpia sus datos).
 */
export function PanelQuitable({
  titulo,
  etiquetaQuitar,
  onQuitar,
  children,
  className,
  ...resto
}: {
  titulo: ReactNode;
  /** Nombre accesible y `title` de la ✕ («Quitar crédito»). */
  etiquetaQuitar: string;
  onQuitar: () => void;
  children: ReactNode;
  className?: string;
  "data-testid"?: string;
}) {
  return (
    <section className={cn("border-border grid gap-3 rounded-xl border px-3 pt-2 pb-3", className)} {...resto}>
      <div className="flex items-center justify-between gap-2">
        <h4 className={ETIQUETA_CAMPO}>{titulo}</h4>
        <Button
          type="button"
          variant="ghost"
          size="icon-sm"
          onClick={onQuitar}
          aria-label={etiquetaQuitar}
          title={etiquetaQuitar}
          className="text-muted-foreground"
        >
          <XIcon weight="bold" className="size-4" />
        </Button>
      </div>
      {children}
    </section>
  );
}
