"use client";

import { useCallback, useEffect, useState } from "react";

import { fetchMiTestimonio } from "@/lib/testimonios/api";

const OPINION_REMINDER_KEY = "opinion_reminder_dismissed";

/**
 * Invitación a dejar una opinión: pendiente mientras la persona no tenga testimonio y no haya descartado el aviso.
 * Es un aviso efímero del cliente, no una notificación del servidor: no aparece en el historial, solo en la campana.
 */
export function useRecordatorioOpinion(token: string | null | undefined) {
  const [pendiente, setPendiente] = useState(false);

  useEffect(() => {
    if (!token) return;
    let descartado = false;
    try {
      descartado = localStorage.getItem(OPINION_REMINDER_KEY) === "true";
    } catch {
      /* almacenamiento bloqueado: se muestra el recordatorio */
    }
    if (descartado) return;
    fetchMiTestimonio(token)
      .then((mio) => setPendiente(mio === null))
      .catch(() => {});
  }, [token]);

  const descartar = useCallback(() => {
    try {
      localStorage.setItem(OPINION_REMINDER_KEY, "true");
    } catch {
      /* sin almacenamiento: solo se oculta en esta sesión */
    }
    setPendiente(false);
  }, []);

  return { pendiente, descartar };
}
