"use client";

import { useFormik } from "formik";
import { useFormatter, useTranslations } from "next-intl";
import { useMemo, useState } from "react";

import { Campo } from "@/components/app/campo";
import { Alerta } from "@/components/feedback/alerta";
import { Entrada } from "@/components/formularios/campo";
import { BotonAsync } from "@/components/patrones/boton-async";
import { BOTON_PRIMARIO, TARJETA } from "@/lib/estilos";
import type { CambioConfiguracionIa, ConfiguracionIa } from "@/lib/serums-ia/ia/api";
import { claveErrorIa } from "@/lib/serums-ia/ia/errores";
import {
  aCambio,
  aValores,
  type CampoConfiguracion,
  CAMPOS_CONFIGURACION,
  camposRechazados,
  esquemaConfiguracion,
  RANGOS,
} from "@/lib/serums-ia/ia/reglas";
import { cn } from "@/lib/utils";

const MODO_TECLADO = (campo: CampoConfiguracion) => (campo === "presupuesto_mensual_usd" ? "decimal" : "numeric");

/** Límites por actor, tamaños de solicitud y presupuesto mensual (E6-02 D19). El servidor valida de nuevo y manda. */
export function FormularioLimites({
  configuracion,
  onGuardar,
}: {
  configuracion: ConfiguracionIa;
  /** Llama a `PUT /admin/ia/configuracion`; si rechaza, el error se muestra sin perder lo escrito. */
  onGuardar: (cambio: CambioConfiguracionIa) => Promise<void>;
}) {
  const t = useTranslations("dashboard");
  const formato = useFormatter();
  const [errorServidor, setErrorServidor] = useState<string | null>(null);

  const validationSchema = useMemo(
    () =>
      esquemaConfiguracion({
        rango: (campo) =>
          t(
            campo === "presupuesto_mensual_usd"
              ? "serumsIa.ia.configuracion.validacion.presupuesto"
              : "serumsIa.ia.configuracion.validacion.rango",
            RANGOS[campo],
          ),
        decimales: t("serumsIa.ia.configuracion.validacion.decimales"),
        diaMayorQueMes: t("serumsIa.ia.configuracion.validacion.diaMayorQueMes"),
      }),
    [t],
  );

  const formik = useFormik({
    initialValues: aValores(configuracion),
    validationSchema,
    validateOnMount: true,
    onSubmit: async (valores) => {
      setErrorServidor(null);
      try {
        await onGuardar(aCambio(valores));
      } catch (e) {
        setErrorServidor(t(claveErrorIa(e, t.has)));
        // El servidor dice qué campos rechazó: se marcan sin perder lo escrito.
        for (const campo of camposRechazados(e)) {
          void formik.setFieldTouched(campo, true, false);
          formik.setFieldError(campo, t("serumsIa.ia.configuracion.validacion.rechazado"));
        }
      }
    },
  });

  const errorDe = (campo: CampoConfiguracion) => {
    const e = formik.touched[campo] ? formik.errors[campo] : undefined;
    return typeof e === "string" ? e : undefined;
  };

  return (
    <form
      onSubmit={formik.handleSubmit}
      noValidate
      className={cn(TARJETA, "flex flex-col gap-4 p-4")}
      aria-labelledby="ia-configuracion"
    >
      <div>
        <h2 id="ia-configuracion" className="text-titulo text-foreground">
          {t("serumsIa.ia.configuracion.titulo")}
        </h2>
        <p className="text-control text-muted-foreground">{t("serumsIa.ia.configuracion.ayuda")}</p>
      </div>

      {errorServidor ? <Alerta tono="error">{errorServidor}</Alerta> : null}

      <div className="grid gap-4 md:grid-cols-2">
        {CAMPOS_CONFIGURACION.map((campo) => (
          <Campo
            key={campo}
            id={`ia-${campo}`}
            etiqueta={t(`serumsIa.ia.configuracion.campos.${campo}.etiqueta`)}
            ayuda={t(`serumsIa.ia.configuracion.campos.${campo}.ayuda`)}
            error={errorDe(campo)}
          >
            <Entrada
              id={`ia-${campo}`}
              name={campo}
              mono
              inputMode={MODO_TECLADO(campo)}
              value={formik.values[campo]}
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
              invalido={Boolean(errorDe(campo))}
              aria-describedby={`ia-${campo}-${errorDe(campo) ? "error" : "ayuda"}`}
            />
          </Campo>
        ))}
      </div>

      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="text-etiqueta text-muted-foreground">
          {t("serumsIa.ia.configuracion.actualizada", {
            fecha: formato.dateTime(new Date(configuracion.updated_at), {
              dateStyle: "medium",
              timeStyle: "short",
              timeZone: "America/Lima",
            }),
          })}
        </p>
        <BotonAsync
          type="submit"
          pendiente={formik.isSubmitting}
          textoPendiente={t("serumsIa.ia.configuracion.guardando")}
          className={BOTON_PRIMARIO}
          disabled={!formik.isValid || !formik.dirty}
        >
          {t("serumsIa.ia.configuracion.guardar")}
        </BotonAsync>
      </div>
    </form>
  );
}
