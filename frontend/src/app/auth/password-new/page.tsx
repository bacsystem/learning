"use client";
import { useFormik } from "formik";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
import React, { useEffect, useMemo, useState } from "react";
import * as yup from "yup";

import { esquemaContrasena, useEtiquetasContrasena } from "@/app/auth/esquema-contrasena";
import { ENLACE, TarjetaAuth } from "@/app/auth/tarjeta-auth";
import { Alerta } from "@/components/feedback/alerta";
import { Campo } from "@/components/formularios/campo";
import { Contrasena } from "@/components/formularios/contrasena";
import { BotonAsync } from "@/components/patrones/boton-async";
import { authApi } from "@/lib/auth/auth-api";
import { claveErrorAuth } from "@/lib/auth/errores-auth";
import { ACCION_SECUNDARIA, BOTON_PRIMARIO } from "@/lib/estilos";
import { cn } from "@/lib/utils";
import { ArrowLeftIcon, LockKeyIcon } from "@phosphor-icons/react";

export default function Page() {
  const router = useRouter();
  const t = useTranslations("dashboard");
  const etiquetasContrasena = useEtiquetasContrasena();
  const searchParams = useSearchParams();
  // Read once: the token is then removed from the URL (and the history entry).
  const [token] = useState(() => searchParams.get("token") ?? "");
  const [apiError, setApiError] = useState<string | null>(null);
  // 400 = link expired or already used: offer to request another one.
  const [enlaceVencido, setEnlaceVencido] = useState(false);
  const [success, setSuccess] = useState(false);

  useEffect(() => {
    if (token) window.history.replaceState(null, "", window.location.pathname);
  }, [token]);

  // Same rules as the backend (ResetPasswordRequest): 8+ characters, at most 72 BYTES.
  const validationSchema = useMemo(() => yup.object(esquemaContrasena(t)), [t]);

  const formik = useFormik({
    initialValues: { password: "", confirm_password: "" },
    validationSchema,
    validateOnBlur: false,
    onSubmit: async (values, { setSubmitting }) => {
      setApiError(null);
      setEnlaceVencido(false);
      try {
        await authApi.resetPassword(token, values.password, values.confirm_password);
        setSuccess(true);
        setTimeout(() => router.push("/auth/sign-in"), 2000);
      } catch (err: unknown) {
        const e = err as { status?: number; message?: string };
        setEnlaceVencido(e.status === 400);
        setApiError(t(claveErrorAuth(err, "password-new-error", { 400: "password-new-invalido-detalle" })));
      } finally {
        setSubmitting(false);
      }
    },
  });

  const err = (name: keyof typeof formik.values) => (formik.touched[name] ? formik.errors[name] : undefined);

  return (
    <TarjetaAuth
      titulo={t("password-new-titulo")}
      descripcion={t("password-new-descripcion")}
      pie={
        <Link href="/auth/sign-in" className={cn(ENLACE, "inline-flex items-center gap-1.5")}>
          <ArrowLeftIcon weight="bold" className="size-3.5" />
          {t("auth-volver-sign-in")}
        </Link>
      }
    >
      {!token ? (
        <Alerta
          tono="error"
          titulo={t("password-new-invalido")}
          accion={
            <Link href="/auth/password-reset" className={ACCION_SECUNDARIA}>
              {t("password-new-solicitar-otro")}
            </Link>
          }
        >
          {t("password-new-invalido-detalle")}
        </Alerta>
      ) : success ? (
        <Alerta tono="ok" titulo={t("password-new-actualizada")}>
          {t("password-new-redirigiendo")}
        </Alerta>
      ) : (
        <form onSubmit={formik.handleSubmit} noValidate className="flex flex-col gap-4">
          {apiError ? (
            <Alerta
              tono="error"
              accion={
                enlaceVencido ? (
                  <Link href="/auth/password-reset" className={ACCION_SECUNDARIA}>
                    {t("password-new-solicitar-otro")}
                  </Link>
                ) : undefined
              }
            >
              {apiError}
            </Alerta>
          ) : null}

          <Campo
            id="password"
            etiqueta={t("password-new-titulo")}
            ayuda={t("password-new-ayuda")}
            error={err("password")}
          >
            <Contrasena
              id="password"
              valor={formik.values.password}
              onCambio={(v) => formik.setFieldValue("password", v)}
              autoComplete="new-password"
              conFuerza
              {...etiquetasContrasena}
              invalido={!!err("password")}
              describidoPor={err("password") ? "password-error" : "password-ayuda"}
            />
          </Campo>

          <Campo id="confirm_password" etiqueta={t("auth-confirmar-contrasena")} error={err("confirm_password")}>
            <Contrasena
              id="confirm_password"
              valor={formik.values.confirm_password}
              onCambio={(v) => formik.setFieldValue("confirm_password", v)}
              autoComplete="new-password"
              {...etiquetasContrasena}
              invalido={!!err("confirm_password")}
              describidoPor={err("confirm_password") ? "confirm_password-error" : undefined}
            />
          </Campo>

          <BotonAsync
            type="submit"
            pendiente={formik.isSubmitting}
            textoPendiente={t("comun-guardando")}
            icon={<LockKeyIcon className="size-4" />}
            className={cn(BOTON_PRIMARIO, "mt-2 w-full")}
          >
            {t("password-new-restablecer")}
          </BotonAsync>
        </form>
      )}
    </TarjetaAuth>
  );
}
