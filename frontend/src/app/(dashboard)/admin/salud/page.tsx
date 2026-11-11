"use client";

import pkg from "../../../../../package.json";
import { PillEstadoServicio, SkeletonTarjetas, TarjetaFrontend, TarjetaServicio, ZONA_HORARIA } from "./componentes";
import { useSalud } from "./use-salud";
import { useFormatter, useTranslations } from "next-intl";

import { Alerta } from "@/components/feedback/alerta";
import { BotonAsync } from "@/components/patrones/boton-async";
import { Chip } from "@/components/patrones/pill-estado";
import { useAuth } from "@/lib/auth/auth-context";
import { ACCION_SECUNDARIA, AYUDA_CAMPO, CONTENEDOR_PAGINA, ROTULO } from "@/lib/estilos";
import { PESO_ICONO_FUNCIONAL } from "@/lib/iconos";
import { versionBackend, versionesDistintas } from "@/lib/salud/reglas";
import { ArrowClockwiseIcon } from "@phosphor-icons/react";

/** Tarjetas de la carga inicial: API, base de datos, correo y frontend. */
const TARJETAS_EN_CARGA = 4;

export default function SaludServiciosPage() {
  const { token } = useAuth();
  const t = useTranslations("dashboard");
  const formato = useFormatter();
  const { salud, fase, enCurso, actualizar } = useSalud(token);

  const desactualizado = fase === "desactualizado";
  const hora = (iso: string) =>
    formato.dateTime(new Date(iso), { dateStyle: "medium", timeStyle: "medium", timeZone: ZONA_HORARIA });

  return (
    <div className={CONTENEDOR_PAGINA}>
      <section aria-labelledby="salud-titulo" className="flex flex-wrap items-center justify-between gap-3">
        <div className="grid gap-1">
          <h1 id="salud-titulo" className={ROTULO}>
            {t("saludServicios.estadoGlobal")}
          </h1>
          <div className="flex flex-wrap items-center gap-2">
            {salud ? <PillEstadoServicio estado={salud.estado} /> : null}
            {salud ? (
              <span className="text-control text-muted-foreground tabular-nums">
                {t("saludServicios.ultimaComprobacion", { fecha: hora(salud.comprobadoEn) })}
              </span>
            ) : null}
            {desactualizado ? <Chip>{t("saludServicios.desactualizado")}</Chip> : null}
          </div>
        </div>
        <BotonAsync
          pendiente={enCurso}
          disabled={!token}
          onClick={actualizar}
          className={ACCION_SECUNDARIA}
          icon={<ArrowClockwiseIcon weight={PESO_ICONO_FUNCIONAL} className="size-4" />}
          textoPendiente={t("saludServicios.actualizando")}
        >
          {t("saludServicios.actualizar")}
        </BotonAsync>
      </section>
      <p className={AYUDA_CAMPO}>{t("saludServicios.ayuda")}</p>

      {fase === "error" || desactualizado ? (
        <Alerta tono="error" titulo={t("saludServicios.error.titulo")}>
          {salud
            ? t("saludServicios.error.conDatos", { fecha: hora(salud.comprobadoEn) })
            : t("saludServicios.error.sinDatos")}
        </Alerta>
      ) : null}

      {fase === "cargando" ? (
        <div className="grid gap-3 sm:grid-cols-2" aria-busy="true">
          <SkeletonTarjetas cantidad={TARJETAS_EN_CARGA} />
        </div>
      ) : salud ? (
        <>
          {versionesDistintas(pkg.version, versionBackend(salud)) ? (
            <Alerta tono="info">{t("saludServicios.versionesDistintas")}</Alerta>
          ) : null}
          <div className="grid gap-3 sm:grid-cols-2">
            {salud.servicios.map((s) => (
              <TarjetaServicio key={s.id} servicio={s} desactualizada={desactualizado} />
            ))}
            <TarjetaFrontend version={pkg.version} />
          </div>
        </>
      ) : null}
    </div>
  );
}
