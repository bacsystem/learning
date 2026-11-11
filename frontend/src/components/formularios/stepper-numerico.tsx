"use client";

import { cn } from "@/lib/utils";
import { NumberField } from "@base-ui/react/number-field";
import { MinusIcon, PlusIcon } from "@phosphor-icons/react";

// Las dos variantes miden h-control (altura única de control); `variante` se conserva por compatibilidad.
const TAMANOS = {
  filtro: { grupo: "h-control", boton: "size-control" },
  campo: { grupo: "h-control", boton: "size-control" },
} as const;

/**
 * Cantidad numérica con +/− (ítems de una línea, plazos en días, cupos). El valor se escribe directo o se pisa
 * con las flechas; `min`/`max` deshabilitan el botón correspondiente al llegar al límite (los pinta base-ui solo).
 * Mide h-control como cualquier otro control; `variante` ya no cambia la altura.
 */
export function StepperNumerico({
  id,
  valor,
  onCambio,
  min,
  max,
  paso = 1,
  disabled,
  variante = "campo",
  etiquetaDisminuir = "Disminuir",
  etiquetaAumentar = "Aumentar",
  className,
}: {
  id?: string;
  valor: number | null;
  onCambio: (v: number | null) => void;
  min?: number;
  max?: number;
  /**
   * Paso de las flechas ±. **Ojo:** también es el `step` del `<input type="number">` oculto que base-ui usa para la
   * validación nativa del `<form>`. Con `paso={1}` (el default) y `min` definido, cualquier valor no entero cae en
   * `stepMismatch` y el navegador aborta el submit en silencio, anclando la burbuja de error a un input de 1×1 px:
   * el botón parece muerto. Para cantidades con decimales (kilos, horas, metros) pasa `paso="any"`: las flechas
   * siguen sumando de a 1 y la validación de paso se desactiva.
   */
  paso?: number | "any";
  disabled?: boolean;
  variante?: "filtro" | "campo";
  /** Nombres de los botones − y + para el lector de pantalla, para traducirlos (por defecto «Disminuir» y «Aumentar»). */
  etiquetaDisminuir?: string;
  etiquetaAumentar?: string;
  className?: string;
}) {
  const t = TAMANOS[variante];
  return (
    <NumberField.Root
      id={id}
      value={valor}
      onValueChange={onCambio}
      min={min}
      max={max}
      step={paso}
      disabled={disabled}
    >
      <NumberField.Group
        className={cn(
          "border-input bg-card focus-within:border-acento-texto focus-within:ring-acento/55 dark:focus-within:border-acento dark:focus-within:ring-acento/25 inline-flex w-fit items-stretch overflow-hidden rounded-full border focus-within:ring-3",
          t.grupo,
          className,
        )}
      >
        <NumberField.Decrement
          aria-label={etiquetaDisminuir}
          className={cn(
            "text-muted-foreground hover:bg-secondary hover:text-foreground focus-visible:bg-secondary flex shrink-0 items-center justify-center transition-colors outline-none disabled:pointer-events-none disabled:opacity-40",
            t.boton,
          )}
        >
          <MinusIcon weight="bold" className="size-3.5" />
        </NumberField.Decrement>
        <NumberField.Input className="text-campo md:text-control text-foreground w-12 bg-transparent text-center font-semibold tabular-nums outline-none disabled:cursor-not-allowed disabled:opacity-60" />
        <NumberField.Increment
          aria-label={etiquetaAumentar}
          className={cn(
            "text-muted-foreground hover:bg-secondary hover:text-foreground focus-visible:bg-secondary flex shrink-0 items-center justify-center transition-colors outline-none disabled:pointer-events-none disabled:opacity-40",
            t.boton,
          )}
        >
          <PlusIcon weight="bold" className="size-3.5" />
        </NumberField.Increment>
      </NumberField.Group>
    </NumberField.Root>
  );
}
