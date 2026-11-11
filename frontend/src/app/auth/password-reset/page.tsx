"use client";
import { useFormik } from "formik";
import Link from "next/link";
import { useTranslations } from "next-intl";
import React, { useMemo, useState } from "react";
import * as yup from "yup";

import { ENLACE, TarjetaAuth } from "@/app/auth/tarjeta-auth";
import { Campo } from "@/components/app/campo";
import { Alerta } from "@/components/feedback/alerta";
import { Entrada } from "@/components/formularios/campo";
import { BotonAsync } from "@/components/patrones/boton-async";
import { authApi } from "@/lib/auth/auth-api";
import { claveErrorAuth } from "@/lib/auth/errores-auth";
import { BOTON_PRIMARIO } from "@/lib/estilos";
import { cn } from "@/lib/utils";
import { ArrowLeftIcon, PaperPlaneTiltIcon } from "@phosphor-icons/react";

export default function Page() {
  const [sent, setSent] = useState(false);
  const [apiError, setApiError] = useState<string | null>(null);
  const t = useTranslations("dashboard");
  const validationSchema = useMemo(
    () => yup.object({ email: yup.string().required(t("auth-campo-requerido")).email(t("auth-correo-invalido")) }),
    [t],
  );

  const formik = useFormik({
    initialValues: { email: "" },
    validationSchema,
    validateOnBlur: false,
    onSubmit: async (values, { setSubmitting }) => {
      setApiError(null);
      try {
        await authApi.forgotPassword(values.email);
        setSent(true);
      } catch (err: unknown) {
        // 429 (rate limit), 422 (validation), 5xx and network errors each get their own message.
        setApiError(t(claveErrorAuth(err, "password-reset-error")));
      } finally {
        setSubmitting(false);
      }
    },
  });

  const emailError = formik.touched.email ? formik.errors.email : undefined;

  return (
    <TarjetaAuth
      titulo={t("auth-recuperar-contrasena")}
      descripcion={t("password-reset-descripcion")}
      pie={
        <Link href="/auth/sign-in" className={cn(ENLACE, "inline-flex items-center gap-1.5")}>
          <ArrowLeftIcon weight="bold" className="size-3.5" />
          {t("auth-volver-sign-in")}
        </Link>
      }
    >
      {sent ? (
        <Alerta tono="ok" titulo={t("password-reset-enviado")}>
          {t("password-reset-enviado-detalle")}
        </Alerta>
      ) : (
        <form onSubmit={formik.handleSubmit} noValidate className="flex flex-col gap-4">
          {apiError ? <Alerta tono="error">{apiError}</Alerta> : null}

          <Campo id="email" etiqueta={t("auth-correo")} error={emailError}>
            <Entrada
              id="email"
              name="email"
              type="email"
              placeholder={t("auth-correo-placeholder")}
              autoComplete="email"
              autoFocus
              value={formik.values.email}
              onChange={formik.handleChange}
              onBlur={formik.handleBlur}
              invalido={!!emailError}
              aria-describedby={emailError ? "email-error" : undefined}
            />
          </Campo>

          <BotonAsync
            type="submit"
            pendiente={formik.isSubmitting}
            textoPendiente={t("auth-enviando")}
            icon={<PaperPlaneTiltIcon className="size-4" />}
            className={cn(BOTON_PRIMARIO, "mt-2 w-full")}
          >
            {t("auth-enviar-enlace")}
          </BotonAsync>
        </form>
      )}
    </TarjetaAuth>
  );
}
