"use client";

import { CargandoGrafico, Seccion, VacioSeccion } from "./seccion";
import { useTranslations } from "next-intl";
import { useMemo } from "react";

import { GraficoLineas } from "@/components/datos/grafico";
import { type HistorialItem } from "@/lib/practices/api";
import { PencilSimpleLineIcon } from "@phosphor-icons/react";

interface Props {
  practicas: HistorialItem[];
  loading: boolean;
}

export default function SectionEvolucionPracticas({ practicas, loading }: Props) {
  const t = useTranslations("dashboard");
  const titulo = t("dashboard-evolucion-practicas");

  const recent = useMemo(() => [...practicas].slice(0, 12).reverse(), [practicas]);

  if (!loading && recent.length > 0) {
    return (
      <GraficoLineas
        titulo={titulo}
        alto={220}
        className="h-full"
        categorias={recent.map((p) => `#${p.numero}`)}
        series={[{ nombre: titulo, valores: recent.map((p) => p.porcentaje) }]}
        formato={(v) => `${v.toLocaleString("es-PE", { maximumFractionDigits: 1 })}%`}
      />
    );
  }

  return (
    <Seccion icon={PencilSimpleLineIcon} titulo={titulo} className="h-full">
      {loading ? (
        <CargandoGrafico alto={200} />
      ) : (
        <VacioSeccion icon={PencilSimpleLineIcon}>{t("dashboard-sin-datos")}</VacioSeccion>
      )}
    </Seccion>
  );
}
