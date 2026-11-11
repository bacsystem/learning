"use client";

import { useTranslations } from "next-intl";
import { useState } from "react";

import { BotonAsync } from "@/components/patrones/boton-async";
import { ACCION_SECUNDARIA } from "@/lib/estilos";

/**
 * «Reintentar» de una alerta de error: `BotonAsync` de khipu con su indicador mientras dura la promesa que devuelve
 * `onReintentar`, para que el usuario vea que la recarga está en curso aunque la alerta siga a la vista.
 */
export function BotonReintentar({ onReintentar }: { onReintentar: () => Promise<unknown> }) {
  const t = useTranslations("dashboard");
  const [pendiente, setPendiente] = useState(false);

  async function reintentar() {
    setPendiente(true);
    try {
      await onReintentar();
    } finally {
      setPendiente(false);
    }
  }

  return (
    <BotonAsync
      type="button"
      pendiente={pendiente}
      textoPendiente={t("comun-reintentando")}
      onClick={reintentar}
      className={ACCION_SECUNDARIA}
    >
      {t("comun-reintentar")}
    </BotonAsync>
  );
}
