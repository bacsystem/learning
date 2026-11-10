const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

async function req<T>(path: string, token: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${token}`,
      ...(options?.headers ?? {}),
    },
  });
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

// Public testimonios carry only "Nombre I." (first name + initial of the
// apellido paterno) and no grado.
export interface TestimonioPublico {
  id: number;
  nombre_publico: string;
  calificacion: number;
  contenido: string;
  created_at: string;
}

export interface MiTestimonio {
  id: number;
  calificacion: number;
  contenido: string;
  estado: "pendiente" | "aprobado" | "rechazado";
  created_at: string;
}

export interface TestimonioAdmin {
  id: number;
  usuario_id: number;
  username: string;
  nombre_completo: string;
  grado: string;
  calificacion: number;
  contenido: string;
  estado: "pendiente" | "aprobado" | "rechazado";
  created_at: string;
}

export interface TestimonioAdminListResponse {
  data: TestimonioAdmin[];
  total: number;
  page: number;
  per_page: number;
}

export async function fetchTestimoniosPublicos(): Promise<TestimonioPublico[]> {
  const res = await fetch(`${API_BASE}/testimonios/publico`);
  if (!res.ok) return [];
  return res.json();
}

export async function fetchMiTestimonio(token: string): Promise<MiTestimonio | null> {
  const res = await fetch(`${API_BASE}/testimonios/me`, {
    headers: { Authorization: `Bearer ${token}` },
  });
  if (!res.ok) return null;
  return res.json();
}

export async function submitTestimonio(
  token: string,
  data: { contenido: string; calificacion: number },
): Promise<void> {
  await req<{ message: string }>("/testimonios", token, {
    method: "POST",
    body: JSON.stringify(data),
  });
}

export async function fetchTestimoniosAdmin(
  token: string,
  estado = "",
  page = 1,
  perPage = 20,
): Promise<TestimonioAdminListResponse> {
  const params = new URLSearchParams({ page: String(page), per_page: String(perPage) });
  if (estado) params.set("estado", estado);
  return req<TestimonioAdminListResponse>(`/admin/testimonios?${params}`, token);
}

export async function updateTestimonioEstado(
  token: string,
  id: number,
  estado: "aprobado" | "rechazado",
): Promise<void> {
  await req<{ message: string }>(`/admin/testimonios/${id}/estado`, token, {
    method: "PATCH",
    body: JSON.stringify({ estado }),
  });
}
