import { apiErrorFrom } from "@/lib/http/api-error";
import { avisarSiSesionExpirada } from "@/lib/http/sesion";

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
    avisarSiSesionExpirada(res.status);
    throw await apiErrorFrom(res);
  }
  return res.json() as Promise<T>;
}

// ── Interfaces ────────────────────────────────────────────────────────────────

export interface ListQuery {
  estado?: string;
  institucion_id?: number;
  tipo?: string;
  anio?: number;
  nivel?: string;
  limit?: string; // "all" or a number as string
}

export interface Institucion {
  id: number;
  nombre: string;
  siglas: string;
  estado: string;
  created_at: string;
}

export interface Grado {
  id: number;
  institucion_id: number;
  institucion: string;
  nombre: string;
  abreviatura: string;
  nivel: "suboficial" | "oficial";
  orden: number;
  estado: string;
  created_at: string;
}

export interface Especialidad {
  id: number;
  institucion_id: number;
  institucion: string;
  nombre: string;
  codigo: string;
  nivel: string;
  estado: string;
  created_at: string;
}

export interface Asignatura {
  id: number;
  nombre: string;
  codigo: string;
  estado: string;
  created_at: string;
}

export interface Proceso {
  id: number;
  institucion_id: number;
  institucion: string;
  nombre: string;
  tipo: "ascenso" | "pase_efectividad" | "admision";
  grado_id?: number;
  grado?: string;
  descripcion: string;
  anio: number;
  estado: string;
  created_at: string;
}

export interface ProcesoAsignaturaItem {
  asignatura_id: number;
  nombre: string;
  codigo: string;
  numero_preguntas: number;
  peso_porcentual: number;
  coeficiente: number;
  preguntas_disponibles: number;
}

export interface ProcesoDetail extends Proceso {
  asignaturas: ProcesoAsignaturaItem[];
}

// ── Helper ────────────────────────────────────────────────────────────────────

function buildQuery(params: ListQuery): string {
  const p = new URLSearchParams();
  if (params.estado) p.set("estado", params.estado);
  if (params.institucion_id) p.set("institucion_id", String(params.institucion_id));
  if (params.tipo) p.set("tipo", params.tipo);
  if (params.anio) p.set("anio", String(params.anio));
  if (params.nivel) p.set("nivel", params.nivel);
  if (params.limit) p.set("limit", params.limit);
  const s = p.toString();
  return s ? `?${s}` : "";
}

// ── API Object ────────────────────────────────────────────────────────────────

