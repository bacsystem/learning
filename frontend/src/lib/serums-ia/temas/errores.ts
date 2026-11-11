import { apiErrorCode } from "@/lib/http/api-error";

/**
 * Códigos de error de los endpoints de temas con traducción propia (docs/api/README.md §8). Un código fuera de la
 * lista, o una respuesta sin código, cae en el `message` del servidor o en el texto genérico de la pantalla.
 */
const CODIGOS_TRADUCIDOS = new Set([
  "validacion",
  "referencia_invalida",
  "tema_duplicado",
  "limite_temas",
  "orden_invalido",
  "orden_desactualizado",
  "estado_invalido",
  "no_encontrado",
  "error_interno",
]);

/** Clave i18n (namespace `dashboard`) del código del error, o `null` si no tiene traducción. */
export function claveErrorTema(err: unknown): string | null {
  const code = apiErrorCode(err);
  return code && CODIGOS_TRADUCIDOS.has(code) ? `serumsIa.errores.${code}` : null;
}

/**
 * Texto a mostrar para un error de la API de temas. Orden: traducción del código, mensaje del servidor y, si no hay
 * respuesta del servidor (fallo de red: sin `status`), el texto genérico.
 */
export function mensajeErrorTema(err: unknown, t: (clave: string) => string, generico: string): string {
  const clave = claveErrorTema(err);
  if (clave) return t(clave);
  const { status, message } = (err ?? {}) as { status?: unknown; message?: unknown };
  return typeof status === "number" && typeof message === "string" && message ? message : generico;
}
