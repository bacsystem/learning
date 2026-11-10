import Link from "next/link";
import type { ReactNode } from "react";

/**
 * Marca del sidebar: icono en caja con gradiente primary → primary-light, nombre y sufijo en primary.
 * `icono` acepta cualquier SVG de 20px (por defecto, un glifo neutro).
 */
export function LogoMarca({
  nombre,
  sufijo,
  href = "/",
  icono,
}: {
  nombre: string;
  sufijo?: string;
  href?: string;
  icono?: ReactNode;
}) {
  return (
    <Link href={href} className="group flex min-w-0 items-center gap-2.5">
      <div className="from-primary to-primary-light text-primary-foreground flex size-8 shrink-0 items-center justify-center rounded-lg bg-linear-to-br shadow-xs">
        {icono ?? (
          <svg className="size-5" viewBox="0 0 24 24" fill="none" aria-hidden="true">
            <path d="M4 6H20" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" />
            <path d="M8 6V19M12 6V19M16 6V19" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
          </svg>
        )}
      </div>
      <span className="flex min-w-0 items-baseline gap-0.5">
        <span className="text-titulo text-foreground truncate font-bold">{nombre}</span>
        {sufijo ? <span className="text-etiqueta text-muted-foreground font-medium">{sufijo}</span> : null}
      </span>
    </Link>
  );
}
