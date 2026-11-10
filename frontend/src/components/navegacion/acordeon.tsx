"use client";

import type { ReactNode } from "react";

import { cn } from "@/lib/utils";
import { Accordion as AccordionPrimitive } from "@base-ui/react/accordion";
import { CaretDownIcon } from "@phosphor-icons/react";

export type ItemAcordeon = { id: string; titulo: ReactNode; detalle?: ReactNode; contenido: ReactNode };

/** Acordeón de secciones plegables (FAQ, configuración avanzada, historial). Varios abiertos a la vez con `multiple`. */
export function Acordeon({
  items,
  defaultAbiertos = [],
  multiple = false,
  className,
}: {
  items: ItemAcordeon[];
  defaultAbiertos?: string[];
  multiple?: boolean;
  className?: string;
}) {
  return (
    <AccordionPrimitive.Root
      defaultValue={defaultAbiertos}
      multiple={multiple}
      className={cn("divide-border bg-secondary divide-y rounded-[14px]", className)}
    >
      {items.map((it) => (
        <AccordionPrimitive.Item key={it.id} value={it.id}>
          <AccordionPrimitive.Header>
            <AccordionPrimitive.Trigger className="group focus-visible:ring-acento/55 flex w-full items-center gap-3 px-4 py-3 text-left outline-none focus-visible:ring-2">
              <span className="min-w-0 flex-1">
                <span className="text-cuerpo text-foreground block font-semibold">{it.titulo}</span>
                {it.detalle ? <span className="text-etiqueta text-texto-terciario block">{it.detalle}</span> : null}
              </span>
              <CaretDownIcon
                weight="bold"
                className="text-muted-foreground size-4 shrink-0 transition-transform group-data-panel-open:rotate-180"
              />
            </AccordionPrimitive.Trigger>
          </AccordionPrimitive.Header>
          <AccordionPrimitive.Panel className="text-control text-muted-foreground h-(--accordion-panel-height) overflow-hidden transition-[height] duration-(--duracion-media) data-ending-style:h-0 data-starting-style:h-0">
            <div className="px-4 pb-4">{it.contenido}</div>
          </AccordionPrimitive.Panel>
        </AccordionPrimitive.Item>
      ))}
    </AccordionPrimitive.Root>
  );
}
