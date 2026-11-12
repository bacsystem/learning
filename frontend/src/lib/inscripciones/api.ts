import { API_BASE } from "@/lib/http/api-base";
import { apiErrorFrom } from "@/lib/http/api-error";
import { avisarSiSesionExpirada } from "@/lib/http/sesion";

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

export interface Proceso {
  id: number;
  nombre: string;
  tipo: string;
  grado: string;
  nivel: string;
  descripcion?: string;
  anio: number;
}

export interface Inscripcion {
  id: number;
  proceso_id: number;
  proceso_nombre: string;
  proceso_tipo: string;
  proceso_anio: number;
  estado: "pendiente" | "aprobado" | "rechazado";
  observacion?: string;
}

export interface InscripcionAdmin {
  id: number;
  usuario_id: number;
  username: string;
  nombre_completo: string;
  dni: string;
  grado: string;
  especialidad: string;
  proceso_nombre: string;
  proceso_tipo: string;
  proceso_anio: number;
  estado: string;
  observacion?: string;
  fecha_solicitud: string;
  /** Username of the admin who decided it (absent while pendiente). */
  revisado_por?: string;
  /** Decision date, "dd/mm/yyyy hh:mm" (absent while pendiente). */
  revisado_at?: string;
}

/** Server-side cap for GET /admin/inscripciones?per_page. */
export const INSCRIPCIONES_MAX_PER_PAGE = 500;
/** Safety stop for listAllInscripciones (MAX_PAGES × 500 rows). */
const INSCRIPCIONES_MAX_PAGES = 200;

export interface UsuarioPendiente {
  id: number;
  username: string;
  nombre_completo: string;
  dni: string;
  email: string;
  grado: string;
  especialidad: string;
  estado: string;
  fecha_solicitud: string;
}

export interface UsuarioOnlineItem {
  id: number;
  username: string;
  nombre_completo: string;
  rol: string;
  ip: string;
  last_seen_at: string;
}

export interface SesionHistorialItem {
  ip: string;
  user_agent: string;
  created_at: string;
  last_seen_at: string;
  expires_at: string;
  activa: boolean;
}

export interface StatsPublicos {
  total_usuarios: number;
  total_simulacros: number;
  total_practicas: number;
  total_procesos: number;
}

export async function fetchStatsPublicos(): Promise<StatsPublicos> {
  const res = await fetch(`${API_BASE}/stats/publico`);
  if (!res.ok) throw new Error("stats unavailable");
  return res.json();
}

export async function fetchProcesosPublico(): Promise<Proceso[]> {
  const res = await fetch(`${API_BASE}/procesos/publico`);
  if (!res.ok) return [];
  return res.json();
}

/** One page (1-based) of GET /admin/inscripciones; the server returns a plain array. */
function listInscripciones(
  token: string,
  estado = "",
  page = 1,
  perPage = INSCRIPCIONES_MAX_PER_PAGE,
): Promise<InscripcionAdmin[]> {
  const qs = new URLSearchParams({ page: String(page), per_page: String(perPage) });
  if (estado) qs.set("estado", estado);
  return req<InscripcionAdmin[]>(`/admin/inscripciones?${qs}`, token);
}

/** Every page of GET /admin/inscripciones, requested until a short page. */
async function listAllInscripciones(token: string, estado = ""): Promise<InscripcionAdmin[]> {
  const all: InscripcionAdmin[] = [];
  for (let page = 1; page <= INSCRIPCIONES_MAX_PAGES; page++) {
    const rows = await listInscripciones(token, estado, page, INSCRIPCIONES_MAX_PER_PAGE);
    all.push(...rows);
    if (rows.length < INSCRIPCIONES_MAX_PER_PAGE) break;
  }
  // A row inserted while paging shifts OFFSET and repeats a row: keep the first.
  const seen = new Set<number>();
  return all.filter((r) => {
    if (seen.has(r.id)) return false;
    seen.add(r.id);
    return true;
  });
}

export const inscripcionesApi = {
  listProcesos: (token: string) => req<Proceso[]>("/procesos", token),

  inscribir: (token: string, proceso_id: number) =>
    req<Inscripcion>("/inscripciones", token, {
      method: "POST",
      body: JSON.stringify({ proceso_id }),
    }),

  misInscripciones: (token: string) => req<Inscripcion[]>("/inscripciones/me", token),

  // Admin
  listUsuarios: (token: string, estado = "") =>
    req<UsuarioPendiente[]>(`/admin/usuarios${estado ? `?estado=${estado}` : ""}`, token),

  updateUsuarioEstado: (token: string, id: number, estado: string, observacion = "") =>
    req<{ message: string }>(`/admin/usuarios/${id}/estado`, token, {
      method: "PATCH",
      body: JSON.stringify({ estado, observacion }),
    }),

  updateUsuarioEmail: (token: string, id: number, email: string) =>
    req<{ message: string }>(`/admin/usuarios/${id}/email`, token, {
      method: "PATCH",
      body: JSON.stringify({ email }),
    }),

  resetTourUsuario: (token: string, id: number) =>
    req<{ tour_completado: boolean }>(`/admin/usuarios/${id}/tour`, token, { method: "DELETE" }),

  listUsuariosOnline: (token: string) =>
    req<{ data: UsuarioOnlineItem[]; total: number }>("/admin/usuarios/online", token),

  listSesionesUsuario: (token: string, id: number) =>
    req<{ data: SesionHistorialItem[]; total: number }>(`/admin/usuarios/${id}/sesiones`, token),

  listInscripciones,
  listAllInscripciones,

  updateInscripcionEstado: (token: string, id: number, estado: string, observacion = "") =>
    req<{ message: string }>(`/admin/inscripciones/${id}/estado`, token, {
      method: "PATCH",
      body: JSON.stringify({ estado, observacion }),
    }),
};
