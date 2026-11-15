import type { OpcionesError } from "@/lib/errores-api";

/**
 * Errores de acceso a un proceso (docs/api/plataforma-multi-institucion.md §2.3 y §2.4), en formato 2 con `code`.
 * Cada texto le dice al postulante qué pasa y qué hacer.
 */
const CODIGOS_PROCESO = {
  institucion_no_permitida: "acceso-proceso-institucion-no-permitida",
  nivel_no_permitido: "acceso-proceso-nivel-no-permitido",
} as const;

const FORBIDDEN = 403;
const UNPROCESSABLE = 422;

/** `POST /inscripciones`: además de los 403, sus 404 y 409 con código. */
export const ERROR_INSCRIBIR = {
  generico: "procesos-error-inscribir",
  porCodigo: {
    ...CODIGOS_PROCESO,
    no_encontrado: "acceso-proceso-no-encontrado",
    ya_inscrito: "acceso-proceso-ya-inscrito",
    sin_monto_configurado: "acceso-proceso-sin-monto",
  },
} satisfies OpcionesError;

/** `POST /practicas`: los 403 con código y, para los 403 y 422 sin código, los textos que ya tenía la pantalla. */
export const ERROR_CREAR_PRACTICA = {
  generico: "practicas-error-crear",
  porCodigo: { ...CODIGOS_PROCESO, inscripcion_requerida: "acceso-proceso-inscripcion-requerida" },
  porStatus: { [FORBIDDEN]: "practicas-error-sin-acceso", [UNPROCESSABLE]: "practicas-error-preguntas" },
} satisfies OpcionesError;

/** `POST /simulacros`: igual que las prácticas, más el 400 de la configuración. */
export const ERROR_CREAR_SIMULACRO = {
  generico: "simulacros-error-crear",
  porCodigo: { ...CODIGOS_PROCESO, inscripcion_requerida: "acceso-proceso-inscripcion-requerida" },
  porStatus: {
    [FORBIDDEN]: "simulacros-error-sin-acceso",
    [UNPROCESSABLE]: "simulacros-error-preguntas",
    400: "simulacros-error-configuracion",
  },
} satisfies OpcionesError;
