"use client";

import { CargaPorClave, vistaDeCarga } from "./carga-por-clave";
import { useCallback, useEffect, useRef, useState, useSyncExternalStore } from "react";

/**
 * Enlace React de `CargaPorClave`: carga al montar y cada vez que cambia `clave` (con `null` no pide nada).
 * `cargar` puede cambiar en cada render; la carga se dispara solo por la clave.
 */
export function useCarga<T>(clave: string | null, cargar: () => Promise<T>) {
  const [carga] = useState(() => new CargaPorClave<T>());
  const estado = useSyncExternalStore(carga.suscribir, carga.obtenerEstado, carga.obtenerEstado);
  const cargarActual = useRef(cargar);
  useEffect(() => {
    cargarActual.current = cargar;
  });

  const recargar = useCallback(() => carga.cargar(clave, () => cargarActual.current()), [carga, clave]);

  useEffect(() => {
    recargar();
  }, [recargar]);

  const reemplazar = useCallback(
    (datos: T) => {
      if (clave) carga.reemplazar(clave, datos);
    },
    [carga, clave],
  );

  return { ...vistaDeCarga(estado, clave), recargar, reemplazar };
}
