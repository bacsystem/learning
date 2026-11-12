"use client";

import { useFormatter, useTranslations } from "next-intl";

import { Alerta } from "@/components/feedback/alerta";
import { ProgresoLineal, type TonoProgreso } from "@/components/feedback/progreso";
import { FilaMetricas, Metrica } from "@/components/patrones/metricas";
import { CONTENEDOR_TABLA, FilaVacia } from "@/components/patrones/pie-tabla";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { CABECERA_TABLA, TARJETA } from "@/lib/estilos";
import { formatearMonto } from "@/lib/formato";
import type { GastoIa } from "@/lib/serums-ia/ia/api";
import { type EstadoGasto, estadoGasto, porcentajeBarra } from "@/lib/serums-ia/ia/reglas";
import { cn } from "@/lib/utils";
import { CoinsIcon, WalletIcon } from "@phosphor-icons/react";

const TONO_BARRA: Record<EstadoGasto, TonoProgreso> = { normal: "success", alerta: "warning", agotado: "destructive" };
const TONO_ALERTA = { normal: "ok", alerta: "aviso", agotado: "error" } as const;

/** Gasto del mes en curso: presupuesto, avance, estado de la alerta del 80 % y detalle por operación (sin cifras por usuario). */
export function TarjetaGasto({ gasto }: { gasto: GastoIa }) {
  const t = useTranslations("dashboard");
  const formato = useFormatter();
  const estado = estadoGasto(gasto);
  const fecha = (iso: string) =>
    formato.dateTime(new Date(iso), { dateStyle: "medium", timeStyle: "short", timeZone: "America/Lima" });
  // El mes llega como `AAAA-MM`; se ancla a mediodía de Lima para que la zona no lo corra de mes.
  const mes = formato.dateTime(new Date(`${gasto.mes}-01T12:00:00-05:00`), {
    month: "long",
    year: "numeric",
    timeZone: "America/Lima",
  });
  const porcentaje = formato.number(gasto.porcentaje, { maximumFractionDigits: 2 });
  const textoEstado =
    estado === "agotado"
      ? t("serumsIa.ia.gasto.estado.agotado", { fecha: fecha(gasto.reinicia_at) })
      : estado === "alerta" && gasto.alerta_80_at
        ? t("serumsIa.ia.gasto.estado.alerta", { fecha: fecha(gasto.alerta_80_at) })
        : t("serumsIa.ia.gasto.estado.normal");

  return (
    <section className={cn(TARJETA, "flex flex-col gap-4 p-4")} aria-labelledby="ia-gasto">
      <div>
        <h2 id="ia-gasto" className="text-titulo text-foreground">
          {t("serumsIa.ia.gasto.titulo")}
        </h2>
        <p className="text-control text-muted-foreground">{t("serumsIa.ia.gasto.mes", { mes })}</p>
      </div>

      <FilaMetricas columnas={2}>
        <Metrica
          etiqueta={t("serumsIa.ia.gasto.presupuesto")}
          icono={WalletIcon}
          ayuda={t("serumsIa.ia.gasto.reinicia", { fecha: fecha(gasto.reinicia_at) })}
        >
          {formatearMonto("USD", gasto.presupuesto_usd)}
        </Metrica>
        <Metrica
          etiqueta={t("serumsIa.ia.gasto.gastado")}
          icono={CoinsIcon}
          tono={estado === "normal" ? "neutro" : TONO_ALERTA[estado]}
          ayuda={t("serumsIa.ia.gasto.porcentaje", { porcentaje })}
        >
          {formatearMonto("USD", gasto.gastado_usd)}
        </Metrica>
      </FilaMetricas>

      <ProgresoLineal
        valor={porcentajeBarra(gasto)}
        tono={TONO_BARRA[estado]}
        etiqueta={t("serumsIa.ia.gasto.avance")}
      />
      <Alerta tono={TONO_ALERTA[estado]}>{textoEstado}</Alerta>

      <div className="flex flex-col gap-2">
        <h3 className="text-cuerpo text-foreground font-medium">{t("serumsIa.ia.gasto.porOperacion")}</h3>
        <div className={CONTENEDOR_TABLA}>
          <Table>
            <TableHeader>
              <TableRow className="hover:bg-transparent">
                <TableHead className={cn(CABECERA_TABLA, "pl-4")}>{t("serumsIa.ia.gasto.operacion")}</TableHead>
                <TableHead className={cn(CABECERA_TABLA, "text-right")}>
                  {t("serumsIa.ia.gasto.invocaciones")}
                </TableHead>
                <TableHead className={cn(CABECERA_TABLA, "text-right")}>{t("serumsIa.ia.gasto.desdeCache")}</TableHead>
                <TableHead className={cn(CABECERA_TABLA, "pr-4 text-right")}>{t("serumsIa.ia.gasto.costo")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody className="text-control">
              {gasto.por_operacion.map((o) => (
                <TableRow key={o.operacion}>
                  <TableCell className="px-3 py-2 pl-4">
                    {t.has(`serumsIa.ia.gasto.operaciones.${o.operacion}`)
                      ? t(`serumsIa.ia.gasto.operaciones.${o.operacion}`)
                      : o.operacion}
                  </TableCell>
                  <TableCell className="px-3 py-2 text-right font-mono tabular-nums">
                    {formato.number(o.invocaciones)}
                  </TableCell>
                  <TableCell className="px-3 py-2 text-right font-mono tabular-nums">
                    {formato.number(o.desde_cache)}
                  </TableCell>
                  <TableCell className="px-3 py-2 pr-4 text-right font-mono tabular-nums">
                    {formatearMonto("USD", o.costo_usd)}
                  </TableCell>
                </TableRow>
              ))}
              {gasto.por_operacion.length === 0 ? (
                <FilaVacia columnas={4} icon={<CoinsIcon className="size-6" />}>
                  {t("serumsIa.ia.gasto.vacio")}
                </FilaVacia>
              ) : null}
            </TableBody>
          </Table>
        </div>
        <p className="text-etiqueta text-muted-foreground">{t("serumsIa.ia.gasto.sinCifrasPorUsuario")}</p>
      </div>
    </section>
  );
}
