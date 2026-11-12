import { API_BASE } from "@/lib/http/api-base";
import { apiErrorFrom } from "@/lib/http/api-error";
import { avisarSiSesionExpirada } from "@/lib/http/sesion";

/** Cliente de la búsqueda de pasajes del admin (docs/api/serums-ia-e3-07-revision-basica.md §2). */

export interface PasajeEncontrado {
  pasaje_id: number;
  fuente_id: number;
  fuente_titulo: string;
  version: number;
  seccion: string | null;
  linea_inicio: number;
  linea_fin: number;
  /** Texto plano de hasta 300 caracteres. */
  extracto: string;
}

export const pasajesApi = {
  /** `GET /admin/pasajes?q=`: hasta 20 pasajes de fuentes activas, por relevancia. `q` debe tener 3–300 caracteres. */
  async buscar(token: string, consulta: string, signal?: AbortSignal): Promise<PasajeEncontrado[]> {
    const res = await fetch(`${API_BASE}/admin/pasajes?q=${encodeURIComponent(consulta)}`, {
      headers: { Authorization: `Bearer ${token}` },
      signal,
    });
    if (!res.ok) {
      avisarSiSesionExpirada(res.status);
      throw await apiErrorFrom(res);
    }
    const cuerpo = (await res.json()) as { data?: PasajeEncontrado[] };
    return cuerpo.data ?? [];
  },
};
