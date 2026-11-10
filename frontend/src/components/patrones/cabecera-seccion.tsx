import type { ReactNode } from "react";

import { type IconoComponente, PESO_ICONO } from "@/lib/iconos";
import { cn } from "@/lib/utils";

/** Icono de cabecera de sección y de diálogo: sin caja ni fondo (docs «Iconos»), en el color de la paleta. */
export function IconoSeccion({ icon: Icon, className }: { icon: IconoComponente; className?: string }) {
  // Peso explícito: desde un Server Component el icono llega de dist/ssr, que no lee IconContext.
  return (
    <Icon
      aria-hidden
      weight={PESO_ICONO}
      className={cn("text-acento-2-texto dark:text-acento-2 size-[18px] shrink-0", className)}
    />
  );
}

/** Cabecera de tarjeta/sección: icono + título + subtítulo, y un slot a la derecha (chips, acciones). Sin separador: lo da el espacio. */
export function CabeceraSeccion({
  icon,
  titulo,
  subtitulo,
  derecha,
  conBorde = true,
  className,
}: {
  icon: IconoComponente;
  titulo: ReactNode;
  subtitulo?: ReactNode;
  derecha?: ReactNode;
  conBorde?: boolean;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "flex min-w-0 flex-wrap items-center gap-x-2.5 gap-y-2 px-4 pt-3.5",
        conBorde ? "pb-2" : "pb-3.5",
        className,
      )}
    >
      <IconoSeccion icon={icon} />
      <div className="flex min-w-0 flex-1 basis-48 flex-col">
        <h2 className="text-titulo text-foreground">{titulo}</h2>
        {subtitulo ? <span className="text-etiqueta text-texto-terciario truncate">{subtitulo}</span> : null}
      </div>
      {derecha}
    </div>
  );
}

/** Pie de tarjeta (notas técnicas): texto terciario, sin franja ni borde. */
export function PieSeccion({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <div
      className={cn(
        "text-etiqueta text-texto-terciario flex flex-wrap items-center gap-x-4 gap-y-1 px-4 pt-1 pb-3",
        className,
      )}
    >
      {children}
    </div>
  );
}
