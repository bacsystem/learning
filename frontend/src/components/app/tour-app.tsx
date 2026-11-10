"use client";

import "driver.js/dist/driver.css";
import "./tour-app.css";

import { driver } from "driver.js";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useEffect, useRef } from "react";

import { tourApi } from "@/lib/auth/auth-api";
import { useAuth } from "@/lib/auth/auth-context";

// Page to navigate to before highlighting the element at that step index
const STEP_ROUTES: Record<number, string> = {
  3: "/practicas",
  4: "/practicas",
  5: "/practicas/nueva",
  6: "/practicas/nueva",
  8: "/simulacros",
  9: "/simulacros",
  10: "/simulacros/nueva",
  11: "/simulacros/nueva",
};

/**
 * Elemento del sidebar para un paso del tour. El menú de la plantilla ponía `id="practicas"`, `id="settings"`…; el
 * `SidebarNav` de khipu no admite ids, así que se busca el enlace por `href` (o el `data-tour` del bloque de usuario) solo
 * en el sidebar visible. Si no hay ninguno visible (teléfono, sidebar en el cajón cerrado), driver.js centra el popover.
 */
function enSidebar(idOriginal: string, selector: string): () => Element {
  return () => {
    const porId = document.getElementById(idOriginal);
    if (porId) return porId;
    const visible = Array.from(document.querySelectorAll(`aside ${selector}`)).find(
      (el) => el.getClientRects().length > 0,
    );
    return visible as Element;
  };
}

