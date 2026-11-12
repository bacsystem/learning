import { API_BASE } from "@/lib/http/api-base";
import { apiErrorFrom } from "@/lib/http/api-error";
import { avisarSiSesionExpirada } from "@/lib/http/sesion";

/** Cliente del control de costos de la IA (docs/api/serums-ia-e6-02-control-costos.md §3). */

/** Una ventana de consumo (día o mes calendario en America/Lima). */
export interface VentanaConsumo {
  usados: number;
  limite: number;
  disponibles: number;
  reinicia_at: string;
}

/** `GET /serums-ia/ia/consumo`: el consumo de quien consulta. Para un admin, `mes` es `null`. */
export interface ConsumoIa {
  dia: VentanaConsumo;
  mes: VentanaConsumo | null;
  presupuesto_agotado: boolean;
}

/** Límites y presupuesto (§3.2). `PUT` lleva todos los campos salvo `updated_at`. */
export interface ConfiguracionIa {
  limite_usuario_dia: number;
  limite_usuario_mes: number;
  limite_admin_dia: number;
  max_preguntas_solicitud: number;
  lote_clasificacion: number;
  presupuesto_mensual_usd: number;
  updated_at: string;
}

export type CambioConfiguracionIa = Omit<ConfiguracionIa, "updated_at">;

export interface GastoPorOperacion {
  operacion: string;
  invocaciones: number;
  desde_cache: number;
  costo_usd: number;
}

/** `GET /admin/ia/gasto` (§3.4): solo el mes en curso y sin cifras por usuario. */
export interface GastoIa {
  /** `AAAA-MM`. */
  mes: string;
  presupuesto_usd: number;
  gastado_usd: number;
  /** Con 2 decimales; puede superar 100. */
  porcentaje: number;
  /** Cuándo se cruzó el 80 %, o `null` si aún no. */
  alerta_80_at: string | null;
  agotado: boolean;
  reinicia_at: string;
  por_operacion: GastoPorOperacion[];
}

async function req<T>(path: string, token: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers: { Authorization: `Bearer ${token}`, ...options?.headers },
  });
  if (!res.ok) {
    avisarSiSesionExpirada(res.status);
    throw await apiErrorFrom(res);
  }
  return res.json() as Promise<T>;
}

export const iaApi = {
  /** Para la primera pantalla del postulante que dé una acción IA real; esta historia no tiene ninguna (D21). */
  consumo: (token: string) => req<ConsumoIa>("/serums-ia/ia/consumo", token),

  configuracion: (token: string) => req<ConfiguracionIa>("/admin/ia/configuracion", token),

  guardarConfiguracion: (token: string, cambio: CambioConfiguracionIa) =>
    req<ConfiguracionIa>("/admin/ia/configuracion", token, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(cambio),
    }),

  gasto: (token: string) => req<GastoIa>("/admin/ia/gasto", token),
};
