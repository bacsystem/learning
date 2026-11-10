"use client";

import { Alternativa, type EstadoAlternativa } from "./alternativa";
import { useTranslations } from "next-intl";

import { Tooltip } from "@/components/feedback/tooltip";
import { Chip } from "@/components/patrones/pill-estado";
import { BOTON_PRIMARIO, BOTON_SECUNDARIO, FOCO, TARJETA } from "@/lib/estilos";
import type { PreguntaPractica, ValidarPreguntaResponse } from "@/lib/practices/api";
import { cn } from "@/lib/utils";
import {
  ArrowLeftIcon,
  ArrowRightIcon,
  CheckCircleIcon,
  FlagIcon,
  ListChecksIcon,
  XCircleIcon,
} from "@phosphor-icons/react";

interface QuestionCardProps {
  pregunta: PreguntaPractica;
  selectedIds: number[];
  onToggle: (respuestaId: number) => void;
  mode: "inmediato" | "al_final";
  validacion?: ValidarPreguntaResponse;
  onComprobar?: () => void;
  onNext?: () => void;
  onPrev?: () => void;
  onMarcar: () => void;
  marcada: boolean;
}

function getAnswerState(
  respuestaId: number,
  isSelected: boolean,
  validacion?: ValidarPreguntaResponse,
): EstadoAlternativa {
  if (!validacion) return isSelected ? "selected" : "none";
  // respuesta_correcta_id only exists for 'unica'; the set covers both tipos (H83).
  if (validacion.respuesta_correcta_ids.includes(respuestaId)) return "correct";
  if (isSelected && !validacion.es_correcta) return "wrong";
  return "none";
}

export function QuestionCard({
  pregunta,
  selectedIds,
  onToggle,
  mode,
  validacion,
  onComprobar,
  onNext,
  onPrev,
  onMarcar,
  marcada,
}: QuestionCardProps) {
  const t = useTranslations("dashboard");
  const hasSelection = selectedIds.length > 0;
  const isValidated = !!validacion;

  return (
    <section className={cn(TARJETA, "flex flex-col gap-5 p-4 sm:p-6")} aria-labelledby={`pregunta-${pregunta.id}`}>
      {/* Cabecera: número, asignatura y marcar */}
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          <span className="text-control text-foreground font-semibold">
            {t("practicas-pregunta")} <span className="tabular-nums">{pregunta.orden}</span>
          </span>
          {pregunta.asignatura ? <Chip className="font-sans">{pregunta.asignatura}</Chip> : null}
        </div>
        <Tooltip texto={t("practicas-marcar")}>
          <button
            type="button"
            onClick={onMarcar}
            aria-pressed={marcada}
            aria-label={t("practicas-marcar")}
            className={cn(
              "size-control ease-salida grid place-items-center rounded-full transition-colors duration-(--duracion-rapida)",
              FOCO,
              marcada
                ? "bg-warning text-warning-foreground"
                : "text-muted-foreground hover:bg-secondary hover:text-foreground",
            )}
          >
            <FlagIcon weight={marcada ? "fill" : "duotone"} className="size-[18px]" />
          </button>
        </Tooltip>
      </div>

      {/* Enunciado */}
      <h2 id={`pregunta-${pregunta.id}`} className="text-titulo-pagina text-foreground whitespace-pre-line">
        {pregunta.enunciado}
      </h2>

      {/* Alternativas */}
      <div className="flex flex-col gap-2.5" role="group" aria-labelledby={`pregunta-${pregunta.id}`}>
        {pregunta.respuestas.map((r) => {
          const isSelected = selectedIds.includes(r.id);
          return (
            <Alternativa
              key={r.id}
              letra={r.letra}
              texto={r.texto}
              estado={getAnswerState(r.id, isSelected, validacion)}
              onSeleccionar={isValidated ? undefined : () => onToggle(r.id)}
              elegida={isValidated ? isSelected : undefined}
            />
          );
        })}
      </div>

      {/* Retroalimentación tras comprobar */}
      {isValidated && validacion && (
        <div
          role="status"
          className={cn(
            "flex items-start gap-3 rounded-xl px-4 py-3",
            validacion.es_correcta
              ? "bg-success text-success-foreground"
              : "bg-destructive-suave text-destructive-texto",
          )}
        >
          {validacion.es_correcta ? (
            <CheckCircleIcon aria-hidden weight="duotone" className="mt-0.5 size-5 shrink-0" />
          ) : (
            <XCircleIcon aria-hidden weight="duotone" className="mt-0.5 size-5 shrink-0" />
          )}
          <div className="min-w-0 flex-1">
            <p className="text-titulo">
              {validacion.es_correcta ? t("practicas-correcto") : t("practicas-incorrecto")}
            </p>
            {validacion.explicacion && (
              <p className="text-cuerpo text-foreground mt-1 leading-relaxed whitespace-pre-line">
                {validacion.explicacion}
              </p>
            )}
          </div>
        </div>
      )}

      {/* Acciones */}
      <div className="border-border/60 flex items-center justify-between gap-2 border-t pt-4">
        {mode === "al_final" ? (
          <>
            <button type="button" className={BOTON_SECUNDARIO} onClick={onPrev} disabled={!onPrev}>
              <ArrowLeftIcon weight="bold" className="size-3.5" />
              {t("practicas-anterior")}
            </button>
            <button type="button" className={BOTON_PRIMARIO} onClick={onNext} disabled={!onNext}>
              {t("practicas-siguiente")}
              <ArrowRightIcon weight="bold" className="size-3.5" />
            </button>
          </>
        ) : (
          <>
            <span />
            {!isValidated && (
              <button type="button" className={BOTON_PRIMARIO} disabled={!hasSelection} onClick={onComprobar}>
                <ListChecksIcon weight="duotone" className="size-4" />
                {t("practicas-comprobar")}
              </button>
            )}
            {isValidated && (
              <button type="button" className={BOTON_PRIMARIO} onClick={onNext} disabled={!onNext}>
                {t("practicas-siguiente")}
                <ArrowRightIcon weight="bold" className="size-3.5" />
              </button>
            )}
          </>
        )}
      </div>
    </section>
  );
}
