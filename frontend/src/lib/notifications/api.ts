import { API_BASE } from "@/lib/http/api-base";

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
    const body = await res.json().catch(() => ({ error: res.statusText }));
    throw Object.assign(new Error(body.error ?? res.statusText), { status: res.status });
  }
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

export type NotifTipo = "sistema" | "mantenimiento" | "funcionalidad" | "alerta";
// Must match usuarios.rol on the backend ("usuario", not "user").
export type NotifDestinatario = "todos" | "rol:admin" | "rol:usuario";

export interface Notificacion {
  id: number;
  titulo: string;
  cuerpo: string;
  tipo: NotifTipo;
  destinatario: NotifDestinatario;
  creado_por?: number;
  created_at: string;
  leida: boolean;
}

export interface CreateNotificacionPayload {
  titulo: string;
  cuerpo: string;
  tipo: NotifTipo;
  destinatario: NotifDestinatario;
}

export const notificacionesApi = {
  list: (token: string) => req<Notificacion[]>("/notificaciones", token),

  unreadCount: (token: string) => req<{ count: number }>("/notificaciones/no-leidas", token),

  markRead: (token: string, id: number) => req<void>(`/notificaciones/${id}/leer`, token, { method: "POST" }),

  markAllRead: (token: string) => req<void>("/notificaciones/leer-todo", token, { method: "POST" }),

  // Admin
  adminList: (token: string) => req<Notificacion[]>("/admin/notificaciones", token),

  adminCreate: (token: string, payload: CreateNotificacionPayload) =>
    req<Notificacion>("/admin/notificaciones", token, {
      method: "POST",
      body: JSON.stringify(payload),
    }),

  adminDelete: (token: string, id: number) => req<void>(`/admin/notificaciones/${id}`, token, { method: "DELETE" }),
};
