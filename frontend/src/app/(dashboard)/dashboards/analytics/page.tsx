"use client";

import SectionAsignaturas from "./sections/section-asignaturas";
import SectionBarrasApiladas from "./sections/section-barras-apiladas";
import SectionCategorias from "./sections/section-categorias";
import SectionEvolucionPracticas from "./sections/section-evolucion-practicas";
import SectionEvolucionSimulacros from "./sections/section-evolucion-simulacros";
import SectionInscripciones from "./sections/section-inscripciones";
import SectionNotaChart from "./sections/section-nota-chart";
import SectionProgresoLineal from "./sections/section-progreso-lineal";
import SectionRadar from "./sections/section-radar";
import SectionRecientes from "./sections/section-recientes";
import SectionStatCards from "./sections/section-stat-cards";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { useEffect, useState } from "react";

import { Alerta } from "@/components/feedback/alerta";
import { TarjetaDestacada } from "@/components/patrones/tarjeta-destacada";
import { useAuth } from "@/lib/auth/auth-context";
import { ACCION_PRINCIPAL, ACCION_SECUNDARIA, CONTENEDOR_PAGINA } from "@/lib/estilos";
import { PESO_ICONO_FUNCIONAL } from "@/lib/iconos";
import { type Inscripcion, inscripcionesApi } from "@/lib/inscripciones/api";
import { cargarTodo } from "@/lib/paginar-todo";
import { type HistorialItem, practicasApi } from "@/lib/practices/api";
import {
  type EnCursoResponse,
  type HistorialSimulacroItem,
  type SimulacroResultado,
  simulacrosApi,
} from "@/lib/simulations/api";
import { PlusIcon, TimerIcon } from "@phosphor-icons/react";

export default function Page() {
  const { user, token } = useAuth();
  const t = useTranslations("dashboard");

  const [simHistorial, setSimHistorial] = useState<HistorialSimulacroItem[]>([]);
  const [pracHistorial, setPracHistorial] = useState<HistorialItem[]>([]);
  const [inscripciones, setInscripciones] = useState<Inscripcion[]>([]);
  const [enCurso, setEnCurso] = useState<EnCursoResponse | null>(null);
  const [lastResultado, setLastResultado] = useState<SimulacroResultado | null>(null);
  const [loading, setLoading] = useState(true);
  // El historial no se pudo cargar: las métricas muestran «—» y un aviso en vez de ceros que parecen datos reales.
  const [errorMetricas, setErrorMetricas] = useState(false);

  useEffect(() => {
    if (!token) return;
    // Las métricas se calculan sobre el historial completo (todas las páginas), no sobre la primera.
    const historiales = Promise.all([
      cargarTodo((q) => simulacrosApi.getHistorial(token, q)),
      cargarTodo((q) => practicasApi.listHistorial(token, q)),
    ])
      .then(async ([sim, prac]) => {
        setSimHistorial(sim);
        setPracHistorial(prac);
        setErrorMetricas(false);

        const lastCompleted = sim.find((s) => s.estado === "completado");
        if (lastCompleted) {
          try {
            const resultado = await simulacrosApi.getResultado(token, lastCompleted.id);
            setLastResultado(resultado);
          } catch {
            // non-critical
          }
        }
      })
      .catch(() => setErrorMetricas(true));
    // Inscripciones y simulacro en curso van aparte: si fallan, su sección queda vacía sin tumbar las métricas.
    const resto = Promise.all([
      inscripcionesApi.misInscripciones(token).then(setInscripciones),
      simulacrosApi.getEnCurso(token).then(setEnCurso),
    ]).catch(() => {});
    Promise.all([historiales, resto]).finally(() => setLoading(false));
  }, [token]);

  const nombre = user ? user.nombres.split(" ")[0] : "";

  const simCompletados = simHistorial.filter((s) => s.estado === "completado");
  const simAprobados = simCompletados.filter((s) => s.aprobado).length;
  const tasaAprobacion = simCompletados.length > 0 ? Math.round((simAprobados / simCompletados.length) * 100) : null;
  const notaPromedio =
    simCompletados.length > 0
      ? (simCompletados.reduce((sum, s) => sum + s.nota_centesimal, 0) / simCompletados.length).toFixed(1)
      : "—";
  const pracCompletadas = pracHistorial.filter((p) => p.estado === "completada");

  return (
    <div className={CONTENEDOR_PAGINA}>
      {/* ── Bienvenida + acciones ─────────────────────────────── */}
      <TarjetaDestacada
        tono="info"
        className="items-center p-5"
        titulo={
          loading ? (
            <span aria-hidden className="bg-card/60 inline-block h-5 w-56 animate-pulse rounded-md" />
          ) : (
            t("dashboard-bienvenido", { nombre })
          )
        }
        descripcion={t("dashboard-subtitulo")}
        accion={
          <div className="flex flex-wrap items-center gap-2">
            {enCurso?.en_curso && (
              <Link href={`/simulacros/${enCurso.simulacro_id}`} className={ACCION_SECUNDARIA}>
                <TimerIcon aria-hidden className="text-warning-foreground size-4" />
                {t("dashboard-continuar-simulacro")}
              </Link>
            )}
            <Link href="/simulacros/nueva" className={ACCION_PRINCIPAL}>
              <PlusIcon aria-hidden weight={PESO_ICONO_FUNCIONAL} className="size-4" />
              {t("dashboard-nuevo-simulacro")}
            </Link>
          </div>
        }
      />

      {/* ── Métricas ─────────────────────────────────────────── */}
      {errorMetricas ? <Alerta tono="error">{t("metricas-error-cargar")}</Alerta> : null}
      <SectionStatCards
        simCount={simCompletados.length}
        notaPromedio={notaPromedio}
        pracCount={pracCompletadas.length}
        tasaAprobacion={tasaAprobacion}
        loading={loading}
        error={errorMetricas}
      />

      {/* ── Notas + Inscripciones ────────────────────────────── */}
      <div className="grid grid-cols-1 items-stretch gap-3 lg:grid-cols-12">
        <div className="min-w-0 lg:col-span-7">
          <SectionNotaChart simulacros={simCompletados} loading={loading} />
        </div>
        <div className="min-w-0 lg:col-span-5">
          <SectionInscripciones inscripciones={inscripciones} enCurso={enCurso} loading={loading} />
        </div>
      </div>

      {/* ── Distribución + Radar + Progreso por asignatura ───── */}
      <div className="grid grid-cols-1 items-stretch gap-3 lg:grid-cols-3">
        <SectionCategorias resultado={lastResultado} loading={loading} />
        <SectionRadar resultado={lastResultado} loading={loading} />
        <SectionProgresoLineal resultado={lastResultado} loading={loading} />
      </div>

      {/* ── Aciertos y errores apilados ──────────────────────── */}
      <SectionBarrasApiladas resultado={lastResultado} loading={loading} />

      {/* ── Rendimiento por asignatura ───────────────────────── */}
      <SectionAsignaturas resultado={lastResultado} loading={loading} />

      {/* ── Evolución simulacros + prácticas ─────────────────── */}
      <div className="grid grid-cols-1 items-stretch gap-3 lg:grid-cols-2">
        <SectionEvolucionSimulacros simulacros={simCompletados} loading={loading} />
        <SectionEvolucionPracticas practicas={pracCompletadas} loading={loading} />
      </div>

      {/* ── Últimos simulacros ───────────────────────────────── */}
      <SectionRecientes simulacros={simCompletados} loading={loading} />
    </div>
  );
}
