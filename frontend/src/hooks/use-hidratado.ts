"use client";

import { useSyncExternalStore } from "react";

const sinSuscripcion = () => () => {};
const enCliente = () => true;
const enServidor = () => false;

/**
 * `false` en el servidor y durante la hidratación, `true` después. Para lo que solo se conoce en el cliente (la
 * sesión de `localStorage`): renderizarlo antes desajusta la hidratación (React #418).
 */
export function useHidratado(): boolean {
  return useSyncExternalStore(sinSuscripcion, enCliente, enServidor);
}
