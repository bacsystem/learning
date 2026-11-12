"use client";

import { useCarga } from "@/hooks/use-carga";
import { catalogsApi } from "@/lib/catalogs/api";

/**
 * Procesos y profesiones activos para confirmar la importación, y las áreas del proceso elegido (el área inicial tiene
 * que estar asignada a su proceso, docs/api/…balotario.md §4.1 paso 10). Cada profesión conserva su institución: el
 * servidor exige que sea la del proceso.
 */
export function useOpcionesImportacion(token: string | null | undefined, procesoId: number | null) {
  const base = useCarga(token ? "opciones-importacion" : null, async () => {
    const [procesos, especialidades] = await Promise.all([
      catalogsApi.listProcesos(token ?? "", { estado: "activo", limit: "all" }),
      catalogsApi.listEspecialidades(token ?? "", { estado: "activo", limit: "all" }),
    ]);
    return { procesos, especialidades };
  });
  const areas = useCarga(token && procesoId ? `areas-importacion-${procesoId}` : null, () =>
    catalogsApi.listProcesoAsignaturas(token ?? "", procesoId ?? 0),
  );
  return {
    procesos: base.datos?.procesos ?? [],
    especialidades: base.datos?.especialidades ?? [],
    areas: areas.datos ?? [],
    cargandoOpciones: base.cargando,
    cargandoAreas: areas.cargando,
    errorOpciones: Boolean(base.error || areas.error),
  };
}
