"use client";

import { useTranslations } from "next-intl";

import { AccionesBarra } from "@/components/app/acciones-barra";
import { CLAVES_BARRA_SUPERIOR } from "@/components/app/barra-superior-textos";
import { NAVEGACION, RUTAS_EXTRA } from "@/components/app/navegacion";
import { SidebarApp } from "@/components/app/sidebar-app";
import { MobileNav } from "@/components/nav/mobile-nav";
import { type Miga, TopBar } from "@/components/nav/top-bar";

/** TopBar de khipu con la miga sacada de NAVEGACION (prefijos más largos primero: gana la primera coincidencia). */
export function BarraSuperior() {
  const t = useTranslations("dashboard");

  const migas: Miga[] = [
    ...RUTAS_EXTRA.map((r) => ({ prefijo: r.prefijo, seccion: t(r.seccion), pagina: t(r.pagina) })),
    ...NAVEGACION.flatMap((g) =>
      g.items.map((i) => ({ prefijo: i.href, seccion: t(g.titulo), pagina: t(i.etiqueta) })),
    ),
  ].sort((a, b) => b.prefijo.length - a.prefijo.length);

  return (
    <TopBar
      migas={migas}
      etiquetaUbicacion={t(CLAVES_BARRA_SUPERIOR.ubicacion)}
      menuMovil={
        <MobileNav etiqueta={t(CLAVES_BARRA_SUPERIOR.abrirMenu)}>
          <SidebarApp />
        </MobileNav>
      }
      buscador={false}
      acciones={<AccionesBarra />}
    />
  );
}
