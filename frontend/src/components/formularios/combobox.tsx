"use client";

import { CAMPO, POPUP_LISTA } from "@/lib/estilos";
import { cn } from "@/lib/utils";
import { Combobox as ComboboxPrimitive } from "@base-ui/react/combobox";
import { CaretDownIcon, CheckIcon } from "@phosphor-icons/react";

export type ItemCombobox = { value: string; label: string; detalle?: string };

/**
 * Selector con búsqueda (clientes, productos, catálogos largos). Filtra por etiqueta y detalle mientras se escribe.
 */
export function Combobox({
  id,
  items,
  valor,
  onCambio,
  placeholder = "Buscar…",
  vacio = "Sin resultados",
  disabled,
  invalido,
  describidoPor,
  etiquetaAbrir = "Abrir opciones",
  className,
}: {
  id: string;
  items: ItemCombobox[];
  valor?: string | null;
  onCambio?: (v: string | null) => void;
  placeholder?: string;
  vacio?: string;
  disabled?: boolean;
  /** Marca el campo con error (`aria-invalid`, borde rojo de `CAMPO`) y lo une a su mensaje (`aria-describedby`). */
  invalido?: boolean;
  describidoPor?: string;
  /** Nombre accesible del botón que abre la lista. */
  etiquetaAbrir?: string;
  className?: string;
}) {
  const seleccionado = items.find((i) => i.value === valor) ?? null;
  return (
    <ComboboxPrimitive.Root
      items={items}
      value={seleccionado}
      onValueChange={(v: ItemCombobox | null) => onCambio?.(v?.value ?? null)}
      itemToStringLabel={(i: ItemCombobox) => i.label}
      disabled={disabled}
    >
      <div className="relative">
        <ComboboxPrimitive.Input
          id={id}
          placeholder={placeholder}
          aria-invalid={invalido || undefined}
          aria-describedby={describidoPor}
          className={cn(CAMPO, "pr-9", className)}
        />
        <ComboboxPrimitive.Trigger
          aria-label={etiquetaAbrir}
          className="text-muted-foreground/70 hover:text-foreground absolute top-1/2 right-2 -translate-y-1/2 rounded p-0.5"
        >
          <CaretDownIcon weight="bold" className="size-4" />
        </ComboboxPrimitive.Trigger>
      </div>
      <ComboboxPrimitive.Portal>
        <ComboboxPrimitive.Positioner sideOffset={6} className="isolate z-50">
          <ComboboxPrimitive.Popup className={POPUP_LISTA}>
            <ComboboxPrimitive.Empty className="text-control text-muted-foreground px-2 py-2 empty:hidden">
              {vacio}
            </ComboboxPrimitive.Empty>
            <ComboboxPrimitive.List>
              {(item: ItemCombobox) => (
                <ComboboxPrimitive.Item
                  key={item.value}
                  value={item}
                  className="text-control data-highlighted:bg-accent/70 data-highlighted:text-accent-foreground relative flex w-full cursor-default items-center gap-2 rounded-md py-1.5 pr-8 pl-2 font-medium outline-none select-none"
                >
                  <span className="grid min-w-0 flex-1">
                    <span className="truncate">{item.label}</span>
                    {item.detalle ? (
                      <span className="text-etiqueta text-muted-foreground truncate font-mono font-normal">
                        {item.detalle}
                      </span>
                    ) : null}
                  </span>
                  <ComboboxPrimitive.ItemIndicator className="text-primary absolute right-2">
                    <CheckIcon weight="bold" className="size-4" />
                  </ComboboxPrimitive.ItemIndicator>
                </ComboboxPrimitive.Item>
              )}
            </ComboboxPrimitive.List>
          </ComboboxPrimitive.Popup>
        </ComboboxPrimitive.Positioner>
      </ComboboxPrimitive.Portal>
    </ComboboxPrimitive.Root>
  );
}
