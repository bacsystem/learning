"use client";

import { BarraIdioma } from "@/components/app/barra-idioma";
import { BarraNotificaciones } from "@/components/app/barra-notificaciones";
import { BarraRecursos } from "@/components/app/barra-recursos";

/**
 * Acciones de la barra superior: «Recursos» (accesos directos, solo si hay alguno), idioma y notificaciones.
 * El perfil, el tema, la apariencia y cerrar sesión viven en el menú de usuario del pie del sidebar.
 */
export function AccionesBarra() {
  return (
    <div className="flex items-center gap-1.5">
      <BarraRecursos />
      <BarraIdioma />
      <BarraNotificaciones />
    </div>
  );
}
