"use client";

import type { ReactNode } from "react";

import { useApariencia } from "@/components/apariencia";
import { useDiseno } from "@/components/diseno";
import { SelectorPaleta, SelectorTipografia, SelectorTonoOscuro } from "@/components/nav/selector-apariencia";
import { SelectorDiseno } from "@/components/nav/selector-diseno";
import { type TextosThemeToggle, ThemeToggle } from "@/components/nav/theme-toggle";
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

/** Textos de `PanelApariencia` (incluye los de `ThemeToggle`): pasa los que quieras traducir, el resto queda en castellano. */
export type TextosPanelApariencia = {
  aparienciaTitulo: string;
  aparienciaDescripcion: string;
  /** Rótulo de la sección del tema claro/oscuro; el menú de usuario usa el mismo. */
  tema: string;
  paleta: string;
  paletaAyuda: string;
  tonoOscuro: string;
  tonoOscuroAyuda: string;
  tipografia: string;
  tipografiaAyuda: string;
  /** Frase de muestra bajo el selector de tipografía. */
  tipografiaMuestra: ReactNode;
  diseno: string;
  /** `aria-label` del selector de diseño. */
  disenoInterfaz: string;
  /** `aria-label` de los grupos de muestras de paleta, de tono oscuro y de tipografía. */
  paletaColor: string;
  tonoModoOscuro: string;
  tipografiaFuente: string;
  /** `aria-label` del botón de cerrar el panel. */
  cerrar: string;
};

export const TEXTOS_PANEL_APARIENCIA_ES: TextosPanelApariencia = {
  aparienciaTitulo: "Apariencia",
  aparienciaDescripcion: "Se aplica al instante y se guarda en este navegador.",
  tema: "Tema",
  paleta: "Paleta",
  paletaAyuda: "El color de acento: ítem activo, métricas destacadas y gráficos.",
  tonoOscuro: "Tono oscuro",
  tonoOscuroAyuda: "Se usa cuando el tema es oscuro.",
  tipografia: "Tipografía",
  tipografiaAyuda: "Los datos (correlativos, ids) siguen en JetBrains Mono.",
  tipografiaMuestra: (
    <>
      Factura <span className="font-mono">F001-00000136</span> · S/ 4,820.00 aceptada por SUNAT.
    </>
  ),
  diseno: "Diseño",
  disenoInterfaz: "Diseño de la interfaz",
  paletaColor: "Paleta de color",
  tonoModoOscuro: "Tono del modo oscuro",
  tipografiaFuente: "Tipografía",
  cerrar: "Cerrar",
};

/**
 * Panel derecho con las preferencias de apariencia del usuario: tema, paleta, tono oscuro, tipografía y diseño. Cada
 * cambio se aplica al instante (la página de atrás cambia en vivo) y se guarda como el resto de las preferencias.
 * Lo abre «Apariencia…» del menú de usuario; las secciones sin opciones (sin proveedor) no se muestran.
 */
export function PanelApariencia({
  open,
  onOpenChange,
  textos: textosParciales,
}: {
  open: boolean;
  onOpenChange: (abierto: boolean) => void;
  textos?: Partial<TextosPanelApariencia & TextosThemeToggle>;
}) {
  const textos = { ...TEXTOS_PANEL_APARIENCIA_ES, ...textosParciales };
  const { paletas, tonosOscuro, fuentes } = useApariencia();
  const { disenos } = useDiseno();

  return (
    <PanelLateral
      open={open}
      onOpenChange={onOpenChange}
      icon={PaletteIcon}
      titulo={textos.aparienciaTitulo}
      descripcion={textos.aparienciaDescripcion}
      ancho="sm:max-w-sm"
      etiquetaCerrar={textos.cerrar}
    >
      <div className="grid gap-6 pb-4">
        <Seccion titulo={textos.tema}>
          <ThemeToggle conTexto textos={textosParciales} />
        </Seccion>
        {paletas.length > 1 ? (
          <Seccion titulo={textos.paleta} ayuda={textos.paletaAyuda}>
            <SelectorPaleta grande etiqueta={textos.paletaColor} />
          </Seccion>
        ) : null}
        {tonosOscuro.length > 1 ? (
          <Seccion titulo={textos.tonoOscuro} ayuda={textos.tonoOscuroAyuda}>
            <SelectorTonoOscuro grande etiqueta={textos.tonoModoOscuro} />
          </Seccion>
        ) : null}
        {fuentes.length > 1 ? (
          <Seccion titulo={textos.tipografia} ayuda={textos.tipografiaAyuda}>
            <SelectorTipografia grande etiqueta={textos.tipografiaFuente} />
            <p className="bg-secondary text-cuerpo text-foreground rounded-xl px-3 py-2.5">
              {textos.tipografiaMuestra}
            </p>
          </Seccion>
        ) : null}
        {disenos.length > 1 ? (
          <Seccion titulo={textos.diseno}>
            <SelectorDiseno etiqueta={textos.disenoInterfaz} />
          </Seccion>
        ) : null}
      </div>
    </PanelLateral>
  );
}
