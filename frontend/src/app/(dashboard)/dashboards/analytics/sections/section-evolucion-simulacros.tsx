"use client";

import { CargandoGrafico, Seccion, VacioSeccion } from "./seccion";
import { useTranslations } from "next-intl";
import { useMemo } from "react";

import { GraficoLineas } from "@/components/datos/grafico";
import { type HistorialSimulacroItem } from "@/lib/simulations/api";
import { ChartLineUpIcon } from "@phosphor-icons/react";

interface Props {
  simulacros: HistorialSimulacroItem[];
  loading: boolean;
}

export default function SectionEvolucionSimulacros({ simulacros, loading }: Props) {
  const t = useTranslations("dashboard");
  const titulo = t("dashboard-evolucion-simulacros");

  const recent = useMemo(() => [...simulacros].slice(0, 12).reverse(), [simulacros]);

  if (!loading && recent.length > 0) {
    return (
      <GraficoLineas
        titulo={titulo}
        alto={220}
        className="h-full"
        categorias={recent.map((s) => `#${s.numero}`)}
        series={[{ nombre: titulo, valores: recent.map((s) => s.nota_centesimal) }]}
        formato={(v) => v.toLocaleString("es-PE", { maximumFractionDigits: 1 })}
      />
    );
  }

  return (
    <Seccion icon={ChartLineUpIcon} titulo={titulo} className="h-full">
      {loading ? (
        <CargandoGrafico alto={200} />
      ) : (
        <VacioSeccion icon={ChartLineUpIcon}>{t("dashboard-sin-datos")}</VacioSeccion>
      )}
    </Seccion>
  );
}
