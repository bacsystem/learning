/**
 * Marcas del eje Y de un gráfico: de 0 al primer múltiplo del paso que cubre `max`, con unos `pasos` tramos.
 * Con `enteros` (conteos: prácticas, comprobantes) el paso es un entero ≥ 1, así que el eje nunca muestra 0,25 ni 0,5.
 */
export function escalaEje(max: number, pasos = 4, enteros = false): number[] {
  if (max <= 0) return [0, 1];
  const bruto = enteros ? Math.max(1, max / pasos) : max / pasos;
  const mag = 10 ** Math.floor(Math.log10(bruto));
  const paso =
    [1, 2, 2.5, 5, 10].map((m) => m * mag).find((p) => p >= bruto && (!enteros || Number.isInteger(p))) ??
    Math.ceil(bruto);
  const tope = Math.ceil(max / paso) * paso;
  return Array.from({ length: Math.round(tope / paso) + 1 }, (_, i) => i * paso);
}
