import { type GrupoNav, SidebarNav } from "./sidebar-nav";
import type { ReactNode } from "react";

/**
 * Contenido del sidebar (248px en desktop, cajón lateral en móvil). Se compone por slots para no acoplarlo a ninguna app:
 * `marca` (LogoMarca), `chip` (p. ej. versión/estándar), `selector` (SelectorContexto), `grupos` (SidebarNav),
 * `pie` (tarjetas de plan, avisos) y `usuario` (MenuUsuario).
 */
export function SidebarContent({
  marca,
  chip,
  selector,
  grupos,
  etiquetaMenu,
  pie,
  usuario,
}: {
  marca: ReactNode;
  chip?: ReactNode;
  selector?: ReactNode;
  grupos: GrupoNav[];
  /** Nombre accesible del menú (`aria-label` de su `nav`). */
  etiquetaMenu?: string;
  pie?: ReactNode;
  usuario?: ReactNode;
}) {
  return (
    <div className="flex h-full w-full min-w-0 flex-col justify-between overflow-hidden select-none">
      <div className="flex w-full min-w-0 flex-col">
        <div className="flex h-14 w-full min-w-0 shrink-0 items-center justify-between gap-2 overflow-hidden px-4">
          {marca}
          {chip}
        </div>
        {selector ? <div className="px-3 pt-3 pb-2">{selector}</div> : null}
        <SidebarNav grupos={grupos} etiqueta={etiquetaMenu} />
      </div>

      {pie || usuario ? (
        <div className="flex w-full min-w-0 flex-col gap-2.5 p-3">
          {pie}
          {usuario}
        </div>
      ) : null}
    </div>
  );
}

/** Chip pequeño para la cabecera del sidebar (versión, estándar, entorno). */
export function ChipSidebar({ children }: { children: ReactNode }) {
  return (
    <span className="border-border/60 bg-secondary text-etiqueta text-secondary-foreground/70 inline-flex shrink-0 items-center rounded border px-1.5 py-0.5 font-mono font-medium whitespace-nowrap">
      {children}
    </span>
  );
}

/** Tarjeta de pie deshabilitada (p. ej. "Plan y consumo" antes de tener datos). */
export function TarjetaPieDeshabilitada({
  titulo,
  valor = "— / —",
  title,
}: {
  titulo: string;
  valor?: string;
  title?: string;
}) {
  return (
    <div
      title={title}
      aria-disabled="true"
      className="border-border/60 bg-muted/90 grid w-full cursor-not-allowed grid-cols-1 gap-1.5 overflow-hidden rounded-lg border p-2.5 opacity-60"
    >
      <div className="text-etiqueta flex items-center justify-between gap-2">
        <span className="text-foreground/80 truncate font-medium">{titulo}</span>
        <span className="text-etiqueta text-muted-foreground shrink-0 font-mono whitespace-nowrap">{valor}</span>
      </div>
      <div className="bg-border h-1 w-full overflow-hidden rounded-full">
        <div className="bg-primary h-full w-0 rounded-full" />
      </div>
    </div>
  );
}
