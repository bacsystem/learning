"use client";

import { useTranslations } from "next-intl";
import { useState } from "react";

import { CabeceraDialogo, PieDialogo } from "@/components/patrones/cabecera-dialogo";
import { IconoSeccion } from "@/components/patrones/cabecera-seccion";
import { Dialog, DialogContent } from "@/components/ui/dialog";
import { Sheet, SheetContent, SheetTitle } from "@/components/ui/sheet";
import { BOTON_DESTRUCTIVO, BOTON_SECUNDARIO, FOCO, TARJETA } from "@/lib/estilos";
import { PESO_ICONO, PESO_ICONO_ACTIVO } from "@/lib/iconos";
import type { PreguntaSimulacro } from "@/lib/simulations/api";
import { cn } from "@/lib/utils";
import { FlagIcon, ListNumbersIcon, PaperPlaneTiltIcon } from "@phosphor-icons/react";

interface NavigationPanelProps {
  preguntas: PreguntaSimulacro[];
  currentIdx: number;
  onNavigate: (idx: number) => void;
  markedIds: Set<number>;
  onFinish: () => void;
  /** Answers are still being saved: «Terminar» waits (H123). */
  guardando?: boolean;
}

type EstadoCelda = "actual" | "respondida" | "marcada" | "pendiente";

/** Colores de cada estado de la grilla: el mismo mapa pinta las celdas y la leyenda. */
const CELDA: Record<EstadoCelda, string> = {
  actual: "bg-primary text-primary-foreground font-semibold ring-2 ring-primary ring-offset-2 ring-offset-card",
  respondida:
    "bg-success text-success-foreground hover:bg-[color-mix(in_srgb,var(--success),var(--success-foreground)_10%)]",
  marcada:
    "bg-warning text-warning-foreground hover:bg-[color-mix(in_srgb,var(--warning),var(--warning-foreground)_10%)]",
  pendiente: "bg-secondary text-muted-foreground hover:bg-[color-mix(in_srgb,var(--secondary),var(--foreground)_6%)]",
};

function Bandera({ className }: { className?: string }) {
  return <FlagIcon aria-hidden weight={PESO_ICONO_ACTIVO} className={cn("text-warning-solid size-3", className)} />;
}

/** Relleno de cada estado sin hover ni anillo, para la leyenda. */
const MUESTRA: Record<EstadoCelda, string> = {
  actual: "bg-primary",
  respondida: "bg-success ring-1 ring-success-solid/40",
  marcada: "bg-warning ring-1 ring-warning-solid/40",
  pendiente: "bg-secondary ring-1 ring-border",
};

function Muestra({ estado, conBandera }: { estado: EstadoCelda; conBandera?: boolean }) {
  return (
    <span className="relative shrink-0">
      <span className={cn("block size-4 rounded-md", MUESTRA[estado])} />
      {conBandera ? <Bandera className="absolute -top-1.5 -right-1.5" /> : null}
    </span>
  );
}

