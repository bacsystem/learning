"use client";

import { GraficoBarrasApiladas } from "./graficos-extra";
import { CargandoGrafico, Seccion, VacioSeccion } from "./seccion";
import { useTranslations } from "next-intl";
import { useId } from "react";

import { type SimulacroResultado } from "@/lib/simulations/api";
import { ChartBarHorizontalIcon } from "@phosphor-icons/react";

interface Props {
  resultado: SimulacroResultado | null;
  loading: boolean;
}

export default function SectionBarrasApiladas({ resultado, loading }: Props) {
  const t = useTranslations("dashboard");
  const idTitulo = useId();
  const asignaturas = resultado?.por_asignatura ?? [];
  const maxVal = Math.max(...asignaturas.map((a) => a.total_preguntas), 1);

  return (
    <Seccion icon={ChartBarHorizontalIcon} titulo={<span id={idTitulo}>{t("dashboard-barras-titulo")}</span>}>
      {loading ? (
        <CargandoGrafico alto={260} />
      ) : asignaturas.length === 0 ? (
        <VacioSeccion icon={ChartBarHorizontalIcon}>{t("dashboard-sin-datos")}</VacioSeccion>
      ) : (
        <GraficoBarrasApiladas
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
