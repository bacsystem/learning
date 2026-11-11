import { avisarSiSesionExpirada } from "@/lib/http/sesion";

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
    avisarSiSesionExpirada(res.status);
    const body = await res.json().catch(() => ({ error: res.statusText }));
    throw Object.assign(new Error(body.error ?? res.statusText), {
      status: res.status,
      simulacro_id: body.simulacro_id,
      resultado_id: body.resultado_id,
    });
  }
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

/**
 * The server answers 409 {"error":"tiempo_agotado"} when the time limit is
 * over (answers) and, on GET of an expired attempt, also finalizes it and
 * returns resultado_id so the client can go straight to the result.
 */
export function tiempoAgotado(e: unknown): { resultadoId?: number } | null {
  const err = e as { status?: number; message?: string; resultado_id?: number };
  if (err?.status !== 409 || err.message !== "tiempo_agotado") return null;
  return { resultadoId: err.resultado_id };
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

export interface CreateSimulacroPayload {
  proceso_id: number;
}

export interface RespuestaSimulacro {
  id: number;
  texto: string;
  letra: string;
  orden: number;
}

export interface PreguntaSimulacro {
  id: number;
  orden: number;
  enunciado: string;
  tipo: "unica" | "multiple";
  asignatura: string;
  respuestas: RespuestaSimulacro[];
  estado: "sin_responder" | "respondida";
  /** First selected alternative (kept for 'unica'). */
  respuesta_id: number | null;
  /** All selected alternatives (several for 'multiple'). */
  respuesta_ids: number[];
  marcada: boolean;
}

export interface SimulacroEnCurso {
  id: number;
  proceso: string;
  tiempo_limite_seg: number;
  fecha_inicio: string;
  tiempo_restante_seg: number;
  total_preguntas: number;
  preguntas: PreguntaSimulacro[];
}

export interface ResultadoAsignaturaSimulacro {
  asignatura: string;
  total_preguntas: number;
  correctas: number;
  incorrectas: number;
  sin_responder: number;
  puntaje: number;
  porcentaje: number;
  peso: number;
}

export interface PosicionEstimada {
  posicion: number;
  total_usuarios: number;
}

export interface SimulacroResultado {
  id: number;
  proceso: string;
  total_preguntas: number;
  correctas: number;
  incorrectas: number;
  sin_responder: number;
  puntaje_total: number;
  puntaje_maximo: number;
  porcentaje: number;
  nota_centesimal: number;
  aprobado: boolean;
  tiempo_usado_seg: number;
  tiempo_limite_seg: number;
  fecha_inicio: string;
  fecha_fin: string;
  por_asignatura: ResultadoAsignaturaSimulacro[];
  posicion?: PosicionEstimada;
}

export interface RespuestaRevisionSimulacro {
  id: number;
  texto: string;
  letra: string;
  es_correcta: boolean;
}

export interface RevisionPreguntaSimulacro {
  pregunta_id: number;
  tipo: "unica" | "multiple";
  orden: number;
  enunciado: string;
  asignatura: string;
  respuesta_usuario_id: number | null;
  respuesta_correcta_id: number;
  respuesta_usuario_ids: number[];
  respuesta_correcta_ids: number[];
  es_correcta: boolean | null;
  respuestas: RespuestaRevisionSimulacro[];
  explicacion?: string;
  referencia?: string;
}

export interface RevisionSimulacro {
  id: number;
  preguntas: RevisionPreguntaSimulacro[];
}

export interface HistorialSimulacroItem {
  id: number;
  numero: number;
  proceso: string;
  fecha: string;
  puntaje_total: number;
  puntaje_maximo: number;
  /** puntaje_total / puntaje_maximo * 100 */
  porcentaje: number;
  nota_centesimal: number;
  aprobado: boolean;
  tiempo_usado_seg: number;
  estado: string;
}

export interface HistorialSimulacroResponse {
  data: HistorialSimulacroItem[];
  total: number;
  page: number;
  per_page: number;
}

export interface EnCursoResponse {
  en_curso: boolean;
  simulacro_id: number | null;
}

export interface HistorialQuery {
  page?: number;
  per_page?: number;
  proceso_id?: number;
  aprobado?: boolean;
}

export interface AsignaturaConfig {
  asignatura_id: number;
  nombre: string;
  numero_preguntas: number;
  peso_porcentual: number;
  preguntas_disponibles: number;
}

export interface ConfiguracionExamen {
  id: number;
  proceso_id: number;
  proceso: string;
  tiempo_total_seg: number;
  total_preguntas: number;
  nota_minima: number;
  nota_maxima: number;
  coeficiente: number;
  descripcion?: string;
  estado: string;
  asignaturas: AsignaturaConfig[];
}

export interface CreateConfiguracionPayload {
  proceso_id: number;
  tiempo_total_seg: number;
  total_preguntas: number;
  nota_minima?: number;
  nota_maxima?: number;
  descripcion?: string;
}

/** PATCH semantics: absent fields keep their current value. */
export interface UpdateConfiguracionPayload {
  tiempo_total_seg?: number;
  total_preguntas?: number;
  nota_minima?: number;
  nota_maxima?: number;
  descripcion?: string;
  estado?: string;
}

export interface SeleccionResponse {
  respuesta_ids: number[];
}

// ── API Client ─────────────────────────────────────────────────────────────────

export const simulacrosApi = {
  createSimulacro(token: string, payload: CreateSimulacroPayload): Promise<SimulacroEnCurso> {
    return req<SimulacroEnCurso>("/simulacros", token, {
      method: "POST",
      body: JSON.stringify(payload),
    });
  },

  getSimulacro(token: string, id: number): Promise<SimulacroEnCurso> {
    return req<SimulacroEnCurso>(`/simulacros/${id}`, token);
  },

  responderPregunta(
    token: string,
    simulacroId: number,
    preguntaId: number,
    respuestaId: number,
  ): Promise<{ ok: boolean } & SeleccionResponse> {
    return req<{ ok: boolean } & SeleccionResponse>(`/simulacros/${simulacroId}/responder`, token, {
      method: "POST",
      body: JSON.stringify({ pregunta_id: preguntaId, respuesta_id: respuestaId }),
    });
  },

  /** Replaces the whole selection of a question (idempotent; [] clears it). */
  setRespuestas(
    token: string,
    simulacroId: number,
    preguntaId: number,
    respuestaIds: number[],
  ): Promise<SeleccionResponse> {
    return req<SeleccionResponse>(`/simulacros/${simulacroId}/preguntas/${preguntaId}/respuestas`, token, {
      method: "PUT",
      body: JSON.stringify({ respuesta_ids: respuestaIds }),
    });
  },

  finalizarSimulacro(token: string, simulacroId: number): Promise<SimulacroResultado> {
    return req<SimulacroResultado>(`/simulacros/${simulacroId}/finalizar`, token, {
      method: "POST",
      body: JSON.stringify({}),
    });
  },

  getResultado(token: string, simulacroId: number): Promise<SimulacroResultado> {
    return req<SimulacroResultado>(`/simulacros/${simulacroId}/resultado`, token);
  },

  getRevision(token: string, simulacroId: number): Promise<RevisionSimulacro> {
    return req<RevisionSimulacro>(`/simulacros/${simulacroId}/revision`, token);
  },

  getHistorial(token: string, query?: HistorialQuery): Promise<HistorialSimulacroResponse> {
    const qs = buildQuery(query as Record<string, string | number | boolean | undefined | null>);
    return req<HistorialSimulacroResponse>(`/simulacros/historial${qs}`, token);
  },

  getEnCurso(token: string): Promise<EnCursoResponse> {
    return req<EnCursoResponse>("/simulacros/en-curso", token);
  },

  getConfiguracion(token: string, procesoId: number): Promise<{ configuracion: ConfiguracionExamen | null }> {
    return req<{ configuracion: ConfiguracionExamen | null }>(
      `/simulacros/configuracion?proceso_id=${procesoId}`,
      token,
    );
  },

  createConfiguracion(token: string, payload: CreateConfiguracionPayload): Promise<ConfiguracionExamen> {
    return req<ConfiguracionExamen>("/simulacros/configuracion", token, {
      method: "POST",
      body: JSON.stringify(payload),
    });
  },

  listConfiguraciones(token: string): Promise<{ data: ConfiguracionExamen[] }> {
    return req<{ data: ConfiguracionExamen[] }>("/simulacros/configuraciones", token);
  },

  updateConfiguracion(token: string, id: number, payload: UpdateConfiguracionPayload): Promise<ConfiguracionExamen> {
    return req<ConfiguracionExamen>(`/simulacros/configuracion/${id}`, token, {
      method: "PATCH",
      body: JSON.stringify(payload),
    });
  },
};
