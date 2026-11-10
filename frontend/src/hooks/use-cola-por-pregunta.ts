"use client";

import { ColaPorPregunta, EstadoCola, ModoCola } from "./cola-por-pregunta";
import { useState, useSyncExternalStore } from "react";

/** Thin React binding of ColaPorPregunta (H37): one queue per page instance. */
export function useColaPorPregunta(modo: ModoCola): { cola: ColaPorPregunta } & EstadoCola {
  const [cola] = useState(() => new ColaPorPregunta(modo));
  const estado = useSyncExternalStore(cola.suscribir, cola.obtenerEstado, cola.obtenerEstado);
  return { cola, ...estado };
}
