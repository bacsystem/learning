"use client";

import { CAMPO, POPUP_LISTA } from "@/lib/estilos";
import { cn } from "@/lib/utils";
import { Autocomplete as AutocompletePrimitive } from "@base-ui/react/autocomplete";
import { MagnifyingGlassIcon, XIcon } from "@phosphor-icons/react";

/**
 * Búsqueda de texto libre con sugerencias (a diferencia de `Combobox`, no obliga a elegir un valor de la
 * lista: el usuario puede quedarse con lo que escribió). Útil para nombres de producto, glosas o cualquier
 * campo donde las sugerencias ayudan pero no son las únicas respuestas válidas.
 */
export function Autocomplete({
  id,
  items,
  valor,
  onCambio,
  placeholder = "Buscar…",
  vacio = "Sin sugerencias",
  disabled,
  className,
}: {
  id: string;
  items: string[];
  valor?: string;
  onCambio?: (v: string) => void;
  placeholder?: string;
  vacio?: string;
  disabled?: boolean;
  className?: string;
}) {
  return (
    <AutocompletePrimitive.Root items={items} value={valor} onValueChange={(v) => onCambio?.(v)} disabled={disabled}>
      <div className="relative flex items-center">
        <AutocompletePrimitive.Input id={id} placeholder={placeholder} className={cn(CAMPO, "pr-9", className)} />
        <AutocompletePrimitive.Clear
          aria-label="Limpiar"
          className="text-muted-foreground/70 hover:text-foreground absolute top-1/2 right-2 -translate-y-1/2 rounded p-0.5"
        >
          <XIcon weight="bold" className="size-4" />
        </AutocompletePrimitive.Clear>
        <MagnifyingGlassIcon
          weight="bold"
          className="text-muted-foreground/70 pointer-events-none absolute top-1/2 right-3 size-4 -translate-y-1/2"
        />
      </div>
      <AutocompletePrimitive.Portal>
        <AutocompletePrimitive.Positioner sideOffset={6} className="isolate z-50">
          <AutocompletePrimitive.Popup className={POPUP_LISTA}>
            <AutocompletePrimitive.Empty className="text-control text-muted-foreground px-2 py-2 empty:hidden">
              {vacio}
            </AutocompletePrimitive.Empty>
            <AutocompletePrimitive.List>
              {(item: string) => (
                <AutocompletePrimitive.Item
                  key={item}
                  value={item}
                  className="text-control data-highlighted:bg-accent/70 data-highlighted:text-accent-foreground relative flex w-full cursor-default items-center rounded-md px-2 py-1.5 font-medium outline-none select-none"
                >
                  {item}
                </AutocompletePrimitive.Item>
              )}
            </AutocompletePrimitive.List>
          </AutocompletePrimitive.Popup>
        </AutocompletePrimitive.Positioner>
      </AutocompletePrimitive.Portal>
    </AutocompletePrimitive.Root>
  );
}
