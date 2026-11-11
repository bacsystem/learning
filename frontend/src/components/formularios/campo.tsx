import type { ReactNode } from "react";

import { AYUDA_CAMPO, CAMPO, ETIQUETA_CAMPO } from "@/lib/estilos";
import { cn } from "@/lib/utils";

/**
 * Campo compuesto: etiqueta + control + ayuda (mono) o error (rojo). El control se pasa como `children` con el `id` indicado.
 * `<Campo id="serie" etiqueta="Serie" ayuda="4 caracteres"><Entrada id="serie" … /></Campo>`
 * Con `opcional` añade «(opcional)» junto a la etiqueta; `textoOpcional` lo traduce.
 */
export function Campo({
  id,
  etiqueta,
  ayuda,
  error,
  opcional,
  textoOpcional = "(opcional)",
  children,
  className,
}: {
  id: string;
  etiqueta: ReactNode;
  ayuda?: ReactNode;
  error?: ReactNode;
  opcional?: boolean;
  /** Texto que acompaña a la etiqueta con `opcional`, para traducirlo (por defecto «(opcional)»). */
  textoOpcional?: string;
  children: ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("grid gap-1.5", className)}>
      <label htmlFor={id} className={cn(ETIQUETA_CAMPO, "flex items-center gap-1.5")}>
        {etiqueta}
        {opcional ? <span className="text-etiqueta text-muted-foreground font-normal">{textoOpcional}</span> : null}
      </label>
      {children}
      {error ? (
        <p id={`${id}-error`} role="alert" className="text-control text-destructive">
          {error}
        </p>
      ) : ayuda ? (
        <p id={`${id}-ayuda`} className={AYUDA_CAMPO}>
          {ayuda}
        </p>
      ) : null}
    </div>
  );
}

/** Input con la receta `CAMPO`; `invalido` pinta el borde rojo. `mono` para códigos, series, RUC. */
export function Entrada({
  className,
  invalido,
  mono,
  ...props
}: React.ComponentProps<"input"> & { invalido?: boolean; mono?: boolean }) {
  return (
    <input
      aria-invalid={invalido || undefined}
      className={cn(
        CAMPO,
        mono && "font-mono",
        invalido && "border-destructive focus:border-destructive focus:ring-destructive/20",
        className,
      )}
      {...props}
    />
  );
}

/** Textarea con la misma receta (altura mínima de 3 líneas, redimensionable en vertical). */
export function AreaTexto({
  className,
  invalido,
  ...props
}: React.ComponentProps<"textarea"> & { invalido?: boolean }) {
  return (
    <textarea
      aria-invalid={invalido || undefined}
      className={cn(
        CAMPO,
        "h-auto min-h-20 resize-y py-2 leading-relaxed",
        invalido && "border-destructive focus:border-destructive focus:ring-destructive/20",
        className,
      )}
      {...props}
    />
  );
}

/** Rejilla de campos: 1 columna en móvil, N en desktop. */
export function Formulario({
  columnas = 2,
  children,
  className,
  ...props
}: React.ComponentProps<"form"> & { columnas?: 1 | 2 | 3 }) {
  return (
    <form
      className={cn(
        "grid grid-cols-1 gap-4",
        columnas === 2 && "md:grid-cols-2",
        columnas === 3 && "md:grid-cols-3",
        className,
      )}
      {...props}
    >
      {children}
    </form>
  );
}