/** Recorrido de bienvenida (driver.js) para quien aún no lo completó; al cerrarlo se marca como hecho en el backend. */
export function TourApp() {
  const { user, token, updateUser } = useAuth();
  const router = useRouter();
  const t = useTranslations("dashboard");
  const started = useRef(false);
  const completadoAntes = useRef(user?.tour_completado);

  // «Repetir recorrido» (ajustes) vuelve a poner `tour_completado` en false: se permite iniciarlo otra vez en la sesión.
  // Va antes del efecto que lo inicia para que, en el mismo render, este vea `started` ya reseteado.
  useEffect(() => {
    if (completadoAntes.current && user?.tour_completado === false) started.current = false;
    completadoAntes.current = user?.tour_completado;
  }, [user?.tour_completado]);

  useEffect(() => {
    if (!user || !token || user.tour_completado || started.current) return;

    const timer = setTimeout(() => {
      started.current = true;

      const driverObj = driver({
        showProgress: true,
        // driver.js sustituye `{{current}}` y `{{total}}`; se pasan como valores para no escapar llaves en el ICU.
        progressText: t("tour-progreso", { current: "{{current}}", total: "{{total}}" }),
        nextBtnText: t("tour-siguiente"),
        prevBtnText: t("tour-anterior"),
        doneBtnText: t("tour-listo"),
        allowClose: false,
        overlayOpacity: 0.6,
        stagePadding: 8,
        stageRadius: 10,
        popoverClass: "app-tour-popover",
        smoothScroll: true,
        onDestroyStarted: () => {
          tourApi.completar(token).catch(() => {});
          updateUser({ tour_completado: true });
          driverObj.destroy();
        },
        onNextClick: () => {
          const current = driverObj.getActiveIndex() ?? 0;
          const nextIndex = current + 1;
          const route = STEP_ROUTES[nextIndex];
          if (route) {
            router.push(route);
            setTimeout(() => driverObj.moveNext(), 700);
          } else {
            driverObj.moveNext();
          }
        },
        onPrevClick: () => {
          const current = driverObj.getActiveIndex() ?? 0;
          const prevIndex = current - 1;
          const route = STEP_ROUTES[prevIndex];
          if (route) {
            router.push(route);
            setTimeout(() => driverObj.movePrevious(), 700);
          } else {
            driverObj.movePrevious();
          }
        },
        steps: [
          // 0 — Bienvenida
          {
            popover: {
              title: t("tour-bienvenida-titulo"),
              description: t("tour-bienvenida-descripcion"),
              side: "over",
              align: "center",
            },
          },
          // 1 — Prácticas (módulo principal)
          {
            element: enSidebar("practicas", 'a[href="/practicas"]'),
            popover: {
              title: t("tour-practicas-titulo"),
              description: t("tour-practicas-descripcion"),
              side: "right",
              align: "center",
            },
          },
          // 2 — Sidebar: Nueva práctica
          {
            element: enSidebar("practicas-nueva", 'a[href="/practicas/nueva"]'),
            popover: {
              title: t("tour-practicas-nueva-titulo"),
              description: t("tour-practicas-nueva-descripcion"),
              side: "right",
              align: "center",
            },
          },
          // 3 — Lista de prácticas: historial
          {
            element: "#tour-practicas-btn-nueva",
            popover: {
              title: t("tour-practicas-historial-titulo"),
              description: t("tour-practicas-historial-descripcion"),
              side: "bottom",
              align: "end",
            },
          },
          // 4 — Lista de prácticas: botón nueva
          {
            element: "#tour-practicas-btn-nueva",
            popover: {
              title: t("tour-practicas-iniciar-titulo"),
              description: t("tour-practicas-iniciar-descripcion"),
              side: "bottom",
              align: "end",
            },
          },
          // 5 — Formulario nueva práctica: selector de proceso
          {
            element: "#tour-practica-proceso",
            popover: {
              title: t("tour-practica-proceso-titulo"),
              description: t("tour-practica-proceso-descripcion"),
              side: "bottom",
              align: "start",
            },
          },
          // 6 — Formulario nueva práctica: botón iniciar
          {
            element: "#tour-practica-iniciar",
            popover: {
              title: t("tour-practica-comenzar-titulo"),
              description: t("tour-practica-comenzar-descripcion"),
              side: "top",
              align: "center",
            },
          },
          // 7 — Simulacros (módulo principal)
          {
            element: enSidebar("simulacros", 'a[href="/simulacros"]'),
            popover: {
              title: t("tour-simulacros-titulo"),
              description: t("tour-simulacros-descripcion"),
              side: "right",
              align: "center",
            },
          },
          // 8 — Lista de simulacros: historial
          {
            element: "#tour-simulacros-btn-nuevo",
            popover: {
              title: t("tour-simulacros-historial-titulo"),
              description: t("tour-simulacros-historial-descripcion"),
              side: "bottom",
              align: "end",
            },
          },
          // 9 — Lista de simulacros: botón nuevo
          {
            element: "#tour-simulacros-btn-nuevo",
            popover: {
              title: t("tour-simulacros-nuevo-titulo"),
              description: t("tour-simulacros-nuevo-descripcion"),
              side: "bottom",
              align: "end",
            },
          },
          // 10 — Formulario nuevo simulacro: selector de proceso
          {
            element: "#tour-simulacro-proceso",
            popover: {
              title: t("tour-simulacro-proceso-titulo"),
              description: t("tour-simulacro-proceso-descripcion"),
              side: "bottom",
              align: "start",
            },
          },
          // 11 — Formulario nuevo simulacro: botón iniciar
          {
            element: "#tour-simulacro-iniciar",
            popover: {
              title: t("tour-simulacro-iniciar-titulo"),
              description: t("tour-simulacro-iniciar-descripcion"),
              side: "top",
              align: "center",
            },
          },
          // 12 — Settings
          {
            element: enSidebar("settings", '[data-tour="settings"]'),
            popover: {
              title: t("tour-perfil-titulo"),
              description: t("tour-perfil-descripcion"),
              side: "right",
              align: "center",
            },
          },
        ],
      });

      driverObj.drive();
    }, 800);

    return () => clearTimeout(timer);
  }, [user, token, updateUser, router, t]);

  return null;
}