function PanelContent({
  preguntas,
  currentIdx,
  onNavigate,
  markedIds,
  onFinish,
  guardando = false,
}: NavigationPanelProps) {
  const t = useTranslations("dashboard");
  const [confirmOpen, setConfirmOpen] = useState(false);

  const respondidas = preguntas.filter((p) => p.respuesta_ids.length > 0).length;
  const sinResponder = preguntas.length - respondidas;
  const marcadas = markedIds.size;

  return (
    <div className="flex flex-col gap-4 p-4">
      <div className="flex items-center gap-2 pr-8 md:pr-0">
        <IconoSeccion icon={ListNumbersIcon} />
        <h2 className="text-titulo text-foreground">{t("simulacros-navegacion")}</h2>
      </div>

      {/* Resumen */}
      <div className="text-etiqueta text-muted-foreground flex flex-wrap gap-x-3 gap-y-1.5">
        <span className="inline-flex items-center gap-1.5">
          <span className="bg-success-solid size-1.5 rounded-full" />
          {t("simulacros-respondidas", { count: respondidas })}
        </span>
        <span className="inline-flex items-center gap-1.5">
          <span className="bg-muted-foreground/50 size-1.5 rounded-full" />
          {t("simulacros-sin-responder-count", { count: sinResponder })}
        </span>
        <span className={cn("inline-flex items-center gap-1.5", marcadas === 0 && "text-texto-terciario")}>
          <FlagIcon
            aria-hidden
            weight={marcadas > 0 ? PESO_ICONO_ACTIVO : PESO_ICONO}
            className={cn("size-3", marcadas > 0 && "text-warning-solid")}
          />
          <span className="tabular-nums">{marcadas}</span> {t("simulacros-marcadas-label")}
        </span>
      </div>

      {/* Grilla de preguntas */}
      <div className="grid grid-cols-[repeat(auto-fill,minmax(36px,1fr))] gap-1.5">
        {preguntas.map((pq, idx) => {
          const isAnswered = pq.respuesta_ids.length > 0;
          const isMarked = markedIds.has(pq.id);
          const isCurrent = idx === currentIdx;
          const estado: EstadoCelda = isCurrent
            ? "actual"
            : isAnswered
              ? "respondida"
              : isMarked
                ? "marcada"
                : "pendiente";

          return (
            <div key={pq.id} className="relative">
              <button
                type="button"
                onClick={() => onNavigate(idx)}
                aria-current={isCurrent ? "step" : undefined}
                aria-label={`${t("simulacros-pregunta")} ${idx + 1}`}
                className={cn(
                  "text-control ease-salida grid h-9 w-full place-items-center rounded-lg tabular-nums transition-colors duration-(--duracion-rapida)",
                  FOCO,
                  CELDA[estado],
                )}
              >
                {idx + 1}
              </button>
              {isMarked ? <Bandera className="pointer-events-none absolute -top-1 -right-1" /> : null}
            </div>
          );
        })}
      </div>

      {/* Leyenda */}
      <ul className="border-border/60 text-etiqueta text-muted-foreground grid gap-2 border-t pt-3">
        <li className="flex items-center gap-2">
          <Muestra estado="actual" />
          {t("simulacros-leyenda-actual")}
        </li>
        <li className="flex items-center gap-2">
          <Muestra estado="respondida" />
          {t("simulacros-leyenda-respondida")}
        </li>
        <li className="flex items-center gap-2">
          <Muestra estado="pendiente" />
          {t("simulacros-leyenda-sin-responder")}
        </li>
        <li className="flex items-center gap-2">
          <Muestra estado="marcada" conBandera />
          {t("simulacros-leyenda-marcada")}
        </li>
      </ul>

      <button
        type="button"
        className={cn(BOTON_DESTRUCTIVO, "w-full")}
        disabled={guardando}
        onClick={() => setConfirmOpen(true)}
      >
        <PaperPlaneTiltIcon weight={PESO_ICONO} className="size-4" />
        {guardando ? t("comun-guardando") : t("simulacros-terminar")}
      </button>

      <Dialog open={confirmOpen} onOpenChange={setConfirmOpen}>
        <DialogContent className="gap-0 p-0 sm:max-w-md">
          <CabeceraDialogo
            icon={PaperPlaneTiltIcon}
            tono="peligro"
            titulo={t("simulacros-terminar")}
            descripcion={t("simulacros-confirmar-terminar", { sinResponder })}
          />
          <PieDialogo>
            <button type="button" className={BOTON_SECUNDARIO} onClick={() => setConfirmOpen(false)}>
              {t("simulacros-volver")}
            </button>
            <button
              type="button"
              className={BOTON_DESTRUCTIVO}
              onClick={() => {
                setConfirmOpen(false);
                onFinish();
              }}
            >
              {t("simulacros-enviar-todo")}
            </button>
          </PieDialogo>
        </DialogContent>
      </Dialog>
    </div>
  );
}

export function SimulacroNavigationPanel(props: NavigationPanelProps) {
  const t = useTranslations("dashboard");
  const [drawerOpen, setDrawerOpen] = useState(false);

  return (
    <>
      {/* Escritorio: panel fijo al hacer scroll */}
      <aside
        className={cn(TARJETA, "sticky top-20 hidden max-h-[calc(100vh-100px)] overflow-y-auto md:block")}
        aria-label={t("simulacros-navegacion")}
      >
        <PanelContent {...props} />
      </aside>

      {/* Móvil: botón flotante + hoja inferior */}
      <div className="md:hidden">
        <button
          type="button"
          onClick={() => setDrawerOpen(true)}
          aria-label={t("simulacros-navegacion")}
          title={t("simulacros-navegacion")}
          className={cn(
            "bg-primary text-primary-foreground hover:bg-primary/85 fixed right-4 bottom-20 z-40 grid size-12 place-items-center rounded-full shadow-lg transition-colors",
            FOCO,
          )}
        >
          <ListNumbersIcon weight={PESO_ICONO} className="size-5" />
        </button>
        <Sheet open={drawerOpen} onOpenChange={setDrawerOpen}>
          <SheetContent side="bottom" className="max-h-[70vh] gap-0 overflow-y-auto">
            <SheetTitle className="sr-only">{t("simulacros-navegacion")}</SheetTitle>
            <PanelContent {...props} />
          </SheetContent>
        </Sheet>
      </div>
    </>
  );
}
