import { avisarSiSesionExpirada } from "@/lib/http/sesion";

const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

async function requestMultipart<T>(path: string, token: string, form: FormData): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    method: "POST",
    headers: { Authorization: `Bearer ${token}` },
    body: form,
  });
  if (!res.ok) {
    avisarSiSesionExpirada(res.status);
    const body = await res.json().catch(() => ({ error: res.statusText }));
    throw Object.assign(new Error(body.error ?? res.statusText), { status: res.status });
  }
  return res.json() as Promise<T>;
}

export interface UserProfile {
  id: number;
  username: string;
  nombres: string;
  apellido_paterno: string;
  apellido_materno: string;
  rol: string;
  estado: string;
  grado_id: number;
  grado: string;
  nivel: "suboficial" | "oficial" | string;
  especialidad: string;
  institucion: string;
  tour_completado: boolean;
}

/** Public DNI pre-check: names are reduced to the first given name and the
 * initial of the first surname (e.g. "Juan", "P."). */
export interface CheckDNIResult {
  nombres: string;
  apellido_paterno: string;
  /** Masked e-mail on file, e.g. "j***@g***.com"; "" when there is none. */
  email_masked: string;
}

export interface LoginResponse {
  token: string;
  expires_at: number;
  user: UserProfile;
}

async function request<T>(
  path: string,
  options?: RequestInit,
  { avisarExpiracion = true }: { avisarExpiracion?: boolean } = {},
): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers: { "Content-Type": "application/json", ...options?.headers },
  });
  if (!res.ok) {
    // Only an authenticated request can mean "session expired": a 401 from a
    // public endpoint (e.g. POST /auth/login with bad credentials) must reach
    // the caller so it can show its own message. `avisarExpiracion: false` lets
    // the caller handle the 401 itself (the initial session restore).
    if (avisarExpiracion && new Headers(options?.headers).has("Authorization")) avisarSiSesionExpirada(res.status);
    const body = await res.json().catch(() => ({ error: res.statusText }));
    throw Object.assign(new Error(body.error ?? res.statusText), { status: res.status });
  }
  return res.json() as Promise<T>;
}

function authHeader(token: string): HeadersInit {
  return { Authorization: `Bearer ${token}` };
}

export const authApi = {
  login(username: string, password: string): Promise<LoginResponse> {
    return request("/auth/login", {
      method: "POST",
      body: JSON.stringify({ username, password }),
    });
  },

  register(payload: { dni: string; email: string }): Promise<{ message: string }> {
    // 202 with one generic message whether or not the data matched: the
    // account is only created when the e-mailed link is opened (verifyEmail).
    return request("/auth/register", {
      method: "POST",
      body: JSON.stringify(payload),
    });
  },

  /** Completes a registration from the e-mailed link, choosing the credentials.
   * 201 → account created (pending admin activation); 400 → invalid/expired/used
   * link; 409 → username (or e-mail) taken — the link stays usable; 422 → field
   * validation; 429 → too many attempts. */
  verifyEmail(payload: {
    token: string;
    username: string;
    password: string;
    confirm_password: string;
  }): Promise<{ message: string }> {
    return request("/auth/verify-email", {
      method: "POST",
      body: JSON.stringify(payload),
    });
  },

  logout(token: string): Promise<{ message: string }> {
    return request("/auth/logout", {
      method: "POST",
      headers: authHeader(token),
    });
  },

  forgotPassword(email: string): Promise<{ message: string }> {
    return request("/auth/forgot-password", {
      method: "POST",
      body: JSON.stringify({ email }),
    });
  },

  resetPassword(token: string, password: string, confirm_password: string): Promise<{ message: string }> {
    return request("/auth/reset-password", {
      method: "POST",
      body: JSON.stringify({ token, password, confirm_password }),
    });
  },

  /**
   * `silencioso: true` does not fire the session-expired event on a 401: the caller clears
   * the session itself. Used by the initial restore of a stored token, which
   * must not redirect away from a public page (e.g. an e-mailed link).
   */
  getMe(token: string, { silencioso = false }: { silencioso?: boolean } = {}): Promise<UserProfile> {
    return request("/auth/me", { headers: authHeader(token) }, { avisarExpiracion: !silencioso });
  },

  /**
   * Public DNI pre-check. Never returns the e-mail on file in clear text:
   * only a masked hint (e.g. "j***@g***.com", "" when none). Rate-limited
   * per IP and per DNI (429).
   */
  checkDNI(dni: string): Promise<CheckDNIResult> {
    return request(`/auth/check-dni?dni=${encodeURIComponent(dni)}`);
  },

  checkUsername(username: string): Promise<{ available: boolean }> {
    return request(`/auth/check-username?username=${encodeURIComponent(username)}`);
  },
};

