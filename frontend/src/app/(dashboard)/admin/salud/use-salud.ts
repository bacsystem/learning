"use client";

import { useCallback, useEffect, useState, useSyncExternalStore } from "react";

import { usePestanaVisible } from "@/hooks/use-pestana-visible";
import { useSondeo } from "@/hooks/use-sondeo";
import { saludApi } from "@/lib/salud/api";
import { ConsultaSalud, faseSalud } from "@/lib/salud/consulta";
import { normalizarSalud, REFRESCO_SALUD_MS } from "@/lib/salud/reglas";

/**
 * Estado de salud de la pantalla (PLT-02 regla 4): consulta al montar y al volver a la pestaña, cada 30 s solo
 * mientras está visible (en segundo plano no se pide nada) y a mano con `actualizar`.
 */
export function useSalud(token: string | null) {
  const [consulta] = useState(() => new ConsultaSalud());
  const estado = useSyncExternalStore(consulta.suscribir, consulta.obtenerEstado, consulta.obtenerEstado);
  const visible = usePestanaVisible();

  const actualizar = useCallback(
    () =>
      token ? consulta.consultar(async () => normalizarSalud(await saludApi.consultar(token))) : Promise.resolve(),
    [consulta, token],
  );

  useEffect(() => {
    if (visible) actualizar();
  }, [visible, actualizar]);

  useSondeo(visible && token !== null, actualizar, REFRESCO_SALUD_MS);

  return { ...estado, fase: faseSalud(estado), actualizar };
}
