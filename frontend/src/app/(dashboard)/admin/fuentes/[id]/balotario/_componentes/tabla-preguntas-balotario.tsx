"use client";

import { useTranslations } from "next-intl";

import { CONTENEDOR_TABLA, FilaVacia } from "@/components/patrones/pie-tabla";
import { Chip } from "@/components/patrones/pill-estado";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { CABECERA_TABLA } from "@/lib/estilos";
import type { PreguntaBalotario } from "@/lib/serums-ia/balotario/api";
import { cn } from "@/lib/utils";
import { CheckIcon, ListChecksIcon } from "@phosphor-icons/react";

/** Las primeras preguntas válidas con sus alternativas y la clave marcada, para comprobar que se leyeron bien. */
export function TablaPreguntasBalotario({ preguntas }: { preguntas: PreguntaBalotario[] }) {
  const t = useTranslations("dashboard");
  return (
    <section className="flex flex-col gap-2" aria-labelledby="balotario-preguntas">
      <h2 id="balotario-preguntas" className="text-titulo text-foreground">
        {t("serumsIa.balotario.preguntas.titulo", { n: preguntas.length })}
      </h2>
      <div className={CONTENEDOR_TABLA}>
        <Table>
          <TableHeader>
            <TableRow className="hover:bg-transparent">
              <TableHead className={cn(CABECERA_TABLA, "w-16 pl-4")}>
                {t("serumsIa.balotario.preguntas.numero")}
              </TableHead>
              <TableHead className={CABECERA_TABLA}>{t("serumsIa.balotario.preguntas.enunciado")}</TableHead>
              <TableHead className={CABECERA_TABLA}>{t("serumsIa.balotario.preguntas.tipo")}</TableHead>
              <TableHead className={cn(CABECERA_TABLA, "pr-4")}>{t("serumsIa.balotario.preguntas.alertas")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody className="text-control">
            {preguntas.map((p) => (
              <TableRow key={p.linea_inicio} className="align-top">
                <TableCell className="px-3 py-2 pl-4 font-mono tabular-nums">
                  {p.numero}
                  <span className="text-etiqueta text-muted-foreground block font-sans">
                    {t("serumsIa.balotario.preguntas.lineas", { inicio: p.linea_inicio, fin: p.linea_fin })}
                  </span>
                </TableCell>
                <TableCell className="px-3 py-2">
                  <p className="text-foreground wrap-anywhere">{p.enunciado}</p>
                  <ol
                    className="mt-1.5 grid gap-1"
                    aria-label={t("serumsIa.balotario.preguntas.alternativas", { n: p.alternativas.length })}
                  >
                    {p.alternativas.map((a) => (
                      <li
                        key={a.letra}
                        className={cn(
                          "flex items-start gap-2 wrap-anywhere",
                          a.correcta ? "text-foreground font-medium" : "text-muted-foreground",
                        )}
                      >
                        <span className="font-mono">{a.letra})</span>
                        <span>{a.texto}</span>
                        {a.correcta ? (
                          <span className="text-success-foreground inline-flex shrink-0 items-center gap-1">
                            <CheckIcon weight="bold" className="size-3.5" aria-hidden />
                            <span className="sr-only">{t("serumsIa.balotario.preguntas.correcta")}</span>
                          </span>
                        ) : null}
                      </li>
                    ))}
                  </ol>
                </TableCell>
                <TableCell className="px-3 py-2">
                  <Chip>{t(`serumsIa.balotario.preguntas.tipos.${p.tipo}`)}</Chip>
                </TableCell>
                <TableCell className="px-3 py-2 pr-4">
                  {p.alertas_estructura.length === 0 ? (
                    <span className="text-muted-foreground">{t("serumsIa.balotario.preguntas.sinAlertas")}</span>
                  ) : (
                    <ul className="grid gap-1">
                      {p.alertas_estructura.map((alerta) => (
                        <li key={alerta} className="text-muted-foreground">
                          {t(`serumsIa.balotario.alertas.${alerta}`)}
                        </li>
                      ))}
                    </ul>
                  )}
                </TableCell>
              </TableRow>
            ))}
            {preguntas.length === 0 ? (
              <FilaVacia columnas={4} icon={<ListChecksIcon className="size-6" />}>
                {t("serumsIa.balotario.preguntas.vacio")}
              </FilaVacia>
            ) : null}
          </TableBody>
        </Table>
      </div>
    </section>
  );
}
