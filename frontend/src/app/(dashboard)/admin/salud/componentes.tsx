"use client";

import { useFormatter, useTranslations } from "next-intl";
import type { ReactNode } from "react";

import { type DatoLista, ListaDatos } from "@/components/datos/lista-datos";
import { Skeleton } from "@/components/feedback/skeleton";
import type { Tono } from "@/components/patrones/metricas";
import { PillEstado } from "@/components/patrones/pill-estado";
import { Card, CardAction, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { AYUDA_CAMPO } from "@/lib/estilos";
import type { IconoComponente } from "@/lib/iconos";
import {
  type CategoriaFallo,
  desglosarDuracion,
  type EstadoServicio,
  type ServicioSalud,
  type UltimoEnvio,
  ultimoEnvio,
} from "@/lib/salud/reglas";
import { CheckCircleIcon, MinusCircleIcon, QuestionIcon, WarningIcon, XCircleIcon } from "@phosphor-icons/react";

/** Hora de Lima en toda la pantalla (PLT-02 regla 1, decisión 6 del PO). */
export const ZONA_HORARIA = "America/Lima";
const SIN_DATO = "—";

/** Icono + texto + color por estado: el estado nunca se comunica solo con el color (PLT-02 regla 3). */
const ASPECTO_ESTADO: Record<EstadoServicio, { tono: Tono; icono: IconoComponente }> = {
  operativo: { tono: "ok", icono: CheckCircleIcon },
  degradado: { tono: "aviso", icono: WarningIcon },
  desconocido: { tono: "aviso", icono: QuestionIcon },
  caido: { tono: "error", icono: XCircleIcon },
  no_configurado: { tono: "neutro", icono: MinusCircleIcon },
};

/** Estado traducido, con la categoría de fallo si la hay («Caído — Sin conexión»). */
export function PillEstadoServicio({
  estado,
  categoria = null,
}: {
  estado: EstadoServicio;
  categoria?: CategoriaFallo | null;
}) {
  const t = useTranslations("dashboard");
  const texto = t(`saludServicios.estado.${estado}`);
  return (
    <PillEstado tono={ASPECTO_ESTADO[estado].tono} icon={ASPECTO_ESTADO[estado].icono}>
      {categoria
        ? t("saludServicios.estadoConCategoria", {
            estado: texto,
            categoria: t(`saludServicios.categoria.${categoria}`),
          })
        : texto}
    </PillEstado>
  );
}

function TarjetaBase({
  titulo,
  estado,
  datos,
  nota,
}: {
  titulo: string;
  estado?: ReactNode;
  datos: DatoLista[];
  nota?: string;
}) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>
          <h2>{titulo}</h2>
        </CardTitle>
        {estado ? <CardAction>{estado}</CardAction> : null}
      </CardHeader>
      <CardContent className="grid gap-3">
        <ListaDatos datos={datos} />
        {nota ? <p className={AYUDA_CAMPO}>{nota}</p> : null}
      </CardContent>
    </Card>
  );
}

function useDatosServicio(servicio: ServicioSalud): DatoLista[] {
  const t = useTranslations("dashboard");
  const formato = useFormatter();
  const numero = (n: number | null, clave?: string) =>
    n === null ? SIN_DATO : clave ? t(clave, { valor: n }) : formato.number(n);
  const textoUltimoEnvio = (envio: UltimoEnvio) => {
    switch (envio.tipo) {
      case "fecha":
        return formato.dateTime(new Date(envio.iso), {
          dateStyle: "medium",
          timeStyle: "short",
          timeZone: ZONA_HORARIA,
        });
      case "nunca":
        return t("saludServicios.dato.nunca");
      case "sin_dato":
        return SIN_DATO;
    }
  };

  switch (servicio.id) {
    case "api": {
      const { version, en_marcha_s } = servicio.datos;
      return [
        { etiqueta: t("saludServicios.dato.version"), valor: version ?? SIN_DATO, mono: true },
        {
          etiqueta: t("saludServicios.dato.enMarcha"),
          valor:
            en_marcha_s === null ? SIN_DATO : t("saludServicios.dato.enMarchaValor", desglosarDuracion(en_marcha_s)),
          mono: true,
        },
      ];
    }
    case "base_datos":
      return [
        {
          etiqueta: t("saludServicios.dato.latencia"),
          valor: numero(servicio.datos.latencia_ms, "saludServicios.dato.latenciaValor"),
          mono: true,
        },
      ];
    case "correo": {
      const d = servicio.datos;
      return [
        { etiqueta: t("saludServicios.dato.sinEnviar"), valor: numero(d.sin_enviar), mono: true },
        {
          etiqueta: t("saludServicios.dato.antiguedad"),
          valor: numero(d.antiguedad_max_min, "saludServicios.dato.antiguedadValor"),
          mono: true,
        },
        { etiqueta: t("saludServicios.dato.fallidos24h"), valor: numero(d.fallidos_24h), mono: true },
        { etiqueta: t("saludServicios.dato.ultimoEnviado"), valor: textoUltimoEnvio(ultimoEnvio(d)), mono: true },
      ];
    }
  }
}

/** Tarjeta de un servicio conocido: nombre, estado con su categoría y sus datos (PLT-02 regla 2, PLT-03 regla 3). */
export function TarjetaServicio({ servicio }: { servicio: ServicioSalud }) {
  const t = useTranslations("dashboard");
  return (
    <TarjetaBase
      titulo={t(`saludServicios.servicio.${servicio.id}`)}
      estado={<PillEstadoServicio estado={servicio.estado} categoria={servicio.categoria} />}
      datos={useDatosServicio(servicio)}
    />
  );
}

/** Versión del frontend que se está viendo, sin estado operativo/caído (decisión 10 del PO, PLT-03 regla 4). */
export function TarjetaFrontend({ version }: { version: string }) {
  const t = useTranslations("dashboard");
  return (
    <TarjetaBase
      titulo={t("saludServicios.servicio.frontend")}
      datos={[{ etiqueta: t("saludServicios.dato.version"), valor: version, mono: true }]}
      nota={t("saludServicios.frontendNota")}
    />
  );
}

/** Carga inicial: la forma de las tarjetas, sin ningún estado inventado. */
export function SkeletonTarjetas({ cantidad }: { cantidad: number }) {
  return (
    <>
      {Array.from({ length: cantidad }).map((_, i) => (
        <Card key={i}>
          <CardHeader>
            <Skeleton className="h-4 w-32" />
            <CardAction>
              <Skeleton className="h-5 w-20 rounded-full" />
            </CardAction>
          </CardHeader>
          <CardContent className="grid gap-2">
            <Skeleton className="h-3 w-3/4" />
            <Skeleton className="h-3 w-1/2" />
          </CardContent>
        </Card>
      ))}
    </>
  );
}
