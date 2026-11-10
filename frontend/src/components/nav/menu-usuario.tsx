"use client";

import { useState } from "react";

import { useApariencia } from "@/components/apariencia";
import { useDiseno } from "@/components/diseno";
import { PanelApariencia } from "@/components/nav/panel-apariencia";
import { ThemeToggle } from "@/components/nav/theme-toggle";
import { Menu, MenuContent, MenuItem, MenuLinkItem, MenuSeparator, MenuTrigger } from "@/components/ui/menu";
import { ROTULO } from "@/lib/estilos";
import type { IconoComponente } from "@/lib/iconos";
import { cn } from "@/lib/utils";
import { CaretRightIcon, CaretUpDownIcon, PaletteIcon, ShieldCheckIcon, SignOutIcon } from "@phosphor-icons/react";

export type UsuarioMenu = {
  /** Nombre visible; si falta se usa la parte local del correo. */
  nombre?: string;
  email: string;
  /** Etiqueta legible del rol ("Administrador"). */
  rol?: string;
};

export type AccionMenu = {
  label: string;
  icon: IconoComponente;
  /** Con `href` se renderiza como enlace (abre en pestaña nueva si `externo`). */
  href?: string;
  externo?: boolean;
  onClick?: () => void;
  disabled?: boolean;
  title?: string;
};

function iniciales(texto: string): string {
  const local = texto.split("@")[0] ?? texto;
  return local.slice(0, 2).toUpperCase();
}

/**
 * Bloque de usuario del sidebar: disparador tipo tarjeta (igual que SelectorContexto) y menú del mismo ancho con
 * la cuenta, el tema en línea (solo iconos), «Apariencia…» (abre `PanelApariencia` a la derecha con paleta, tono oscuro,
 * tipografía y diseño), acciones propias y "Cerrar sesión".
 */
export function MenuUsuario({
  usuario,
  acciones = [],
  onCerrarSesion,
  textoCerrarSesion = "Cerrar sesión",
  mostrarTema = true,
  mostrarDiseno = true,
  mostrarApariencia = true,
}: {
  usuario: UsuarioMenu;
  acciones?: AccionMenu[];
  onCerrarSesion: () => void | Promise<void>;
  textoCerrarSesion?: string;
  mostrarTema?: boolean;
  /** Incluye el diseño en el panel «Apariencia». Solo aparece con `DisenoProvider` y dos diseños o más. */
  mostrarDiseno?: boolean;
  /** Muestra «Apariencia…» (paleta, tono oscuro y tipografía). Solo con `AparienciaProvider`. */
  mostrarApariencia?: boolean;
}) {
  const [saliendo, setSaliendo] = useState(false);
  const [panelAbierto, setPanelAbierto] = useState(false);
  const { disenos } = useDiseno();
  const conDiseno = mostrarDiseno && disenos.length > 1;
  const { paletas } = useApariencia();
  const conApariencia = mostrarApariencia && paletas.length > 1;
  const nombre = usuario.nombre ?? usuario.email.split("@")[0] ?? usuario.email;

  async function salir() {
    setSaliendo(true);
    try {
      await onCerrarSesion();
    } finally {
      setSaliendo(false);
    }
  }

  return (
    <>
      <Menu>
        <MenuTrigger
          aria-label="Menú de usuario"
          className={cn(
            "bg-secondary flex w-full min-w-0 cursor-pointer items-center justify-between gap-2 overflow-hidden rounded-xl p-2 text-left transition-colors outline-none select-none",
            "hover:bg-secondary focus-visible:ring-ring/50 data-popup-open:bg-secondary focus-visible:ring-2",
          )}
        >
          <div className="flex min-w-0 items-center gap-2.5">
            <div className="bg-acento text-etiqueta text-acento-texto flex size-6 shrink-0 items-center justify-center rounded-full font-bold shadow-xs">
              {iniciales(usuario.nombre ?? usuario.email)}
            </div>
            <div className="min-w-0 overflow-hidden">
              <p className="text-control text-foreground truncate leading-tight font-medium">{nombre}</p>
              <p className="text-etiqueta text-muted-foreground truncate leading-tight">{usuario.email}</p>
            </div>
          </div>
          <CaretUpDownIcon weight="bold" className="text-muted-foreground/70 size-4 shrink-0" />
        </MenuTrigger>

        {/* Mismo ancho y alineación que el disparador: no invade el contenido principal. */}
        <MenuContent side="top" align="start" className="w-(--anchor-width) min-w-0">
          <div className="text-etiqueta text-muted-foreground px-2 pt-1.5 pb-1 font-medium">Tu cuenta</div>
          <div className="bg-accent text-accent-foreground flex items-start gap-2.5 rounded-lg py-1.5 pr-2 pl-2">
            <div className="bg-acento text-etiqueta text-acento-texto flex size-6 shrink-0 items-center justify-center rounded-full font-bold shadow-xs">
              {iniciales(usuario.nombre ?? usuario.email)}
            </div>
            <div className="min-w-0 flex-1">
              <p className="text-control leading-tight font-medium">{nombre}</p>
              <p className="text-etiqueta text-muted-foreground mt-0.5 leading-snug break-all">{usuario.email}</p>
              {usuario.rol ? (
                <p className="text-etiqueta text-muted-foreground mt-0.5 leading-tight">{usuario.rol}</p>
              ) : null}
            </div>
            <ShieldCheckIcon className="text-accent-foreground mt-0.5 size-4 shrink-0" />
          </div>

          {mostrarTema ? (
            <>
              <MenuSeparator />
              <div className="flex items-center justify-between gap-3 px-2 py-1.5">
                <span className={ROTULO}>Tema</span>
                <ThemeToggle />
              </div>
            </>
          ) : null}

          {conDiseno || conApariencia ? (
            <MenuItem onClick={() => setPanelAbierto(true)}>
              <PaletteIcon />
              <span className="flex-1">Apariencia…</span>
              <CaretRightIcon weight="bold" className="text-texto-terciario size-3.5" />
            </MenuItem>
          ) : null}

          {acciones.length > 0 ? (
            <>
              <MenuSeparator />
              {acciones.map((a) => {
                const Icono = a.icon;
                return a.href ? (
                  <MenuLinkItem key={a.label} href={a.href} target={a.externo ? "_blank" : undefined} title={a.title}>
                    <Icono />
                    {a.label}
                  </MenuLinkItem>
                ) : (
                  <MenuItem key={a.label} onClick={a.onClick} disabled={a.disabled} title={a.title}>
                    <Icono />
                    {a.label}
                  </MenuItem>
                );
              })}
            </>
          ) : null}

          <MenuSeparator />
          <MenuItem variant="destructive" disabled={saliendo} onClick={salir}>
            <SignOutIcon />
            {textoCerrarSesion}
          </MenuItem>
        </MenuContent>
      </Menu>
      {conDiseno || conApariencia ? <PanelApariencia open={panelAbierto} onOpenChange={setPanelAbierto} /> : null}
    </>
  );
}
