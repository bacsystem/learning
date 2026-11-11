"use client";
import { useFormik } from "formik";
import Link from "next/link";
import { useTranslations } from "next-intl";
import React, { useMemo, useRef, useState } from "react";
import * as yup from "yup";

import { AvisoLegal } from "@/app/auth/aviso-legal";
import { ENLACE, TarjetaAuth } from "@/app/auth/tarjeta-auth";
import { Campo } from "@/components/app/campo";
import { Alerta } from "@/components/feedback/alerta";
import { Spinner } from "@/components/feedback/spinner";
import { Entrada } from "@/components/formularios/campo";
import { BotonAsync } from "@/components/patrones/boton-async";
import { Chip } from "@/components/patrones/pill-estado";
import { authApi, type CheckDNIResult } from "@/lib/auth/auth-api";
import { claveErrorAuth } from "@/lib/auth/errores-auth";
import { BOTON_PRIMARIO, BOTON_SECUNDARIO } from "@/lib/estilos";
import { cn } from "@/lib/utils";
import {
  ArrowLeftIcon,
  ArrowRightIcon,
  EnvelopeSimpleIcon,
  IdentificationCardIcon,
  PaperPlaneTiltIcon,
} from "@phosphor-icons/react";

type Persona = CheckDNIResult;

function initials(p: Persona) {
  return `${p.nombres[0] ?? ""}${p.apellido_paterno[0] ?? ""}`.toUpperCase();
}

function fullName(p: Persona) {
  return `${p.nombres} ${p.apellido_paterno}`;
}

/** Indicador dentro del campo mientras se verifica el DNI. */
function Verificando() {
  return (
    <span className="pointer-events-none absolute inset-y-0 right-3 flex items-center">
      <Spinner tamano="sm" className="text-muted-foreground" />
    </span>
  );
}

