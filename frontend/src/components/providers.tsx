"use client";

import { ThemeProvider } from "next-themes";
import type { ComponentProps } from "react";

import { AparienciaProvider } from "@/components/apariencia";
import { DisenoProvider } from "@/components/diseno";
import { ToastProvider } from "@/components/feedback/toast";
import { TooltipProvider } from "@/components/feedback/tooltip";
import { ProveedorMovimiento } from "@/components/movimiento";
import { PESO_ICONO } from "@/lib/iconos";
import { IconContext, type IconProps } from "@phosphor-icons/react";

// Fuera del render: un objeto nuevo en cada render volvería a pintar todos los iconos.
const ICONOS: IconProps = { weight: PESO_ICONO };

/**
 * attribute="class" pone `.dark` en <html>, que es lo que espera el @custom-variant de src/styles/theme.css.
 * `DisenoProvider` maneja `data-diseno`; `AparienciaProvider` la paleta y el tono oscuro (`apariencia` le pasa
 * `inicial` y `onCambio` para guardarlos en el perfil del usuario). Los iconos de Phosphor salen en duotono.
 */
export function Providers({
  children,
  apariencia,
}: {
  children: React.ReactNode;
  apariencia?: Omit<ComponentProps<typeof AparienciaProvider>, "children">;
}) {
  return (
    <ThemeProvider attribute="class" defaultTheme="system" enableSystem disableTransitionOnChange>
      <DisenoProvider>
        <AparienciaProvider {...apariencia}>
          <IconContext.Provider value={ICONOS}>
            <ProveedorMovimiento>
              <TooltipProvider delay={300}>
                <ToastProvider>{children}</ToastProvider>
              </TooltipProvider>
            </ProveedorMovimiento>
          </IconContext.Provider>
        </AparienciaProvider>
      </DisenoProvider>
    </ThemeProvider>
  );
}
