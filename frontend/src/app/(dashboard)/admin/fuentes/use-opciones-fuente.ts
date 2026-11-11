"use client";

import { useCarga } from "@/hooks/use-carga";
import { catalogsApi } from "@/lib/catalogs/api";

/**
 * Profesiones y procesos activos para los filtros y formularios de fuentes. Un nombre de profesión repetido entre
 * instituciones se distingue con la institución.
 */
export function useOpcionesFuente(token: string | null | undefined) {
  const { datos } = useCarga(token ? "opciones-fuente" : null, async () => {
    const [especialidades, procesos] = await Promise.all([
      catalogsApi.listEspecialidades(token ?? "", { estado: "activo", limit: "all" }),
      catalogsApi.listProcesos(token ?? "", { estado: "activo", limit: "all" }),
    ]);
    const repetidos = new Set(especialidades.map((e) => e.nombre).filter((n, i, todos) => todos.indexOf(n) !== i));
    return {
      profesiones: especialidades.map((e) => ({
        id: e.id,
        nombre: repetidos.has(e.nombre) ? `${e.nombre} (${e.institucion})` : e.nombre,
      })),
      procesos: procesos.map((p) => ({ id: p.id, nombre: `${p.nombre} (${p.anio})` })),
    };
  });
  return { profesiones: datos?.profesiones ?? [], procesos: datos?.procesos ?? [] };
}
