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

interface Almacen extends EstadoRecordatorio {
  /** Sesión a la que pertenece el estado: el de otra persona nunca se muestra. */
  token: string | null;
}

// Un solo estado para toda la app: la campana y la página de notificaciones leen la misma invitación, así que descartarla
// en una se refleja en la otra sin volver a consultar.
let almacen: Almacen = { token: null, ...SIN_RESOLVER };
const oyentes = new Set<() => void>();

function publicar(nuevo: Almacen) {
  almacen = nuevo;
  oyentes.forEach((oyente) => oyente());
}

export function suscribirRecordatorio(oyente: () => void) {
  oyentes.add(oyente);
  return () => oyentes.delete(oyente);
}

/** Estado del almacén, tal cual (para `useSyncExternalStore` y las pruebas). */
export const almacenRecordatorio = () => almacen;

/** Lo que ve la sesión `token`: el estado propio o, si el almacén es de otra sesión (o no hay sesión), «sin resolver». */
export function recordatorioPara(token: string | null | undefined, actual: Almacen = almacen): EstadoRecordatorio {
  return token && actual.token === token ? { resuelto: actual.resuelto, pendiente: actual.pendiente } : SIN_RESOLVER;
}

function estaDescartada(): boolean {
  try {
    return localStorage.getItem(OPINION_REMINDER_KEY) === "true";
  } catch {
    return false; // almacenamiento bloqueado: se muestra el recordatorio
  }
}

/** Consulta una vez por sesión si la invitación sigue pendiente (sin testimonio y sin haberla descartado). */
export function cargarRecordatorio(token: string): void {
  if (almacen.token === token) return;
  if (estaDescartada()) {
    publicar({ token, resuelto: true, pendiente: false });
    return;
  }
  publicar({ token, ...SIN_RESOLVER });
  // Una respuesta tardía de otra sesión no pisa a la actual.
  const siSigueVigente = (estado: EstadoRecordatorio) => {
    if (almacen.token === token) publicar({ token, ...estado });
  };
  fetchMiTestimonio(token)
    .then((mio) => siSigueVigente({ resuelto: true, pendiente: mio === null }))
    .catch(() => siSigueVigente({ resuelto: true, pendiente: false }));
}

/** La invitación ya no está pendiente: se descartó o la persona ya opinó. */
function resolverSinPendiente(): void {
  publicar({ ...almacen, resuelto: true, pendiente: false });
}

export function descartarRecordatorio(): void {
  try {
    localStorage.setItem(OPINION_REMINDER_KEY, "true");
  } catch {
    /* sin almacenamiento: solo se oculta en esta sesión */
  }
  resolverSinPendiente();
}

/** La persona envió su opinión: la invitación se resuelve sola, sin tocar lo que se guardó al descartar. */
export const marcarOpinionEnviada = resolverSinPendiente;

/** Solo para las pruebas. */
export function reiniciarRecordatorio(): void {
  publicar({ token: null, ...SIN_RESOLVER });
}

/**
 * Invitación a dejar una opinión. Es un aviso efímero del cliente, no una notificación del servidor: no aparece en el
 * historial, solo en la campana. `resuelto` indica que ya se sabe si está pendiente.
 */
export function useRecordatorioOpinion(token: string | null | undefined) {
  const actual = useSyncExternalStore(suscribirRecordatorio, almacenRecordatorio, almacenRecordatorio);
  useEffect(() => {
    if (token) cargarRecordatorio(token);
  }, [token]);
  const descartar = useCallback(() => descartarRecordatorio(), []);
  return { ...recordatorioPara(token, actual), descartar };
}
