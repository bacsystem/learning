"use client";

import { useTransition } from "react";

import { SelectContent } from "@/components/ui/select";
import { cn } from "@/lib/utils";
import { Select as SelectPrimitive } from "@base-ui/react/select";
import { CaretUpDownIcon, CheckIcon } from "@phosphor-icons/react";

export type ContextoItem = {
  id: string;
  /** Línea principal (razón social, nombre del proyecto, organización…). */
  nombre: string;
  /** Línea secundaria en mono (RUC, plan, entorno…). */
  detalle?: string;
  /** Inicial del avatar; por defecto la primera letra del nombre. */
  inicial?: string;
};

function Avatar({ item, activo }: { item: ContextoItem; activo: boolean }) {
  return (
    <div
      className={cn(
        "text-etiqueta flex size-6 shrink-0 items-center justify-center rounded-md font-bold",
        activo ? "bg-primary text-primary-foreground shadow-xs" : "bg-secondary text-foreground/80",
      )}
    >
      {(item.inicial ?? item.nombre.slice(0, 1)).toUpperCase()}
    </div>
  );
}

/**
 * Selector de contexto del sidebar (empresa/organización activa): tarjeta como disparador y popup del mismo ancho.
 * `onCambio` puede ser async (p. ej. una server action que fija una cookie); mientras resuelve, el disparador se atenúa.
 */
export function SelectorContexto({
  items,
  activoId,
  etiqueta = "Cambiar de contexto",
  onCambio,
}: {
  items: ContextoItem[];
  activoId?: string;
  etiqueta?: string;
  onCambio: (id: string) => void | Promise<void>;
}) {
  const [cambiando, startTransition] = useTransition();
  const activo = items.find((i) => i.id === activoId) ?? items[0];
  const mapa = Object.fromEntries(items.map((i) => [i.id, i.nombre]));

  function cambiar(id: string | null) {
    if (!id || id === activo?.id) return;
    startTransition(async () => {
      await onCambio(id);
    });
  }

  return (
    <SelectPrimitive.Root items={mapa} value={activo?.id ?? null} onValueChange={cambiar}>
      <SelectPrimitive.Trigger
        aria-label={etiqueta}
        disabled={cambiando}
        className={cn(
          "border-border/60 bg-muted/80 flex w-full min-w-0 cursor-pointer items-center justify-between gap-2 overflow-hidden rounded-lg border p-2 text-left transition-colors outline-none select-none",
          "hover:bg-secondary focus-visible:ring-ring/50 data-[popup-open]:bg-secondary focus-visible:ring-2 disabled:pointer-events-none disabled:opacity-60",
        )}
      >
        <div className="flex min-w-0 items-center gap-2.5">
          {activo ? <Avatar item={activo} activo /> : null}
          <div className="min-w-0 overflow-hidden">
            <p className="text-control text-foreground truncate leading-tight font-medium">{activo?.nombre}</p>
            {activo?.detalle ? (
              <p className="text-etiqueta text-muted-foreground truncate font-mono leading-tight">{activo.detalle}</p>
            ) : null}
          </div>
        </div>
        <CaretUpDownIcon weight="bold" className="text-muted-foreground/70 size-4 shrink-0" />
      </SelectPrimitive.Trigger>

      <SelectContent
        align="start"
        alignItemWithTrigger={false}
        sideOffset={6}
        className="w-(--anchor-width) min-w-0 p-1"
      >
        <div className="text-etiqueta text-muted-foreground px-2 pt-1.5 pb-1 font-medium">{etiqueta}</div>
        {items.map((item) => (
          <SelectPrimitive.Item
            key={item.id}
            value={item.id}
            className="data-highlighted:bg-accent/70 data-highlighted:text-accent-foreground relative flex w-full cursor-default items-center gap-2.5 rounded-md py-1.5 pr-8 pl-2 outline-none select-none"
          >
            <Avatar item={item} activo={item.id === activo?.id} />
            <div className="min-w-0 flex-1 overflow-hidden">
              <SelectPrimitive.ItemText className="text-control block truncate leading-tight font-medium">
                {item.nombre}
              </SelectPrimitive.ItemText>
              {item.detalle ? (
                <span className="text-etiqueta text-muted-foreground block truncate font-mono leading-tight">
                  {item.detalle}
                </span>
              ) : null}
            </div>
            <SelectPrimitive.ItemIndicator className="text-primary absolute right-2 flex size-4 items-center justify-center">
              <CheckIcon weight="bold" className="size-4" />
            </SelectPrimitive.ItemIndicator>
          </SelectPrimitive.Item>
        ))}
      </SelectContent>
    </SelectPrimitive.Root>
  );
}
