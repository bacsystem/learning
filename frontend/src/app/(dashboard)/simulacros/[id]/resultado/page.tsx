"use client";

import { useParams, useRouter, useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
import { useEffect, useState } from "react";

import { Alerta } from "@/components/feedback/alerta";
import { ProgresoLineal } from "@/components/feedback/progreso";
import { Spinner } from "@/components/feedback/spinner";
import { CabeceraSeccion } from "@/components/patrones/cabecera-seccion";
import { Button } from "@/components/ui/button";
import { useAuth } from "@/lib/auth/auth-context";
import { BOTON_PRIMARIO, CONTENEDOR_PAGINA, TARJETA } from "@/lib/estilos";
import { PESO_ICONO, PESO_ICONO_ACTIVO } from "@/lib/iconos";
import { ResultadoAsignaturaSimulacro, SimulacroResultado, simulacrosApi } from "@/lib/simulations/api";
import { cn } from "@/lib/utils";
import {
  ArrowLeftIcon,
  ChartBarIcon,
  CheckCircleIcon,
  ListChecksIcon,
  PlusIcon,
  TrophyIcon,
  XCircleIcon,
} from "@phosphor-icons/react";

function formatTime(seg: number): string {
  const m = Math.floor(seg / 60);
  const s = seg % 60;
  return m > 0 ? `${m}m ${s}s` : `${s}s`;
}

export default function SimulacroResultadoPage() {
  const { id } = useParams<{ id: string }>();
  const { token } = useAuth();
  const router = useRouter();
  const t = useTranslations("dashboard");
  // H38: the attempt was closed before the last answer was confirmed.
  const sinGuardar = useSearchParams().get("sin_guardar") === "1";

  const [resultado, setResultado] = useState<SimulacroResultado | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!token) return;
    simulacrosApi
      .getResultado(token, Number(id))
      .then(setResultado)
      .catch(() => router.push("/simulacros"))
      .finally(() => setLoading(false));
  }, [token, id, router]);

  if (loading)
    return (
      <div className="text-muted-foreground flex justify-center p-10">
        <Spinner tamano="xl" />
      </div>
    );
  if (!resultado) return null;

  const nota = resultado.nota_centesimal;
  const passed = resultado.aprobado;
  const TEXTO_RESULTADO = passed ? "text-success-foreground" : "text-destructive-texto";

  const stats = [
    { label: t("simulacros-total"), value: resultado.total_preguntas, color: "text-foreground" },
    { label: t("simulacros-correctas"), value: resultado.correctas, color: "text-success-foreground" },
    { label: t("simulacros-incorrectas"), value: resultado.incorrectas, color: "text-destructive-texto" },
    { label: t("simulacros-sin_responder"), value: resultado.sin_responder, color: "text-muted-foreground" },
    { label: t("simulacros-porcentaje"), value: `${resultado.porcentaje.toFixed(1)}%`, color: TEXTO_RESULTADO },
    { label: t("simulacros-tiempo"), value: formatTime(resultado.tiempo_usado_seg), color: "text-foreground" },
  ];

  return (
    <div className={CONTENEDOR_PAGINA}>
      {sinGuardar && <Alerta tono="aviso">{t("simulacros-respuesta-no-guardada")}</Alerta>}

      {/* Nota */}
      <section className={cn(TARJETA, "flex flex-col gap-5 p-4 sm:p-6 md:flex-row md:items-stretch md:gap-8")}>
        <div
          className={cn(
            "flex shrink-0 flex-col items-center justify-center gap-1 rounded-xl px-8 py-5 text-center md:min-w-44",
            passed ? "bg-success text-success-foreground" : "bg-destructive-suave text-destructive-texto",
          )}
        >
          {/* H36: nota_maxima is configurable and the DTO does not carry it, so no fixed «/ 100». */}
          <span className="text-metrica tabular-nums">{nota.toFixed(1)}</span>
          <span className="bg-card text-etiqueta mt-2 inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 font-semibold">
            {passed ? (
              <CheckCircleIcon aria-hidden weight={PESO_ICONO_ACTIVO} className="size-3.5" />
            ) : (
              <XCircleIcon aria-hidden weight={PESO_ICONO_ACTIVO} className="size-3.5" />
            )}
            {passed ? t("simulacros-aprobado-si") : t("simulacros-aprobado-no")}
          </span>
        </div>

        <div className="flex min-w-0 flex-1 flex-col gap-4">
          <h1 className="text-titulo-pagina text-foreground">{resultado.proceso}</h1>

          <dl className="grid grid-cols-2 gap-x-4 gap-y-4 sm:grid-cols-3 lg:grid-cols-6">
            {stats.map(({ label, value, color }) => (
              <div key={label} className="border-border/60 flex min-w-0 flex-col gap-0.5 border-l pl-3">
                <dt className="text-etiqueta text-muted-foreground truncate">{label}</dt>
                <dd className={cn("text-metrica tabular-nums", color)}>{value}</dd>
              </div>
            ))}
          </dl>

          {/* Posición estimada */}
          {resultado.posicion && (
            <div className="bg-secondary text-control inline-flex w-fit items-center gap-2 rounded-full px-3 py-1.5">
              <TrophyIcon aria-hidden weight={PESO_ICONO} className="text-warning-solid size-4" />
              <span className="text-muted-foreground">{t("simulacros-posicion")}:</span>
              <span className="text-foreground font-semibold tabular-nums">#{resultado.posicion.posicion}</span>
              <span className="text-muted-foreground">
                {t("simulacros-de")} <span className="tabular-nums">{resultado.posicion.total_usuarios}</span>
              </span>
            </div>
          )}
        </div>
      </section>

      {/* Desglose por asignatura */}
      {resultado.por_asignatura.length > 0 && (
        <section className={TARJETA}>
          <CabeceraSeccion icon={ChartBarIcon} titulo={t("simulacros-por-asignatura")} />
          <ul className="flex flex-col gap-4 px-4 pt-2 pb-4">
            {resultado.por_asignatura.map((a: ResultadoAsignaturaSimulacro) => {
              const subPassed = a.porcentaje >= 60;
              return (
                <li key={a.asignatura} className="flex flex-col gap-2">
                  <div className="flex flex-wrap items-center justify-between gap-x-3 gap-y-1">
                    <div className="flex min-w-0 items-center gap-2">
                      {subPassed ? (
                        <CheckCircleIcon
                          aria-hidden
                          weight={PESO_ICONO}
                          className="text-success-foreground size-4 shrink-0"
                        />
                      ) : (
                        <XCircleIcon
                          aria-hidden
                          weight={PESO_ICONO}
                          className="text-destructive-texto size-4 shrink-0"
                        />
                      )}
                      <span className="text-control text-foreground truncate">{a.asignatura}</span>
                      <span className="text-etiqueta text-texto-terciario shrink-0 tabular-nums">
                        ({t("simulacros-peso")}: {a.peso.toFixed(0)}%)
                      </span>
                    </div>
                    <div className="flex items-baseline gap-2 tabular-nums">
                      <span className="text-control text-foreground font-semibold">
                        {a.correctas}/{a.total_preguntas}
                      </span>
                      <span
                        className={cn(
                          "text-etiqueta font-semibold",
                          subPassed ? "text-success-foreground" : "text-destructive-texto",
                        )}
                      >
                        {a.porcentaje.toFixed(0)}%
                      </span>
                    </div>
                  </div>
                  <ProgresoLineal
                    valor={a.porcentaje}
                    tono={subPassed ? "success" : "destructive"}
                    mostrarValor={false}
                  />
                </li>
              );
            })}
          </ul>
        </section>
      )}

      {/* Acciones */}
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
        <button type="button" className={BOTON_PRIMARIO} onClick={() => router.push(`/simulacros/${id}/revision`)}>
          <ListChecksIcon weight={PESO_ICONO} className="size-4" />
          {t("simulacros-revisar")}
        </button>
        <Button variant="suave" size="lg" onClick={() => router.push("/simulacros/nueva")}>
          <PlusIcon weight="bold" />
          {t("simulacros-nuevo-simulacro")}
        </Button>
        <Button variant="ghost" size="lg" className="sm:ml-auto" onClick={() => router.push("/simulacros")}>
          <ArrowLeftIcon weight="bold" />
          {t("simulacros-ver-historial")}
        </Button>
      </div>
    </div>
  );
}
