"use client";

import { useCarga } from "@/hooks/use-carga";
import { cargarTodo } from "@/lib/paginar-todo";
import { type EstadoTema, type Tema, temasApi } from "@/lib/serums-ia/temas/api";

export interface FiltroTemas {
  asignaturaId: number;
  especialidadId?: number;
  estado?: EstadoTema;
}

/**
 * Todos los temas de una asignatura (recorre las páginas de T1), para que la tabla pagine, busque y cuente en el
 * cliente. Sin asignatura no pide nada: T1 la exige. Al cambiar de filtro no quedan a la vista (ni operables) las
 * filas del anterior (ver `useCarga`).
 */
export function useTemas(token: string | null | undefined, filtro: FiltroTemas | null) {
  const asignaturaId = filtro?.asignaturaId;
  const especialidadId = filtro?.especialidadId;
  const estado = filtro?.estado;
  const clave = token && asignaturaId ? `${asignaturaId}|${especialidadId ?? ""}|${estado ?? ""}` : null;

  const { datos, error, cargando, recargar } = useCarga<Tema[]>(clave, () =>
    cargarTodo((pagina) =>
      temasApi.listTemas(token ?? "", {
        asignatura_id: asignaturaId ?? 0,
        especialidad_id: especialidadId,
        estado,
        ...pagina,
      }),
    ),
  );

  return { temas: datos ?? [], error, cargando, recargar };
}
