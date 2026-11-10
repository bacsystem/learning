"use client";

import { m } from "motion/react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useId } from "react";

import { ROTULO } from "@/lib/estilos";
import type { IconoComponente } from "@/lib/iconos";
import { PESO_ICONO, PESO_ICONO_ACTIVO } from "@/lib/iconos";
import { RESORTE } from "@/lib/movimiento";
import { cn } from "@/lib/utils";

export type ItemNav = {
  /** Sin `href` el ítem se muestra deshabilitado con la etiqueta `pendiente` (patrón "próximamente"). */
  href?: string;
  label: string;
  icon: IconoComponente;
  /** Texto del chip cuando no hay href (por defecto "Pronto"). */
  pendiente?: string;
  /** Tooltip explicando por qué todavía no está disponible. */
  title?: string;
  /** Activo solo si la ruta es exactamente `href` (un «Inicio» en `/admin` no debe marcarse en todas las páginas que cuelgan de él). Por defecto, por prefijo. */
  exacto?: boolean;
};

export type GrupoNav = { titulo: string; items: ItemNav[] };

// Sin overflow-hidden: recortaría el fondo activo mientras se desliza desde otro ítem. La etiqueta trunca por su cuenta (min-w-0).
// `isolate` deja el fondo activo (-z-10) detrás del icono y del texto.
const ITEM_BASE = "relative isolate flex w-full min-w-0 items-center gap-2.5 rounded-full px-3 py-[7px] text-control";

/** Navegación del sidebar por grupos; el ítem activo se detecta por prefijo de la ruta actual y su fondo se desliza al cambiar. */
export function SidebarNav({ grupos, etiqueta }: { grupos: GrupoNav[]; etiqueta?: string }) {
  const pathname = usePathname();
  // Un layoutId por instancia: el sidebar de escritorio y el del menú móvil no comparten indicador.
  const idIndicador = useId();

  return (
    <nav aria-label={etiqueta} className="grid w-full grid-cols-1 gap-4 px-2.5 pt-2">
      {grupos.map((grupo) => (
        <div key={grupo.titulo} className="grid w-full grid-cols-1 gap-0.5">
          <span className={cn("truncate px-3 pt-1 pb-1.5", ROTULO)}>{grupo.titulo}</span>
          {grupo.items.map((item) => {
            const Icono = item.icon;
            if (!item.href) {
              return (
                <span
                  key={item.label}
                  aria-disabled="true"
                  title={item.title ?? `${item.label}: próximamente`}
                  className={cn(ITEM_BASE, "text-muted-foreground/60 cursor-not-allowed")}
                >
                  <Icono weight={PESO_ICONO} className="text-muted-foreground/50 size-[18px] shrink-0" />
                  <span className="min-w-0 flex-1 truncate">{item.label}</span>
                  <span className="bg-secondary text-etiqueta shrink-0 rounded-full px-2 py-0.5 font-medium whitespace-nowrap">
                    {item.pendiente ?? "Pronto"}
                  </span>
                </span>
              );
            }
            const active = item.exacto ? pathname === item.href : pathname.startsWith(item.href);
            return (
              <Link
                key={item.href}
                href={item.href}
                aria-current={active ? "page" : undefined}
                className={cn(
                  ITEM_BASE,
                  "ease-salida transition-colors duration-(--duracion-rapida)",
                  active
                    ? "text-sidebar-primary-foreground font-semibold"
                    : "text-sidebar-foreground hover:bg-sidebar-accent hover:text-sidebar-accent-foreground",
                )}
              >
                {active ? (
                  <m.span
                    layoutId={`${idIndicador}-activo`}
                    aria-hidden
                    transition={RESORTE}
                    className="border-acento-borde bg-sidebar-primary absolute inset-0 -z-10 rounded-full border"
                  />
                ) : null}
                <Icono
                  weight={active ? PESO_ICONO_ACTIVO : PESO_ICONO}
                  className={cn("size-[18px] shrink-0", active ? "text-current" : "text-muted-foreground")}
                />
                <span className="min-w-0 flex-1 truncate">{item.label}</span>
              </Link>
            );
          })}
        </div>
      ))}
    </nav>
  );
}
