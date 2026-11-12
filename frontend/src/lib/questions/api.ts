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

async function reqMultipart<T>(path: string, token: string, form: FormData): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${token}`,
    },
    body: form,
  });
  if (!res.ok) {
    avisarSiSesionExpirada(res.status);
    throw await apiErrorFrom(res);
  }
  return res.json() as Promise<T>;
}

function buildQuery(params: Record<string, string | number | boolean | undefined | null> | object): string {
  const parts: string[] = [];
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== null && value !== "") {
      parts.push(`${encodeURIComponent(key)}=${encodeURIComponent(String(value))}`);
    }
  }
  return parts.length > 0 ? `?${parts.join("&")}` : "";
}

// ── Interfaces ────────────────────────────────────────────────────────────────

export interface RespuestaItem {
  id: number;
  texto: string;
  es_correcta: boolean;
  orden: number;
}

export interface Retroalimentacion {
  explicacion: string;
  referencia?: string | null;
}

export interface Estadistica {
  total_intentos: number;
  total_aciertos: number;
  indice_dificultad: number;
  indice_discriminacion: number;
}

/** Ciclo de vida (SIA-E3-02, docs/api/serums-ia-e3-02-ciclo-preguntas.md §3). `activo` es el banco heredado. */
export type EstadoPregunta = "borrador" | "en_revision" | "aprobada" | "rechazada" | "activo" | "eliminada";
export type OrigenPregunta = "manual" | "balotario" | "ia";

/** Cita a un pasaje de una fuente (§4.4); solo en el detalle (`GET /preguntas/{id}`). */
export interface CitaPregunta {
  pasaje_id: number;
  fuente_id: number;
  version: number;
  fuente_titulo: string;
  fuente_estado: string;
  /** SIA-E3-07 §5: ubicación y texto del pasaje citado (`texto` es el pasaje completo, en texto plano). */
  seccion: string | null;
  linea_inicio: number;
  linea_fin: number;
  extracto: string;
  texto: string;
}

/** Cuerpo de `PATCH /preguntas/{id}/estado` (SIA-E3-07 §3.1). */
export interface CambiarEstadoPayload {
  estado: "aprobada" | "rechazada" | "borrador" | "en_revision";
  motivo?: string | null;
  verificado?: boolean | null;
  nota?: string | null;
}

export interface Pregunta {
  id: number;
  proceso_id: number;
  proceso: string;
  asignatura_id: number;
  asignatura: string;
  grado?: string | null;
  especialidad?: string | null;
  /** Detail view only (`GET /preguntas/{id}`); undefined on older backends. */
  grado_id?: number | null;
  especialidad_id?: number | null;
  enunciado: string;
  tipo: "unica" | "multiple";
  dificultad: "facil" | "medio" | "dificil";
  estado: EstadoPregunta;
  /** SIA-E3-02 §4.4. Opcionales hasta que el backend de E3-02 esté desplegado. */
  origen?: OrigenPregunta;
  motivo_rechazo?: string | null;
  /** Solo en el detalle. */
  citas?: CitaPregunta[];
  /** Solo en el listado. */
  citas_count?: number;
  etiquetas: string[];
  /** Ids of `etiquetas` (detail view; empty in list views). */
  etiqueta_ids: number[];
  respuestas: RespuestaItem[];
  retroalimentacion?: Retroalimentacion | null;
  estadisticas?: Estadistica | null;
  created_at: string;
}

export interface PreguntaList {
  data: Pregunta[];
  total: number;
  page: number;
  per_page: number;
}

export interface Etiqueta {
  id: number;
  asignatura_id: number;
  asignatura: string;
  nombre: string;
  estado: "activo" | "inactivo";
}

export interface CreateRespuestaPayload {
  /** Existing alternative id on update: keeps it (and its attempts) in place. */
  id?: number;
  texto: string;
  es_correcta: boolean;
  orden: number;
}

export interface CreateRetroPayload {
  explicacion: string;
  referencia?: string | null;
}

export interface CreatePreguntaPayload {
  proceso_id: number;
  asignatura_id: number;
  grado_id?: number | null;
  especialidad_id?: number | null;
  enunciado: string;
  tipo: "unica" | "multiple";
  dificultad: "facil" | "medio" | "dificil";
  etiqueta_ids: number[];
  respuestas: CreateRespuestaPayload[];
  retroalimentacion?: CreateRetroPayload | null;
}

export interface UpdatePreguntaPayload {
  proceso_id?: number;
  asignatura_id: number;
  grado_id?: number | null;
  especialidad_id?: number | null;
  enunciado: string;
  tipo: "unica" | "multiple";
  dificultad: "facil" | "medio" | "dificil";
  etiqueta_ids: number[];
  respuestas: CreateRespuestaPayload[];
  retroalimentacion?: CreateRetroPayload | null;
}

export interface AsignaturaSetItem {
  asignatura_id: number;
  numero_preguntas: number;
  dificultad?: "facil" | "medio" | "dificil";
}

export interface GenerateSetPayload {
  proceso_id: number;
  grado_id?: number | null;
  especialidad_id?: number | null;
  asignaturas: AsignaturaSetItem[];
  excluir_ids?: number[];
}

export interface RespuestaExamen {
  id: number;
  texto: string;
  orden: number;
}

export interface PreguntaExamenItem {
  id: number;
  enunciado: string;
  tipo: string;
  asignatura: string;
  respuestas: RespuestaExamen[];
  orden: number;
}

export interface GenerateSetResponse {
  preguntas: PreguntaExamenItem[];
  total: number;
}

export interface ImportErrorRow {
  row: number;
  message: string;
}

export interface ImportResult {
  imported: number;
  errors: ImportErrorRow[];
}

export interface PreviewRow {
  row: number;
  enunciado: string;
  tipo: string;
  dificultad: string;
  grado_id: number | null;
  especialidad_id: number | null;
  num_respuestas: number;
  has_retro: boolean;
  estado: "valido" | "error";
  error?: string;
}

export interface PreviewResponse {
  rows: PreviewRow[];
  total_valido: number;
  total_error: number;
}

export interface PorDificultad {
  facil: number;
  medio: number;
  dificil: number;
}

export interface PorTipo {
  unica: number;
  multiple: number;
}

export interface PorProcesoItem {
  proceso: string;
  total: number;
}

export interface StatsGlobales {
  total_preguntas: number;
  por_dificultad: PorDificultad;
  por_tipo: PorTipo;
  por_proceso: PorProcesoItem[];
}

export interface PreguntaListQuery {
  proceso_id?: number;
  asignatura_id?: number;
  grado_id?: number;
  especialidad_id?: number;
  dificultad?: string;
  etiqueta_id?: number;
  /** Valor exacto; sin filtro salen todas las no eliminadas (`eliminada` no se lista). */
  estado?: Exclude<EstadoPregunta, "eliminada">;
  origen?: OrigenPregunta;
  search?: string;
  page?: number;
  per_page?: number;
}

export interface EtiquetaListQuery {
  asignatura_id?: number;
  estado?: string;
}

// ── API Client ────────────────────────────────────────────────────────────────

export const questionsApi = {
  // Preguntas
  listPreguntas(token: string, query?: PreguntaListQuery): Promise<PreguntaList> {
    const qs = buildQuery(query ?? {});
    return req<PreguntaList>(`/preguntas${qs}`, token);
  },

  getPregunta(token: string, id: number): Promise<Pregunta> {
    return req<Pregunta>(`/preguntas/${id}`, token);
  },

  createPregunta(token: string, payload: CreatePreguntaPayload): Promise<Pregunta> {
    return req<Pregunta>("/preguntas", token, {
      method: "POST",
      body: JSON.stringify(payload),
    });
  },

  updatePregunta(token: string, id: number, payload: UpdatePreguntaPayload): Promise<Pregunta> {
    return req<Pregunta>(`/preguntas/${id}`, token, {
      method: "PUT",
      body: JSON.stringify(payload),
    });
  },

  deletePregunta(token: string, id: number): Promise<void> {
    return req<void>(`/preguntas/${id}`, token, { method: "DELETE" });
  },

  /** `PATCH /preguntas/{id}/estado` (SIA-E3-07 §3). Sin pantalla hasta SIA-E3-09. */
  cambiarEstado(token: string, id: number, payload: CambiarEstadoPayload): Promise<Pregunta> {
    return req<Pregunta>(`/preguntas/${id}/estado`, token, { method: "PATCH", body: JSON.stringify(payload) });
  },

  /** `PUT /preguntas/{id}/citas` (SIA-E3-07 §4): la lista reemplaza el conjunto de citas, siempre completa. */
  reemplazarCitas(token: string, id: number, pasajeIds: number[]): Promise<Pregunta> {
    return req<Pregunta>(`/preguntas/${id}/citas`, token, {
      method: "PUT",
      body: JSON.stringify({ pasaje_ids: pasajeIds }),
    });
  },

  getStats(token: string): Promise<StatsGlobales> {
    return req<StatsGlobales>("/preguntas/stats", token);
  },

  generateSet(token: string, payload: GenerateSetPayload): Promise<GenerateSetResponse> {
    return req<GenerateSetResponse>("/preguntas/generate-set", token, {
      method: "POST",
      body: JSON.stringify(payload),
    });
  },

  previewPreguntas(token: string, file: File): Promise<PreviewResponse> {
    const form = new FormData();
    form.append("file", file);
    return reqMultipart<PreviewResponse>("/preguntas/preview", token, form);
  },

  importPreguntas(token: string, file: File, procesoId: number, asignaturaId: number): Promise<ImportResult> {
    const form = new FormData();
    form.append("file", file);
    form.append("proceso_id", String(procesoId));
    form.append("asignatura_id", String(asignaturaId));
    return reqMultipart<ImportResult>("/preguntas/import", token, form);
  },

  // Etiquetas
  listEtiquetas(token: string, query?: EtiquetaListQuery): Promise<Etiqueta[]> {
    const qs = buildQuery(query ?? {});
    return req<Etiqueta[]>(`/etiquetas${qs}`, token);
  },

  createEtiqueta(token: string, payload: { asignatura_id: number; nombre: string }): Promise<Etiqueta> {
    return req<Etiqueta>("/etiquetas", token, {
      method: "POST",
      body: JSON.stringify(payload),
    });
  },

  updateEtiqueta(token: string, id: number, payload: { nombre?: string; estado?: string }): Promise<Etiqueta> {
    return req<Etiqueta>(`/etiquetas/${id}`, token, {
      method: "PUT",
      body: JSON.stringify(payload),
    });
  },

  deleteEtiqueta(token: string, id: number): Promise<void> {
    return req<void>(`/etiquetas/${id}`, token, { method: "DELETE" });
  },
};
