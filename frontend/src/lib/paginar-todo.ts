/** Máximo `per_page` que aceptan los historiales del backend (por encima lo cambia a 20). */
export const POR_PAGINA_MAXIMO = 100;

/**
 * Recorre todas las páginas de un listado hasta `total` y devuelve todos sus elementos: las métricas (simulacros
 * realizados, nota promedio, prácticas completadas…) se calculan sobre el historial completo, no sobre una página.
 * Si una página falla, la promesa se rechaza: quien llama decide cómo mostrar el error.
 */
export async function cargarTodo<T>(
  pedir: (q: { page: number; per_page: number }) => Promise<{ data: T[]; total: number }>,
): Promise<T[]> {
  const todos: T[] = [];
  for (let page = 1; ; page++) {
    const { data, total } = await pedir({ page, per_page: POR_PAGINA_MAXIMO });
    todos.push(...data);
    if (data.length === 0 || todos.length >= total) return todos;
  }
}
