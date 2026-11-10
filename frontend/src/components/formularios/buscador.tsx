"use client";

import type { InputHTMLAttributes } from "react";

import { CAMPO, CONTROL_FILTRO } from "@/lib/estilos";
import { cn } from "@/lib/utils";
import { MagnifyingGlassIcon, XIcon } from "@phosphor-icons/react";

/**
 * Input de búsqueda con icono y botón de limpiar, para filtrar listas/tablas dentro de la página (barra de
 * filtros, `PanelLateral`…). El buscador de la `TopBar` es otra cosa: un disparador deshabilitado que abre
 * `Paleta` (⌘K), no un campo que filtra en el sitio.
 * `variante="filtro"` (por defecto, en una barra) o `"campo"` (dentro de un `Formulario`): las dos miden h-control, como todo control.
 */
export function Buscador({
  id,
  etiqueta,
  valor,
  onCambio,
  placeholder = "Buscar…",
  variante = "filtro",
  className,
  ...props
}: {
  id?: string;
  /** Nombre accesible del campo. Obligatorio en la práctica salvo que lo envuelvas en un `Campo` (que ya pone
   * su propio `<label htmlFor>`, detectado aquí por la presencia de `id`): un `placeholder` no forma parte del
   * cálculo de nombre accesible de la spec ARIA y no todos los lectores de pantalla lo exponen. Sin `etiqueta`
   * y sin `id`, cae al `placeholder` para no quedar completamente mudo, pero no depende de eso a propósito. */
  etiqueta?: string;
  valor: string;
  onCambio: (v: string) => void;
  placeholder?: string;
  variante?: "filtro" | "campo";
  className?: string;
} & Omit<InputHTMLAttributes<HTMLInputElement>, "id" | "value" | "onChange" | "placeholder" | "className" | "type">) {
  return (
    <div className="relative flex items-center">
      <MagnifyingGlassIcon
        weight="bold"
        className="text-muted-foreground/70 pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2"
      />
      <input
        id={id}
        type="search"
        aria-label={etiqueta ?? (id ? undefined : placeholder)}
        value={valor}
        onChange={(e) => onCambio(e.target.value)}
        placeholder={placeholder}
        className={cn(
          variante === "campo" ? CAMPO : CONTROL_FILTRO,
          // 16 px hasta `md`: iOS Safari amplía la página al enfocar un campo de menos.
          "text-campo md:text-control placeholder:text-muted-foreground/70 w-full pr-8 pl-8 [&::-webkit-search-cancel-button]:hidden",
          className,
        )}
        {...props}
      />
      {valor ? (
        <button
          type="button"
          aria-label="Limpiar búsqueda"
          onClick={() => onCambio("")}
          className="text-muted-foreground/70 hover:bg-secondary hover:text-foreground absolute top-1/2 right-1.5 -translate-y-1/2 rounded p-1 transition-colors"
        >
          <XIcon weight="bold" className="size-3.5" />
        </button>
      ) : null}
    </div>
  );
}
