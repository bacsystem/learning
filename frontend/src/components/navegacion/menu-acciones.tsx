"use client";

import { Menu, MenuContent, MenuItem, MenuLinkItem, MenuSeparator, MenuTrigger } from "@/components/ui/menu";
import type { IconoComponente } from "@/lib/iconos";
import { cn } from "@/lib/utils";
import { DotsThreeIcon } from "@phosphor-icons/react";

export type AccionFila = {
  label: string;
  icon?: IconoComponente;
  href?: string;
  onClick?: () => void;
  destructiva?: boolean;
  disabled?: boolean;
  title?: string;
  /** Pinta un separador antes de esta acción. */
  separador?: boolean;
};

/** Botón "⋯" de fila que abre un menú con las acciones secundarias (ver, duplicar, descargar, eliminar…). */
export function MenuAcciones({
  acciones,
  etiqueta = "Más acciones",
  className,
}: {
  acciones: AccionFila[];
  etiqueta?: string;
  className?: string;
}) {
  return (
    <Menu>
      <MenuTrigger
        aria-label={etiqueta}
        title={etiqueta}
        className={cn(
          "size-control text-muted-foreground/70 hover:bg-secondary hover:text-foreground focus-visible:ring-ring/50 data-popup-open:bg-secondary data-popup-open:text-foreground flex items-center justify-center rounded-lg transition-colors outline-none focus-visible:ring-2",
          className,
        )}
      >
        <DotsThreeIcon weight="bold" className="size-4" />
      </MenuTrigger>
      <MenuContent side="bottom" align="end" className="min-w-44">
        {acciones.map((a) => {
          const Icono = a.icon;
          return (
            <span key={a.label} className="contents">
              {a.separador ? <MenuSeparator /> : null}
              {a.href ? (
                <MenuLinkItem href={a.href} title={a.title}>
                  {Icono ? <Icono /> : null}
                  {a.label}
                </MenuLinkItem>
              ) : (
                <MenuItem
                  variant={a.destructiva ? "destructive" : "default"}
                  disabled={a.disabled}
                  title={a.title}
                  onClick={a.onClick}
                >
                  {Icono ? <Icono /> : null}
                  {a.label}
                </MenuItem>
              )}
            </span>
          );
        })}
      </MenuContent>
    </Menu>
  );
}
