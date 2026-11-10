"use client";

import { EnlaceVerTodos, Seccion, VacioSeccion } from "./seccion";
import { useTranslations } from "next-intl";

import { type Columna, TablaDatos } from "@/components/datos/tabla-datos";
import { Skeleton } from "@/components/feedback/skeleton";
import { PillEstado } from "@/components/patrones/pill-estado";
import { formatearFechaHora } from "@/lib/formato";
import { type HistorialSimulacroItem } from "@/lib/simulations/api";
import { cn } from "@/lib/utils";
import { ClockCounterClockwiseIcon } from "@phosphor-icons/react";

interface Props {
  simulacros: HistorialSimulacroItem[];
  loading: boolean;
}

const fechaCorta = (valor?: string | null) => (valor ? formatearFechaHora(valor) : "—");

export default function SectionRecientes({ simulacros, loading }: Props) {
  const t = useTranslations("dashboard");

  const columnas: Columna<HistorialSimulacroItem>[] = [
    {
      id: "numero",
      titulo: "#",
      ancho: "4rem",
      ordenable: true,
      render: (s) => <span className="text-foreground font-medium tabular-nums">{s.numero}</span>,
    },
    {
      id: "proceso",
      titulo: t("simulacros-proceso"),
      ordenable: true,
      className: "max-w-0 w-full",
      render: (s) => (
        <span className="block truncate" title={s.proceso}>
          {s.proceso}
        </span>
      ),
    },
    {
      id: "fecha",
      titulo: t("simulacros-fecha"),
      ancho: "10rem",
      ordenable: true,
      valor: (s) => (s.fecha ? new Date(s.fecha).getTime() : 0),
      render: (s) => <span className="text-muted-foreground tabular-nums">{fechaCorta(s.fecha)}</span>,
    },
    {
      id: "nota_centesimal",
      titulo: t("simulacros-nota"),
      ancho: "5rem",
      alinear: "derecha",
      ordenable: true,
      render: (s) => (
        <span className={cn("font-semibold", s.aprobado ? "text-success-foreground" : "text-destructive-texto")}>
          {s.nota_centesimal?.toFixed(1)}
        </span>
      ),
    },
    {
      id: "aprobado",
      titulo: t("simulacros-aprobado"),
      ancho: "8rem",
      valor: (s) => (s.aprobado ? 1 : 0),
      ordenable: true,
      render: (s) => (
        <PillEstado tono={s.aprobado ? "ok" : "error"}>
          {s.aprobado ? t("simulacros-aprobado-si") : t("simulacros-aprobado-no")}
        </PillEstado>
      ),
    },
  ];

  return (
    <Seccion
      icon={ClockCounterClockwiseIcon}
      titulo={t("dashboard-recientes-titulo")}
      derecha={<EnlaceVerTodos href="/simulacros">{t("dashboard-ver-todos")}</EnlaceVerTodos>}
      cuerpoClassName="px-0 pb-0"
    >
      {loading ? (
        <div aria-busy="true" className="flex flex-col gap-[3px] px-4 pb-4">
          {Array.from({ length: 5 }).map((_, i) => (
            <Skeleton key={i} className="h-10 rounded-xl" />
          ))}
        </div>
      ) : simulacros.length === 0 ? (
        <VacioSeccion icon={ClockCounterClockwiseIcon} className="px-4 pb-4">
          {t("dashboard-sin-datos")}
        </VacioSeccion>
      ) : (
        <TablaDatos
          columnas={columnas}
          filas={simulacros}
          clave={(s) => String(s.id)}
          unidad={t("dashboard-unidad-simulacros")}
          className="rounded-none border-0 bg-transparent shadow-none"
        />
      )}
    </Seccion>
  );
}
