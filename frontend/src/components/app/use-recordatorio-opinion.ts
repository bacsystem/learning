"use client";

import { useCallback, useEffect, useSyncExternalStore } from "react";

import { fetchMiTestimonio } from "@/lib/testimonios/api";

const OPINION_REMINDER_KEY = "opinion_reminder_dismissed";

export interface EstadoRecordatorio {
  /** `false` hasta saber si la invitación está pendiente (consulta de «mi testimonio» en curso). */
  resuelto: boolean;
  pendiente: boolean;
}

const SIN_RESOLVER: EstadoRecordatorio = { resuelto: false, pendiente: false };

// Un solo estado para toda la app: la campana y la página de notificaciones leen la misma invitación, así que descartarla
// en una se refleja en la otra sin volver a consultar.
let estado: EstadoRecordatorio = SIN_RESOLVER;
let tokenConsultado: string | null = null;
const oyentes = new Set<() => void>();

function publicar(nuevo: EstadoRecordatorio) {
  estado = nuevo;
  oyentes.forEach((oyente) => oyente());
}

function suscribir(oyente: () => void) {
  oyentes.add(oyente);
  return () => oyentes.delete(oyente);
}

/** Estado actual, para quien no es un componente (y para las pruebas). */
export const estadoRecordatorio = () => estado;

function estaDescartada(): boolean {
  try {
    return localStorage.getItem(OPINION_REMINDER_KEY) === "true";
  } catch {
    return false; // almacenamiento bloqueado: se muestra el recordatorio
  }
}

/** Consulta una vez por sesión si la invitación sigue pendiente (sin testimonio y sin haberla descartado). */
export function cargarRecordatorio(token: string): void {
  if (tokenConsultado === token) return;
  tokenConsultado = token;
  if (estaDescartada()) {
    publicar({ resuelto: true, pendiente: false });
    return;
  }
  publicar(SIN_RESOLVER);
  fetchMiTestimonio(token)
    .then((mio) => {
      if (tokenConsultado === token) publicar({ resuelto: true, pendiente: mio === null });
    })
    .catch(() => {
      if (tokenConsultado === token) publicar({ resuelto: true, pendiente: false });
    });
}

export function descartarRecordatorio(): void {
  try {
    localStorage.setItem(OPINION_REMINDER_KEY, "true");
  } catch {
    /* sin almacenamiento: solo se oculta en esta sesión */
  }
  publicar({ resuelto: true, pendiente: false });
}

/** Solo para las pruebas. */
export function reiniciarRecordatorio(): void {
  tokenConsultado = null;
  publicar(SIN_RESOLVER);
}

/**
 * Invitación a dejar una opinión. Es un aviso efímero del cliente, no una notificación del servidor: no aparece en el
 * historial, solo en la campana. `resuelto` indica que ya se sabe si está pendiente.
 */
export function useRecordatorioOpinion(token: string | null | undefined) {
  const actual = useSyncExternalStore(suscribir, estadoRecordatorio, () => SIN_RESOLVER);
  useEffect(() => {
    if (token) cargarRecordatorio(token);
  }, [token]);
  const descartar = useCallback(() => descartarRecordatorio(), []);
  return { ...actual, descartar };
}
