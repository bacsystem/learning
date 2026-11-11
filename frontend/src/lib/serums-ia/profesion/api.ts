import { API_BASE } from "@/lib/http/api-base";
import { apiErrorFrom } from "@/lib/http/api-error";

/** Cliente de la profesión del postulante (docs/api/serums-ia-e1-taxonomia.md §3, F1–F3). Sin `usuario_id`: la sesión. */

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

/** `completa` si el proceso tiene pesos cargados para la especialidad; si no, se usan pesos estimados. */
export type PesosEspecialidad = "completa" | "estimada";

export interface EspecialidadOpcion {
  id: number;
  nombre: string;
  codigo: string;
  pesos: PesosEspecialidad;
}

export interface ProcesoOpcion {
  id: number;
  nombre: string;
  anio: number;
  especialidades: EspecialidadOpcion[];
}

/** Sugerencia del servidor: el perfil activo o la especialidad de persona. Sus ids siempre están en `procesos`. */
export interface Preseleccion {
  proceso_id: number | null;
  especialidad_id: number;
}

/** F1. `procesos` vacío ⇔ el usuario no tiene ninguna inscripción aprobada. */
export interface OpcionesProfesion {
  procesos: ProcesoOpcion[];
  preseleccion: Preseleccion | null;
}

export interface AreaPesoProfesion {
  asignatura_id: number;
  nombre: string;
  peso_porcentual: number;
}

/** F2/F3: el perfil activo. */
export interface Profesion {
  id: number;
  proceso: { id: number; nombre: string };
  especialidad: { id: number; nombre: string };
  estado: string;
  created_at: string;
  pesos: { pesos_estimados: boolean; areas: AreaPesoProfesion[] };
}

export interface GuardarProfesionPayload {
  proceso_id: number;
  especialidad_id: number;
  /** `true` solo después de que el usuario confirma el reinicio (409 `confirmacion_requerida`). */
  confirmar_reinicio: boolean;
}

// ── API Client ────────────────────────────────────────────────────────────────

export const profesionApi = {
  getOpciones(token: string): Promise<OpcionesProfesion> {
    return req<OpcionesProfesion>("/serums-ia/profesion/opciones", token);
  },

  /** 404 `profesion_no_definida` si aún no eligió; 403 `inscripcion_requerida` si se revocó la inscripción. */
  getProfesion(token: string): Promise<Profesion> {
    return req<Profesion>("/serums-ia/profesion", token);
  },

  guardar(token: string, payload: GuardarProfesionPayload): Promise<Profesion> {
    return req<Profesion>("/serums-ia/profesion", token, { method: "PUT", body: JSON.stringify(payload) });
  },
};
