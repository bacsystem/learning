"use client";

import { useId } from "react";

import { Menu, MenuContent, MenuItem, MenuLinkItem, MenuSeparator, MenuTrigger } from "@/components/ui/menu";
import { OBJETIVO_TACTIL } from "@/lib/estilos";
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
  /** Por qué la acción está deshabilitada: texto visible bajo el ítem, enlazado con `aria-describedby`. Un `title` no sirve en un ítem deshabilitado: no recibe puntero (`pointer-events-none`) y en el móvil no hay hover. Con teclado el ítem sí recibe foco (base-ui usa `focusableWhenDisabled`), y ahí `aria-describedby` anuncia la ayuda. */
  ayuda?: string;
  /** Pinta un separador antes de esta acción. */
  separador?: boolean;
};

/** Texto de ayuda de una acción deshabilitada, fuera del ítem para que no herede su opacidad y se lea con contraste normal. */
export function AyudaAccion({ id, children }: { id: string; children: string }) {
  return (
    <p id={id} aria-hidden="true" className="text-etiqueta text-muted-foreground -mt-0.5 px-2.5 pb-1.5">
      {children}
    </p>
  );
}

function ItemAccion({ accion: a }: { accion: AccionFila }) {
  const idAyuda = useId();
  const Icono = a.icon;
  return (
    <>
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
          aria-describedby={a.ayuda ? idAyuda : undefined}
          onClick={a.onClick}
        >
          {Icono ? <Icono /> : null}
          {a.label}
        </MenuItem>
      )}
      {a.ayuda ? <AyudaAccion id={idAyuda}>{a.ayuda}</AyudaAccion> : null}
    </>
  );
}

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
          OBJETIVO_TACTIL,
          "size-control text-muted-foreground/70 hover:bg-secondary hover:text-foreground focus-visible:ring-ring/50 data-popup-open:bg-secondary data-popup-open:text-foreground flex items-center justify-center rounded-lg transition-colors outline-none focus-visible:ring-2",
          className,
        )}
      >
        <DotsThreeIcon weight="bold" className="size-4" />
      </MenuTrigger>
      <MenuContent side="bottom" align="end" className="min-w-44">
        {acciones.map((a) => (
          <span key={a.label} className="contents">
            <ItemAccion accion={a} />
          </span>
        ))}
      </MenuContent>
    </Menu>
  );
}
