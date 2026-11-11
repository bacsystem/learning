"use client";
import { useFormik } from "formik";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
import React, { useEffect, useMemo, useRef, useState } from "react";
import * as yup from "yup";

import { AvisoLegal } from "@/app/auth/aviso-legal";
import { esquemaContrasena, useEtiquetasContrasena } from "@/app/auth/esquema-contrasena";
import { ENLACE, TarjetaAuth } from "@/app/auth/tarjeta-auth";
import { Campo } from "@/components/app/campo";
import { Alerta } from "@/components/feedback/alerta";
import { Spinner } from "@/components/feedback/spinner";
import { Entrada } from "@/components/formularios/campo";
import { Contrasena } from "@/components/formularios/contrasena";
import { BotonAsync } from "@/components/patrones/boton-async";
import { authApi } from "@/lib/auth/auth-api";
import { claveErrorAuth } from "@/lib/auth/errores-auth";
import { ACCION_SECUNDARIA, BOTON_PRIMARIO } from "@/lib/estilos";
import { cn } from "@/lib/utils";
import { ArrowLeftIcon, CheckCircleIcon, UserPlusIcon, XCircleIcon } from "@phosphor-icons/react";

type Status = "form" | "success" | "invalid";
type UsernameStatus = "idle" | "checking" | "available" | "taken";

/** Indicador dentro del campo mientras se verifica la disponibilidad del usuario. */
function EstadoDisponibilidad({ estado }: { estado: UsernameStatus }) {
  const t = useTranslations("dashboard");
  if (estado === "idle") return null;
  return (
    <span className="pointer-events-none absolute inset-y-0 right-3 flex items-center">
      {estado === "checking" ? <Spinner tamano="sm" className="text-muted-foreground" /> : null}
      {estado === "available" ? (
        <CheckCircleIcon weight="fill" aria-label={t("auth-disponible")} className="text-success-foreground size-4" />
      ) : null}
      {estado === "taken" ? (
        <XCircleIcon weight="fill" aria-label={t("auth-en-uso")} className="text-destructive-texto size-4" />
      ) : null}
    </span>
  );
}

