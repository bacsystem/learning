export const POR_PAGINA_DEFECTO = 10;
export const OPCIONES_POR_PAGINA = [10, 20, 50] as const;

/** Normaliza un valor (p. ej. de la URL) a una de las opciones permitidas. */
export function porPaginaValido(valor: string | number | null | undefined): number {
  const n = Number(valor);
  return (OPCIONES_POR_PAGINA as readonly number[]).includes(n) ? n : POR_PAGINA_DEFECTO;
}

/** Números de página a mostrar en el paginador: siempre las 2 primeras, las 2 últimas y las vecinas de la actual, con "…" entre huecos. */
export function paginasVisibles(actual: number, ultima: number): (number | "…")[] {
  if (ultima <= 7) return Array.from({ length: ultima }, (_, i) => i + 1);
  const cerca = new Set(
    [1, 2, actual - 1, actual, actual + 1, ultima - 1, ultima].filter((p) => p >= 1 && p <= ultima),
  );
  const resultado: (number | "…")[] = [];
  for (let p = 1; p <= ultima; p++) {
    if (cerca.has(p)) resultado.push(p);
    else if (resultado[resultado.length - 1] !== "…") resultado.push("…");
  }
  return resultado;
}
