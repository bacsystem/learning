import { API_BASE } from "@/lib/http/api-base";
import { apiErrorFrom } from "@/lib/http/api-error";

/** Cliente de fuentes (docs/api/serums-ia-e2-fuentes.md, S1–S7). */

async function req<T>(path: string, token: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers: { Authorization: `Bearer ${token}`, ...options?.headers },
  });
  if (!res.ok) {
    if (res.status === 401) window.dispatchEvent(new Event("auth:expired"));
    throw await apiErrorFrom(res);
  }
  return res.json() as Promise<T>;
}

function json<T>(path: string, token: string, method: string, body: unknown): Promise<T> {
  return req<T>(path, token, { method, headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
}

// ── Interfaces ────────────────────────────────────────────────────────────────

export type EstadoFuente = "procesando" | "activa" | "fallida" | "retirada";
export type EstadoVersion = "procesando" | "lista" | "fallida" | "archivada";
export type MotivoFallo = "sin_contenido" | "limite_pasajes" | "error_interno";
export type TipoFuente = "balotario" | "guia" | "norma" | "otro";

export interface VersionVigente {
  numero: number;
  oficial_verificada: boolean;
  pasajes_count: number;
}

/** Última versión posterior a la vigente que aún no está lista. */
export interface VersionEnProceso {
  numero: number;
  estado: "procesando" | "fallida";
  motivo_fallo: MotivoFallo | null;
}

export interface FuenteResumen {
  id: number;
  titulo: string;
  tipo: TipoFuente;
  estado: EstadoFuente;
  aplica_a_todas: boolean;
  especialidades: { id: number; nombre: string }[];
  version_vigente: VersionVigente | null;
  version_en_proceso: VersionEnProceso | null;
  created_at: string;
  updated_at: string;
}

export interface VersionFuente {
  numero: number;
  estado: EstadoVersion;
  motivo_fallo: MotivoFallo | null;
  nombre_archivo: string;
  sha256: string;
  tamano_bytes: number;
  lineas: number;
  pasajes_count: number;
  oficial_verificada: boolean;
  oficial_entidad: string | null;
  created_at: string;
  procesada_at: string | null;
}

export interface FuenteDetalle extends FuenteResumen {
  anio: number | null;
  proceso: { id: number; nombre: string } | null;
  motivo_retiro: string | null;
  oficial: { entidad_emisora: string; nota: string; marcado_at: string } | null;
  contenidos_citantes: number;
  versiones: VersionFuente[];
  /** Solo en la respuesta de retirar (S5). */
  contenidos_marcados?: number;
}

export interface FuenteList {
  data: FuenteResumen[];
  total: number;
  page: number;
  per_page: number;
}

export interface FuenteListQuery {
  estado?: EstadoFuente;
  tipo?: TipoFuente;
  especialidad_id?: number;
  oficial?: boolean;
  q?: string;
  page?: number;
  per_page?: number;
}

export interface NuevaFuente {
  archivo: File;
  titulo: string;
  tipo: TipoFuente;
  especialidadIds: number[];
  anio?: number | null;
  procesoId?: number | null;
}

/** Cuerpo de S5: retirar (con motivo), restaurar o reintentar. */
export type CambioEstadoFuente =
  | { estado: "retirada"; motivo: string }
  | { estado: "activa" }
  | { estado: "procesando" };

// ── Armado de los formularios multipart ───────────────────────────────────────

/**
 * El `.md` va como `Blob` `text/markdown` con su nombre: algunos sistemas mandan `.md` con tipo vacío o
 * `application/octet-stream`, y el servidor exige `text/markdown` o `text/plain` (S1, paso 4).
 */
function adjuntarArchivo(form: FormData, archivo: File) {
  form.append("file", new Blob([archivo], { type: "text/markdown" }), archivo.name);
}

/** S1: `especialidad_ids` va como campo repetido; `anio` y `proceso_id` se omiten si están vacíos. */
export function formularioNuevaFuente(f: NuevaFuente): FormData {
  const form = new FormData();
  adjuntarArchivo(form, f.archivo);
  form.append("titulo", f.titulo.trim());
  form.append("tipo", f.tipo);
  for (const id of f.especialidadIds) form.append("especialidad_ids", String(id));
  if (f.anio) form.append("anio", String(f.anio));
  if (f.procesoId) form.append("proceso_id", String(f.procesoId));
  return form;
}

/** S2: ruta fija con `fuente_id` en el formulario (el límite de 20 MB solo aplica a rutas sin `{id}`). */
export function formularioNuevaVersion(fuenteId: number, archivo: File): FormData {
  const form = new FormData();
  form.append("fuente_id", String(fuenteId));
  adjuntarArchivo(form, archivo);
  return form;
}

function buildQuery(q: FuenteListQuery): string {
  const p = new URLSearchParams();
  for (const [k, v] of Object.entries(q)) if (v !== undefined && v !== "") p.set(k, String(v));
  const s = p.toString();
  return s ? `?${s}` : "";
}

// ── API Client ────────────────────────────────────────────────────────────────

export const fuentesApi = {
  /** S1. 202 con la fuente en `procesando`. Sin `Content-Type`: el navegador pone el `boundary` del multipart. */
  crear(token: string, fuente: NuevaFuente): Promise<FuenteDetalle> {
    return req<FuenteDetalle>("/admin/fuentes", token, { method: "POST", body: formularioNuevaFuente(fuente) });
  },

  /** S2. */
  nuevaVersion(token: string, fuenteId: number, archivo: File): Promise<FuenteDetalle> {
    return req<FuenteDetalle>("/admin/fuentes/versiones", token, {
      method: "POST",
      body: formularioNuevaVersion(fuenteId, archivo),
    });
  },

  /** S3. */
  listar(token: string, query: FuenteListQuery): Promise<FuenteList> {
    return req<FuenteList>(`/admin/fuentes${buildQuery(query)}`, token);
  },

  /** S4. */
  detalle(token: string, id: number): Promise<FuenteDetalle> {
    return req<FuenteDetalle>(`/admin/fuentes/${id}`, token);
  },

  /** S5. */
  cambiarEstado(token: string, id: number, cambio: CambioEstadoFuente): Promise<FuenteDetalle> {
    return json<FuenteDetalle>(`/admin/fuentes/${id}/estado`, token, "PATCH", cambio);
  },

  /** S6, marcar la versión vigente como oficial verificada. */
  marcarOficial(token: string, id: number, entidadEmisora: string, nota: string): Promise<FuenteDetalle> {
    return json<FuenteDetalle>(`/admin/fuentes/${id}/oficial`, token, "PATCH", {
      oficial_verificada: true,
      entidad_emisora: entidadEmisora.trim(),
      nota: nota.trim(),
    });
  },

  /** S6, desmarcar (con motivo). */
  desmarcarOficial(token: string, id: number, motivo: string): Promise<FuenteDetalle> {
    return json<FuenteDetalle>(`/admin/fuentes/${id}/oficial`, token, "PATCH", {
      oficial_verificada: false,
      motivo: motivo.trim(),
    });
  },

  /** S7: reemplaza título y profesiones (vacío = todas). */
  editar(token: string, id: number, titulo: string, especialidadIds: number[]): Promise<FuenteDetalle> {
    return json<FuenteDetalle>(`/admin/fuentes/${id}`, token, "PUT", {
      titulo: titulo.trim(),
      especialidad_ids: especialidadIds,
    });
  },
};
