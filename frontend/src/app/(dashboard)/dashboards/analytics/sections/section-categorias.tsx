"use client";

import { CargandoGrafico, Seccion, VacioSeccion } from "./seccion";
import { useTranslations } from "next-intl";
import { useMemo } from "react";

import { GraficoAnillos, type SerieAnillo } from "@/components/datos/grafico";
import { type SimulacroResultado } from "@/lib/simulations/api";
import { ChartDonutIcon } from "@phosphor-icons/react";

interface Props {
  resultado: SimulacroResultado | null;
  loading: boolean;
}

/** GraficoAnillos admite 5 anillos: con más asignaturas, las 4 mayores y el resto agrupado en «Otras». */
const MAX_ANILLOS = 5;

export default function SectionCategorias({ resultado, loading }: Props) {
  const t = useTranslations("dashboard");

  const asignaturas = useMemo(() => resultado?.por_asignatura ?? [], [resultado]);
  const total = asignaturas.reduce((sum, a) => sum + a.total_preguntas, 0);

  const series = useMemo<SerieAnillo[]>(() => {
    if (total === 0) return [];
    const ordenadas = [...asignaturas].sort((a, b) => b.total_preguntas - a.total_preguntas);
    const visibles = ordenadas.length > MAX_ANILLOS ? ordenadas.slice(0, MAX_ANILLOS - 1) : ordenadas;
    const resto = ordenadas.slice(visibles.length);
    const filas = visibles.map((a) => ({ nombre: a.asignatura, cantidad: a.total_preguntas }));
    if (resto.length > 0) {
      filas.push({
        nombre: t("dashboard-categorias-otras", { n: resto.length }),
        cantidad: resto.reduce((s, a) => s + a.total_preguntas, 0),
      });
    }
    return filas.map((f) => ({
      nombre: f.nombre,
      valor: Math.round((f.cantidad / total) * 100),
      detalle: t("dashboard-categorias-preguntas", { n: f.cantidad }),
    }));
  }, [asignaturas, total, t]);

  return (
    <Seccion icon={ChartDonutIcon} titulo={t("dashboard-categorias-titulo")} className="h-full">
      {loading ? (
        <CargandoGrafico alto={280} />
      ) : series.length === 0 ? (
        <VacioSeccion icon={ChartDonutIcon} className="min-h-72">
          {t("dashboard-sin-datos")}
        </VacioSeccion>
      ) : (
        <GraficoAnillos
          series={series}
          tamano={148}
          centro={
            <span className="flex flex-col items-center leading-tight">
              <span className="text-metrica tabular-nums">{total}</span>
              <span className="text-etiqueta text-texto-terciario font-medium">{t("dashboard-categorias-centro")}</span>
            </span>
          }
          className="rounded-none border-0 bg-transparent px-0 py-1 shadow-none dark:bg-none"
        />
      )}
    </Seccion>
  );
}
