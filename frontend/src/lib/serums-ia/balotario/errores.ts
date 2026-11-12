import { apiErrorCode } from "@/lib/http/api-error";
import { claveErrorModulo } from "@/lib/serums-ia/errores";

/** Clave i18n de un error de la vista previa o de la confirmación (docs/api/…balotario.md §7). `tiene` es `t.has`. */
export const claveErrorBalotario = (err: unknown, tiene: (clave: string) => boolean): string =>
  claveErrorModulo(err, "balotario", tiene);

/** Los 409 tras los que hay que recargar la vista previa: otro admin importó la versión o subió una nueva. */
export const CODIGOS_QUE_RECARGAN = ["version_desactualizada", "ya_importado"] as const;

export const pideRecargar = (err: unknown): boolean => {
  const code = apiErrorCode(err);
  return code !== undefined && (CODIGOS_QUE_RECARGAN as readonly string[]).includes(code);
};

/** Parámetros de un texto de error: números del `detalle` que la clave i18n interpola. */
export function valoresDeError(err: unknown): Record<string, number> {
  const detalle = (err as { detalle?: unknown } | null)?.detalle;
  if (typeof detalle !== "object" || detalle === null) return {};
  const { maximo, detectadas } = detalle as Record<string, unknown>;
  return {
    ...(typeof maximo === "number" && { maximo }),
    ...(typeof detectadas === "number" && { detectadas }),
  };
}