export default function Page() {
  const t = useTranslations("dashboard");
  const dniRef = useRef<HTMLInputElement>(null);
  // Step 2 only confirms the e-mail: username and password are chosen on the
  // page the e-mailed link opens (/auth/verify-email).
  const detailsSchema = useMemo(
    () =>
      yup.object({
        email: yup.string().required(t("sign-up-correo-requerido")).email(t("auth-correo-invalido")),
      }),
    [t],
  );

  // Step 1 state
  const [dni, setDni] = useState("");
  const [dniError, setDniError] = useState<string | null>(null);
  const [checking, setChecking] = useState(false);
  const [persona, setPersona] = useState<Persona | null>(null);
  // check-dni answers one generic 404 whether the DNI is unknown or already has
  // an account (no oracle): offer both ways out without telling which one applies.
  const [dniNoDisponible, setDniNoDisponible] = useState(false);

  // Step 2 state
  const [apiError, setApiError] = useState<string | null>(null);
  // Step 3: registration submitted — the account is created only when the
  // link e-mailed to this (masked) address is opened.
  const [sentTo, setSentTo] = useState<string | null>(null);
  // Masked hint of the e-mail on file (the user must type the full address).
  const emailHint = persona?.email_masked || null;

  const isStep2 = persona !== null && sentTo === null;

  const handleCheckDNI = async () => {
    if (checking) return;
    if (!/^\d{8}$/.test(dni)) {
      setDniError(t("sign-up-dni-invalido"));
      return;
    }
    setDniError(null);
    setDniNoDisponible(false);
    setChecking(true);
    try {
      const result = await authApi.checkDNI(dni);
      setPersona(result);
    } catch (err: unknown) {
      const e = err as { status?: number; message?: string };
      setDniNoDisponible(e.status === 404);
      setDniError(
        t(claveErrorAuth(err, "sign-up-dni-error", { 400: "sign-up-dni-invalido", 404: "sign-up-dni-no-disponible" })),
      );
    } finally {
      setChecking(false);
    }
  };

  const handleDNIKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter") {
      e.preventDefault();
      handleCheckDNI();
    }
  };

  const handleChangeDNI = () => {
    setPersona(null);
    setDniError(null);
    setDniNoDisponible(false);
    setApiError(null);
    formik.resetForm();
    setTimeout(() => dniRef.current?.focus(), 50);
  };

  const formik = useFormik({
    initialValues: { email: "" },
    validationSchema: detailsSchema,
    validateOnBlur: true,
    validateOnChange: false,
    onSubmit: async (values, { setSubmitting }) => {
      setApiError(null);
      try {
        // The backend answers 202 with the same message whether or not the
        // data matched (no oracle), so the UI always shows "check your e-mail".
        await authApi.register({ dni, email: values.email });
        setSentTo(emailHint ?? values.email);
      } catch (err: unknown) {
        setApiError(t(claveErrorAuth(err, "sign-up-error")));
      } finally {
        setSubmitting(false);
      }
    },
  });

  const err = (name: keyof typeof formik.values) => (formik.touched[name] ? formik.errors[name] : undefined);

  // ── Step 3: check your e-mail ────────────────────────────────────────
  if (sentTo !== null) {
    return (
      <TarjetaAuth centrado>
        <EnvelopeSimpleIcon weight="duotone" aria-hidden className="text-acento-superficie-texto size-10" />
        <header className="flex flex-col gap-2">
          <h1 className="text-titulo-pagina text-foreground text-balance">{t("sign-up-revisa-correo")}</h1>
          <p className="text-cuerpo text-foreground leading-relaxed">
            {t.rich("sign-up-enlace-enviado", {
              correo: sentTo,
              destacado: (texto) => <strong className="font-semibold">{texto}</strong>,
            })}
          </p>
          <p className="text-control text-muted-foreground leading-relaxed">{t("sign-up-enlace-vence")}</p>
        </header>
        <Link href="/auth/sign-in" className={BOTON_SECUNDARIO}>
          <ArrowLeftIcon weight="bold" className="size-4" />
          {t("auth-volver-sign-in")}
        </Link>
      </TarjetaAuth>
    );
  }

  // ── Step 1: DNI ──────────────────────────────────────────────────────────
  if (!isStep2 || !persona) {
    return (
      <TarjetaAuth
        titulo={t("sign-up-titulo")}
        descripcion={t("sign-up-descripcion")}
        pie={
          <>
            {t("sign-up-con-cuenta")}{" "}
            <Link href="/auth/sign-in" className={ENLACE}>
              {t("auth-iniciar-sesion")}
            </Link>
          </>
        }
      >
        <div className="flex flex-col gap-4">
          <div className="grid gap-1.5">
            <Campo id="dni" etiqueta={t("sign-up-dni")} ayuda={t("sign-up-dni-ayuda")} error={dniError}>
              <div className="relative">
                <Entrada
                  ref={dniRef}
                  id="dni"
                  placeholder="12345678"
                  inputMode="numeric"
                  autoComplete="off"
                  autoFocus
                  className="pr-9 tabular-nums"
                  value={dni}
                  invalido={!!dniError}
                  aria-describedby={dniError ? "dni-error" : "dni-ayuda"}
                  onChange={(e) => {
                    setDni(e.target.value.replace(/\D/g, "").slice(0, 8));
                    setDniError(null);
                    setDniNoDisponible(false);
                  }}
                  onKeyDown={handleDNIKeyDown}
                />
                {checking ? <Verificando /> : null}
              </div>
            </Campo>
            {dniNoDisponible ? (
              <p className="text-control text-muted-foreground flex flex-wrap gap-x-4 gap-y-1">
                <Link href="/auth/sign-in" className={ENLACE}>
                  {t("auth-iniciar-sesion")}
                </Link>
                <Link href="/auth/password-reset" className={ENLACE}>
                  {t("auth-recuperar-contrasena")}
                </Link>
              </p>
            ) : null}
          </div>

          <BotonAsync
            pendiente={checking}
            textoPendiente={t("sign-up-verificando")}
            disabled={dni.length !== 8}
            onClick={handleCheckDNI}
            icon={<ArrowRightIcon weight="bold" className="size-4" />}
            className={cn(BOTON_PRIMARIO, "mt-2 w-full")}
          >
            {t("sign-up-siguiente")}
          </BotonAsync>
        </div>
      </TarjetaAuth>
    );
  }

  // ── Step 2: identidad a la izquierda, confirmación del correo a la derecha ─
  // H122: no availability check for the e-mail (GET /auth/check-email is gone).
  const emailError = err("email");
  const emailAyuda = emailHint ? <>{t("sign-up-correo-pista", { correo: emailHint })}</> : undefined;

  return (
    <TarjetaAuth ancho="amplio" bare>
      <div className="flex min-h-0 flex-col sm:flex-row">
        {/* Panel izquierdo — identidad */}
        <aside className="border-border/60 bg-muted/60 flex flex-col items-center justify-center gap-4 border-b px-8 py-10 text-center sm:w-[280px] sm:shrink-0 sm:border-r sm:border-b-0">
          <span
            aria-hidden
            className="bg-acento-superficie text-titulo-pagina text-acento-superficie-texto flex size-16 items-center justify-center rounded-full"
          >
            {initials(persona)}
          </span>

          <div className="flex w-full min-w-0 flex-col gap-0.5">
            <p className="text-titulo text-foreground">{fullName(persona)}</p>
            {formik.values.email ? (
              <p className="text-etiqueta text-muted-foreground truncate" title={formik.values.email}>
                {formik.values.email}
              </p>
            ) : null}
          </div>

          <Chip>
            <IdentificationCardIcon className="size-3.5" />
            {t("sign-up-dni-chip", { dni })}
          </Chip>

          <button type="button" onClick={handleChangeDNI} className={cn(ENLACE, "text-control")}>
            {t("sign-up-cambiar-dni")}
          </button>
        </aside>

        {/* Panel derecho — formulario */}
        <div className="flex flex-1 flex-col gap-5 px-6 py-8 sm:px-10 sm:py-10">
          <header className="flex flex-col gap-1.5">
            <h1 className="text-titulo-pagina text-foreground">{t("sign-up-confirma-correo")}</h1>
            <p className="text-cuerpo text-muted-foreground">{t("sign-up-confirma-correo-descripcion")}</p>
          </header>

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

          <form onSubmit={formik.handleSubmit} className="flex flex-col gap-4" noValidate>
            <Campo id="email" etiqueta={t("auth-correo")} error={emailError} ayuda={emailAyuda}>
              <Entrada
                id="email"
                name="email"
                type="email"
                placeholder={t("auth-correo-placeholder")}
                autoComplete="email"
                autoFocus
                invalido={!!emailError}
                aria-describedby={emailError ? "email-error" : emailAyuda ? "email-ayuda" : undefined}
                value={formik.values.email}
                onChange={formik.handleChange}
                onBlur={formik.handleBlur}
              />
            </Campo>

            <BotonAsync
              type="submit"
              pendiente={formik.isSubmitting}
              textoPendiente={t("auth-enviando")}
              icon={<PaperPlaneTiltIcon className="size-4" />}
              className={cn(BOTON_PRIMARIO, "mt-1 w-full")}
            >
              {t("auth-enviar-enlace")}
            </BotonAsync>
            <AvisoLegal />
          </form>
        </div>
      </div>
    </TarjetaAuth>
  );
}
