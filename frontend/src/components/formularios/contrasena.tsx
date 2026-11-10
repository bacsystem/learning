"use client";

import { useMemo, useState } from "react";

import { CAMPO } from "@/lib/estilos";
import { cn } from "@/lib/utils";
import { Meter } from "@base-ui/react/meter";
import { EyeIcon, EyeSlashIcon } from "@phosphor-icons/react";

type TonoFuerza = "destructive" | "warning" | "success";
const COLOR_TONO: Record<TonoFuerza, string> = {
  destructive: "bg-destructive",
  warning: "bg-warning-solid",
  success: "bg-success-solid",
};

/** Heurística simple (longitud + variedad de caracteres) solo para dar una pista visual; valida de verdad en el backend. */
export type EtiquetasFuerza = { debil: string; media: string; fuerte: string };
const FUERZA_ES: EtiquetasFuerza = { debil: "Débil", media: "Media", fuerte: "Fuerte" };

function calcularFuerza(valor: string, e: EtiquetasFuerza): { puntaje: number; etiqueta: string; tono: TonoFuerza } {
  let p = 0;
  if (valor.length >= 8) p++;
  if (valor.length >= 12) p++;
  if (/[a-z]/.test(valor) && /[A-Z]/.test(valor)) p++;
  if (/\d/.test(valor)) p++;
  if (/[^A-Za-z0-9]/.test(valor)) p++;
  const puntaje = Math.round((p / 5) * 100);
  if (p <= 1) return { puntaje, etiqueta: e.debil, tono: "destructive" };
  if (p <= 3) return { puntaje, etiqueta: e.media, tono: "warning" };
  return { puntaje, etiqueta: e.fuerte, tono: "success" };
}

/**
 * Input de contraseña: botón de mostrar/ocultar y, con `conFuerza`, un medidor debajo (útil en registro; en
 * login sobra). Receta `CAMPO`, igual que `Entrada`.
 */
export function Contrasena({
  id,
  valor,
  onCambio,
  onBlur,
  conFuerza = false,
  placeholder,
  invalido,
  disabled,
  autoComplete,
  etiquetaMostrar = "Mostrar contraseña",
  etiquetaOcultar = "Ocultar contraseña",
  describidoPor,
  etiquetasFuerza = FUERZA_ES,
  className,
}: {
  id: string;
  valor: string;
  onCambio: (v: string) => void;
  /** Al salir del campo (p. ej. marcarlo como tocado). Hacer clic en el ojo no lo dispara (no le quita el foco); pasar al ojo con Tab sí, como cualquier salida del campo. */
  onBlur?: () => void;
  conFuerza?: boolean;
  placeholder?: string;
  invalido?: boolean;
  disabled?: boolean;
  autoComplete?: string;
  /** Nombre accesible del botón del ojo, para traducirlo (por defecto en castellano). */
  etiquetaMostrar?: string;
  etiquetaOcultar?: string;
  /** `aria-describedby` del campo: el id de su ayuda o de su error. */
  describidoPor?: string;
  /** Textos del medidor de `conFuerza`, para traducirlo. */
  etiquetasFuerza?: EtiquetasFuerza;
  className?: string;
}) {
  const [visible, setVisible] = useState(false);
  const fuerza = useMemo(() => calcularFuerza(valor, etiquetasFuerza), [valor, etiquetasFuerza]);

  return (
    <div className={cn("grid gap-1.5", className)}>
      <div className="relative flex items-center">
        <input
          id={id}
          type={visible ? "text" : "password"}
          value={valor}
          onChange={(e) => onCambio(e.target.value)}
          onBlur={onBlur}
          placeholder={placeholder}
          disabled={disabled}
          autoComplete={autoComplete}
          aria-invalid={invalido || undefined}
          aria-describedby={describidoPor}
          className={cn(
            CAMPO,
            "pr-9",
            invalido && "border-destructive focus:border-destructive focus:ring-destructive/20",
          )}
        />
        <button
          type="button"
          // Evita que el clic o el toque le quiten el foco al campo (y disparen su onBlur / validación); con Tab el botón sigue siendo enfocable.
          onMouseDown={(e) => e.preventDefault()}
          onClick={() => setVisible((v) => !v)}
          disabled={disabled}
          aria-label={visible ? etiquetaOcultar : etiquetaMostrar}
          className="text-muted-foreground/70 hover:bg-secondary hover:text-foreground absolute right-2.5 rounded p-0.5 transition-colors disabled:pointer-events-none disabled:opacity-60"
        >
          {visible ? <EyeSlashIcon className="size-4" /> : <EyeIcon className="size-4" />}
        </button>
      </div>
      {conFuerza && valor ? (
        <Meter.Root value={fuerza.puntaje} getAriaValueText={() => fuerza.etiqueta} className="flex items-center gap-2">
          <Meter.Track className="bg-secondary h-1 flex-1 overflow-hidden rounded-full">
            <Meter.Indicator
              className={cn(
                "block h-full rounded-full transition-[width] duration-(--duracion-lenta)",
                COLOR_TONO[fuerza.tono],
              )}
            />
          </Meter.Track>
          <span className="text-etiqueta text-muted-foreground font-medium">{fuerza.etiqueta}</span>
        </Meter.Root>
      ) : null}
    </div>
  );
}
