import { API_BASE } from "@/lib/http/api-base";
import { apiErrorFrom } from "@/lib/http/api-error";
import { avisarSiSesionExpirada } from "@/lib/http/sesion";

/** Cliente de la importación del balotario (docs/api/serums-ia-e3-01-e3-03-balotario.md §3 y §4). */

/** Motivos de «con problemas» (contrato §2.4 más `sin_pasaje`, que añade la importación). Cada uno tiene su texto en i18n. */
export const MOTIVOS_PROBLEMA = [
  "numeracion_rota",
  "enunciado_vacio",
  "enunciado_largo",
  "alternativas_insuficientes",
  "alternativas_excedidas",
  "etiquetas_ambiguas",
  "alternativa_vacia",
  "alternativa_larga",
  "alternativas_duplicadas",
  "sin_clave",
  "sin_pasaje",
] as const;
export type MotivoProblema = (typeof MOTIVOS_PROBLEMA)[number];
export type AlertaEstructura = "alternativas_separadas" | "vinetas_normalizadas" | "html_eliminado";
export type RutaRevision = "estructural" | "verificacion_clave";
export type MotivoNoConfirmable = "sin_validas" | "ya_importado";

export interface AlternativaBalotario {
  letra: string;
  texto: string;
  correcta: boolean;
}

export interface PreguntaBalotario {
  numero: number;
  linea_inicio: number;
  linea_fin: number;
  tipo: "unica" | "multiple";
  enunciado: string;
  alternativas: AlternativaBalotario[];
  alertas_estructura: AlertaEstructura[];
}

export interface ProblemaBalotario {
  numero: number | null;
  linea_inicio: number;
  linea_fin: number;
  motivos: MotivoProblema[];
}

export interface ImportacionBalotario {
  id: number;
  creadas: number;
  created_at: string;
}

export interface OtraImportacion {
  version: number;
  creadas: number;
  created_at: string;
}

export interface VistaPreviaBalotario {
  fuente: {
    id: number;
    titulo: string;
    version: number;
    oficial_verificada: boolean;
    oficial_entidad: string | null;
  };
  ruta_revision: RutaRevision;
  totales: { validas: number; con_problemas: number; unica: number; multiple: number; con_alertas: number };
  /** Las primeras 20 válidas, sin paginar. */
  preguntas: PreguntaBalotario[];
  problemas: { data: ProblemaBalotario[]; total: number; page: number; per_page: number };
  importacion: ImportacionBalotario | null;
  otras_importaciones: OtraImportacion[];
  puede_confirmar: boolean;
  motivo_no_confirmable: MotivoNoConfirmable | null;
}

/** Cuerpo de `POST …/importaciones`: todos los campos son obligatorios; `asignatura_id` es el área inicial provisional. */
export interface ConfirmarImportacion {
  version: number;
  proceso_id: number;
  especialidad_id: number;
  asignatura_id: number;
}

export interface ImportacionCreada {
  importacion_id: number;
  fuente_id: number;
  version: number;
  creadas: number;
  con_problemas: number;
  unica: number;
  multiple: number;
  con_alertas: number;
}

export const PROBLEMAS_POR_PAGINA = 20;

async function req<T>(path: string, token: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers: { Authorization: `Bearer ${token}`, ...options?.headers },
  });
  if (!res.ok) {
    avisarSiSesionExpirada(res.status);
    throw await apiErrorFrom(res);
  }
  return res.json() as Promise<T>;
}

export const balotarioApi = {
  /** `GET /admin/fuentes/{id}/balotario`: analiza la versión vigente sin guardar nada. `page` pagina solo `problemas`. */
  vistaPrevia(
    token: string,
    fuenteId: number,
    page = 1,
    perPage = PROBLEMAS_POR_PAGINA,
  ): Promise<VistaPreviaBalotario> {
    const qs = new URLSearchParams({ page: String(page), per_page: String(perPage) });
    return req<VistaPreviaBalotario>(`/admin/fuentes/${fuenteId}/balotario?${qs}`, token);
  },

  /** `POST /admin/fuentes/{id}/balotario/importaciones`: crea las preguntas válidas como borradores. */
  confirmar(token: string, fuenteId: number, cuerpo: ConfirmarImportacion): Promise<ImportacionCreada> {
    return req<ImportacionCreada>(`/admin/fuentes/${fuenteId}/balotario/importaciones`, token, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(cuerpo),
    });
  },
};
