import { apiErrorFrom } from "@/lib/http/api-error";

/** Cliente de temas (docs/api/serums-ia-e1-taxonomia.md §1, T1–T6). */

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

function buildQuery(params: Record<string, string | number | undefined>): string {
  const p = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== "") p.set(key, String(value));
  }
  const s = p.toString();
  return s ? `?${s}` : "";
}

// ── Interfaces ────────────────────────────────────────────────────────────────

export type EstadoTema = "activo" | "archivado";

export interface EspecialidadTema {
  id: number;
  nombre: string;
}

export interface Tema {
  id: number;
  asignatura_id: number;
  nombre: string;
  descripcion: string | null;
  estado: EstadoTema;
  orden: number;
  /** `true` ⇔ `especialidades` vacío: el tema aplica a todas las profesiones. */
  aplica_a_todas: boolean;
  especialidades: EspecialidadTema[];
  preguntas_count: number;
  created_at: string;
  updated_at: string;
}

export interface TemaList {
  data: Tema[];
  total: number;
  page: number;
  per_page: number;
}

export interface TemaListQuery {
  asignatura_id: number;
  especialidad_id?: number;
  estado?: EstadoTema;
  page?: number;
  per_page?: number;
}

/** Campos editables (T4). La asignatura no se envía al editar: el servidor rechaza campos desconocidos. */
export interface TemaPayload {
  nombre: string;
  descripcion: string | null;
  especialidad_ids: number[];
}

/** Alta (T3): los campos editables más la asignatura. */
export interface CreateTemaPayload extends TemaPayload {
  asignatura_id: number;
}

export interface OrdenTemaItem {
  id: number;
  orden: number;
}

// ── API Client ────────────────────────────────────────────────────────────────

export const temasApi = {
  listTemas(token: string, query: TemaListQuery): Promise<TemaList> {
    return req<TemaList>(`/admin/temas${buildQuery({ ...query })}`, token);
  },

  getTema(token: string, id: number): Promise<Tema> {
    return req<Tema>(`/admin/temas/${id}`, token);
  },

  createTema(token: string, payload: CreateTemaPayload): Promise<Tema> {
    return req<Tema>("/admin/temas", token, { method: "POST", body: JSON.stringify(payload) });
  },

  updateTema(token: string, id: number, payload: TemaPayload): Promise<Tema> {
    return req<Tema>(`/admin/temas/${id}`, token, { method: "PUT", body: JSON.stringify(payload) });
  },

  updateTemaEstado(token: string, id: number, estado: EstadoTema): Promise<Tema> {
    return req<Tema>(`/admin/temas/${id}/estado`, token, { method: "PATCH", body: JSON.stringify({ estado }) });
  },

  /** `temaIds` es la lista completa de los temas activos de la asignatura, en el orden nuevo. */
  reordenarTemas(token: string, asignaturaId: number, temaIds: number[]): Promise<{ data: OrdenTemaItem[] }> {
    return req<{ data: OrdenTemaItem[] }>(`/admin/asignaturas/${asignaturaId}/temas/orden`, token, {
      method: "PUT",
      body: JSON.stringify({ tema_ids: temaIds }),
    });
  },
};
