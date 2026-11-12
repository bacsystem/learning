import type { CambioConfiguracionIa, ConfiguracionIa, GastoIa } from "./api";
import * as yup from "yup";

/** Rangos de la configuración (D19, contrato §3.3). El servidor los vuelve a aplicar y manda. */
export const RANGOS = {
  limite_usuario_dia: { min: 0, max: 50 },
  limite_usuario_mes: { min: 0, max: 1000 },
  limite_admin_dia: { min: 1, max: 1000 },
  presupuesto_mensual_usd: { min: 1, max: 1000 },
  max_preguntas_solicitud: { min: 1, max: 50 },
  lote_clasificacion: { min: 1, max: 50 },
} as const satisfies Record<keyof CambioConfiguracionIa, { min: number; max: number }>;

export type CampoConfiguracion = keyof typeof RANGOS;
export const CAMPOS_CONFIGURACION = Object.keys(RANGOS) as CampoConfiguracion[];

/** Todos los campos son enteros salvo el presupuesto, que admite hasta 2 decimales. */
const esEntero = (campo: CampoConfiguracion): boolean => campo !== "presupuesto_mensual_usd";

/** Alerta de gasto: se avisa al llegar a este porcentaje del presupuesto (E6-02 D20). */
export const UMBRAL_ALERTA_PORCENTAJE = 80;

export type ValoresConfiguracion = Record<CampoConfiguracion, string>;

export interface MensajesConfiguracion {
  /** Mensaje del campo, ya con sus límites: recibe el campo y devuelve el texto. */
  rango: (campo: CampoConfiguracion) => string;
  decimales: string;
  diaMayorQueMes: string;
}

/** El formulario edita texto; `""` es «vacío» y no pasa por `Number`. */
export const aValores = (c: CambioConfiguracionIa | ConfiguracionIa): ValoresConfiguracion =>
  Object.fromEntries(CAMPOS_CONFIGURACION.map((campo) => [campo, String(c[campo])])) as ValoresConfiguracion;

export const aCambio = (v: ValoresConfiguracion): CambioConfiguracionIa =>
  Object.fromEntries(CAMPOS_CONFIGURACION.map((campo) => [campo, Number(v[campo])])) as CambioConfiguracionIa;

/** ¿Tiene a lo sumo 2 decimales? (`dec2`, docs/api/README.md §1). Tolera el error de coma flotante de `n * 100`. */
const tieneDosDecimales = (n: number): boolean => Math.abs(n * 100 - Math.round(n * 100)) < 1e-6;

export function esquemaConfiguracion(m: MensajesConfiguracion) {
  const numero = (campo: CampoConfiguracion) => {
    const { min, max } = RANGOS[campo];
    const base = yup.number().typeError(m.rango(campo)).required(m.rango(campo)).min(min, m.rango(campo));
    const acotado = base.max(max, m.rango(campo));
    return esEntero(campo)
      ? acotado.integer(m.rango(campo))
      : acotado.test("dec2", m.decimales, (v) => v == null || tieneDosDecimales(v));
  };
  return yup.object({
    limite_usuario_dia: numero("limite_usuario_dia").test("dia-mes", m.diaMayorQueMes, function (dia) {
      const mes = Number(this.parent.limite_usuario_mes);
      return dia == null || Number.isNaN(mes) || dia <= mes;
    }),
    limite_usuario_mes: numero("limite_usuario_mes"),
    limite_admin_dia: numero("limite_admin_dia"),
    presupuesto_mensual_usd: numero("presupuesto_mensual_usd"),
    max_preguntas_solicitud: numero("max_preguntas_solicitud"),
    lote_clasificacion: numero("lote_clasificacion"),
  });
}

export type EstadoGasto = "agotado" | "alerta" | "normal";

/** El presupuesto se cortó, se cruzó el 80 % o va bien. `agotado` lo decide el servidor, no este porcentaje. */
export function estadoGasto(g: Pick<GastoIa, "agotado" | "alerta_80_at">): EstadoGasto {
  if (g.agotado) return "agotado";
  return g.alerta_80_at ? "alerta" : "normal";
}

/** Porcentaje para la barra de progreso: entre 0 y 100, aunque el gasto supere el presupuesto (D14). */
export const porcentajeBarra = (g: Pick<GastoIa, "porcentaje">): number => Math.min(100, Math.max(0, g.porcentaje));