export const catalogsApi = {
  // Instituciones
  listInstituciones: (token: string, q: ListQuery = {}) => req<Institucion[]>(`/instituciones${buildQuery(q)}`, token),

  getInstitucion: (token: string, id: number) => req<Institucion>(`/instituciones/${id}`, token),

  createInstitucion: (token: string, body: { nombre: string; siglas: string }) =>
    req<Institucion>("/instituciones", token, {
      method: "POST",
      body: JSON.stringify(body),
    }),

  updateInstitucion: (token: string, id: number, body: { nombre: string; siglas: string }) =>
    req<Institucion>(`/instituciones/${id}`, token, {
      method: "PUT",
      body: JSON.stringify(body),
    }),

  updateInstitucionEstado: (token: string, id: number, estado: string) =>
    req<{ message: string }>(`/instituciones/${id}/estado`, token, {
      method: "PATCH",
      body: JSON.stringify({ estado }),
    }),

  // Grados
  listGrados: (token: string, q: ListQuery = {}) => req<Grado[]>(`/grados${buildQuery(q)}`, token),

  getGrado: (token: string, id: number) => req<Grado>(`/grados/${id}`, token),

  createGrado: (
    token: string,
    body: {
      institucion_id: number;
      nombre: string;
      abreviatura: string;
      nivel: string;
      orden: number;
    },
  ) =>
    req<Grado>("/grados", token, {
      method: "POST",
      body: JSON.stringify(body),
    }),

  updateGrado: (
    token: string,
    id: number,
    body: {
      institucion_id: number;
      nombre: string;
      abreviatura: string;
      nivel: string;
      orden: number;
    },
  ) =>
    req<Grado>(`/grados/${id}`, token, {
      method: "PUT",
      body: JSON.stringify(body),
    }),

  updateGradoEstado: (token: string, id: number, estado: string) =>
    req<{ message: string }>(`/grados/${id}/estado`, token, {
      method: "PATCH",
      body: JSON.stringify({ estado }),
    }),

  // Especialidades
  listEspecialidades: (token: string, q: ListQuery = {}) =>
    req<Especialidad[]>(`/especialidades${buildQuery(q)}`, token),

  getEspecialidad: (token: string, id: number) => req<Especialidad>(`/especialidades/${id}`, token),

  createEspecialidad: (
    token: string,
    body: { institucion_id: number; nombre: string; codigo: string; nivel?: string },
  ) =>
    req<Especialidad>("/especialidades", token, {
      method: "POST",
      body: JSON.stringify(body),
    }),

  updateEspecialidad: (
    token: string,
    id: number,
    body: { institucion_id: number; nombre: string; codigo: string; nivel?: string },
  ) =>
    req<Especialidad>(`/especialidades/${id}`, token, {
      method: "PUT",
      body: JSON.stringify(body),
    }),

  updateEspecialidadEstado: (token: string, id: number, estado: string) =>
    req<{ message: string }>(`/especialidades/${id}/estado`, token, {
      method: "PATCH",
      body: JSON.stringify({ estado }),
    }),

  // Asignaturas
  listAsignaturas: (token: string, q: ListQuery = {}) => req<Asignatura[]>(`/asignaturas${buildQuery(q)}`, token),

  getAsignatura: (token: string, id: number) => req<Asignatura>(`/asignaturas/${id}`, token),

  createAsignatura: (token: string, body: { nombre: string; codigo: string }) =>
    req<Asignatura>("/asignaturas", token, {
      method: "POST",
      body: JSON.stringify(body),
    }),

  updateAsignatura: (token: string, id: number, body: { nombre: string; codigo: string }) =>
    req<Asignatura>(`/asignaturas/${id}`, token, {
      method: "PUT",
      body: JSON.stringify(body),
    }),

  updateAsignaturaEstado: (token: string, id: number, estado: string) =>
    req<{ message: string }>(`/asignaturas/${id}/estado`, token, {
      method: "PATCH",
      body: JSON.stringify({ estado }),
    }),

  // Procesos
  listProcesos: (token: string, q: ListQuery = {}) => req<Proceso[]>(`/admin/procesos${buildQuery(q)}`, token),

  getProceso: (token: string, id: number) => req<ProcesoDetail>(`/admin/procesos/${id}`, token),

  createProceso: (
    token: string,
    body: {
      institucion_id: number;
      nombre: string;
      tipo: string;
      grado_id?: number | null;
      descripcion?: string;
      anio: number;
    },
  ) =>
    req<Proceso>("/admin/procesos", token, {
      method: "POST",
      body: JSON.stringify(body),
    }),

  updateProceso: (
    token: string,
    id: number,
    body: {
      institucion_id: number;
      nombre: string;
      tipo: string;
      grado_id?: number | null;
      descripcion?: string;
      anio: number;
    },
  ) =>
    req<Proceso>(`/admin/procesos/${id}`, token, {
      method: "PUT",
      body: JSON.stringify(body),
    }),

  updateProcesoEstado: (token: string, id: number, estado: string) =>
    req<{ message: string }>(`/admin/procesos/${id}/estado`, token, {
      method: "PATCH",
      body: JSON.stringify({ estado }),
    }),

  // Proceso-Asignaturas (admin)
  listProcesoAsignaturas: (token: string, procesoId: number) =>
    req<ProcesoAsignaturaItem[]>(`/admin/procesos/${procesoId}/asignaturas`, token),

  // Proceso-Asignaturas (user — no admin required)
  listProcesoAsignaturasPublic: (token: string, procesoId: number) =>
    req<ProcesoAsignaturaItem[]>(`/procesos/${procesoId}/asignaturas`, token),

  asignarAsignatura: (
    token: string,
    procesoId: number,
    body: { asignatura_id: number; numero_preguntas: number; peso_porcentual: number; coeficiente: number },
  ) =>
    req<ProcesoAsignaturaItem>(`/admin/procesos/${procesoId}/asignaturas`, token, {
      method: "POST",
      body: JSON.stringify(body),
    }),

  updateProcesoAsignatura: (
    token: string,
    procesoId: number,
    asignaturaId: number,
    body: { numero_preguntas: number; peso_porcentual: number; coeficiente: number },
  ) =>
    req<{ message: string }>(`/admin/procesos/${procesoId}/asignaturas/${asignaturaId}`, token, {
      method: "PUT",
      body: JSON.stringify(body),
    }),

  deleteProcesoAsignatura: (token: string, procesoId: number, asignaturaId: number) =>
    req<{ message: string }>(`/admin/procesos/${procesoId}/asignaturas/${asignaturaId}`, token, {
      method: "DELETE",
    }),
};
