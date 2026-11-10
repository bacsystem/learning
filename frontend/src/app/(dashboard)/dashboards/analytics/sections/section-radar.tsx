"use client";

import { GraficoRadar } from "./graficos-extra";
import { CargandoGrafico, Seccion, VacioSeccion } from "./seccion";
import { useTranslations } from "next-intl";
import { useId } from "react";

import { type SimulacroResultado } from "@/lib/simulations/api";
import { PolygonIcon } from "@phosphor-icons/react";

interface Props {
  resultado: SimulacroResultado | null;
  loading: boolean;
}

export default function SectionRadar({ resultado, loading }: Props) {
  const t = useTranslations("dashboard");
  const idTitulo = useId();

  const asignaturas = resultado?.por_asignatura ?? [];
  const maxVal = Math.max(...asignaturas.map((a) => a.total_preguntas), 1);

  return (
    <Seccion icon={PolygonIcon} titulo={<span id={idTitulo}>{t("dashboard-radar-titulo")}</span>} className="h-full">
      {loading ? (
        <CargandoGrafico alto={280} />
      ) : asignaturas.length < 2 ? (
        <VacioSeccion icon={PolygonIcon} className="min-h-72">
          {t("dashboard-sin-datos")}
        </VacioSeccion>
      ) : (
        <GraficoRadar
          etiquetadoPor={idTitulo}
          categorias={asignaturas.map((a) => a.asignatura)}
          max={maxVal}
          series={[
            {
              nombre: t("dashboard-radar-aciertos"),
              valores: asignaturas.map((a) => a.correctas),
              color: "var(--chart-2)",
            },
            {
              nombre: t("dashboard-radar-errores"),
              valores: asignaturas.map((a) => a.incorrectas),
              color: "var(--chart-4)",
            },
          ]}
        />
      )}
    </Seccion>
  );
}
