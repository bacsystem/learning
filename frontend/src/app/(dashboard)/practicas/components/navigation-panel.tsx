"use client";

import { useTranslations } from "next-intl";
import { useState } from "react";

import { DialogContent } from "@/components/app/dialogo";
import { SheetContent } from "@/components/app/dialogo";
import { Tooltip } from "@/components/feedback/tooltip";
import { CabeceraDialogo, PieDialogo } from "@/components/patrones/cabecera-dialogo";
import { Dialog } from "@/components/ui/dialog";
import { Sheet, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { BOTON_DESTRUCTIVO, BOTON_SECUNDARIO, FOCO, TARJETA } from "@/lib/estilos";
import type { PreguntaPractica } from "@/lib/practices/api";
import { cn } from "@/lib/utils";
import { FlagCheckeredIcon, FlagIcon, SquaresFourIcon } from "@phosphor-icons/react";

interface NavigationPanelProps {
  preguntas: PreguntaPractica[];
  currentIdx: number;
  onNavigate: (idx: number) => void;
  markedIds: Set<number>;
  onFinish: () => void;
  /** Answers are still being saved: «Finalizar» waits (H124). */
  guardando?: boolean;
  modoCalificacion: "inmediato" | "al_final";
}

/** Estado de una celda de la grilla de preguntas (y de su muestra en la leyenda). */
type TipoCelda = "actual" | "correcta" | "incorrecta" | "respondida" | "marcada" | "sin-responder";

const CELDA: Record<TipoCelda, string> = {
  actual: "bg-primary text-primary-foreground font-semibold ring-2 ring-primary ring-offset-2 ring-offset-card",
  correcta: "bg-success-foreground text-card",
  incorrecta: "bg-destructive text-destructive-foreground",
  respondida: "bg-success text-success-foreground ring-1 ring-success-border",
  marcada: "bg-warning text-warning-foreground ring-1 ring-warning-border",
  "sin-responder": "bg-secondary text-muted-foreground",
};

const HOVER_CELDA: Record<TipoCelda, string> = {
  actual: "hover:bg-primary/85",
  correcta: "hover:bg-success-foreground/85",
  incorrecta: "hover:bg-destructive/90",
  respondida: "hover:bg-[color-mix(in_oklch,var(--success),var(--success-foreground)_10%)]",
  marcada: "hover:bg-[color-mix(in_oklch,var(--warning),var(--warning-foreground)_10%)]",
  "sin-responder": "hover:bg-[color-mix(in_oklch,var(--secondary),var(--foreground)_6%)] hover:text-foreground",
};

/** Bandera pequeña sobre la esquina de una celda marcada. */
function BanderaCelda({ className }: { className?: string }) {
  return (
    <span
      aria-hidden
      className={cn(
        "bg-card text-warning-foreground ring-border pointer-events-none absolute -top-1.5 -right-1.5 grid size-4 place-items-center rounded-full shadow-xs ring-1",
        className,
      )}
    >
      <FlagIcon weight="fill" className="size-2.5" />
    </span>
  );
}

function Muestra({ tipo, marcada }: { tipo: TipoCelda; marcada?: boolean }) {
  return (
    <span className="relative inline-flex shrink-0">
      <span aria-hidden className={cn("size-5 rounded-md", CELDA[tipo], tipo === "actual" && "ring-1 ring-offset-1")} />
      {marcada ? <BanderaCelda className="-top-1 -right-1 size-3 [&_svg]:size-2" /> : null}
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
  modoCalificacion,
}: NavigationPanelProps) {
  const showValidationColors = modoCalificacion === "inmediato";
  const t = useTranslations("dashboard");
  const [confirmOpen, setConfirmOpen] = useState(false);

  const respondidas = preguntas.filter((p) => p.respuesta_ids.length > 0).length;
  const sinResponder = preguntas.length - respondidas;
  const marcadas = markedIds.size;

  const leyenda: { tipo: TipoCelda; etiqueta: string; marcada?: boolean; visible: boolean }[] = [
    { tipo: "actual", etiqueta: t("practicas-leyenda-actual"), visible: true },
    { tipo: "respondida", etiqueta: t("practicas-leyenda-respondida"), visible: true },
    { tipo: "correcta", etiqueta: t("practicas-leyenda-correcta"), visible: showValidationColors },
    { tipo: "incorrecta", etiqueta: t("practicas-leyenda-incorrecta"), visible: showValidationColors },
    { tipo: "sin-responder", etiqueta: t("practicas-leyenda-sin-responder"), visible: true },
    { tipo: "marcada", etiqueta: t("practicas-leyenda-marcada"), marcada: true, visible: true },
  ];

  return (
    <div className="flex flex-col gap-4 p-4">
      {/* Resumen */}
      <div className="text-etiqueta text-muted-foreground flex flex-wrap items-center gap-x-3 gap-y-1.5">
        <span className="inline-flex items-center gap-1.5">
          <span aria-hidden className="bg-success-solid size-2 rounded-full" />
          <span className="tabular-nums">{t("practicas-respondidas", { count: respondidas })}</span>
        </span>
        <span className="inline-flex items-center gap-1.5">
          <span aria-hidden className="bg-muted-foreground/50 size-2 rounded-full" />
          <span className="tabular-nums">{t("practicas-sin-responder-count", { count: sinResponder })}</span>
        </span>
        <span className={cn("inline-flex items-center gap-1.5", marcadas === 0 && "text-texto-terciario")}>
          <FlagIcon
            aria-hidden
            weight={marcadas > 0 ? "fill" : "duotone"}
            className={cn("size-3.5", marcadas > 0 && "text-warning-foreground")}
          />
          <span className="tabular-nums">
            {marcadas} {t("practicas-marcadas-label")}
          </span>
        </span>
      </div>

      <h2 className="text-titulo text-foreground">{t("practicas-navegacion")}</h2>

      {/* Grilla de preguntas */}
      <div className="grid grid-cols-[repeat(auto-fill,minmax(2.25rem,1fr))] gap-2">
        {preguntas.map((pq, idx) => {
          const isCorrecta = pq.estado === "correcta";
          const isIncorrecta = pq.estado === "incorrecta";
          const isAnswered = pq.respuesta_ids.length > 0 && !isCorrecta && !isIncorrecta;
          const isMarked = markedIds.has(pq.id);
          const isCurrent = idx === currentIdx;
          const markedOnly = isMarked && pq.respuesta_ids.length === 0 && !isCurrent;

          let tipo: TipoCelda = "sin-responder";
          if (isCurrent) tipo = "actual";
          else if (isCorrecta && showValidationColors) tipo = "correcta";
          else if (isIncorrecta && showValidationColors) tipo = "incorrecta";
          else if (isAnswered || ((isCorrecta || isIncorrecta) && !showValidationColors)) tipo = "respondida";
          else if (markedOnly) tipo = "marcada";

          return (
            <div key={pq.id} className="relative">
              <button
                type="button"
                onClick={() => onNavigate(idx)}
                aria-label={`${t("practicas-pregunta")} ${idx + 1}`}
                aria-current={isCurrent ? "step" : undefined}
                className={cn(
                  "text-etiqueta ease-salida grid h-9 w-full place-items-center rounded-lg font-medium tabular-nums transition-colors duration-(--duracion-rapida)",
                  FOCO,
                  CELDA[tipo],
                  HOVER_CELDA[tipo],
                )}
              >
                {idx + 1}
              </button>
              {isMarked && <BanderaCelda />}
            </div>
          );
        })}
      </div>

      {/* Leyenda */}
      <ul className="bg-secondary grid gap-2 rounded-xl px-3 py-2.5">
        {leyenda
          .filter((l) => l.visible)
          .map((l) => (
            <li key={l.tipo} className="text-etiqueta text-muted-foreground flex items-center gap-2.5">
              <Muestra tipo={l.tipo} marcada={l.marcada} />
              {l.etiqueta}
            </li>
          ))}
      </ul>

      <button
        type="button"
        className={cn(BOTON_DESTRUCTIVO, "w-full")}
        disabled={guardando}
        onClick={() => setConfirmOpen(true)}
      >
        <FlagCheckeredIcon weight="duotone" className="size-4" />
        {guardando ? t("comun-guardando") : t("practicas-finalizar")}
      </button>

      <Dialog open={confirmOpen} onOpenChange={setConfirmOpen}>
        <DialogContent className="gap-0 p-0 sm:max-w-md">
          <CabeceraDialogo
            icon={FlagCheckeredIcon}
            tono="peligro"
            titulo={t("practicas-finalizar")}
            descripcion={t("practicas-confirmar-finalizar")}
          />
          <PieDialogo>
            <button type="button" className={BOTON_SECUNDARIO} onClick={() => setConfirmOpen(false)}>
              {t("practicas-volver")}
            </button>
            <button
              type="button"
              className={BOTON_DESTRUCTIVO}
              onClick={() => {
                setConfirmOpen(false);
                onFinish();
              }}
            >
              {t("practicas-enviar-todo")}
            </button>
          </PieDialogo>
        </DialogContent>
      </Dialog>
    </div>
  );
}

export function NavigationPanel(props: NavigationPanelProps) {
  const t = useTranslations("dashboard");
  const [drawerOpen, setDrawerOpen] = useState(false);

  return (
    <>
      {/* Escritorio: panel fijo al hacer scroll */}
      <aside
        aria-label={t("practicas-navegacion")}
        className={cn(TARJETA, "sticky top-20 hidden max-h-[calc(100vh-6rem)] overflow-y-auto md:block")}
      >
        <PanelContent {...props} />
      </aside>

      {/* Móvil: botón flotante + hoja inferior */}
      <div className="md:hidden">
        <Tooltip texto={t("practicas-navegacion")} lado="left">
          <button
            type="button"
            aria-label={t("practicas-navegacion")}
            onClick={() => setDrawerOpen(true)}
            className={cn(
              "bg-primary text-primary-foreground hover:bg-primary/85 fixed right-4 bottom-20 z-30 grid size-12 place-items-center rounded-full shadow-lg transition-colors",
              FOCO,
            )}
          >
            <SquaresFourIcon weight="duotone" className="size-5" />
          </button>
        </Tooltip>
        <Sheet open={drawerOpen} onOpenChange={setDrawerOpen}>
          <SheetContent side="bottom" className="max-h-[75vh] gap-0 overflow-y-auto">
            <SheetHeader className="sr-only">
              <SheetTitle>{t("practicas-navegacion")}</SheetTitle>
            </SheetHeader>
            <div className="pt-8">
              <PanelContent {...props} />
            </div>
          </SheetContent>
        </Sheet>
      </div>
    </>
  );
}
