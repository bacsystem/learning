"use client";

import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectLabel,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { cn } from "@/lib/utils";

export type OpcionSelect = {
  value: string;
  label: string;
  /** Opciones con el mismo `grupo` van juntas bajo ese rótulo (lo que un `<optgroup>` hacía), en el orden en que aparece cada grupo. */
  grupo?: string;
  disabled?: boolean;
};

/**
 * El combo de un formulario: el `Select` del design system (lista redondeada con check) con el aspecto de `CAMPO`, a lo ancho del campo.
 * Reemplaza al `<select>` nativo, que el sistema operativo dibuja a su manera (lista gris en Windows, rueda en iOS).
 *
 * - `valor` y `onCambio` hablan en texto, como un `<select>`: `""` es «nada elegido» y muestra `placeholder`, salvo que una opción tenga `value: ""`
 *   (p. ej. «Sin incoterm»), que entonces se muestra como cualquier otra.
 * - `id` va en el disparador: un `<label htmlFor>` (el de `Campo`) lo nombra y lo enfoca.
 * - `nombre` y `required`: el input oculto de base-ui, para un `<form>` nativo.
 * - Para catálogos largos donde conviene escribir para buscar (países, clientes), usa `Combobox`.
 */
export function SelectCampo({
  id,
  opciones,
  valor,
  onCambio,
  placeholder,
  invalido,
  describidoPor,
  disabled,
  nombre,
  required,
  mono,
  etiqueta,
  className,
}: {
  id: string;
  opciones: readonly OpcionSelect[];
  valor: string;
  onCambio?: (valor: string) => void;
  placeholder?: string;
  /** Marca el campo con error (`aria-invalid`, borde rojo) y lo une a su mensaje (`aria-describedby`). */
  invalido?: boolean;
  describidoPor?: string;
  disabled?: boolean;
  /** Nombre del input oculto que lleva el valor en un `<form>` nativo. */
  nombre?: string;
  required?: boolean;
  /** Valor en monoespaciada (series, códigos). */
  mono?: boolean;
  /** Nombre accesible cuando no hay un `<label htmlFor>` (una celda de tabla, una barra). */
  etiqueta?: string;
  className?: string;
}) {
  const conVacio = opciones.some((o) => o.value === "");
  const grupos: { grupo: string | undefined; opciones: OpcionSelect[] }[] = [];
  for (const o of opciones) {
    const g = grupos.find((x) => x.grupo === o.grupo);
    if (g) g.opciones.push(o);
    else grupos.push({ grupo: o.grupo, opciones: [o] });
  }
  const item = (o: OpcionSelect) => (
    <SelectItem key={o.value} value={o.value} disabled={o.disabled} className={cn(mono && "font-mono")}>
      {o.label}
    </SelectItem>
  );

  return (
    <Select
      items={opciones.map((o) => ({ value: o.value, label: o.label }))}
      value={valor === "" && !conVacio ? null : valor}
      onValueChange={(v) => onCambio?.((v as string | null) ?? "")}
      disabled={disabled}
      name={nombre}
      required={required}
    >
      <SelectTrigger
        id={id}
        size="campo"
        aria-label={etiqueta}
        aria-invalid={invalido || undefined}
        aria-describedby={describidoPor}
        className={cn("w-full min-w-0", mono && "font-mono", className)}
      >
        <SelectValue placeholder={placeholder} className="min-w-0 truncate" />
      </SelectTrigger>
      <SelectContent>
        {grupos.map((g, i) =>
          g.grupo === undefined ? (
            g.opciones.map(item)
          ) : (
            <SelectGroup key={`${g.grupo}-${i}`}>
              <SelectLabel>{g.grupo}</SelectLabel>
              {g.opciones.map(item)}
            </SelectGroup>
          ),
        )}
      </SelectContent>
    </Select>
  );
}
