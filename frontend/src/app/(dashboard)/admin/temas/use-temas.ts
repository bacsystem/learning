"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import { cargarTodo } from "@/lib/paginar-todo";
import { type EstadoTema, type Tema, temasApi } from "@/lib/temas/api";

export interface FiltroTemas {
  asignaturaId: number;
  especialidadId?: number;
  estado?: EstadoTema;
}

/** Resultado de una carga, con la clave del filtro que lo produjo. */
type Carga = { clave: string; temas: Tema[]; error: unknown };

const SIN_CARGA: Carga = { clave: "", temas: [], error: null };

/**
 * Todos los temas de una asignatura (recorre las páginas de T1), para que la tabla pagine, busque y cuente en el
 * cliente. Sin asignatura no pide nada: T1 la exige. Solo se aplica la respuesta de la última carga, y el resultado de
 * otro filtro nunca se devuelve: al cambiar de asignatura no quedan a la vista (ni operables) las filas de la anterior.
 */
export function useTemas(token: string | null | undefined, filtro: FiltroTemas | null) {
  const [carga, setCarga] = useState<Carga>(SIN_CARGA);
  const [cargando, setCargando] = useState(false);
  const ultima = useRef(0);

  const asignaturaId = filtro?.asignaturaId;
  const especialidadId = filtro?.especialidadId;
  const estado = filtro?.estado;
  const clave = asignaturaId ? `${asignaturaId}|${especialidadId ?? ""}|${estado ?? ""}` : "";

  const recargar = useCallback(async () => {
    const id = ++ultima.current;
    if (!token || !asignaturaId) {
      setCargando(false);
      return;
    }
    setCargando(true);
    try {
      const temas = await cargarTodo((pagina) =>
        temasApi.listTemas(token, { asignatura_id: asignaturaId, especialidad_id: especialidadId, estado, ...pagina }),
      );
      if (id === ultima.current) setCarga({ clave, temas, error: null });
    } catch (error) {
      if (id === ultima.current) setCarga({ clave, temas: [], error });
    } finally {
      if (id === ultima.current) setCargando(false);
    }
  }, [token, asignaturaId, especialidadId, estado, clave]);

  useEffect(() => {
    recargar();
  }, [recargar]);

  const vigente = Boolean(clave) && carga.clave === clave;
  return {
    temas: vigente ? carga.temas : [],
    error: vigente ? carga.error : null,
    // Hasta que llega la carga del filtro actual se considera cargando (evita un parpadeo del estado vacío).
    cargando: Boolean(clave) && (cargando || !vigente),
    recargar,
  };
}
