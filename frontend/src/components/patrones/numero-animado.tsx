"use client";

import { animate, useReducedMotion } from "motion/react";
import { useEffect, useRef } from "react";

import { CURVA_SALIDA, DURACION, type OpcionesContador, textoContador } from "@/lib/movimiento";

/**
 * Número que cuenta desde 0 hasta `valor` al montarse. El HTML del servidor ya trae el valor final (sin desajuste de
 * hidratación) y con «reducir movimiento» no cuenta. Props serializables: se usa también desde un Server Component,
 * como hijo de `Metrica` o como `valor` de `Kpi`. El lector de pantalla oye solo el valor final.
 */
export function NumeroAnimado({
  valor,
  decimales,
  prefijo,
  sufijo,
  className,
}: { valor: number; className?: string } & OpcionesContador) {
  const ref = useRef<HTMLSpanElement>(null);
  const reducir = useReducedMotion();
  const final = textoContador(valor, { decimales, prefijo, sufijo });

  useEffect(() => {
    const nodo = ref.current;
    if (!nodo || reducir) return;
    const fin = textoContador(valor, { decimales, prefijo, sufijo });
    const control = animate(0, valor, {
      duration: DURACION.contador,
      ease: CURVA_SALIDA,
      onUpdate: (v) => {
        nodo.textContent = textoContador(v, { decimales, prefijo, sufijo });
      },
    });
    return () => {
      control.stop();
      // React no repinta un texto que cree igual: si se corta a mitad de cuenta, se deja el valor final a mano.
      nodo.textContent = fin;
    };
  }, [valor, decimales, prefijo, sufijo, reducir]);

  return (
    <span className={className}>
      <span className="sr-only">{final}</span>
      <span ref={ref} aria-hidden className="tabular-nums">
        {final}
      </span>
    </span>
  );
}
