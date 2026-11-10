"use client";

import { CargandoGrafico, Seccion, VacioSeccion } from "./seccion";
import { useTranslations } from "next-intl";
import { useId, useMemo } from "react";

import { GraficoBarras } from "@/components/datos/grafico";
import { Chip } from "@/components/patrones/pill-estado";
import { type HistorialSimulacroItem } from "@/lib/simulations/api";
import { ChartBarIcon } from "@phosphor-icons/react";

interface Props {
  simulacros: HistorialSimulacroItem[];
  loading: boolean;
}

export default function SectionNotaChart({ simulacros, loading }: Props) {
  const t = useTranslations("dashboard");
  const idTitulo = useId();

  const recent = useMemo(() => [...simulacros].slice(0, 10).reverse(), [simulacros]);
  const aprobados = recent.filter((s) => s.aprobado).length;

  return (
    <Seccion
      icon={ChartBarIcon}
      className="h-full"
      titulo={<span id={idTitulo}>{t("dashboard-evolucion-titulo")}</span>}
      subtitulo={!loading && recent.length > 0 ? t("dashboard-evolucion-subtitulo", { n: recent.length }) : undefined}
      derecha={
        !loading && recent.length > 0 ? (
          <Chip>{t("dashboard-evolucion-aprobados", { aprobados, total: recent.length })}</Chip>
        ) : undefined
      }
    >
      {loading ? (
        <CargandoGrafico alto={240} />
      ) : recent.length === 0 ? (
        <VacioSeccion icon={ChartBarIcon}>{t("dashboard-sin-datos")}</VacioSeccion>
      ) : (
        <GraficoBarras
          plano
          etiquetadoPor={idTitulo}
          alto={260}
          categorias={recent.map((s) => `#${s.numero}`)}
          series={[{ nombre: t("simulacros-nota"), valores: recent.map((s) => s.nota_centesimal) }]}
          destacado={recent.length - 1}
          formato={(v) => v.toLocaleString("es-PE", { maximumFractionDigits: 1 })}
          tituloDeCategoria={(i) => {
            const s = recent[i];
            return `#${s.numero}: ${s.nota_centesimal.toFixed(1)} · ${s.aprobado ? t("simulacros-aprobado-si") : t("simulacros-aprobado-no")}`;
          }}
          descripcion={t("dashboard-evolucion-descripcion", { aprobados, total: recent.length })}
        />
      )}
    </Seccion>
  );
}
