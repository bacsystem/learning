"use client";

import { useParams, useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { type ReactNode, useEffect, useMemo, useState } from "react";

import { EstadoVacio } from "@/components/feedback/estado-vacio";
import { Spinner } from "@/components/feedback/spinner";
import { Acordeon } from "@/components/navegacion/acordeon";
import { Tabs } from "@/components/navegacion/tabs";
import { CabeceraPagina, EnlaceVolver } from "@/components/patrones/cabecera-pagina";
import { Chip } from "@/components/patrones/pill-estado";
import { useAuth } from "@/lib/auth/auth-context";
import { CONTENEDOR_PAGINA, TARJETA } from "@/lib/estilos";
import { PESO_ICONO, PESO_ICONO_ACTIVO } from "@/lib/iconos";
import {
  RespuestaRevisionSimulacro,
  RevisionPreguntaSimulacro,
  RevisionSimulacro,
  simulacrosApi,
} from "@/lib/simulations/api";
import { cn } from "@/lib/utils";
import { CheckCircleIcon, CircleIcon, FunnelIcon, XCircleIcon } from "@phosphor-icons/react";

type FilterTab = "all" | "correcta" | "incorrecta" | "sin_responder";

function getEstado(pq: RevisionPreguntaSimulacro): FilterTab {
  if (pq.respuesta_usuario_ids.length === 0) return "sin_responder";
  if (pq.es_correcta === true) return "correcta";
  if (pq.es_correcta === false) return "incorrecta";
  return "all";
}

type RevisionAnswerState = "correct" | "wrong" | "neutral";

function getRevisionState(r: RespuestaRevisionSimulacro, userSelectedIds: number[]): RevisionAnswerState {
  if (r.es_correcta) return "correct";
  if (userSelectedIds.includes(r.id)) return "wrong";
  return "neutral";
}

const REV_CAJA: Record<RevisionAnswerState, string> = {
  correct: "bg-success text-success-foreground font-medium",
  wrong: "bg-destructive-suave text-destructive-texto font-medium",
  neutral: "bg-secondary text-muted-foreground",
};

const REV_LETRA: Record<RevisionAnswerState, string> = {
  correct: "bg-success-foreground text-success",
  wrong: "bg-destructive text-destructive-foreground",
  neutral: "bg-card text-muted-foreground",
};

function Estadistica({ valor, etiqueta, className }: { valor: ReactNode; etiqueta: string; className?: string }) {
  return (
    <div
      className={cn(
        "bg-card ring-border flex min-w-20 flex-col items-center rounded-xl px-3 py-1.5 text-center ring-1",
        className,
      )}
    >
      <span className="text-metrica tabular-nums">{valor}</span>
      <span className="text-etiqueta text-muted-foreground">{etiqueta}</span>
    </div>
  );
}

export default function SimulacroRevisionPage() {
  const { id } = useParams<{ id: string }>();
  const { token } = useAuth();
  const router = useRouter();
  const t = useTranslations("dashboard");

  const [revision, setRevision] = useState<RevisionSimulacro | null>(null);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState<FilterTab>("all");
  // H35: «aprobado» comes from the server (nota vs. nota_minima of the
  // configuración), not from a fixed 60 % threshold. Unknown → neutral tone.
  const [aprobado, setAprobado] = useState<boolean | null>(null);

  useEffect(() => {
    if (!token) return;
    simulacrosApi
      .getRevision(token, Number(id))
      .then(setRevision)
      .catch(() => router.push("/simulacros"))
      .finally(() => setLoading(false));
    simulacrosApi
      .getResultado(token, Number(id))
      .then((r) => setAprobado(r.aprobado))
      .catch(() => setAprobado(null));
  }, [token, id, router]);

  const stats = useMemo(() => {
    if (!revision) return null;
    const total = revision.preguntas.length;
    const correctas = revision.preguntas.filter((p: RevisionPreguntaSimulacro) => p.es_correcta === true).length;
    const incorrectas = revision.preguntas.filter((p: RevisionPreguntaSimulacro) => p.es_correcta === false).length;
    const sin_responder = revision.preguntas.filter(
      (p: RevisionPreguntaSimulacro) => p.respuesta_usuario_ids.length === 0,
    ).length;
    const porcentaje = total > 0 ? (correctas / total) * 100 : 0;
    return { total, correctas, incorrectas, sin_responder, porcentaje };
  }, [revision]);

  if (loading)
    return (
      <div className="text-muted-foreground flex justify-center p-10">
        <Spinner tamano="xl" />
      </div>
    );
  if (!revision) return null;

  const filtered =
    filter === "all"
      ? revision.preguntas
      : revision.preguntas.filter((pq: RevisionPreguntaSimulacro) => getEstado(pq) === filter);

  return (
    <div className={CONTENEDOR_PAGINA}>
      <CabeceraPagina
        volver={<EnlaceVolver href="/simulacros">{t("simulacros-ver-historial")}</EnlaceVolver>}
        titulo={t("simulacros-revision")}
        acciones={
          stats && (
            <>
              <Estadistica
                valor={stats.correctas}
                etiqueta={t("simulacros-correctas")}
                className="text-success-foreground"
              />
              <Estadistica
                valor={stats.incorrectas}
                etiqueta={t("simulacros-incorrectas")}
                className="text-destructive-texto"
              />
              <Estadistica
                valor={stats.sin_responder}
                etiqueta={t("simulacros-sin_responder")}
                className="text-foreground"
              />
              <Estadistica
                valor={`${stats.porcentaje.toFixed(0)}%`}
                etiqueta={t("simulacros-porcentaje-aciertos")}
                className={cn(
                  aprobado !== null && "ring-0",
                  aprobado === true && "bg-success text-success-foreground",
                  aprobado === false && "bg-destructive-suave text-destructive-texto",
                )}
              />
            </>
          )
        }
      />

      <Tabs<FilterTab>
        estilo="segmentado"
        valor={filter}
        onCambio={setFilter}
        items={[
          { id: "all", etiqueta: t("simulacros-todos"), contador: stats?.total },
          { id: "correcta", etiqueta: t("simulacros-correctas"), contador: stats?.correctas },
          { id: "incorrecta", etiqueta: t("simulacros-incorrectas"), contador: stats?.incorrectas },
          { id: "sin_responder", etiqueta: t("simulacros-sin_responder"), contador: stats?.sin_responder },
        ]}
      >
        {() =>
          filtered.length === 0 ? (
            <EstadoVacio icon={FunnelIcon} titulo={t("simulacros-revision-filtro-vacio")} />
          ) : (
            <ol className="flex flex-col gap-3">
              {filtered.map((pq, idx) => {
                const estado = getEstado(pq);
                return (
                  <li key={idx} className={cn(TARJETA, "flex flex-col gap-3 p-4 sm:p-5")}>
                    {/* Cabecera de la pregunta */}
                    <div className="flex items-start gap-3">
                      <span className="mt-0.5 shrink-0">
                        {estado === "correcta" ? (
                          <CheckCircleIcon
                            aria-hidden
                            weight={PESO_ICONO_ACTIVO}
                            className="text-success-solid size-6"
                          />
                        ) : estado === "incorrecta" ? (
                          <XCircleIcon aria-hidden weight={PESO_ICONO_ACTIVO} className="text-destructive size-6" />
                        ) : (
                          <CircleIcon aria-hidden weight={PESO_ICONO} className="text-texto-terciario size-6" />
                        )}
                      </span>
                      <div className="flex min-w-0 flex-1 flex-col gap-1.5">
                        <div className="flex flex-wrap items-center gap-2">
                          <span className="text-etiqueta text-muted-foreground tabular-nums">
                            {t("simulacros-pregunta")} {pq.orden}
                          </span>
                          <Chip>{pq.asignatura}</Chip>
                          {pq.tipo === "multiple" && (
                            <span className="text-etiqueta text-muted-foreground">{t("simulacros-multiple-hint")}</span>
                          )}
                        </div>
                        <p className="text-titulo text-foreground whitespace-pre-line">{pq.enunciado}</p>
                      </div>
                    </div>

                    {/* Respuestas */}
                    <ul className="flex flex-col gap-1.5 sm:pl-9">
                      {pq.respuestas.map((r) => {
                        const state = getRevisionState(r, pq.respuesta_usuario_ids);
                        const userSelected = pq.respuesta_usuario_ids.includes(r.id);
                        // H28 a11y: the state in text, not only in color and icon.
                        const estadoSr = [
                          userSelected && t("simulacros-revision-tu-respuesta"),
                          state === "correct" && t("simulacros-revision-sr-correcta"),
                          state === "wrong" && t("simulacros-revision-sr-incorrecta"),
                        ]
                          .filter(Boolean)
                          .join(", ");

                        return (
                          <li
                            key={r.id}
                            className={cn("text-control flex items-center gap-3 rounded-lg px-3 py-2", REV_CAJA[state])}
                          >
                            <span
                              className={cn(
                                "text-etiqueta grid size-7 shrink-0 place-items-center rounded-full font-semibold",
                                REV_LETRA[state],
                              )}
                            >
                              {r.letra}
                            </span>
                            <span className="flex-1">
                              {r.texto}
                              {estadoSr && <span className="sr-only">{` (${estadoSr})`}</span>}
                            </span>
                            {userSelected && state === "correct" && (
                              <CheckCircleIcon weight={PESO_ICONO_ACTIVO} className="size-4 shrink-0" />
                            )}
                            {userSelected && state === "wrong" && (
                              <XCircleIcon weight={PESO_ICONO_ACTIVO} className="size-4 shrink-0" />
                            )}
                            {!userSelected && state === "correct" && (
                              <CheckCircleIcon weight={PESO_ICONO} className="size-4 shrink-0 opacity-60" />
                            )}
                          </li>
                        );
                      })}
                    </ul>

                    {/* Explicación */}
                    {pq.explicacion && (
                      <div className="sm:pl-9">
                        <Acordeon
                          items={[
                            {
                              id: "explicacion",
                              titulo: t("simulacros-ver-explicacion"),
                              contenido: (
                                <div className="flex flex-col gap-1">
                                  <p className="text-cuerpo text-foreground whitespace-pre-line">{pq.explicacion}</p>
                                  {pq.referencia && (
                                    <p className="text-etiqueta text-texto-terciario">
                                      {t("simulacros-referencia")}: {pq.referencia}
                                    </p>
                                  )}
                                </div>
                              ),
                            },
                          ]}
                        />
                      </div>
                    )}
                  </li>
                );
              })}
            </ol>
          )
        }
      </Tabs>
    </div>
  );
}
