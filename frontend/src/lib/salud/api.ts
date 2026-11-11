import { API_BASE } from "@/lib/http/api-base";
import { apiErrorFrom } from "@/lib/http/api-error";
import { avisarSiSesionExpirada } from "@/lib/http/sesion";

/** Cliente del panel de salud (docs/api/plataforma-salud.md, S1 `GET /admin/salud`). */

/** Cuerpo tal como llega de la API: entrada externa, se valida con `normalizarSalud` antes de mostrarla. */
export interface RespuestaSaludApi {
  estado: unknown;
  comprobado_en: unknown;
  servicios: unknown;
}

async function req<T>(path: string, token: string): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, { headers: { Authorization: `Bearer ${token}` } });
  if (!res.ok) {
    // Solo un 401 cierra la sesión: el 503 de la BD caída se muestra como «No se pudo consultar» (PLT-02 regla 5).
    avisarSiSesionExpirada(res.status);
    throw await apiErrorFrom(res);
  }
  return res.json() as Promise<T>;
}

export const saludApi = {
  consultar: (token: string) => req<RespuestaSaludApi>("/admin/salud", token),
};
