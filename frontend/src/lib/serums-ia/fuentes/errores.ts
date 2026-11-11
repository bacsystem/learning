import { apiErrorCode } from "@/lib/http/api-error";
import { type ClavesDeError, mensajeDeError } from "@/lib/serums-ia/errores";

/** Códigos de error de S1–S7 con traducción propia (docs/api/serums-ia-e2-fuentes.md). */
const CLAVES_FUENTES: ClavesDeError = {
  archivo_demasiado_grande: "serumsIa.fuentes.errores.archivo_demasiado_grande",
  limite_versiones: "serumsIa.fuentes.errores.limite_versiones",
  estado_invalido: "serumsIa.fuentes.errores.estado_invalido",
  validacion: "serumsIa.fuentes.errores.validacion",
  referencia_invalida: "serumsIa.fuentes.errores.referencia_invalida",
  no_encontrado: "serumsIa.fuentes.errores.no_encontrado",
  cuerpo_invalido: "serumsIa.fuentes.errores.validacion",
  parametro_invalido: "serumsIa.fuentes.errores.validacion",
  error_interno: "serumsIa.errores.error_interno",
  cuerpo_demasiado_grande: "serumsIa.errores.cuerpo_demasiado_grande",
};

const MOTIVOS_ARCHIVO = new Set(["extension", "tipo", "codificacion", "lineas"]);

type Traductor = (clave: string, valores?: Record<string, string | number>) => string;

/** `detalle` del error, si es un objeto. */
function detalleDe(err: unknown): Record<string, unknown> {
  const detalle = (err as { detalle?: unknown } | null)?.detalle;
  return typeof detalle === "object" && detalle !== null ? (detalle as Record<string, unknown>) : {};
}

/**
 * Texto de un error de la API de fuentes. Además del mapa de códigos, traduce los que traen `detalle`:
 * `archivo_invalido.motivo` (extension, tipo, codificacion, lineas) y `limite_fuentes` (por profesión o total).
 * `fuente_duplicada` se muestra aparte (`fuenteDuplicada`), con enlace a la existente.
 */
export function mensajeErrorFuente(err: unknown, t: Traductor, generico: string): string {
  const code = apiErrorCode(err);
  const detalle = detalleDe(err);
  if (code === "archivo_invalido" && MOTIVOS_ARCHIVO.has(String(detalle.motivo))) {
    return t(`serumsIa.fuentes.errores.archivo_invalido.${String(detalle.motivo)}`);
  }
  if (code === "limite_fuentes" && typeof detalle.limite === "number") {
    return typeof detalle.especialidad_id === "number"
      ? t("serumsIa.fuentes.errores.limite_fuentes.especialidad", { limite: detalle.limite })
      : t("serumsIa.fuentes.errores.limite_fuentes.total", { limite: detalle.limite });
  }
  return mensajeDeError(err, CLAVES_FUENTES, t, generico);
}

export interface FuenteDuplicada {
  fuenteId: number;
  titulo: string;
  estado: string;
  version: number | null;
}

/** 409 `fuente_duplicada`: la fuente existente (E2-07 regla 8), o `null` si el error es otro. */
export function fuenteDuplicada(err: unknown): FuenteDuplicada | null {
  if (apiErrorCode(err) !== "fuente_duplicada") return null;
  const d = detalleDe(err);
  if (typeof d.fuente_id !== "number" || typeof d.titulo !== "string" || typeof d.estado !== "string") return null;
  return {
    fuenteId: d.fuente_id,
    titulo: d.titulo,
    estado: d.estado,
    version: typeof d.version === "number" ? d.version : null,
  };
}
