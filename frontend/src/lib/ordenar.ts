/** Orden de una tabla por columna: `null` es el orden en que llegan las filas. */
export type OrdenTabla = { id: string; dir: "asc" | "desc" } | null;

/** Lo que el orden necesita de una columna: su `id` y, si no es `fila[id]`, cómo leer su valor. */
type ColumnaOrdenable<T> = { id: string; valor?: (fila: T) => string | number | null | undefined };

/**
 * Ordena las filas como lo hace `TablaDatos` (números como números, texto con `localeCompare` en castellano, vacíos
 * primero en ascendente). Quien exporta o imprime lo reutiliza para respetar el orden que se ve en pantalla.
 * Sin orden, o si su columna no está en `columnas`, devuelve las filas tal cual.
 */
export function ordenarFilas<T>(filas: T[], columnas: ColumnaOrdenable<T>[], orden: OrdenTabla): T[] {
  if (!orden) return filas;
  const col = columnas.find((c) => c.id === orden.id);
  if (!col) return filas;
  const v = col.valor ?? ((f: T) => (f as Record<string, unknown>)[col.id] as string | number | null | undefined);
  return [...filas].sort((a, b) => {
    const x = v(a) ?? "";
    const y = v(b) ?? "";
    const r = typeof x === "number" && typeof y === "number" ? x - y : String(x).localeCompare(String(y), "es");
    return orden.dir === "asc" ? r : -r;
  });
}
