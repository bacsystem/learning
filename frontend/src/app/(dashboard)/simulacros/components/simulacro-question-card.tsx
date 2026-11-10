"use client";

import { useTranslations } from "next-intl";

import { Tooltip } from "@/components/feedback/tooltip";
import { Chip, PillEstado } from "@/components/patrones/pill-estado";
import { BOTON_PRIMARIO, BOTON_SECUNDARIO, FOCO, TARJETA } from "@/lib/estilos";
import { PESO_ICONO_ACTIVO, PESO_ICONO_FUNCIONAL } from "@/lib/iconos";
import type { PreguntaSimulacro } from "@/lib/simulations/api";
import { cn } from "@/lib/utils";
import { CaretLeftIcon, CaretRightIcon, CheckIcon, FlagIcon } from "@phosphor-icons/react";

interface SimulacroQuestionCardProps {
  pregunta: PreguntaSimulacro;
  /** Selected alternatives (at most one for 'unica'). */
  selectedIds: number[];
  onSelect: (respuestaId: number) => void;
  onNext?: () => void;
  onPrev?: () => void;
  onMarcar: () => void;
  marcada: boolean;
}

export function SimulacroQuestionCard({
  pregunta,
  selectedIds,
  onSelect,
  onNext,
  onPrev,
  onMarcar,
  marcada,
}: SimulacroQuestionCardProps) {
  const t = useTranslations("dashboard");
  const multiple = pregunta.tipo === "multiple";

  return (
    <article className={cn(TARJETA, "flex flex-col gap-5 p-4 sm:p-6")}>
      {/* Cabecera */}
      <header className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          <h2 className="text-titulo text-foreground">
            {t("simulacros-pregunta")} <span className="tabular-nums">{pregunta.orden}</span>
          </h2>
          <Chip>{pregunta.asignatura}</Chip>
          {selectedIds.length > 0 && <PillEstado tono="ok">{t("simulacros-estado-respondida")}</PillEstado>}
        </div>
        <Tooltip texto={t("simulacros-marcar")}>
          <button
            type="button"
            onClick={onMarcar}
            aria-label={t("simulacros-marcar")}
            aria-pressed={marcada}
            className={cn(
              "size-control grid place-items-center rounded-full transition-colors",
              FOCO,
              marcada
                ? "bg-warning text-warning-foreground"
                : "text-muted-foreground hover:bg-secondary hover:text-foreground",
            )}
          >
            <FlagIcon weight={marcada ? PESO_ICONO_ACTIVO : PESO_ICONO_FUNCIONAL} className="size-4" />
          </button>
        </Tooltip>
      </header>

      {/* Enunciado */}
      <p className="text-titulo-pagina text-foreground whitespace-pre-line">{pregunta.enunciado}</p>

      {multiple && <p className="text-cuerpo text-muted-foreground -mt-2">{t("simulacros-multiple-hint")}</p>}

      {/* Respuestas: sin retroalimentación de acierto durante el simulacro */}
      <div
        role={multiple ? "group" : "radiogroup"}
        aria-label={`${t("simulacros-pregunta")} ${pregunta.orden}`}
        className="flex flex-col gap-2"
      >
        {pregunta.respuestas.map((r) => {
          const isSelected = selectedIds.includes(r.id);

          return (
            <button
              key={r.id}
              type="button"
              role={multiple ? "checkbox" : "radio"}
              aria-checked={isSelected}
              onClick={() => onSelect(r.id)}
              className={cn(
                "ease-salida flex w-full items-center gap-3 rounded-xl px-3 py-2.5 text-left transition-[background-color,box-shadow] duration-(--duracion-rapida)",
                FOCO,
                isSelected
                  ? "bg-accent text-accent-foreground ring-acento-texto dark:ring-acento ring-2"
                  : "bg-secondary text-foreground hover:bg-[color-mix(in_srgb,var(--secondary),var(--foreground)_5%)]",
              )}
            >
              {/* Indicador de selección: casilla para 'multiple', radio para 'unica' */}
              <span
                aria-hidden
                className={cn(
                  "grid size-4 shrink-0 place-items-center border-2 transition-colors",
                  multiple ? "rounded-[5px]" : "rounded-full",
                  isSelected
                    ? "border-primary bg-primary text-primary-foreground"
                    : "border-muted-foreground/50 bg-card",
                )}
              >
                {isSelected &&
                  (multiple ? (
                    <CheckIcon weight={PESO_ICONO_ACTIVO} className="size-3" />
                  ) : (
                    <span className="bg-primary-foreground size-1.5 rounded-full" />
                  ))}
              </span>
              <span
                className={cn(
                  "text-control grid size-8 shrink-0 place-items-center rounded-full font-semibold",
                  isSelected ? "bg-primary text-primary-foreground" : "bg-card text-muted-foreground",
                )}
              >
                {r.letra}
              </span>
              <span className={cn("text-titulo flex-1", isSelected ? "font-semibold" : "font-normal")}>{r.texto}</span>
            </button>
          );
        })}
      </div>

      {/* Navegación: solo Anterior/Siguiente, sin Comprobar */}
      <footer className="flex justify-between gap-2">
        <button type="button" className={BOTON_SECUNDARIO} onClick={onPrev} disabled={!onPrev}>
          <CaretLeftIcon weight={PESO_ICONO_FUNCIONAL} className="size-3.5" />
          {t("simulacros-anterior")}
        </button>
        <button type="button" className={BOTON_PRIMARIO} onClick={onNext} disabled={!onNext}>
          {t("simulacros-siguiente")}
          <CaretRightIcon weight={PESO_ICONO_FUNCIONAL} className="size-3.5" />
        </button>
      </footer>
    </article>
  );
}