export interface PersonaListItem {
  id: number;
  dni: string;
  nombre_completo: string;
  nombres: string;
  apellido_paterno: string;
  apellido_materno: string;
  email: string;
  telefono: string;
  grado_id: number;
  grado: string;
  especialidad_id: number;
  especialidad: string;
  institucion_id: number;
  institucion: string;
  estado: string;
  tiene_usuario: boolean;
}

export interface UpdatePersonaRequest {
  nombres: string;
  apellido_paterno: string;
  apellido_materno: string;
  email: string;
  telefono: string;
  grado_id: number;
  especialidad_id: number;
  institucion_id: number;
  estado: string;
}

export const adminPersonasApi = {
  list(token: string): Promise<PersonaListItem[]> {
    return request("/admin/personas", { headers: authHeader(token) });
  },

  get(token: string, id: number): Promise<PersonaListItem> {
    return request(`/admin/personas/${id}`, { headers: authHeader(token) });
  },

  update(token: string, id: number, data: UpdatePersonaRequest): Promise<PersonaListItem> {
    return request(`/admin/personas/${id}`, {
      method: "PUT",
      headers: authHeader(token),
      body: JSON.stringify(data),
    });
  },
};

export interface PersonaPreviewRow {
  row: number;
  dni: string;
  nombres: string;
  apellido_paterno: string;
  apellido_materno: string;
  grado_id: number;
  especialidad_id: number;
  institucion_id: number;
  telefono: string;
  email: string;
  estado: "valido" | "error";
  error?: string;
}

export interface PersonaPreviewResponse {
  rows: PersonaPreviewRow[];
  total_valido: number;
  total_error: number;
}

export interface PersonaImportResult {
  inserted: number;
  duplicates: number;
  errors?: string[];
}

export const personasApi = {
  preview(token: string, file: File): Promise<PersonaPreviewResponse> {
    const form = new FormData();
    form.append("file", file);
    return requestMultipart("/admin/personas/preview", token, form);
  },

  import(token: string, file: File): Promise<PersonaImportResult> {
    const form = new FormData();
    form.append("file", file);
    return requestMultipart("/admin/personas/import", token, form);
  },
};

export interface Recurso {
  id: string;
  titulo: string;
  descripcion: string;
  url: string;
  icono: string;
  orden: number;
  activo: boolean;
}

export const adminConfigApi = {
  getConfigSesiones(token: string): Promise<{ max_sesiones: number }> {
    return request("/admin/config/sesiones", { headers: authHeader(token) });
  },

  setConfigSesiones(token: string, max_sesiones: number): Promise<{ max_sesiones: number }> {
    return request("/admin/config/sesiones", {
      method: "PUT",
      headers: authHeader(token),
      body: JSON.stringify({ max_sesiones }),
    });
  },

  getConfigRecursos(token: string): Promise<Recurso[]> {
    return request("/admin/config/recursos", { headers: authHeader(token) });
  },

  setConfigRecursos(token: string, recursos: Recurso[]): Promise<Recurso[]> {
    return request("/admin/config/recursos", {
      method: "PUT",
      headers: authHeader(token),
      body: JSON.stringify({ recursos }),
    });
  },
};

export interface UsuarioOnline {
  id: number;
  username: string;
  nombre_completo: string;
  rol: string;
  ip: string;
  last_seen_at: string;
}

export interface HealthStatus {
  status: "ok" | "unhealthy";
  version?: string;
}

export const systemApi = {
  getHealth(): Promise<HealthStatus> {
    return request<HealthStatus>("/health");
  },

  listUsuariosOnline(token: string): Promise<{ data: UsuarioOnline[]; total: number }> {
    return request("/admin/usuarios/online", { headers: authHeader(token) });
  },
};

export const tourApi = {
  completar(token: string): Promise<{ tour_completado: boolean }> {
    return request("/auth/tour/completar", { method: "POST", headers: authHeader(token) });
  },

  reset(token: string): Promise<{ tour_completado: boolean }> {
    return request("/auth/tour/completar", { method: "DELETE", headers: authHeader(token) });
  },
};
