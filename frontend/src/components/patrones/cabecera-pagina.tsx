import Link from "next/link";
import type { ReactNode } from "react";

import { ACCION_SECUNDARIA } from "@/lib/estilos";
import { PESO_ICONO } from "@/lib/iconos";
import { cn } from "@/lib/utils";
import { CaretLeftIcon } from "@phosphor-icons/react/dist/ssr";

/**
 * Cabecera de una página que no la tiene en la barra superior (el backoffice, el detalle de un registro): «volver» opcional, título (`h1`), descripción y
 * acciones a la derecha. Va fuera de las tarjetas, en el mismo ancho que el contenido. En el panel del cliente el título vive en la `TopBar`: ahí no se usa.
 */
export function CabeceraPagina({
  titulo,
  descripcion,
  volver,
  acciones,
  className,
}: {
  titulo: ReactNode;
  descripcion?: ReactNode;
  /** El enlace de vuelta: `EnlaceVolver`. */
  volver?: ReactNode;
  /** Botones de la página (recetas `ACCION_*`). */
  acciones?: ReactNode;
  className?: string;
}) {
  return (
    <header className={cn("flex flex-col gap-2", className)}>
      {volver ? <div>{volver}</div> : null}
      <div className="flex flex-wrap items-end justify-between gap-x-4 gap-y-2">
        <div className="flex min-w-0 flex-col gap-1">
          <h1 className="text-titulo-pagina text-foreground font-semibold">{titulo}</h1>
          {descripcion ? <p className="text-cuerpo text-muted-foreground max-w-3xl">{descripcion}</p> : null}
        </div>
        {acciones ? <div className="flex flex-wrap items-center gap-2">{acciones}</div> : null}
      </div>
    </header>
  );
}

/** «Volver» de un detalle: la acción secundaria con la flecha, la misma en todas las páginas. */
export function EnlaceVolver({ href, children, className }: { href: string; children: ReactNode; className?: string }) {
  return (
    <Link href={href} className={cn(ACCION_SECUNDARIA, className)}>
      <CaretLeftIcon aria-hidden weight={PESO_ICONO} className="size-4" />
      {children}
    </Link>
  );
}
