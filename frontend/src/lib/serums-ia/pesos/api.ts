import { apiErrorFrom } from "@/lib/http/api-error";

/** Cliente de pesos por profesión (docs/api/serums-ia-e1-taxonomia.md §2, PE1–PE3). */

const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

async function req<T>(path: string, token: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${token}`,
      ...options?.headers,
    },
    ...options,
  });
  if (!res.ok) {
    if (res.status === 401) window.dispatchEvent(new Event("auth:expired"));
    throw await apiErrorFrom(res);
  }
  return res.json() as Promise<T>;
}

// ── Interfaces ────────────────────────────────────────────────────────────────

/** Derivado en el servidor: `completa` ⇔ existe un conjunto guardado (que siempre suma 100). */
export type EstadoPesos = "completa" | "incompleta";

export interface ResumenEspecialidad {
  especialidad_id: number;
  nombre: string;
  estado: EstadoPesos;
  areas: number;
  suma_pesos: number;
  suma_preguntas: number;
}

/** PE1: especialidades activas de la institución del proceso, ordenadas por nombre. */
export interface ResumenPesos {
  proceso_id: number;
  compatibilidad: { areas: number; suma_pesos: number };
  especialidades: ResumenEspecialidad[];
}

export interface AreaPeso {
  asignatura_id: number;
  nombre: string;
  codigo: string;
  peso_porcentual: number;
  numero_preguntas: number;
  coeficiente: number;
}

/** PE2/PE3. Con `incompleta`, `areas` va vacío y `plantilla` trae las filas generales del proceso. */
export interface ConfiguracionPesos {
  proceso_id: number;
  especialidad_id: number;
  especialidad: string;
  estado: EstadoPesos;
  areas: AreaPeso[];
  suma_pesos: number;
  suma_preguntas: number;
  plantilla: AreaPeso[];
}

export interface AreaPesoPayload {
  asignatura_id: number;
  peso_porcentual: number;
  numero_preguntas: number;
  coeficiente: number;
}

// ── API Client ────────────────────────────────────────────────────────────────

const base = (procesoId: number) => `/admin/procesos/${procesoId}/pesos`;

export const pesosApi = {
  getResumen(token: string, procesoId: number): Promise<ResumenPesos> {
    return req<ResumenPesos>(base(procesoId), token);
  },

  getConfiguracion(token: string, procesoId: number, especialidadId: number): Promise<ConfiguracionPesos> {
    return req<ConfiguracionPesos>(`${base(procesoId)}/${especialidadId}`, token);
  },

  /** Reemplaza la configuración entera de (proceso, especialidad): un área que no viene sale de ella. */
  guardar(
    token: string,
    procesoId: number,
    especialidadId: number,
    areas: AreaPesoPayload[],
  ): Promise<ConfiguracionPesos> {
    return req<ConfiguracionPesos>(`${base(procesoId)}/${especialidadId}`, token, {
      method: "PUT",
      body: JSON.stringify({ areas }),
    });
  },
};
