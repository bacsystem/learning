"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import type { ReactNode } from "react";

import { Skeleton } from "@/components/feedback/skeleton";
import { FilaMetricas, Metrica } from "@/components/patrones/metricas";
import { NumeroAnimado } from "@/components/patrones/numero-animado";
import { FOCO } from "@/lib/estilos";
import { cn } from "@/lib/utils";
import { ChartLineIcon, CheckSquareIcon, PencilSimpleLineIcon, TimerIcon } from "@phosphor-icons/react";

interface Props {
  simCount: number;
  notaPromedio: string;
  pracCount: number;
  tasaAprobacion: number | null;
  loading: boolean;
  /** El historial no se pudo cargar: cada valor muestra «—» en vez de un cero que parecería real. */
  error?: boolean;
}

function Enlace({ href, children }: { href: string; children: ReactNode }) {
  return (
    <Link href={href} className={cn("block min-w-0 rounded-xl", FOCO)}>
      {children}
    </Link>
  );
}

function Valor({ loading, error, children }: { loading: boolean; error: boolean; children: ReactNode }) {
  if (loading) return <Skeleton className="my-1 h-5 w-16" />;
  return error ? "—" : <>{children}</>;
}

export default function SectionStatCards({
  simCount,
  notaPromedio,
  pracCount,
  tasaAprobacion,
  loading,
  error = false,
}: Props) {
  const t = useTranslations("dashboard");
  const listos = !loading && !error;
  const nota = notaPromedio !== "—" ? Number(notaPromedio) : null;

  return (
    <FilaMetricas columnas={4}>
      <Enlace href="/simulacros">
        <Metrica
          etiqueta={t("dashboard-stat-simulacros")}
          icono={TimerIcon}
          tono="acento"
          ayuda={t("dashboard-stat-simulacros-ayuda")}
        >
          <Valor loading={loading} error={error}>
            <NumeroAnimado valor={simCount} />
          </Valor>
        </Metrica>
      </Enlace>
      <Enlace href="/simulacros">
        <Metrica
          etiqueta={t("dashboard-stat-nota")}
          icono={ChartLineIcon}
          tono="neutro"
          sufijo={listos && nota !== null ? "/ 100" : undefined}
          visual={listos && nota !== null ? { tipo: "barra", valor: nota } : undefined}
          ayuda={t("dashboard-stat-nota-ayuda")}
        >
          <Valor loading={loading} error={error}>
            {nota !== null ? <NumeroAnimado valor={nota} decimales={1} /> : "—"}
          </Valor>
        </Metrica>
      </Enlace>
      <Enlace href="/practicas">
        <Metrica
          etiqueta={t("dashboard-stat-practicas")}
          icono={PencilSimpleLineIcon}
          tono="acento"
          ayuda={t("dashboard-stat-practicas-ayuda")}
        >
          <Valor loading={loading} error={error}>
            <NumeroAnimado valor={pracCount} />
          </Valor>
        </Metrica>
      </Enlace>
      <Enlace href="/simulacros">
        <Metrica
          etiqueta={t("dashboard-stat-aprobacion")}
          icono={CheckSquareIcon}
          tono={
            tasaAprobacion === null ? "neutro" : tasaAprobacion >= 60 ? "ok" : tasaAprobacion >= 40 ? "aviso" : "error"
          }
          visual={listos && tasaAprobacion !== null ? { tipo: "barra", valor: tasaAprobacion } : undefined}
          ayuda={t("dashboard-stat-aprobacion-ayuda")}
        >
          <Valor loading={loading} error={error}>
            {tasaAprobacion !== null ? <NumeroAnimado valor={tasaAprobacion} sufijo="%" /> : "—"}
          </Valor>
        </Metrica>
      </Enlace>
    </FilaMetricas>
  );
}
