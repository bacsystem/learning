"use client";

import { Alternativa, type EstadoAlternativa } from "../../components/alternativa";
import { useParams, useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useEffect, useMemo, useState } from "react";

import { EstadoVacio } from "@/components/feedback/estado-vacio";
import { Skeleton } from "@/components/feedback/skeleton";
import { Acordeon } from "@/components/navegacion/acordeon";
import { Tabs } from "@/components/navegacion/tabs";
import { CabeceraPagina, EnlaceVolver } from "@/components/patrones/cabecera-pagina";
import { Chip } from "@/components/patrones/pill-estado";
import { useAuth } from "@/lib/auth/auth-context";
import { CONTENEDOR_PAGINA, TARJETA } from "@/lib/estilos";
import { practicasApi, RespuestaRevision, RevisionPractica, RevisionPregunta } from "@/lib/practices/api";
import { cn } from "@/lib/utils";
import { CheckCircleIcon, CircleDashedIcon, LightbulbIcon, TrayIcon, XCircleIcon } from "@phosphor-icons/react";

type FilterTab = "all" | "correcta" | "incorrecta" | "sin_responder";

function getEstado(pq: RevisionPregunta): FilterTab {
  if (pq.respuesta_usuario_ids.length === 0) return "sin_responder";
  if (pq.es_correcta === true) return "correcta";
  if (pq.es_correcta === false) return "incorrecta";
  return "all";
}

function getRevisionState(r: RespuestaRevision, userSelectedIds: number[]): EstadoAlternativa {
  if (r.es_correcta) return "correct";
  if (userSelectedIds.includes(r.id)) return "wrong";
  return "none";
}

function Estadistica({ valor, etiqueta, className }: { valor: string | number; etiqueta: string; className?: string }) {
  return (
    <div
      className={cn(
        "bg-card ring-border flex min-w-20 flex-col items-center gap-0.5 rounded-xl px-3 py-2 ring-1",
        className,
      )}
    >
      <span className="text-titulo tabular-nums">{valor}</span>
      <span className="text-etiqueta text-texto-terciario">{etiqueta}</span>
    </div>
  );
}

function CargandoRevision() {
  return (
    <div className={CONTENEDOR_PAGINA} aria-busy="true">
      <Skeleton className="h-control w-36 rounded-full" />
      <Skeleton className="h-6 w-44" />
      <Skeleton className="h-control w-96 max-w-full rounded-full" />
      {Array.from({ length: 3 }).map((_, i) => (
        <div key={i} className={cn(TARJETA, "flex flex-col gap-3 p-5")}>
          <Skeleton className="h-3 w-32" />
          <Skeleton className="h-4 w-3/4" />
          {Array.from({ length: 4 }).map((__, j) => (
            <Skeleton key={j} className="h-11 w-full rounded-xl" />
          ))}
        </div>
      ))}
    </div>
  );
}

