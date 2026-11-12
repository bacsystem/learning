import { claveErrorModulo } from "@/lib/serums-ia/errores";

/**
 * Clave i18n de un error del panel de IA (docs/api/serums-ia-e6-02-control-costos.md §5). `validacion` es del recurso
 * `ia`; `presupuesto_agotado`, `limite_ia` y `limite_ia_mensual` son del módulo y las muestran los consumidores.
 */
export const claveErrorIa = (err: unknown, tiene: (clave: string) => boolean): string =>
  claveErrorModulo(err, "ia", tiene);
