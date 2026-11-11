"use client";

import { useCallback, useEffect, useRef, useState } from "react";

type Resultado<T> = { clave: string; datos: T | null; error: unknown };

/**
 * Carga asociada a una `clave` (p. ej. el par proceso-especialidad). Solo se aplica la respuesta de la última carga, y
 * un resultado de otra clave nunca se devuelve: al cambiar de par no quedan a la vista (ni editables) los datos del
 * anterior, lo que evitaría guardar una configuración sobre el par equivocado. Con `clave = null` no pide nada.
 */
export function useCarga<T>(clave: string | null, cargar: () => Promise<T>) {
  const [resultado, setResultado] = useState<Resultado<T>>({ clave: "", datos: null, error: null });
  const [cargando, setCargando] = useState(false);
  const ultima = useRef(0);
  // La función cambia en cada render del llamador; la carga se dispara solo por la clave.
  const cargarActual = useRef(cargar);
  useEffect(() => {
    cargarActual.current = cargar;
  });

  const recargar = useCallback(async () => {
    const id = ++ultima.current;
    if (!clave) {
      setCargando(false);
      return;
    }
    setCargando(true);
    try {
      const datos = await cargarActual.current();
      if (id === ultima.current) setResultado({ clave, datos, error: null });
    } catch (error) {
      if (id === ultima.current) setResultado({ clave, datos: null, error });
    } finally {
      if (id === ultima.current) setCargando(false);
    }
  }, [clave]);

  useEffect(() => {
    recargar();
  }, [recargar]);

  /** Sustituye los datos de la clave actual sin pedirlos (p. ej. con la respuesta de un guardado). */
  const reemplazar = useCallback(
    (datos: T) => {
      if (clave) setResultado({ clave, datos, error: null });
    },
    [clave],
  );

  const vigente = Boolean(clave) && resultado.clave === clave;
  return {
    datos: vigente ? resultado.datos : null,
    error: vigente ? resultado.error : null,
    cargando: Boolean(clave) && (cargando || !vigente),
    recargar,
    reemplazar,
  };
}
