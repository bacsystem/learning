/**
 * Fuente única del movimiento con Motion (docs/design-system.md «Movimiento»). Los primitivos de base-ui animan con CSS
 * usando los mismos valores, declarados en src/styles/ (tokens.css y theme.css): `--ease-salida` = CURVA_SALIDA y `--duracion-*` = DURACION.
 * Si cambias uno, cambia el otro. Los resortes están críticamente amortiguados (ζ ≥ 1): llegan sin rebote.
 */

/** Curva de salida (rápida al inicio, suave al final). Igual a `--ease-salida`. */
export const CURVA_SALIDA: [number, number, number, number] = [0.16, 1, 0.3, 1];

/** Duraciones en segundos. `rapida`/`media`/`lenta` = `--duracion-*` de src/styles/tokens.css. */
export const DURACION = { rapida: 0.12, media: 0.2, lenta: 0.32, trazo: 0.6, contador: 0.8 } as const;

/** Resorte de indicadores y piezas que se desplazan (ζ = 1). */
export const RESORTE = { type: "spring", stiffness: 500, damping: 40, mass: 0.8 } as const;

/** Resorte de avances de valor: barras de gráfico, anillo de progreso (ζ ≈ 1.01). */
export const RESORTE_SUAVE = { type: "spring", stiffness: 300, damping: 35, mass: 1 } as const;

export type OpcionesContador = { decimales?: number; prefijo?: string; sufijo?: string };

/** Texto de un número animado: separador de miles en-US (como lib/formato.ts), decimales fijos, prefijo y sufijo. */
export function textoContador(
  valor: number,
  { decimales = 0, prefijo = "", sufijo = "" }: OpcionesContador = {},
): string {
  const numero = valor.toLocaleString("en-US", { minimumFractionDigits: decimales, maximumFractionDigits: decimales });
  return `${prefijo}${numero}${sufijo}`;
}
