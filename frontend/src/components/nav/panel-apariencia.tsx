"use client";

import type { ReactNode } from "react";

import { useApariencia } from "@/components/apariencia";
import { useDiseno } from "@/components/diseno";
import { SelectorPaleta, SelectorTipografia, SelectorTonoOscuro } from "@/components/nav/selector-apariencia";
import { SelectorDiseno } from "@/components/nav/selector-diseno";
import { ThemeToggle } from "@/components/nav/theme-toggle";
import { PanelLateral } from "@/components/navegacion/panel-lateral";
import { ROTULO } from "@/lib/estilos";
import { PaletteIcon } from "@phosphor-icons/react";

function Seccion({ titulo, ayuda, children }: { titulo: string; ayuda?: string; children: ReactNode }) {
  return (
    <section className="grid gap-2.5">
      <div>
        <h3 className={ROTULO}>{titulo}</h3>
        {ayuda ? <p className="text-etiqueta text-texto-terciario mt-0.5">{ayuda}</p> : null}
      </div>
      {children}
    </section>
  );
}

/**
 * Panel derecho con las preferencias de apariencia del usuario: tema, paleta, tono oscuro, tipografía y diseño. Cada
 * cambio se aplica al instante (la página de atrás cambia en vivo) y se guarda como el resto de las preferencias.
 * Lo abre «Apariencia…» del menú de usuario; las secciones sin opciones (sin proveedor) no se muestran.
 */
export function PanelApariencia({ open, onOpenChange }: { open: boolean; onOpenChange: (abierto: boolean) => void }) {
  const { paletas, tonosOscuro, fuentes } = useApariencia();
  const { disenos } = useDiseno();

  return (
    <PanelLateral
      open={open}
      onOpenChange={onOpenChange}
      icon={PaletteIcon}
      titulo="Apariencia"
      descripcion="Se aplica al instante y se guarda en este navegador."
      ancho="sm:max-w-sm"
    >
      <div className="grid gap-6 pb-4">
        <Seccion titulo="Tema">
          <ThemeToggle conTexto />
        </Seccion>
        {paletas.length > 1 ? (
          <Seccion titulo="Paleta" ayuda="El color de acento: ítem activo, métricas destacadas y gráficos.">
            <SelectorPaleta grande />
          </Seccion>
        ) : null}
        {tonosOscuro.length > 1 ? (
          <Seccion titulo="Tono oscuro" ayuda="Se usa cuando el tema es oscuro.">
            <SelectorTonoOscuro grande />
          </Seccion>
        ) : null}
        {fuentes.length > 1 ? (
          <Seccion titulo="Tipografía" ayuda="Los datos (correlativos, ids) siguen en JetBrains Mono.">
            <SelectorTipografia grande />
            <p className="bg-secondary text-cuerpo text-foreground rounded-xl px-3 py-2.5">
              Factura <span className="font-mono">F001-00000136</span> · S/ 4,820.00 aceptada por SUNAT.
            </p>
          </Seccion>
        ) : null}
        {disenos.length > 1 ? (
          <Seccion titulo="Diseño">
            <SelectorDiseno />
          </Seccion>
        ) : null}
      </div>
    </PanelLateral>
  );
}
