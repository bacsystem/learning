import type { FuenteResumen, TipoFuente } from "./api";
import * as yup from "yup";

/** Límites del contrato (docs/api/serums-ia-e2-fuentes.md). El servidor los vuelve a aplicar. */
export const ARCHIVO_MAX_BYTES = 2 * 1024 * 1024; // 2 MiB (S1 paso 1)
export const LINEAS_MAX = 20_000; // S1 paso 6 (solo lo valida el servidor)
export const TITULO_MIN = 3;
export const TITULO_MAX = 150;
export const ESPECIALIDADES_MAX = 50;
export const ANIO_MIN = 1900;
export const ANIO_MAX = 2100;
export const MOTIVO_MIN = 5;
export const MOTIVO_MAX = 500;
export const ENTIDAD_MIN = 3;
export const ENTIDAD_MAX = 100;
export const NOTA_MIN = 10;
export const NOTA_MAX = 500;
export const BUSQUEDA_MAX = 100;
export const VERSIONES_MAX = 20;
/** Cada cuánto se vuelve a pedir la página mientras hay algo en proceso (E2-07 regla 6). */
export const REFRESCO_MS = 5_000;
export const POR_PAGINA_FUENTES = 20;

export const TIPOS: readonly TipoFuente[] = ["balotario", "guia", "norma", "otro"];

export type ErrorArchivo = "requerido" | "extension" | "tamano";

/** Validación en el cliente antes de enviar (E2-07, «Validación en el cliente»). Arrastrar un archivo se salta `accept`. */
export function validarArchivo(archivo: File | null): ErrorArchivo | null {
  if (!archivo) return "requerido";
  if (!archivo.name.toLowerCase().endsWith(".md")) return "extension";
  if (archivo.size > ARCHIVO_MAX_BYTES) return "tamano";
  return null;
}

/** Hay algo que el worker aún procesa: la fuente o una versión nueva de una fuente activa. */
export function enProceso(f: Pick<FuenteResumen, "estado" | "version_en_proceso">): boolean {
  return f.estado === "procesando" || f.version_en_proceso?.estado === "procesando";
}

export type AccionFuente =
  | "nuevaVersion"
  | "editar"
  | "retirar"
  | "restaurar"
  | "reintentar"
  | "marcarOficial"
  | "desmarcarOficial";

/** Acciones que admite una fuente en su estado actual (S2, S5, S6, S7). */
export function accionesDisponibles(f: Pick<FuenteResumen, "estado" | "version_en_proceso" | "version_vigente">) {
  const versionProcesando = f.version_en_proceso?.estado === "procesando";
  const versionFallida = f.version_en_proceso?.estado === "fallida";
  const marcada = f.version_vigente?.oficial_verificada === true;
  const acciones = new Set<AccionFuente>();
  if ((f.estado === "activa" && !versionProcesando) || f.estado === "fallida") acciones.add("nuevaVersion");
  if (f.estado !== "procesando") acciones.add("editar");
  if (f.estado === "activa" && !versionProcesando) acciones.add("retirar");
  if (f.estado === "retirada") acciones.add("restaurar");
  if (f.estado === "fallida" || (f.estado === "activa" && versionFallida)) acciones.add("reintentar");
  if (f.estado === "activa" && f.version_vigente && !marcada) acciones.add("marcarOficial");
  if ((f.estado === "activa" || f.estado === "retirada") && marcada) acciones.add("desmarcarOficial");
  return acciones;
}

// ── Esquemas de los formularios ───────────────────────────────────────────────

/** Longitud del texto recortado: el servidor aplica `TrimSpace` antes de contar. */
const largo = (v: string | undefined) => (v ?? "").trim().length;
const entre = (min: number, max: number) => (v: string | undefined) => largo(v) >= min && largo(v) <= max;

export interface MensajesFuente {
  titulo: string;
  tipo: string;
  anio: string;
  especialidadesMax: string;
  archivo: Record<ErrorArchivo, string>;
  motivo: string;
  entidad: string;
  nota: string;
}

const titulo = (m: MensajesFuente) => yup.string().test("largo", m.titulo, entre(TITULO_MIN, TITULO_MAX));
const especialidades = (m: MensajesFuente) =>
  yup.array(yup.number().required()).max(ESPECIALIDADES_MAX, m.especialidadesMax).required();

/** S1: archivo, título, tipo, profesiones, año y proceso opcionales. */
export function esquemaSubida(m: MensajesFuente) {
  return yup.object({
    archivo: yup
      .mixed<File>()
      .nullable()
      .test("archivo", (v, ctx) => {
        const error = validarArchivo(v ?? null);
        return error ? ctx.createError({ message: m.archivo[error] }) : true;
      }),
    titulo: titulo(m),
    tipo: yup
      .string()
      .oneOf([...TIPOS], m.tipo)
      .required(m.tipo),
    especialidad_ids: especialidades(m),
    anio: yup
      .string()
      .test("anio", m.anio, (v) => !v || (/^\d+$/.test(v) && Number(v) >= ANIO_MIN && Number(v) <= ANIO_MAX)),
    proceso_id: yup.string(),
  });
}

/** S2: solo el archivo. */
export function esquemaVersion(m: MensajesFuente) {
  return yup.object({ archivo: esquemaSubida(m).fields.archivo });
}

/** S7: título y profesiones. */
export function esquemaEdicion(m: MensajesFuente) {
  return yup.object({ titulo: titulo(m), especialidad_ids: especialidades(m) });
}

/** S5 retirar y S6 desmarcar: motivo de 5–500. */
export function esquemaMotivo(m: MensajesFuente) {
  return yup.object({ motivo: yup.string().test("largo", m.motivo, entre(MOTIVO_MIN, MOTIVO_MAX)) });
}

/** S6 marcar: entidad emisora 3–100 y nota 10–500. */
export function esquemaMarca(m: MensajesFuente) {
  return yup.object({
    entidad_emisora: yup.string().test("largo", m.entidad, entre(ENTIDAD_MIN, ENTIDAD_MAX)),
    nota: yup.string().test("largo", m.nota, entre(NOTA_MIN, NOTA_MAX)),
  });
}
