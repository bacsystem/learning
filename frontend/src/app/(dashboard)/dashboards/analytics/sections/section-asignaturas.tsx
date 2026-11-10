"use client";

import { Seccion, TONO_PROGRESO, tonoRendimiento, VacioSeccion } from "./seccion";
import { useTranslations } from "next-intl";

import { ProgresoCircular } from "@/components/feedback/progreso";
import { Skeleton } from "@/components/feedback/skeleton";
import { type SimulacroResultado } from "@/lib/simulations/api";
import { BooksIcon } from "@phosphor-icons/react";

interface Props {
  resultado: SimulacroResultado | null;
  loading: boolean;
}

export default function SectionAsignaturas({ resultado, loading }: Props) {
  const t = useTranslations("dashboard");
  const asignaturas = resultado?.por_asignatura ?? [];

  return (
    <Seccion
      icon={BooksIcon}
      titulo={t("dashboard-asignaturas-titulo")}
      subtitulo={!loading && asignaturas.length > 0 ? t("dashboard-asignaturas-ultimo") : undefined}
    >
      {loading ? (
        <div aria-busy="true" className="grid grid-cols-1 gap-2 sm:grid-cols-2 xl:grid-cols-3">
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} className="h-[72px] rounded-lg" />
          ))}
        </div>
      ) : asignaturas.length === 0 ? (
        <VacioSeccion icon={BooksIcon}>{t("dashboard-sin-datos")}</VacioSeccion>
      ) : (
        <ul className="grid grid-cols-1 gap-2 sm:grid-cols-2 xl:grid-cols-3">
          {asignaturas.map((a) => (
            <li
              key={a.asignatura}
              className="ring-border flex items-center justify-between gap-3 rounded-lg px-3.5 py-3 ring-1"
            >
              <div className="min-w-0 flex-1">
                <p className="text-control text-muted-foreground truncate" title={a.asignatura}>
                  {a.asignatura}
                </p>
                <p className="text-titulo text-foreground tabular-nums">
                  {a.correctas}{" "}
                  <span className="text-control text-texto-terciario font-normal">/ {a.total_preguntas}</span>
                </p>
              </div>
              <ProgresoCircular valor={Math.round(a.porcentaje)} tono={TONO_PROGRESO[tonoRendimiento(a.porcentaje)]} />
            </li>
          ))}
        </ul>
      )}
    </Seccion>
  );
}
