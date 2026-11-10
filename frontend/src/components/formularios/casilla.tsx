"use client";

import type { ReactNode } from "react";

import { cn } from "@/lib/utils";
import { Checkbox as CheckboxPrimitive } from "@base-ui/react/checkbox";
import { Switch as SwitchPrimitive } from "@base-ui/react/switch";
import { CheckIcon } from "@phosphor-icons/react";

/**
 * Checkbox con etiqueta y descripción opcional. `invalido` y `describidoPor` (el id del error) para una casilla obligatoria de un formulario (p. ej. aceptar
 * los términos); `name` y `required` para que un `<form>` nativo la envíe y la exija.
 */
export function Casilla({
  id,
  etiqueta,
  descripcion,
  checked,
  defaultChecked,
  onCheckedChange,
  disabled,
  invalido,
  describidoPor,
  name,
  required,
  className,
}: {
  id: string;
  etiqueta: ReactNode;
  descripcion?: ReactNode;
  checked?: boolean;
  defaultChecked?: boolean;
  onCheckedChange?: (v: boolean) => void;
  disabled?: boolean;
  invalido?: boolean;
  describidoPor?: string;
  name?: string;
  required?: boolean;
  className?: string;
}) {
  return (
    <label
      htmlFor={id}
      className={cn(
        "text-control flex cursor-pointer items-start gap-2.5",
        disabled && "cursor-not-allowed opacity-60",
        className,
      )}
    >
      <CheckboxPrimitive.Root
        id={id}
        checked={checked}
        defaultChecked={defaultChecked}
        onCheckedChange={onCheckedChange}
        disabled={disabled}
        name={name}
        required={required}
        aria-invalid={invalido || undefined}
        aria-describedby={describidoPor}
        className="border-input bg-card aria-invalid:border-destructive ease-salida focus-visible:ring-acento/55 data-checked:border-primary data-checked:bg-primary mt-px flex size-[18px] shrink-0 items-center justify-center rounded-[6px] border-[1.5px] transition-colors duration-(--duracion-rapida) outline-none focus-visible:ring-3"
      >
        <CheckboxPrimitive.Indicator className="text-primary-foreground">
          <CheckIcon className="size-3" weight="bold" />
        </CheckboxPrimitive.Indicator>
      </CheckboxPrimitive.Root>
      <span className="grid gap-0.5">
        <span className="text-foreground font-medium">{etiqueta}</span>
        {descripcion ? <span className="text-control text-texto-terciario">{descripcion}</span> : null}
      </span>
    </label>
  );
}

/** Interruptor (switch) con etiqueta a la izquierda; ideal para filas de configuración. */
export function Interruptor({
  id,
  etiqueta,
  descripcion,
  checked,
  defaultChecked,
  onCheckedChange,
  disabled,
  className,
}: {
  id: string;
  etiqueta: ReactNode;
  descripcion?: ReactNode;
  checked?: boolean;
  defaultChecked?: boolean;
  onCheckedChange?: (v: boolean) => void;
  disabled?: boolean;
  className?: string;
}) {
  return (
    <label
      htmlFor={id}
      className={cn(
        "text-control flex cursor-pointer items-center justify-between gap-4",
        disabled && "cursor-not-allowed opacity-60",
        className,
      )}
    >
      <span className="grid min-w-0 gap-0.5">
        <span className="text-foreground font-medium">{etiqueta}</span>
        {descripcion ? <span className="text-control text-muted-foreground">{descripcion}</span> : null}
      </span>
      <SwitchPrimitive.Root
        id={id}
        checked={checked}
        defaultChecked={defaultChecked}
        onCheckedChange={onCheckedChange}
        disabled={disabled}
        className="bg-input ease-salida focus-visible:ring-acento/55 data-checked:bg-primary relative h-[22px] w-9 shrink-0 rounded-full p-[3px] transition-colors duration-(--duracion-media) outline-none focus-visible:ring-3"
      >
        <SwitchPrimitive.Thumb className="ease-salida dark:data-checked:bg-primary-foreground block size-4 rounded-full bg-white shadow-sm transition-transform duration-(--duracion-media) data-checked:translate-x-3.5" />
      </SwitchPrimitive.Root>
    </label>
  );
}
