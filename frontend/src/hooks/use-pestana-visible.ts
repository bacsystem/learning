"use client";

import { escucharVisibilidad, pestanaVisible } from "./visibilidad";
import { useSyncExternalStore } from "react";

const visibleEnServidor = () => true;

/** `true` mientras la pestaña está visible; cambia con `visibilitychange`. */
export function usePestanaVisible(): boolean {
  return useSyncExternalStore(escucharVisibilidad, () => pestanaVisible(), visibleEnServidor);
}
