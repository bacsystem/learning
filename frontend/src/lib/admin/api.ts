const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

async function adminFetch<T>(path: string, token: string, options?: RequestInit): Promise<T> {
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
    const body = await res.json().catch(() => ({ error: res.statusText }));
    throw Object.assign(new Error(body.error ?? res.statusText), { status: res.status });
  }
  return res.json() as Promise<T>;
}

// ── Dashboard ─────────────────────────────────────────────────────────────────

export interface DiaMetricaItem {
  fecha: string;
  total: number;
}

export interface AsignaturaMetricaItem {
  nombre: string;
  total: number;
}

export interface DashboardData {
  total_usuarios_registrados: number;
  total_usuarios_activos: number;
  total_usuarios_pendientes: number;
  total_practicas_hoy: number;
  total_simulacros_hoy: number;
  usuarios_online: number;
  practicas_ultima_semana: DiaMetricaItem[];
  simulacros_ultima_semana: DiaMetricaItem[];
  top_asignaturas: AsignaturaMetricaItem[];
}

export interface DashboardActividad {
  practicas_ultima_semana: DiaMetricaItem[];
  simulacros_ultima_semana: DiaMetricaItem[];
  top_asignaturas: AsignaturaMetricaItem[];
}

export function adminGetDashboard(token: string): Promise<DashboardData> {
  return adminFetch("/admin/dashboard", token);
}

export function adminGetDashboardActividad(token: string): Promise<DashboardActividad> {
  return adminFetch("/admin/dashboard/actividad", token);
}

// ── Usuario Detalle ───────────────────────────────────────────────────────────

export interface UsuarioDetalle {
  id: number;
  persona_id: number;
  dni: string;
  nombres: string;
  apellido_paterno: string;
  apellido_materno: string;
  grado: string;
  especialidad: string;
  institucion: string;
  telefono: string;
  email: string;
  username: string;
  rol: string;
  estado: string;
  activated_at?: string;
  activated_by?: string;
  created_at: string;
  updated_at: string;
  total_practicas: number;
  total_simulacros: number;
  nota_promedio: number;
}

export function adminGetUsuario(token: string, id: number): Promise<UsuarioDetalle> {
  return adminFetch(`/admin/usuarios/${id}`, token);
}

export function adminPatchUsuarioRol(token: string, id: number, rol: string): Promise<{ message: string }> {
  return adminFetch(`/admin/usuarios/${id}/rol`, token, {
    method: "PATCH",
    body: JSON.stringify({ rol }),
  });
}

// ── Audit Log ─────────────────────────────────────────────────────────────────

export interface AuditLogItem {
  id: number;
  usuario_id?: number;
  username?: string;
  accion: string;
  entidad: string;
  entidad_id?: number;
  detalle?: Record<string, unknown>;
  ip?: string;
  user_agent?: string;
  created_at: string;
}

export interface AuditLogResponse {
  data: AuditLogItem[];
  total: number;
  page: number;
  per_page: number;
}

export interface AuditLogParams {
  page?: number;
  per_page?: number;
  from?: string;
  to?: string;
  q?: string;
}

export function adminGetAuditLog(token: string, params?: AuditLogParams): Promise<AuditLogResponse> {
  const qs = new URLSearchParams();
  if (params?.page) qs.set("page", String(params.page));
  if (params?.per_page) qs.set("per_page", String(params.per_page));
  if (params?.from) qs.set("from", params.from);
  if (params?.to) qs.set("to", params.to);
  if (params?.q) qs.set("q", params.q);
  const query = qs.toString() ? `?${qs.toString()}` : "";
  return adminFetch(`/admin/audit-log${query}`, token);
}

// ── Ingresos ──────────────────────────────────────────────────────────────────

// Money comes as exact decimal strings with 2 places ("1234.50"); never do
// arithmetic on them, format with formatSoles.
export interface MesIngresoItem {
  mes: string;
  total: string;
}

export interface IngresosData {
  total_recaudado: string;
  total_este_anio: string;
  total_este_mes: string;
  total_inscripciones: number;
  serie_mensual: MesIngresoItem[];
}

export interface ConfigPago {
  anio: number;
  monto: number;
  updated_at: string;
}

const DECIMAL_RE = /^\d+(\.\d+)?$/;

/**
 * Formats an exact decimal string as soles without going through float, with the same output as
 * `formatearMonto("PEN", n)` from lib/formato.ts ("S/ 1,234.56"): thousands separator and 2 decimals,
 * worked on the string so large amounts keep every digit.
 */
export function formatSoles(amount: string | undefined): string {
  const s = amount && DECIMAL_RE.test(amount) ? amount : "0";
  const [int, dec = ""] = s.split(".");
  const miles = int.replace(/\B(?=(\d{3})+$)/g, ",");
  return `S/ ${miles}.${dec.padEnd(2, "0").slice(0, 2)}`;
}

export function adminGetIngresos(token: string): Promise<IngresosData> {
  return adminFetch("/admin/ingresos", token);
}

export function adminGetIngresosConfig(token: string): Promise<ConfigPago[]> {
  return adminFetch("/admin/ingresos/config", token);
}

export function adminPutIngresosConfig(token: string, anio: number, monto: number): Promise<ConfigPago> {
  return adminFetch(`/admin/ingresos/config/${anio}`, token, {
    method: "PUT",
    body: JSON.stringify({ monto }),
  });
}
