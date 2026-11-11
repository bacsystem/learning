import type { RespuestaSaludApi } from "./api";

/**
 * Reglas del panel de salud (docs/api/plataforma-salud.md y PLT-02/03). La respuesta de la API es entrada externa:
 * se valida aquí y la pantalla solo ve valores de los conjuntos cerrados del contrato.
 */

/** PLT-02 regla 4: refresco automático con la pestaña visible. */
export const REFRESCO_SALUD_MS = 30_000;

export const ESTADOS_SERVICIO = ["operativo", "degradado", "caido", "no_configurado", "desconocido"] as const;
export type EstadoServicio = (typeof ESTADOS_SERVICIO)[number];

export const CATEGORIAS_FALLO = ["sin_conexion", "tiempo_agotado", "error_interno", "atrasado", "sin_datos"] as const;
export type CategoriaFallo = (typeof CATEGORIAS_FALLO)[number];

export interface DatosApi {
  version: string | null;
  en_marcha_s: number | null;
}
export interface DatosBaseDatos {
  latencia_ms: number | null;
}
export interface DatosCorreo {
  sin_enviar: number | null;
  antiguedad_max_min: number | null;
  fallidos_24h: number | null;
  ultimo_enviado_en: string | null;
}

interface ServicioBase {
  estado: EstadoServicio;
  categoria: CategoriaFallo | null;
}
export type ServicioSalud =
  | (ServicioBase & { id: "api"; datos: DatosApi })
  | (ServicioBase & { id: "base_datos"; datos: DatosBaseDatos })
  | (ServicioBase & { id: "correo"; datos: DatosCorreo });
export type IdServicio = ServicioSalud["id"];

export interface Salud {
  estado: EstadoServicio;
  /** RFC 3339 UTC, el de la ejecución (cacheada o no) en el servidor. */
  comprobadoEn: string;
  /** Solo los servicios que esta pantalla conoce, en el orden de la API. */
  servicios: ServicioSalud[];
}

/** La respuesta no cumple el contrato (sin fecha válida o sin lista de servicios): se trata como un fallo de consulta. */
export class RespuestaSaludInvalida extends Error {
  constructor() {
    super("Respuesta de salud fuera de contrato");
    this.name = "RespuestaSaludInvalida";
  }
}

type Registro = Record<string, unknown>;

const esRegistro = (v: unknown): v is Registro => typeof v === "object" && v !== null && !Array.isArray(v);
const numero = (v: unknown): number | null => (typeof v === "number" && Number.isFinite(v) ? v : null);
const texto = (v: unknown): string | null => (typeof v === "string" && v !== "" ? v : null);
const fecha = (v: unknown): string | null => {
  const s = texto(v);
  return s !== null && !Number.isNaN(Date.parse(s)) ? s : null;
};
const deConjunto = <T extends string>(conjunto: readonly T[], v: unknown): T | null =>
  conjunto.includes(v as T) ? (v as T) : null;

/** Un estado fuera del conjunto no se inventa como «operativo»: se muestra como desconocido. */
const estadoDe = (v: unknown): EstadoServicio => deConjunto(ESTADOS_SERVICIO, v) ?? "desconocido";

const LECTORES_DATOS: { [K in IdServicio]: (d: Registro) => Extract<ServicioSalud, { id: K }>["datos"] } = {
  api: (d) => ({ version: texto(d.version), en_marcha_s: numero(d.en_marcha_s) }),
  base_datos: (d) => ({ latencia_ms: numero(d.latencia_ms) }),
  correo: (d) => ({
    sin_enviar: numero(d.sin_enviar),
    antiguedad_max_min: numero(d.antiguedad_max_min),
    fallidos_24h: numero(d.fallidos_24h),
    ultimo_enviado_en: fecha(d.ultimo_enviado_en),
  }),
};

const esIdConocido = (v: unknown): v is IdServicio => typeof v === "string" && Object.hasOwn(LECTORES_DATOS, v);

function servicioDe(crudo: unknown): ServicioSalud | null {
  // PLT-01 regla 7: un `id` que la pantalla no conoce (worker_ingesta, copia_r2…) se ignora sin romperla.
  if (!esRegistro(crudo) || !esIdConocido(crudo.id)) return null;
  const datos = esRegistro(crudo.datos) ? crudo.datos : {};
  return {
    id: crudo.id,
    estado: estadoDe(crudo.estado),
    categoria: deConjunto(CATEGORIAS_FALLO, crudo.categoria),
    datos: LECTORES_DATOS[crudo.id](datos),
  } as ServicioSalud;
}

/** Valida la respuesta de S1. El estado global es el de la API: no se recalcula en el cliente. */
export function normalizarSalud(crudo: RespuestaSaludApi): Salud {
  const comprobadoEn = fecha(crudo?.comprobado_en);
  if (comprobadoEn === null || !Array.isArray(crudo.servicios)) throw new RespuestaSaludInvalida();
  return {
    estado: estadoDe(crudo.estado),
    comprobadoEn,
    servicios: crudo.servicios.map(servicioDe).filter((s): s is ServicioSalud => s !== null),
  };
}

/** Versión del backend, la del servicio `api`, o `null` si no vino. */
export function versionBackend(salud: Salud): string | null {
  const api = salud.servicios.find((s) => s.id === "api");
  return api?.id === "api" ? api.datos.version : null;
}

/** PLT-03 regla 4: nota informativa solo si se conoce la versión del backend y difiere de la del frontend. */
export function versionesDistintas(frontend: string, backend: string | null): boolean {
  return backend !== null && backend !== frontend;
}