export default function Page() {
  const t = useTranslations("dashboard");
  const etiquetasContrasena = useEtiquetasContrasena();
  const searchParams = useSearchParams();
  // Read once: the token is then removed from the URL (and the history entry).
  const [token] = useState(() => searchParams.get("token") ?? "");
  const [status, setStatus] = useState<Status>(token ? "form" : "invalid");
  const [apiError, setApiError] = useState<string | null>(null);
  const [usernameError, setUsernameError] = useState<string | null>(null);
  const [usernameStatus, setUsernameStatus] = useState<UsernameStatus>("idle");
  const usernameTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  // Id of the latest availability request: older answers are dropped.
  const usernameReq = useRef(0);

  useEffect(() => {
    if (token) window.history.replaceState(null, "", window.location.pathname);
  }, [token]);

  // Same rules as the backend (VerifyEmailRequest): username alphanumeric 4–30,
  // password 8+ characters and at most 72 BYTES (bcrypt limit).
  const credentialsSchema = useMemo(
    () =>
      yup.object({
        username: yup
          .string()
          .required(t("verify-email-usuario-requerido"))
          .min(4, t("verify-email-usuario-min"))
          .max(30, t("verify-email-usuario-max"))
          .matches(/^[a-zA-Z0-9]+$/, t("verify-email-usuario-alfanumerico")),
        ...esquemaContrasena(t),
      }),
    [t],
  );

  const formik = useFormik({
    initialValues: { username: "", password: "", confirm_password: "" },
    validationSchema: credentialsSchema,
    validateOnBlur: true,
    validateOnChange: false,
    onSubmit: async (values, { setSubmitting }) => {
      setApiError(null);
      setUsernameError(null);
      try {
        await authApi.verifyEmail({ token, ...values });
        setStatus("success");
      } catch (err: unknown) {
        const e = err as { status?: number; message?: string };
        if (e.status === 400) {
          setStatus("invalid");
        } else if (e.status === 409 && e.message?.includes("usuario")) {
          // The link is still valid: let the user pick another username.
          setUsernameError(t("verify-email-usuario-en-uso-elige"));
          setUsernameStatus("taken");
        } else {
          // The other 409 is the e-mail now linked to another persona.
          setApiError(t(claveErrorAuth(err, "verify-email-error", { 409: "verify-email-correo-en-uso" })));
        }
      } finally {
        setSubmitting(false);
      }
    },
  });

  const err = (name: keyof typeof formik.values) => (formik.touched[name] ? formik.errors[name] : undefined);

  // Debounced username availability check.
  useEffect(() => {
    const username = formik.values.username;
    const reqId = ++usernameReq.current;
    if (status !== "form" || username.length < 4) {
      setUsernameStatus("idle");
      return;
    }
    setUsernameStatus("checking");
    if (usernameTimer.current) clearTimeout(usernameTimer.current);
    usernameTimer.current = setTimeout(async () => {
      try {
        const res = await authApi.checkUsername(username);
        if (reqId !== usernameReq.current) return;
        setUsernameStatus(res.available ? "available" : "taken");
      } catch {
        if (reqId !== usernameReq.current) return;
        setUsernameStatus("idle");
      }
    }, 450);
    return () => {
      if (usernameTimer.current) clearTimeout(usernameTimer.current);
    };
  }, [formik.values.username, status]);

  const pie = (
    <Link href="/auth/sign-in" className={cn(ENLACE, "inline-flex items-center gap-1.5")}>
      <ArrowLeftIcon weight="bold" className="size-3.5" />
      {t("auth-volver-sign-in")}
    </Link>
  );

  if (status === "success") {
    return (
      <TarjetaAuth titulo={t("verify-email-confirmacion")} pie={pie}>
        <Alerta tono="ok" titulo={t("verify-email-completado")}>
          {t("verify-email-completado-detalle")}
        </Alerta>
      </TarjetaAuth>
    );
  }

  if (status === "invalid") {
    return (
      <TarjetaAuth titulo={t("verify-email-confirmacion")} pie={pie}>
        <Alerta
          tono="error"
          titulo={t("verify-email-invalido")}
          accion={
            <Link href="/auth/sign-up" className={ACCION_SECUNDARIA}>
              {t("verify-email-volver-registro")}
            </Link>
          }
        >
          {t("verify-email-invalido-detalle")}
        </Alerta>
      </TarjetaAuth>
    );
  }

  const usernameMostrarError =
    usernameError ?? (usernameStatus === "taken" ? t("verify-email-usuario-en-uso") : err("username"));
  const usernameAyuda =
    usernameStatus === "available" ? (
      <span className="text-success-foreground font-medium">{t("auth-disponible")}</span>
    ) : undefined;

  return (
    <TarjetaAuth titulo={t("verify-email-titulo")} descripcion={t("verify-email-descripcion")} pie={pie}>
      <form onSubmit={formik.handleSubmit} noValidate className="flex flex-col gap-4">
        {apiError ? (
          <Alerta
            tono="error"
            accion={
              <button type="button" onClick={() => setApiError(null)} className={cn(ENLACE, "text-control")}>
                {t("comun-cerrar")}
              </button>
            }
          >
            {apiError}
          </Alerta>
        ) : null}

        <Campo id="username" etiqueta={t("verify-email-usuario")} error={usernameMostrarError} ayuda={usernameAyuda}>
          <div className="relative">
            <Entrada
              id="username"
              name="username"
              placeholder={t("verify-email-usuario-placeholder")}
              autoComplete="username"
              autoFocus
              className="pr-9"
              invalido={!!usernameMostrarError}
              aria-describedby={usernameMostrarError ? "username-error" : usernameAyuda ? "username-ayuda" : undefined}
              value={formik.values.username}
              onChange={(e) => {
                formik.handleChange(e);
                setUsernameError(null);
                setUsernameStatus("idle");
              }}
              onBlur={formik.handleBlur}
            />
            {formik.values.username.length >= 4 ? <EstadoDisponibilidad estado={usernameStatus} /> : null}
          </div>
        </Campo>

        <Campo id="password" etiqueta={t("auth-contrasena")} error={err("password")}>
          <Contrasena
            id="password"
            valor={formik.values.password}
            onCambio={(v) => formik.setFieldValue("password", v)}
            onBlur={() => formik.setFieldTouched("password", true)}
            placeholder={t("auth-contrasena-min")}
            autoComplete="new-password"
            conFuerza
            {...etiquetasContrasena}
            invalido={!!err("password")}
            describidoPor={err("password") ? "password-error" : undefined}
          />
        </Campo>

        <Campo id="confirm_password" etiqueta={t("auth-confirmar-contrasena")} error={err("confirm_password")}>
          <Contrasena
            id="confirm_password"
            valor={formik.values.confirm_password}
            onCambio={(v) => formik.setFieldValue("confirm_password", v)}
            onBlur={() => formik.setFieldTouched("confirm_password", true)}
            placeholder={t("verify-email-repite-contrasena")}
            autoComplete="new-password"
            {...etiquetasContrasena}
            invalido={!!err("confirm_password")}
            describidoPor={err("confirm_password") ? "confirm_password-error" : undefined}
          />
        </Campo>

        <BotonAsync
          type="submit"
          pendiente={formik.isSubmitting}
          textoPendiente={t("verify-email-creando")}
          disabled={usernameStatus === "taken" || usernameStatus === "checking"}
          icon={<UserPlusIcon className="size-4" />}
          className={cn(BOTON_PRIMARIO, "mt-2 w-full")}
        >
          {t("sign-up-titulo")}
        </BotonAsync>
        <AvisoLegal />
      </form>
    </TarjetaAuth>
  );
}
