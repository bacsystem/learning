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
    if (res.status === 401) window.dispatchEvent(new Event("auth:expired"));
    const body = await res.json().catch(() => ({ error: res.statusText }));
    throw Object.assign(new Error(body.error ?? res.statusText), {
      status: res.status,
      practica_id: body.practica_id,
      resultado_id: body.resultado_id,
    });
  }
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

/** 409 error codes of the práctica endpoints (body.error). */
export function errorCode(e: unknown): string | undefined {
  const err = e as { status?: number; message?: string };
  return err?.status === 409 ? err.message : undefined;
}

/**
 * 409 tiempo_agotado: the time limit is over. On GET of an expired práctica the
 * server also finalizes it and returns resultado_id.
 */
export function tiempoAgotado(e: unknown): { resultadoId?: number } | null {
  if (errorCode(e) !== "tiempo_agotado") return null;
  return { resultadoId: (e as { resultado_id?: number }).resultado_id };
}

function buildQuery(params: Record<string, string | number | boolean | undefined | null>): string {
  const parts: string[] = [];
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== null && value !== "") {
      parts.push(`${encodeURIComponent(key)}=${encodeURIComponent(String(value))}`);
    }
  }
  return parts.length > 0 ? `?${parts.join("&")}` : "";
}

// ── Interfaces ─────────────────────────────────────────────────────────────────

export interface AsignaturaItem {
  asignatura_id: number;
  numero_preguntas: number;
}

export interface CreatePracticaPayload {
  proceso_id: number;
  asignaturas: AsignaturaItem[];
  tiempo_limite_seg?: number | null;
  modo_calificacion: "inmediato" | "al_final";
}

export interface RespuestaPractica {
  id: number;
  texto: string;
  letra: string;
  orden: number;
}

export interface PreguntaPractica {
  id: number;
  orden: number;
  enunciado: string;
  tipo: "unica" | "multiple";
  asignatura: string;
  respuestas: RespuestaPractica[];
  estado: "sin_responder" | "respondida" | "correcta" | "incorrecta";
  respuesta_ids: number[];
  /** Checked in inmediato mode: the selection is locked by the server. */
  validada: boolean;
  /** Only for validated questions (the key was already revealed). */
  respuesta_correcta_ids?: number[];
  explicacion?: string;
}

export interface PracticaEnCurso {
  id: number;
  proceso: string;
  modo_calificacion: "inmediato" | "al_final";
  tiempo_limite_seg: number | null;
  fecha_inicio: string;
  /** Seconds left by the server clock (null when untimed). */
  tiempo_restante_seg: number | null;
  total_preguntas: number;
  preguntas: PreguntaPractica[];
}

export interface ResponderResponse {
  respuesta_ids: number[];
}

export interface ValidarPreguntaResponse {
  es_correcta: boolean;
  /** Only for tipo 'unica' (legacy). */
  respuesta_correcta_id?: number;
  respuesta_correcta_ids: number[];
  explicacion: string;
}

/** Server-side cap on the ids of one batch delete (422 above it). */
export const MAX_ELIMINAR_PRACTICAS = 100;

/** Server-side cap on the total number of questions of a práctica (422 above it). */
export const MAX_PREGUNTAS_PRACTICA = 200;

export interface ResultadoAsignatura {
  asignatura: string;
  total_preguntas: number;
  correctas: number;
  incorrectas: number;
  sin_responder: number;
  porcentaje: number;
}

export interface PracticaResultado {
  id: number;
  estado: string;
  proceso: string;
  total_preguntas: number;
  correctas: number;
  incorrectas: number;
  sin_responder: number;
  porcentaje: number;
  tiempo_usado_seg: number;
  tiempo_limite_seg: number | null;
  fecha_inicio: string;
  fecha_fin: string;
  por_asignatura: ResultadoAsignatura[];
}

export interface RespuestaRevision {
  id: number;
  texto: string;
  letra: string;
  es_correcta: boolean;
}

export interface RevisionPregunta {
  orden: number;
  enunciado: string;
  asignatura: string;
  respuesta_usuario_ids: number[];
  respuesta_correcta_ids: number[];
  es_correcta: boolean | null;
  respuestas: RespuestaRevision[];
  explicacion: string;
  referencia: string;
}

export interface RevisionPractica {
  id: number;
  preguntas: RevisionPregunta[];
}

export interface HistorialItem {
  id: number;
  numero: number;
  asignaturas: string;
  total_preguntas: number;
  correctas: number;
  porcentaje: number;
  tiempo_usado_seg: number;
  fecha: string;
  estado: "completada" | "abandonada";
}

export interface HistorialResponse {
  data: HistorialItem[];
  total: number;
  page: number;
  per_page: number;
}

export interface HistorialQuery {
  page?: number;
  per_page?: number;
  estado?: string;
}

// ── API Client ─────────────────────────────────────────────────────────────────

export const practicasApi = {
  createPractica(token: string, payload: CreatePracticaPayload): Promise<PracticaEnCurso> {
    return req<PracticaEnCurso>("/practicas", token, {
      method: "POST",
      body: JSON.stringify(payload),
    });
  },

  getPractica(token: string, id: number): Promise<PracticaEnCurso> {
    return req<PracticaEnCurso>(`/practicas/${id}`, token);
  },

  toggleRespuesta(
    token: string,
    practicaId: number,
    preguntaId: number,
    respuestaId: number,
  ): Promise<ResponderResponse> {
    return req<ResponderResponse>(`/practicas/${practicaId}/responder`, token, {
      method: "POST",
      body: JSON.stringify({ pregunta_id: preguntaId, respuesta_id: respuestaId }),
    });
  },

  validarPregunta(token: string, practicaId: number, preguntaId: number): Promise<ValidarPreguntaResponse> {
    return req<ValidarPreguntaResponse>(`/practicas/${practicaId}/validar/${preguntaId}`, token, { method: "POST" });
  },

  finalizarPractica(token: string, practicaId: number, payload?: { abandonar?: boolean }): Promise<PracticaResultado> {
    return req<PracticaResultado>(`/practicas/${practicaId}/finalizar`, token, {
      method: "POST",
      body: JSON.stringify(payload ?? {}),
    });
  },

  getResultado(token: string, practicaId: number): Promise<PracticaResultado> {
    return req<PracticaResultado>(`/practicas/${practicaId}/resultado`, token);
  },

  getRevision(token: string, practicaId: number): Promise<RevisionPractica> {
    return req<RevisionPractica>(`/practicas/${practicaId}/revision`, token);
  },

  listHistorial(token: string, query?: HistorialQuery): Promise<HistorialResponse> {
    const qs = buildQuery(query as Record<string, string | number | boolean | undefined | null>);
    return req<HistorialResponse>(`/practicas${qs}`, token);
  },

  deletePractica(token: string, id: number): Promise<void> {
    return req<void>(`/practicas/${id}`, token, { method: "DELETE" });
  },

  batchDeletePracticas(token: string, ids: number[]): Promise<void> {
    return req<void>("/practicas/batch", token, {
      method: "DELETE",
      body: JSON.stringify({ ids }),
    });
  },
};
