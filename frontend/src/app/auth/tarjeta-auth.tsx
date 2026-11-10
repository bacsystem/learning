import type { ReactNode } from "react";

import { FOCO, TARJETA } from "@/lib/estilos";
import { cn } from "@/lib/utils";

/** Enlace de texto de las páginas públicas: tinta principal con subrayado tenue que se marca al pasar el mouse. */
export const ENLACE = cn(
  "rounded-sm font-medium text-foreground underline decoration-border underline-offset-4 transition-colors duration-(--duracion-rapida) ease-salida hover:decoration-foreground",
  FOCO,
);

const ANCHOS = {
  /** Formularios de una columna (ingresar, recuperar, verificar). */
  formulario: "max-w-[420px]",
  /** Formulario en dos columnas con panel lateral (registro, paso 2). */
  amplio: "max-w-[860px]",
  /** Documentos legales: medida de lectura cómoda. */
  documento: "max-w-3xl",
} as const;

/**
 * Tarjeta de las páginas de auth: título de página, descripción, cuerpo y un pie opcional separado.
 * `bare` deja el cuerpo sin padding para layouts con paneles propios (registro, paso 2).
 */
export function TarjetaAuth({
  titulo,
  descripcion,
  children,
  pie,
  ancho = "formulario",
  centrado = false,
  bare = false,
  className,
}: {
  titulo?: ReactNode;
  descripcion?: ReactNode;
  children?: ReactNode;
  pie?: ReactNode;
  ancho?: keyof typeof ANCHOS;
  centrado?: boolean;
  bare?: boolean;
  className?: string;
}) {
  return (
    <section className={cn(TARJETA, "w-full overflow-hidden", ANCHOS[ancho], className)}>
      {bare ? (
        children
      ) : (
        <div className={cn("flex flex-col gap-6 px-6 py-8 sm:px-10 sm:py-10", centrado && "items-center text-center")}>
          {titulo || descripcion ? (
            <header className="flex flex-col gap-1.5">
              {titulo ? <h1 className="text-titulo-pagina text-foreground text-balance">{titulo}</h1> : null}
              {descripcion ? <p className="text-cuerpo text-muted-foreground leading-relaxed">{descripcion}</p> : null}
            </header>
          ) : null}
          {children}
        </div>
      )}
      {pie ? (
        <footer className="border-border/60 bg-muted/60 text-control text-muted-foreground border-t px-6 py-3.5 text-center sm:px-10">
          {pie}
        </footer>
      ) : null}
    </section>
  );
}
