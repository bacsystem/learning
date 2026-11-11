import { claveDeError, type ClavesDeError, mensajeDeError } from "@/lib/serums-ia/errores";

/**
 * Códigos de error de los endpoints de temas con traducción propia (docs/api/README.md §8). Un código fuera de la
 * lista, o una respuesta sin código, cae en el `message` del servidor o en el texto genérico de la pantalla.
 */
const CLAVES_TEMAS: ClavesDeError = Object.fromEntries(
  [
    "validacion",
    "referencia_invalida",
    "tema_duplicado",
    "limite_temas",
    "orden_invalido",
    "orden_desactualizado",
    "estado_invalido",
    "no_encontrado",
    "error_interno",
    "cuerpo_demasiado_grande",
  ].map((code) => [code, `serumsIa.errores.${code}`]),
);

/** Clave i18n (namespace `dashboard`) del código del error, o `null` si no tiene traducción. */
export function claveErrorTema(err: unknown): string | null {
  return claveDeError(err, CLAVES_TEMAS);
}

/** Texto a mostrar para un error de la API de temas (ver `mensajeDeError`). */
export function mensajeErrorTema(err: unknown, t: (clave: string) => string, generico: string): string {
  return mensajeDeError(err, CLAVES_TEMAS, t, generico);
}
