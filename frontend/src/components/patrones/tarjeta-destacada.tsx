import { cva, type VariantProps } from "class-variance-authority";
import type { ComponentProps, ReactNode } from "react";

import { cn } from "@/lib/utils";

// Panel de color plano, sin borde ni sombra (la «Accent Feature Card» de Notion), solo con los tonos que khipu ya tiene.
// Título y descripción llevan el mismo color del tono (sin opacidad): el contraste calculado vale para los dos.
const variantes = cva("flex flex-wrap items-start gap-4 rounded-xl p-6", {
  variants: {
    tono: {
      neutro: "bg-secondary text-secondary-foreground",
      info: "bg-accent text-accent-foreground",
      ok: "bg-success text-success-foreground",
      aviso: "bg-warning text-warning-foreground",
      invertido: "bg-foreground text-background",
    },
  },
  defaultVariants: { tono: "neutro" },
});

export type PropsTarjetaDestacada = Omit<ComponentProps<"div">, "title"> &
  VariantProps<typeof variantes> & {
    titulo: ReactNode;
    descripcion?: ReactNode;
    /** Icono ya armado (`<FileIcon />`): así se puede pasar desde un Server Component. */
    icono?: ReactNode;
    /** Acción a la derecha o debajo. Sobre `tono="invertido"` usa `BOTON_SECUNDARIO`: `ACCION_PRINCIPAL` es del mismo color que el fondo. */
    accion?: ReactNode;
  };

/**
 * Bloque de color plano para destacar contenido (bienvenida, función nueva, plan contratado, siguiente paso).
 * No es un aviso: no anuncia nada al lector de pantalla; para eso está `Alerta`.
 */
export function TarjetaDestacada({
  tono,
  titulo,
  descripcion,
  icono,
  accion,
  children,
  className,
  ...props
}: PropsTarjetaDestacada) {
  return (
    <div className={cn(variantes({ tono }), className)} {...props}>
      {icono ? <span className="mt-0.5 shrink-0 [&_svg:not([class*='size-'])]:size-5">{icono}</span> : null}
      <div className="min-w-0 flex-1 basis-56">
        <p className="text-titulo break-words">{titulo}</p>
        {descripcion ? <p className="text-control mt-1 leading-relaxed break-words">{descripcion}</p> : null}
        {children ? <div className="mt-3">{children}</div> : null}
      </div>
      {accion ? <div className="shrink-0 self-center">{accion}</div> : null}
    </div>
  );
}
