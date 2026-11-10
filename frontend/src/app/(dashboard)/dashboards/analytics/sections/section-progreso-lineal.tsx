"use client";

import { EnlaceVerTodos, Seccion, TONO_PROGRESO, tonoRendimiento, VacioSeccion } from "./seccion";
import { useTranslations } from "next-intl";

import { ProgresoLineal } from "@/components/feedback/progreso";
import { Skeleton } from "@/components/feedback/skeleton";
import { Chip } from "@/components/patrones/pill-estado";
import { type SimulacroResultado } from "@/lib/simulations/api";
import { ListChecksIcon } from "@phosphor-icons/react";

interface Props {
  resultado: SimulacroResultado | null;
  loading: boolean;
}

export default function SectionProgresoLineal({ resultado, loading }: Props) {
  const t = useTranslations("dashboard");
  const asignaturas = resultado?.por_asignatura ?? [];

  return (
    <Seccion
      icon={ListChecksIcon}
      titulo={t("dashboard-estado-asignaturas")}
      derecha={<EnlaceVerTodos href="/simulacros">{t("dashboard-ver-todos")}</EnlaceVerTodos>}
      className="h-full"
    >
      {loading ? (
        <ul aria-busy="true" className="flex flex-col gap-4">
          {Array.from({ length: 5 }).map((_, i) => (
            <li key={i} className="flex flex-col gap-1.5">
              <div className="flex justify-between">
                <Skeleton className="h-3 w-2/5" />
                <Skeleton className="h-3 w-10" />
              </div>
              <Skeleton className="h-1.5 w-full rounded-full" />
            </li>
          ))}
        </ul>
      ) : asignaturas.length === 0 ? (
        <VacioSeccion icon={ListChecksIcon}>{t("dashboard-sin-datos")}</VacioSeccion>
      ) : (
        <ul className="flex flex-col gap-4">
          {asignaturas.map((a) => {
            const pct = Math.round(a.porcentaje);
            return (
              <li key={a.asignatura} className="flex flex-col gap-1.5">
                <div className="flex items-center justify-between gap-2">
                  <span className="text-control text-foreground truncate font-medium" title={a.asignatura}>
                    {a.asignatura}
                  </span>
                  <Chip className="shrink-0">
                    {a.correctas}/{a.total_preguntas}
                  </Chip>
                </div>
                <ProgresoLineal valor={pct} tono={TONO_PROGRESO[tonoRendimiento(a.porcentaje)]} mostrarValor={false} />
              </li>
            );
          })}
        </ul>
      )}
    </Seccion>
  );
}
