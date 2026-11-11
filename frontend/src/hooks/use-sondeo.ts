"use client";

import { crearSondeo } from "./sondeo";
import { useEffect, useRef } from "react";

/** Mientras `activo`, ejecuta `tarea` cada `intervalo` ms (encadenado, ver `crearSondeo`); se detiene al desmontar. */
export function useSondeo(activo: boolean, tarea: () => Promise<unknown>, intervalo: number) {
  const tareaActual = useRef(tarea);
  const activoActual = useRef(activo);
  useEffect(() => {
    tareaActual.current = tarea;
    activoActual.current = activo;
  });

  useEffect(() => {
    if (!activo) return;
    const sondeo = crearSondeo({ tarea: () => tareaActual.current(), intervalo, seguir: () => activoActual.current });
    sondeo.iniciar();
    return () => sondeo.detener();
  }, [activo, intervalo]);
}
