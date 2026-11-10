"use client";

import type { ButtonHTMLAttributes, ReactNode } from "react";

import { Spinner } from "@/components/feedback/spinner";

/**
 * Envoltorio de un botón nativo que muestra `Spinner` y deshabilita mientras `pendiente`.
 * Agnóstico de receta: dale la clase de `BOTON_PRIMARIO` / `BOTON_SECUNDARIO` / `ACCION_PRINCIPAL` en `className`,
 * igual que a un `<button>` normal (ya traen `inline-flex items-center gap-1.5`, así que el spinner encaja solo).
 * `icon` es lo que se ve en el estado normal; se sustituye por el spinner mientras está pendiente.
 *
 * `<BotonAsync pendiente={enviando} icon={<SendIcon className="size-4" />} textoPendiente="Enviando…" className={BOTON_PRIMARIO} onClick={enviar}>Enviar</BotonAsync>`
 */
export function BotonAsync({
  pendiente,
  children,
  textoPendiente,
  icon,
  disabled,
  className,
  ...props
}: {
  pendiente: boolean;
  children: ReactNode;
  /** Texto a mostrar mientras `pendiente`; por defecto mantiene `children`. */
  textoPendiente?: ReactNode;
  /** Icono del estado normal (se reemplaza por el spinner mientras `pendiente`). */
  icon?: ReactNode;
  className?: string;
} & Omit<ButtonHTMLAttributes<HTMLButtonElement>, "children">) {
  return (
    <button type="button" disabled={disabled || pendiente} aria-busy={pendiente} className={className} {...props}>
      {pendiente ? <Spinner tamano="sm" className="shrink-0" /> : icon}
      {pendiente ? (textoPendiente ?? children) : children}
    </button>
  );
}
