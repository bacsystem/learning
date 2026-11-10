"use client";

import { domAnimation, domMax, LazyMotion, MotionConfig } from "motion/react";
import type { ReactNode } from "react";

/**
 * Habilita Motion en la app. `domMax` y no `domAnimation`: trae las animaciones de layout que necesita `layoutId`
 * (el fondo del ítem activo del sidebar). `strict` hace fallar en desarrollo cualquier `motion.*`: los componentes usan `m.*`,
 * que no carga el bundle completo. `reducedMotion="user"` respeta «reducir movimiento» del sistema.
 *
 * Sin un `LazyMotion` por encima, un `m.*` se queda congelado en su estado `initial` (un gráfico sin barras, una barra
 * masiva que nunca aparece). Por eso los componentes animados se envuelven además en `MovimientoLocal`: funcionan aunque
 * la app que los copió no monte este proveedor. Montarlo sigue siendo lo recomendado: aporta `layoutId` y «reducir movimiento».
 */
export function ProveedorMovimiento({ children }: { children: ReactNode }) {
  return (
    <LazyMotion features={domMax} strict>
      <MotionConfig reducedMotion="user">{children}</MotionConfig>
    </LazyMotion>
  );
}

/**
 * Carga las animaciones básicas de Motion (sin layout) para un componente animado, por si la app no monta
 * `ProveedorMovimiento`. Dentro del proveedor no cambia nada. No renderiza ningún elemento: sirve también dentro de un `<svg>`.
 */
export function MovimientoLocal({ children }: { children: ReactNode }) {
  return <LazyMotion features={domAnimation}>{children}</LazyMotion>;
}