export default function RevisionPage() {
  const { id } = useParams<{ id: string }>();
  const { token } = useAuth();
  const router = useRouter();
  const t = useTranslations("dashboard");

  const [revision, setRevision] = useState<RevisionPractica | null>(null);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState<FilterTab>("all");

  useEffect(() => {
    if (!token) return;
    practicasApi
      .getRevision(token, Number(id))
      .then(setRevision)
      .catch(() => router.push("/practicas"))
      .finally(() => setLoading(false));
  }, [token, id, router]);

  const stats = useMemo(() => {
    if (!revision) return null;
    const total = revision.preguntas.length;
    const correctas = revision.preguntas.filter((p) => p.es_correcta === true).length;
    const incorrectas = revision.preguntas.filter((p) => p.es_correcta === false).length;
    const sin_responder = revision.preguntas.filter((p) => p.respuesta_usuario_ids.length === 0).length;
    const porcentaje = total > 0 ? (correctas / total) * 100 : 0;
    return { total, correctas, incorrectas, sin_responder, porcentaje };
  }, [revision]);

  if (loading) return <CargandoRevision />;
  if (!revision) return null;

  const filtered = filter === "all" ? revision.preguntas : revision.preguntas.filter((pq) => getEstado(pq) === filter);
  const aprobado = (stats?.porcentaje ?? 0) >= 60;

  return (
    <div className={CONTENEDOR_PAGINA}>
      <CabeceraPagina
        volver={<EnlaceVolver href="/practicas">{t("practicas-ver-historial")}</EnlaceVolver>}
        titulo={t("practicas-revision")}
        acciones={
          stats && (
            <div className="flex flex-wrap gap-2">
              <Estadistica
                valor={stats.correctas}
                etiqueta={t("practicas-correctas")}
                className="text-success-foreground"
              />
              <Estadistica
                valor={stats.incorrectas}
                etiqueta={t("practicas-incorrectas")}
                className="text-destructive-texto"
              />
              <Estadistica
                valor={stats.sin_responder}
                etiqueta={t("practicas-sin_responder")}
                className="text-foreground"
              />
              <Estadistica
                valor={`${stats.porcentaje.toFixed(0)}%`}
                etiqueta={t("practicas-porcentaje")}
                className={cn(
                  "ring-0",
                  aprobado ? "bg-success text-success-foreground" : "bg-destructive-suave text-destructive-texto",
                )}
              />
            </div>
          )
        }
      />

      <Tabs<FilterTab>
        estilo="segmentado"
        valor={filter}
        onCambio={setFilter}
        items={[
          { id: "all", etiqueta: t("practicas-todas"), contador: stats?.total },
          { id: "correcta", etiqueta: t("practicas-correctas"), contador: stats?.correctas },
          { id: "incorrecta", etiqueta: t("practicas-incorrectas"), contador: stats?.incorrectas },
          { id: "sin_responder", etiqueta: t("practicas-sin_responder"), contador: stats?.sin_responder },
        ]}
      >
        {() =>
          filtered.length === 0 ? (
            <EstadoVacio icon={TrayIcon} titulo={`${t("practicas-preguntas")}: 0`} />
          ) : (
            <div className="flex flex-col gap-3">
              {filtered.map((pq, idx) => {
                const estado = getEstado(pq);
                return (
                  <article key={idx} className={cn(TARJETA, "flex flex-col gap-4 p-4 sm:p-5")}>
                    {/* Cabecera de la pregunta */}
                    <div className="flex items-start gap-3">
                      <span className="mt-0.5 shrink-0">
                        {estado === "correcta" ? (
                          <CheckCircleIcon
                            aria-label={t("practicas-estado-correcta")}
                            weight="duotone"
                            className="text-success-foreground size-5"
                          />
                        ) : estado === "incorrecta" ? (
                          <XCircleIcon
                            aria-label={t("practicas-estado-incorrecta")}
                            weight="duotone"
                            className="text-destructive-texto size-5"
                          />
                        ) : (
                          <CircleDashedIcon
                            aria-label={t("practicas-estado-sin_responder")}
                            weight="bold"
                            className="text-texto-terciario size-5"
                          />
                        )}
                      </span>
                      <div className="flex min-w-0 flex-1 flex-col gap-1.5">
                        <div className="flex flex-wrap items-center gap-2">
                          <span className="text-etiqueta text-texto-terciario tabular-nums">
                            {t("practicas-pregunta")} {pq.orden}
                          </span>
                          {pq.asignatura ? <Chip className="font-sans">{pq.asignatura}</Chip> : null}
                        </div>
                        <p className="text-titulo text-foreground whitespace-pre-line">{pq.enunciado}</p>
                      </div>
                    </div>

                    {/* Alternativas */}
                    <div className="flex flex-col gap-2 sm:pl-8">
                      {pq.respuestas.map((r) => {
                        const state = getRevisionState(r, pq.respuesta_usuario_ids);
                        const userSelected = pq.respuesta_usuario_ids.includes(r.id);
                        return (
                          <Alternativa
                            key={r.id}
                            compacta
                            letra={r.letra}
                            texto={r.texto}
                            estado={state}
                            elegida={userSelected}
                            estadoSr={[
                              userSelected && t("practicas-revision-tu-respuesta"),
                              state === "correct" && t("practicas-leyenda-correcta"),
                              state === "wrong" && t("practicas-leyenda-incorrecta"),
                            ]
                              .filter(Boolean)
                              .join(", ")}
                          />
                        );
                      })}
                    </div>

                    {/* Explicación */}
                    {pq.explicacion && (
                      <div className="sm:pl-8">
                        <Acordeon
                          items={[
                            {
                              id: `explicacion-${idx}`,
                              titulo: (
                                <span className="inline-flex items-center gap-2">
                                  <LightbulbIcon
                                    aria-hidden
                                    weight="duotone"
                                    className="text-warning-foreground size-4"
                                  />
                                  {t("practicas-ver-explicacion")}
                                </span>
                              ),
                              contenido: (
                                <div className="flex flex-col gap-1.5">
                                  <p className="text-cuerpo text-foreground leading-relaxed whitespace-pre-line">
                                    {pq.explicacion}
                                  </p>
                                  {pq.referencia && (
                                    <p className="text-etiqueta text-texto-terciario">
                                      {t("practicas-referencia")}: {pq.referencia}
                                    </p>
                                  )}
                                </div>
                              ),
                            },
                          ]}
                        />
                      </div>
                    )}
                  </article>
                );
              })}
            </div>
          )
        }
      </Tabs>
    </div>
  );
}
