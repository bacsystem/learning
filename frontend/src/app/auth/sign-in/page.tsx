"use client";
import { useFormik } from "formik";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import React, { useMemo, useState } from "react";
import * as yup from "yup";

import { useEtiquetasContrasena } from "@/app/auth/esquema-contrasena";
import { ENLACE, TarjetaAuth } from "@/app/auth/tarjeta-auth";
import { Campo } from "@/components/app/campo";
import { Alerta } from "@/components/feedback/alerta";
import { Entrada } from "@/components/formularios/campo";
import { Contrasena } from "@/components/formularios/contrasena";
import { BotonAsync } from "@/components/patrones/boton-async";
import { useAuth } from "@/lib/auth/auth-context";
import { claveErrorAuth } from "@/lib/auth/errores-auth";
import { BOTON_PRIMARIO } from "@/lib/estilos";
import { cn } from "@/lib/utils";
import { SignInIcon } from "@phosphor-icons/react";

export default function Page() {
  const router = useRouter();
  const { login } = useAuth();
  const [apiError, setApiError] = useState<string | null>(null);
  const t = useTranslations("dashboard");
  const etiquetasContrasena = useEtiquetasContrasena();
  const validationSchema = useMemo(
    () =>
      yup.object({
        username: yup.string().required(t("auth-campo-requerido")),
        password: yup.string().required(t("auth-campo-requerido")),
      }),
    [t],
  );

  const formik = useFormik({
    initialValues: { username: "", password: "" },
    validationSchema,
    validateOnBlur: false,
    validateOnMount: false,
    onSubmit: async (values, { setSubmitting }) => {
      setApiError(null);
      try {
        await login(values.username, values.password);
        router.push("/procesos");
      } catch (err: unknown) {
        const e = err as { status?: number; message?: string };
        // 403 distinguishes pending / rejected / inactive by the backend message (its only signal).
        if (e.status === 403 && e.message?.includes("pendiente")) {
          router.push("/auth/pending-activation");
          return;
        }
        const cuenta403 = e.message?.includes("rechazada")
          ? "sign-in-error-rechazada"
          : e.message?.includes("desactivada")
            ? "sign-in-error-desactivada"
            : undefined;
        setApiError(t(claveErrorAuth(err, "sign-in-error", { 401: "sign-in-error-credenciales", 403: cuenta403 })));
      } finally {
        setSubmitting(false);
      }
    },
  });

  const err = (name: keyof typeof formik.values) => (formik.touched[name] ? formik.errors[name] : undefined);

  return (
    <TarjetaAuth
      titulo={t("auth-iniciar-sesion")}
      descripcion={t("sign-in-descripcion")}
      pie={
        <>
          {t("sign-in-sin-cuenta")}{" "}
          <Link href="/auth/sign-up" className={ENLACE}>
            {t("sign-in-registrate")}
          </Link>
        </>
      }
    >
      <form onSubmit={formik.handleSubmit} noValidate className="flex flex-col gap-4">
        {apiError ? <Alerta tono="error">{apiError}</Alerta> : null}

        <Campo id="username" etiqueta={t("sign-in-usuario")} error={err("username")}>
          <Entrada
            id="username"
            name="username"
            placeholder={t("sign-in-usuario-placeholder")}
            autoComplete="username"
            autoFocus
            value={formik.values.username}
            onChange={formik.handleChange}
            onBlur={formik.handleBlur}
            invalido={!!err("username")}
            aria-describedby={err("username") ? "username-error" : undefined}
          />
        </Campo>

        <div className="grid gap-1.5">
          <Campo id="password" etiqueta={t("auth-contrasena")} error={err("password")}>
            <Contrasena
              id="password"
              valor={formik.values.password}
              onCambio={(v) => formik.setFieldValue("password", v)}
              onBlur={() => formik.setFieldTouched("password", true)}
              autoComplete="current-password"
              {...etiquetasContrasena}
              invalido={!!err("password")}
              describidoPor={err("password") ? "password-error" : undefined}
            />
          </Campo>
          <Link href="/auth/password-reset" className={cn(ENLACE, "text-control justify-self-end")}>
            {t("sign-in-olvidaste")}
          </Link>
        </div>

        <BotonAsync
          type="submit"
          pendiente={formik.isSubmitting}
          textoPendiente={t("sign-in-ingresando")}
          icon={<SignInIcon className="size-4" />}
          className={cn(BOTON_PRIMARIO, "mt-2 w-full")}
        >
          {t("sign-in-ingresar")}
        </BotonAsync>
      </form>
    </TarjetaAuth>
  );
}
