"use client";

import { InsigniaOficial } from "../../../componentes";
import { useFormatter, useTranslations } from "next-intl";

import { Alerta } from "@/components/feedback/alerta";
import { FilaMetricas, Metrica } from "@/components/patrones/metricas";
import { Chip } from "@/components/patrones/pill-estado";
import { TARJETA } from "@/lib/estilos";
import type { VistaPreviaBalotario } from "@/lib/serums-ia/balotario/api";
import { cn } from "@/lib/utils";
import { CheckCircleIcon, ListChecksIcon, WarningCircleIcon, WarningIcon } from "@phosphor-icons/react";

/** Lo que el servidor encontró: de qué fuente y versión, los totales, la ruta de revisión y las importaciones previas. */
export function ResumenBalotario({ vista }: { vista: VistaPreviaBalotario }) {
  const t = useTranslations("dashboard");
  const formato = useFormatter();
  const { fuente, totales, ruta_revision: ruta, importacion, otras_importaciones: otras } = vista;
  const fecha = (iso: string) =>
    formato.dateTime(new Date(iso), { dateStyle: "medium", timeStyle: "short", timeZone: "America/Lima" });

  return (
    <section className={cn(TARJETA, "flex flex-col gap-4 p-4")} aria-labelledby="balotario-resumen">
      <div className="flex flex-wrap items-center gap-2">
        <h2 id="balotario-resumen" className="text-titulo text-foreground wrap-anywhere">
          {fuente.titulo}
        </h2>
        <Chip>{t("serumsIa.balotario.version", { numero: fuente.version })}</Chip>
        {fuente.oficial_verificada ? <InsigniaOficial /> : <Chip>{t("serumsIa.balotario.noOficial")}</Chip>}
        {fuente.oficial_verificada && fuente.oficial_entidad ? (
          <span className="text-control text-muted-foreground">{fuente.oficial_entidad}</span>
        ) : null}
      </div>

      <FilaMetricas columnas={3}>
        <Metrica
          etiqueta={t("serumsIa.balotario.totales.validas")}
          icono={CheckCircleIcon}
          tono="ok"
          ayuda={t("serumsIa.balotario.totales.tipos", { unica: totales.unica, multiple: totales.multiple })}
        >
          {formato.number(totales.validas)}
        </Metrica>
        <Metrica
          etiqueta={t("serumsIa.balotario.totales.conProblemas")}
          icono={WarningCircleIcon}
          tono={totales.con_problemas > 0 ? "error" : "neutro"}
          ayuda={t("serumsIa.balotario.totales.conProblemasAyuda")}
        >
          {formato.number(totales.con_problemas)}
        </Metrica>
        <Metrica
          etiqueta={t("serumsIa.balotario.totales.conAlertas")}
          icono={WarningIcon}
          tono={totales.con_alertas > 0 ? "aviso" : "neutro"}
          ayuda={t("serumsIa.balotario.totales.conAlertasAyuda")}
        >
          {formato.number(totales.con_alertas)}
        </Metrica>
      </FilaMetricas>

      <div className="bg-secondary grid gap-1 rounded-lg p-3">
        <p className="text-control text-foreground flex items-center gap-1.5 font-medium">
          <ListChecksIcon className="size-4" />
          {t(`serumsIa.balotario.rutaRevision.${ruta}.nombre`)}
        </p>
        <p className="text-control text-muted-foreground">{t(`serumsIa.balotario.rutaRevision.${ruta}.ayuda`)}</p>
      </div>

      {importacion ? (
        <Alerta tono="info">
          {t("serumsIa.balotario.yaImportado", { fecha: fecha(importacion.created_at), n: importacion.creadas })}
        </Alerta>
      ) : null}
      {otras.length > 0 ? (
        <Alerta tono="aviso" titulo={t("serumsIa.balotario.otrasImportaciones.titulo")}>
          <p>{t("serumsIa.balotario.otrasImportaciones.texto")}</p>
          <ul className="mt-1 list-disc pl-5">
            {otras.map((o) => (
              <li key={o.version}>
                {t("serumsIa.balotario.otrasImportaciones.fila", {
                  version: o.version,
                  n: o.creadas,
                  fecha: fecha(o.created_at),
                })}
              </li>
            ))}
          </ul>
        </Alerta>
      ) : null}
    </section>
  );
}
