"use client";

import { useTranslations } from "next-intl";

import { PieTabla } from "@/components/app/pie-tabla";
import { CONTENEDOR_TABLA, FilaVacia } from "@/components/patrones/pie-tabla";
import { Chip } from "@/components/patrones/pill-estado";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { CABECERA_TABLA } from "@/lib/estilos";
import type { VistaPreviaBalotario } from "@/lib/serums-ia/balotario/api";
import { ultimaPaginaProblemas } from "@/lib/serums-ia/balotario/reglas";
import { cn } from "@/lib/utils";
import { CheckCircleIcon } from "@phosphor-icons/react";

/** Preguntas que no se importan, con línea y motivos; sin su texto, el admin las ubica por la línea. Paginada en el servidor. */
export function TablaProblemasBalotario({
  problemas,
  cargando,
  onPagina,
  onPorPagina,
}: {
  problemas: VistaPreviaBalotario["problemas"];
  cargando: boolean;
  onPagina: (pagina: number) => void;
  onPorPagina: (porPagina: number) => void;
}) {
  const t = useTranslations("dashboard");
  const { data, total, page, per_page: porPagina } = problemas;
  const desde = data.length === 0 ? 0 : (page - 1) * porPagina + 1;

  return (
    <section className="flex flex-col gap-2" aria-labelledby="balotario-problemas">
      <div>
        <h2 id="balotario-problemas" className="text-titulo text-foreground">
          {t("serumsIa.balotario.problemas.titulo")}
        </h2>
        <p className="text-control text-muted-foreground">{t("serumsIa.balotario.problemas.ayuda")}</p>
      </div>
      <div className={cn(CONTENEDOR_TABLA, cargando && "opacity-60")} aria-busy={cargando}>
        <Table>
          <TableHeader>
            <TableRow className="hover:bg-transparent">
              <TableHead className={cn(CABECERA_TABLA, "w-28 pl-4")}>
                {t("serumsIa.balotario.problemas.numero")}
              </TableHead>
              <TableHead className={cn(CABECERA_TABLA, "w-36")}>{t("serumsIa.balotario.problemas.lineas")}</TableHead>
              <TableHead className={cn(CABECERA_TABLA, "pr-4")}>{t("serumsIa.balotario.problemas.motivos")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody className="text-control">
            {data.map((p) => (
              <TableRow key={p.linea_inicio} className="align-top">
                <TableCell className="px-3 py-2 pl-4 font-mono tabular-nums">
                  {p.numero ?? (
                    <span className="text-muted-foreground font-sans">
                      {t("serumsIa.balotario.problemas.sinNumero")}
                    </span>
                  )}
                </TableCell>
                <TableCell className="px-3 py-2 font-mono tabular-nums">
                  {p.linea_inicio}–{p.linea_fin}
                </TableCell>
                <TableCell className="px-3 py-2 pr-4">
                  <ul className="flex flex-wrap gap-1.5">
                    {p.motivos.map((m) => (
                      <li key={m}>
                        <Chip>{t(`serumsIa.balotario.motivos.${m}`)}</Chip>
                      </li>
                    ))}
                  </ul>
                </TableCell>
              </TableRow>
            ))}
            {data.length === 0 ? (
              <FilaVacia columnas={3} icon={<CheckCircleIcon className="size-6" />}>
                {t("serumsIa.balotario.problemas.vacio")}
              </FilaVacia>
            ) : null}
          </TableBody>
        </Table>
        {total > 0 ? (
          <PieTabla
            desde={desde}
            hasta={desde + data.length - 1}
            total={total}
            unidad={t("serumsIa.balotario.problemas.unidad")}
            porPagina={porPagina}
            onPorPagina={onPorPagina}
            pagina={page}
            ultimaPagina={ultimaPaginaProblemas(problemas)}
            onPagina={onPagina}
          />
        ) : null}
      </div>
    </section>
  );
}
