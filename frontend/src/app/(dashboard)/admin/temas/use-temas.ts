"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import { cargarTodo } from "@/lib/paginar-todo";
import { type EstadoTema, type Tema, temasApi } from "@/lib/temas/api";

export interface FiltroTemas {
  asignaturaId: number;
  especialidadId?: number;
  estado?: EstadoTema;
}

/**
 * Todos los temas de una asignatura (recorre las páginas de T1), para que la tabla pagine, busque y cuente en el
 * cliente. Sin asignatura no pide nada: T1 la exige. Solo se aplica la respuesta de la última carga.
 */
export function useTemas(token: string | null | undefined, filtro: FiltroTemas | null) {
  const [temas, setTemas] = useState<Tema[]>([]);
  const [cargando, setCargando] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const ultima = useRef(0);

  const asignaturaId = filtro?.asignaturaId;
  const especialidadId = filtro?.especialidadId;
  const estado = filtro?.estado;

  const recargar = useCallback(async () => {
    const id = ++ultima.current;
    if (!token || !asignaturaId) {
      setTemas([]);
      setCargando(false);
      setError(null);
      return;
    }
    setCargando(true);
    setError(null);
    try {
      const data = await cargarTodo((pagina) =>
        temasApi.listTemas(token, { asignatura_id: asignaturaId, especialidad_id: especialidadId, estado, ...pagina }),
      );
      if (id === ultima.current) setTemas(data);
    } catch (e) {
      if (id === ultima.current) setError(e);
    } finally {
      if (id === ultima.current) setCargando(false);
    }
  }, [token, asignaturaId, especialidadId, estado]);

  useEffect(() => {
    recargar();
  }, [recargar]);

  return { temas, cargando, error, recargar };
}
