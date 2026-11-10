"use client";

import { useParams, useRouter, useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
import { useEffect, useState } from "react";

import { Alerta } from "@/components/feedback/alerta";
import { ProgresoLineal } from "@/components/feedback/progreso";
import { Skeleton } from "@/components/feedback/skeleton";
import { CabeceraSeccion } from "@/components/patrones/cabecera-seccion";
import { PillEstado } from "@/components/patrones/pill-estado";
import { useAuth } from "@/lib/auth/auth-context";
import { ACCION_PRINCIPAL, ACCION_SECUNDARIA, CONTENEDOR_PAGINA, FOCO, TARJETA } from "@/lib/estilos";
import { PracticaResultado, practicasApi } from "@/lib/practices/api";
import { cn } from "@/lib/utils";
import {
  ArrowLeftIcon,
  BookOpenIcon,
  CheckCircleIcon,
  ListChecksIcon,
  PlusIcon,
  XCircleIcon,
} from "@phosphor-icons/react";

function formatTime(seg: number): string {
  const m = Math.floor(seg / 60);
  const s = seg % 60;
  return m > 0 ? `${m}m ${s}s` : `${s}s`;
}

/** Anillo grande del puntaje (ProgresoCircular del kit llega a 64 px; aquí el puntaje es el protagonista). */
function AnilloPuntaje({ valor, aprobado, etiqueta }: { valor: number; aprobado: boolean; etiqueta: string }) {
  const caja = 152;
  const grosor = 10;
  const radio = (caja - grosor) / 2;
  const circunferencia = 2 * Math.PI * radio;
  const pct = Math.min(100, Math.max(0, valor)) / 100;
  return (
    <div
      role="img"
      aria-label={`${etiqueta}: ${valor.toFixed(0)}%`}
      className="relative inline-flex shrink-0 items-center justify-center"
      style={{ width: caja, height: caja }}
    >
      <svg width={caja} height={caja} viewBox={`0 0 ${caja} ${caja}`} className="-rotate-90" aria-hidden>
        <circle
          cx={caja / 2}
          cy={caja / 2}
          r={radio}
          strokeWidth={grosor}
          fill="none"
          style={{ stroke: "var(--secondary)" }}
        />
        <circle
          cx={caja / 2}
          cy={caja / 2}
          r={radio}
          strokeWidth={grosor}
          strokeLinecap="round"
          fill="none"
          strokeDasharray={circunferencia}
          strokeDashoffset={circunferencia * (1 - pct)}
          className="ease-salida transition-[stroke-dashoffset] duration-(--duracion-lenta) motion-reduce:transition-none"
          style={{ stroke: aprobado ? "var(--success-solid)" : "var(--destructive)" }}
        />
      </svg>
      <span className="absolute flex flex-col items-center gap-1">
        <span
          className={cn("text-metrica tabular-nums", aprobado ? "text-success-foreground" : "text-destructive-texto")}
        >
          {valor.toFixed(0)}%
        </span>
        <span className="text-etiqueta text-texto-terciario">{etiqueta}</span>
      </span>
    </div>
  );
}

function CargandoResultado() {
  return (
    <div className={CONTENEDOR_PAGINA} aria-busy="true">
      <div className={cn(TARJETA, "flex flex-col items-center gap-6 p-6 md:flex-row md:items-start")}>
        <Skeleton className="size-[152px] rounded-full" />
        <div className="flex w-full flex-1 flex-col gap-4">
          <Skeleton className="h-5 w-60" />
          <div className="flex flex-wrap gap-6">
            {Array.from({ length: 5 }).map((_, i) => (
              <div key={i} className="flex flex-col gap-1.5">
                <Skeleton className="h-6 w-12" />
                <Skeleton className="h-2.5 w-16" />
              </div>
            ))}
          </div>
        </div>
      </div>
      <div className={cn(TARJETA, "flex flex-col gap-4 p-4")}>
        {Array.from({ length: 3 }).map((_, i) => (
          <div key={i} className="flex flex-col gap-2">
            <Skeleton className="h-3 w-40" />
            <Skeleton className="h-1.5 w-full" />
          </div>
        ))}
      </div>
    </div>
  );
}

export default function ResultadoPage() {
  const { id } = useParams<{ id: string }>();
  const { token } = useAuth();
  const router = useRouter();
  const t = useTranslations("dashboard");
  // H38: the attempt was closed before the last answer was confirmed.
  const sinGuardar = useSearchParams().get("sin_guardar") === "1";

  const [resultado, setResultado] = useState<PracticaResultado | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!token) return;
    practicasApi
      .getResultado(token, Number(id))
      .then(setResultado)
      .catch(() => router.push("/practicas"))
      .finally(() => setLoading(false));
  }, [token, id, router]);

  if (loading) return <CargandoResultado />;
  if (!resultado) return null;

  const pct = resultado.porcentaje;
  const passed = pct >= 60;

  const stats: { label: string; value: string | number; className: string }[] = [
    { label: t("practicas-total"), value: resultado.total_preguntas, className: "text-foreground" },
    { label: t("practicas-correctas"), value: resultado.correctas, className: "text-success-foreground" },
    { label: t("practicas-incorrectas"), value: resultado.incorrectas, className: "text-destructive-texto" },
    { label: t("practicas-sin_responder"), value: resultado.sin_responder, className: "text-muted-foreground" },
    { label: t("practicas-tiempo"), value: formatTime(resultado.tiempo_usado_seg), className: "text-foreground" },
  ];

  return (
    <div className={CONTENEDOR_PAGINA}>
      {sinGuardar && <Alerta tono="aviso">{t("practicas-respuesta-no-guardada")}</Alerta>}

      {/* Puntaje */}
      <section
        className={cn(TARJETA, "flex flex-col items-center gap-6 p-5 sm:p-6 md:flex-row md:items-center md:gap-10")}
      >
        <AnilloPuntaje valor={pct} aprobado={passed} etiqueta={t("practicas-porcentaje")} />

        <div className="flex w-full min-w-0 flex-1 flex-col gap-5">
          <div className="flex flex-wrap items-center gap-2.5 max-md:justify-center">
            <h2 className="text-titulo-pagina text-foreground max-md:text-center">{resultado.proceso}</h2>
            <PillEstado tono={passed ? "ok" : "error"}>
              {passed ? t("practicas-aprobado") : t("practicas-reprobado")}
            </PillEstado>
          </div>

          <dl className="grid grid-cols-2 gap-3 sm:grid-cols-5">
            {stats.map(({ label, value, className }) => (
              <div key={label} className="bg-secondary flex flex-col gap-1 rounded-xl px-3 py-2.5">
                <dt className="text-etiqueta text-texto-terciario">{label}</dt>
                <dd className={cn("text-metrica tabular-nums", className)}>{value}</dd>
              </div>
            ))}
          </dl>
        </div>
      </section>

      {/* Desglose por asignatura */}
      {resultado.por_asignatura.length > 0 && (
        <section className={TARJETA}>
          <CabeceraSeccion icon={BookOpenIcon} titulo={t("practicas-asignaturas")} />
          <ul className="flex flex-col gap-4 px-4 pt-2 pb-4">
            {resultado.por_asignatura.map((a) => {
              const subPassed = a.porcentaje >= 60;
              return (
                <li key={a.asignatura} className="flex flex-col gap-2">
                  <div className="flex items-center justify-between gap-3">
                    <span className="flex min-w-0 items-center gap-2">
                      {subPassed ? (
                        <CheckCircleIcon
                          aria-hidden
                          weight="duotone"
                          className="text-success-foreground size-4 shrink-0"
                        />
                      ) : (
                        <XCircleIcon aria-hidden weight="duotone" className="text-destructive-texto size-4 shrink-0" />
                      )}
                      <span className="text-control text-foreground truncate">{a.asignatura}</span>
                    </span>
                    <span className="flex shrink-0 items-baseline gap-2 tabular-nums">
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
                    </span>
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
        <button
          type="button"
          className={cn(ACCION_PRINCIPAL, "justify-center")}
          onClick={() => router.push(`/practicas/${id}/revision`)}
        >
          <ListChecksIcon weight="duotone" className="size-4" />
          {t("practicas-revisar")}
        </button>
        <button
          type="button"
          className={cn(ACCION_SECUNDARIA, "justify-center")}
          onClick={() => router.push("/practicas/nueva")}
        >
          <PlusIcon weight="bold" className="size-3.5" />
          {t("practicas-nueva-practica")}
        </button>
        <button
          type="button"
          onClick={() => router.push("/practicas")}
          className={cn(
            "h-control text-control text-muted-foreground hover:bg-secondary hover:text-foreground inline-flex items-center justify-center gap-1.5 rounded-full px-3.5 font-medium transition-colors sm:ml-auto",
            FOCO,
          )}
        >
          <ArrowLeftIcon weight="bold" className="size-3.5" />
          {t("practicas-ver-historial")}
        </button>
      </div>
    </div>
  );
}
