"use client";

import { useState } from "react";

import { CAMPO } from "@/lib/estilos";
import { formatearNumero } from "@/lib/formato";
import { cn } from "@/lib/utils";

const SIMBOLOS: Record<string, string> = { PEN: "S/", USD: "$", EUR: "€" };

/**
 * Importe con moneda: prefijo con el símbolo, selector de moneda a la derecha y formato `1,234.50` al salir del campo.
 * `valor` es número (o null si está vacío); no admite negativos.
 */
export function EntradaMonto({
  id,
  valor,
  onCambio,
  moneda = "PEN",
  monedas = ["PEN", "USD"],
  onMoneda,
  disabled,
  className,
}: {
  id: string;
  valor: number | null;
  onCambio: (v: number | null) => void;
  moneda?: string;
  monedas?: string[];
  onMoneda?: (m: string) => void;
  /** Las dos variantes miden h-control (altura única de control); se conserva por compatibilidad. Igual que StepperNumerico. */
  variante?: "filtro" | "campo";
  disabled?: boolean;
  className?: string;
}) {
  const [texto, setTexto] = useState(valor == null ? "" : formatearNumero(valor));
  const [editando, setEditando] = useState(false);
  const mostrado = editando ? texto : valor == null ? "" : formatearNumero(valor);

  return (
    <div className={cn("relative flex items-center", className)}>
      <span className="text-control text-muted-foreground pointer-events-none absolute left-3 font-mono">
        {SIMBOLOS[moneda] ?? moneda}
      </span>
      <input
        id={id}
        inputMode="decimal"
        value={mostrado}
        disabled={disabled}
        onFocus={() => {
          setTexto(valor == null ? "" : String(valor));
          setEditando(true);
        }}
        onChange={(e) => {
          const limpio = e.target.value.replace(/[^\d.]/g, "");
          setTexto(limpio);
          const n = Number.parseFloat(limpio);
          onCambio(Number.isFinite(n) ? Math.round(n * 100) / 100 : null);
        }}
        onBlur={() => setEditando(false)}
        className={cn(CAMPO, "pr-20 pl-9 text-right font-mono tabular-nums")}
      />
      {monedas.length > 1 && onMoneda ? (
        <select
          aria-label="Moneda"
          value={moneda}
          disabled={disabled}
          onChange={(e) => onMoneda(e.target.value)}
          className="border-border bg-card text-etiqueta text-foreground focus-visible:ring-ring/30 absolute right-1.5 h-7 rounded-md border px-1.5 font-mono font-medium shadow-xs outline-none focus-visible:ring-2"
        >
          {monedas.map((m) => (
            <option key={m} value={m}>
              {m}
            </option>
          ))}
        </select>
      ) : (
        <span className="text-etiqueta text-muted-foreground absolute right-3 font-mono">{moneda}</span>
      )}
    </div>
  );
}
