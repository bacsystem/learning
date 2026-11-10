"use client";

import { Seccion, VacioSeccion } from "./seccion";
import Link from "next/link";
import { useTranslations } from "next-intl";

import { Alerta } from "@/components/feedback/alerta";
import { Skeleton } from "@/components/feedback/skeleton";
import type { Tono } from "@/components/patrones/metricas";
import { Chip, PillEstado } from "@/components/patrones/pill-estado";
import { ACCION_SECUNDARIA } from "@/lib/estilos";
import { type Inscripcion } from "@/lib/inscripciones/api";
import { type EnCursoResponse } from "@/lib/simulations/api";
import { GraduationCapIcon } from "@phosphor-icons/react";

interface Props {
  inscripciones: Inscripcion[];
  enCurso: EnCursoResponse | null;
  loading: boolean;
}

const ESTADO_TONO: Record<Inscripcion["estado"], Tono> = {
  aprobado: "ok",
  pendiente: "aviso",
  rechazado: "error",
};

/** Claves i18n existentes de /procesos para el tipo de proceso y el estado de la inscripción. */
const TIPO_CLAVE: Record<string, string> = {
  ascenso: "procesos-tipo-ascenso",
  pase_efectividad: "procesos-tipo-pase-efectividad",
  admision: "admin-procesos-tipo-admision",
};

const ESTADO_CLAVE: Record<Inscripcion["estado"], string> = {
  aprobado: "procesos-estado-aprobado",
  pendiente: "procesos-estado-pendiente",
  rechazado: "procesos-estado-rechazado",
};

export default function SectionInscripciones({ inscripciones, enCurso, loading }: Props) {
  const t = useTranslations("dashboard");

  const estadoPill = (estado: string) => {
    const tono = ESTADO_TONO[estado as Inscripcion["estado"]];
    if (!tono) return null;
    return <PillEstado tono={tono}>{t(ESTADO_CLAVE[estado as Inscripcion["estado"]])}</PillEstado>;
  };

  return (
    <Seccion
      icon={GraduationCapIcon}
      titulo={t("dashboard-inscripciones-titulo")}
      derecha={!loading && inscripciones.length > 0 ? <Chip>{inscripciones.length}</Chip> : undefined}
      className="h-full"
      cuerpoClassName="flex flex-col gap-3"
    >
      {enCurso?.en_curso && (
        <Alerta
          tono="aviso"
          accion={
            <Link href={`/simulacros/${enCurso.simulacro_id}`} className={ACCION_SECUNDARIA}>
              {t("dashboard-continuar")}
            </Link>
          }
        >
          {t("dashboard-simulacro-en-curso")}
        </Alerta>
      )}

      {loading ? (
        <ul aria-busy="true" className="flex flex-col gap-1">
          {Array.from({ length: 3 }).map((_, i) => (
            <li key={i} className="bg-secondary flex flex-col gap-1.5 rounded-lg px-3 py-2.5">
              <Skeleton className="bg-card h-3.5 w-3/5" />
              <Skeleton className="bg-card h-2.5 w-2/5" />
            </li>
          ))}
        </ul>
      ) : inscripciones.length === 0 ? (
        <VacioSeccion
          icon={GraduationCapIcon}
          accion={
            <Link href="/procesos" className={ACCION_SECUNDARIA}>
              {t("dashboard-inscripciones-ir")}
            </Link>
          }
        >
          {t("dashboard-inscripciones-vacio")}
        </VacioSeccion>
      ) : (
        <ul className="flex flex-col gap-1">
          {inscripciones.map((ins) => (
            <li key={ins.id} className="bg-secondary flex items-center justify-between gap-3 rounded-lg px-3 py-2.5">
              <div className="min-w-0">
                <p className="text-control text-foreground truncate font-semibold" title={ins.proceso_nombre}>
                  {ins.proceso_nombre}
                </p>
                <p className="text-etiqueta text-texto-terciario tabular-nums">
                  {TIPO_CLAVE[ins.proceso_tipo] ? t(TIPO_CLAVE[ins.proceso_tipo]) : ins.proceso_tipo} ·{" "}
                  {ins.proceso_anio}
                </p>
                {ins.observacion && <p className="text-etiqueta text-destructive-texto mt-0.5">{ins.observacion}</p>}
              </div>
              {estadoPill(ins.estado)}
            </li>
          ))}
        </ul>
      )}
    </Seccion>
  );
}
