"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { type FormEvent, type ReactNode, useEffect, useRef, useState } from "react";

import { Alerta } from "@/components/feedback/alerta";
import { AreaTexto, Campo, Entrada } from "@/components/formularios/campo";
import { Casilla } from "@/components/formularios/casilla";
import { type OpcionSelect, SelectCampo } from "@/components/formularios/select-campo";
import { BotonAsync } from "@/components/patrones/boton-async";
import { CabeceraSeccion } from "@/components/patrones/cabecera-seccion";
import { Chip } from "@/components/patrones/pill-estado";
import { Button } from "@/components/ui/button";
import { excede, LIMITES } from "@/lib/admin/limites";
import type { Especialidad, Grado, ProcesoAsignaturaItem } from "@/lib/catalogs/api";
import { ACCION_SECUNDARIA, BOTON_PRIMARIO, BOTON_SECUNDARIO, TARJETA } from "@/lib/estilos";
import type { Etiqueta } from "@/lib/questions/api";
import { cn } from "@/lib/utils";
import {
  ArrowDownIcon,
  ArrowUpIcon,
  CheckCircleIcon,
  ListChecksIcon,
  NotePencilIcon,
  PlusIcon,
  SlidersHorizontalIcon,
  TagIcon,
  XIcon,
} from "@phosphor-icons/react";

export interface RespuestaRow {
  /** Existing alternative id (edit); absent for alternatives added in this form. */
  id?: number;
  /** Stable React key that follows the row when it is moved or another is removed. */
  key?: string;
  texto: string;
  es_correcta: boolean;
  orden: number;
}

export type TipoPregunta = "unica" | "multiple";
export type DificultadPregunta = "facil" | "medio" | "dificil";

export const MIN_RESPUESTAS = 2;
export const MAX_RESPUESTAS = 6;
const MAX_ENUNCIADO = 2000;
const LETRAS = ["A", "B", "C", "D", "E", "F"];

/** El proceso se elige (nueva pregunta) o es fijo (edición: no modificable). */
export type CampoProceso =
  | { modo: "elegir"; valor: number; opciones: { id: number; nombre: string }[]; onCambio: (id: number) => void }
  | { modo: "fijo"; nombre: string };

/** Tarjeta de una sección del formulario. */
function Seccion({
  icon,
  titulo,
  subtitulo,
  derecha,
  children,
}: {
  icon: typeof NotePencilIcon;
  titulo: ReactNode;
  subtitulo?: ReactNode;
  derecha?: ReactNode;
  children: ReactNode;
}) {
  return (
    <section className={TARJETA}>
      <CabeceraSeccion icon={icon} titulo={titulo} subtitulo={subtitulo} derecha={derecha} />
      <div className="grid gap-4 px-4 pt-2 pb-4">{children}</div>
    </section>
  );
}

/**
 * Formulario de una pregunta del banco (nueva y edición): clasificación, enunciado, etiquetas, alternativas con la(s)
 * correcta(s) y retroalimentación. El estado vive en la página; aquí solo se pinta y se valida lo mínimo antes de enviar.
 */
export function EditorPregunta({
  proceso,
  asignaturas,
  asignaturaID,
  onAsignatura,
  grados,
  gradoID,
  onGrado,
  especialidades,
  especialidadID,
  onEspecialidad,
  dificultad,
  onDificultad,
  tipo,
  onTipo,
  enunciado,
  onEnunciado,
  mostrarEtiquetas,
  etiquetas,
  etiquetasSeleccionadas,
  onAlternarEtiqueta,
  respuestas,
  onAgregarRespuesta,
  onQuitarRespuesta,
  onTextoRespuesta,
  onAlternarCorrecta,
  onMoverRespuesta,
  retroExplicacion,
  onRetroExplicacion,
  retroReferencia,
  onRetroReferencia,
  retroMinimo,
  error,
  enviando,
  textoEnviar,
  textoEnviando,
  onCancelar,
  onEnviar,
}: {
  proceso: CampoProceso;
  asignaturas: ProcesoAsignaturaItem[];
  asignaturaID: number;
  onAsignatura: (id: number) => void;
  grados: Grado[];
  gradoID: number;
  onGrado: (id: number) => void;
  especialidades: Especialidad[];
  especialidadID: number;
  onEspecialidad: (id: number) => void;
  dificultad: DificultadPregunta;
  onDificultad: (v: DificultadPregunta) => void;
  tipo: TipoPregunta;
  onTipo: (v: TipoPregunta) => void;
  enunciado: string;
  onEnunciado: (v: string) => void;
  mostrarEtiquetas: boolean;
  etiquetas: Etiqueta[];
  etiquetasSeleccionadas: number[];
  onAlternarEtiqueta: (id: number) => void;
  respuestas: RespuestaRow[];
  onAgregarRespuesta: () => void;
  onQuitarRespuesta: (index: number) => void;
  onTextoRespuesta: (index: number, texto: string) => void;
  onAlternarCorrecta: (index: number) => void;
  /** Reordena una alternativa arriba (-1) o abajo (+1); sin él no se muestran los botones de orden. */
  onMoverRespuesta?: (index: number, delta: -1 | 1) => void;
  retroExplicacion: string;
  onRetroExplicacion: (v: string) => void;
  retroReferencia: string;
  onRetroReferencia: (v: string) => void;
  /** Exige entre 10 y 2000 caracteres en la explicación, como el backend (nueva pregunta y edición). */
  retroMinimo?: boolean;
  error: string | null;
  enviando: boolean;
  textoEnviar: string;
  textoEnviando: string;
  onCancelar: () => void;
  onEnviar: (e: FormEvent) => void;
}) {
  const t = useTranslations("dashboard");
  const opcionesDificultad: OpcionSelect[] = [
    { value: "facil", label: t("admin-preguntas-dificultad-facil") },
    { value: "medio", label: t("admin-preguntas-dificultad-medio") },
    { value: "dificil", label: t("admin-preguntas-dificultad-dificil") },
  ];
  const opcionesTipo: OpcionSelect[] = [
    { value: "unica", label: t("admin-preguntas-tipo-unica") },
    { value: "multiple", label: t("admin-preguntas-tipo-multiple") },
  ];
  const [intentado, setIntentado] = useState(false);
  // El error de guardado (p. ej. un 409) se pinta arriba del formulario y el botón está en una columna sticky: con un
  // enunciado largo quedaría fuera de la vista. Se lleva a la vista cada vez que aparece un error nuevo.
  const alertaRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!error) return;
    const sinMovimiento = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    alertaRef.current?.scrollIntoView({ behavior: sinMovimiento ? "auto" : "smooth", block: "start" });
  }, [error]);
  // En edición el proceso es fijo: cuenta como elegido.
  const procesoID = proceso.modo === "elegir" ? proceso.valor : 1;
  const faltaProceso = intentado && !procesoID;
  const faltaAsignatura = intentado && !asignaturaID;
  const correctas = respuestas.filter((r) => r.es_correcta).length;
  // CreateRespuestaItem.texto: max=500 runas en el backend.
  const respuestaLarga = respuestas.findIndex((r) => excede(r.texto, LIMITES.respuesta.texto));

  function enviar(e: FormEvent) {
    if (!procesoID || !asignaturaID || respuestaLarga >= 0) {
      e.preventDefault();
      setIntentado(true);
      return;
    }
    onEnviar(e);
  }

  return (
    <form onSubmit={enviar} className="flex flex-col gap-3">
      {error ? (
        <div ref={alertaRef} className="scroll-mt-4">
          <Alerta tono="error">{error}</Alerta>
        </div>
      ) : null}

      <div className="grid items-start gap-3 lg:grid-cols-[minmax(0,1fr)_340px]">
        {/* Columna principal: lo que el postulante lee */}
        <div className="flex min-w-0 flex-col gap-3">
          <Seccion icon={NotePencilIcon} titulo={t("admin-preguntas-enunciado")}>
            <Campo
              id="pregunta-enunciado"
              etiqueta={t("admin-preguntas-enunciado")}
              ayuda={
                <span className="tabular-nums">
                  {t("admin-preguntas-enunciado-ayuda", { n: enunciado.length, max: MAX_ENUNCIADO })}
                </span>
              }
            >
              <AreaTexto
                id="pregunta-enunciado"
                required
                rows={5}
                minLength={10}
                maxLength={MAX_ENUNCIADO}
                value={enunciado}
                onChange={(e) => onEnunciado(e.target.value)}
                placeholder={t("admin-preguntas-enunciado-placeholder")}
              />
            </Campo>

            {mostrarEtiquetas ? (
              <div className="grid gap-2">
                <span className="text-etiqueta text-foreground flex items-center gap-1.5 font-medium">
                  <TagIcon aria-hidden className="text-muted-foreground size-3.5" />
                  {t("admin-preguntas-etiquetas")}
                </span>
                {etiquetas.length > 0 ? (
                  <div className="flex flex-wrap gap-1.5" role="group" aria-label={t("admin-preguntas-etiquetas")}>
                    {etiquetas.map((et) => {
                      const activa = etiquetasSeleccionadas.includes(et.id);
                      return (
                        <button
                          key={et.id}
                          type="button"
                          aria-pressed={activa}
                          onClick={() => onAlternarEtiqueta(et.id)}
                          className={cn(
                            "text-control ease-salida focus-visible:ring-ring/50 inline-flex h-7 items-center gap-1 rounded-full px-3 font-medium transition-colors duration-(--duracion-rapida) outline-none focus-visible:ring-2",
                            activa
                              ? "bg-accent text-accent-foreground ring-acento-texto/30 ring-1"
                              : "bg-secondary text-muted-foreground hover:text-foreground",
                          )}
                        >
                          {activa ? <CheckCircleIcon aria-hidden weight="fill" className="size-3.5" /> : null}
                          {et.nombre}
                        </button>
                      );
                    })}
                  </div>
                ) : (
                  <p className="text-etiqueta text-muted-foreground">
                    {t("admin-preguntas-sin-etiquetas")}{" "}
                    <Link href="/admin/etiquetas" className="text-foreground font-medium underline underline-offset-2">
                      {t("admin-preguntas-crear-etiquetas")}
                    </Link>
                  </p>
                )}
              </div>
            ) : null}
          </Seccion>

          <Seccion
            icon={ListChecksIcon}
            titulo={t("admin-preguntas-respuestas")}
            subtitulo={tipo === "unica" ? t("admin-preguntas-marca-unica") : t("admin-preguntas-marca-multiple")}
            derecha={
              <Chip>
                {t("admin-preguntas-resumen-correctas", {
                  correctas,
                  n: respuestas.length,
                  max: MAX_RESPUESTAS,
                })}
              </Chip>
            }
          >
            <ol className="grid gap-2">
              {respuestas.map((resp, idx) => {
                const id = `pregunta-respuesta-${idx}`;
                return (
                  <li
                    key={resp.key ?? idx}
                    className={cn(
                      "ease-salida flex flex-wrap items-center gap-2 rounded-lg p-1.5 transition-colors duration-(--duracion-rapida) sm:flex-nowrap",
                      resp.es_correcta ? "bg-success" : "bg-secondary",
                    )}
                  >
                    <span
                      aria-hidden
                      className={cn(
                        "size-control text-etiqueta grid shrink-0 place-items-center rounded-full font-mono font-semibold",
                        resp.es_correcta
                          ? "bg-card text-success-foreground ring-success-foreground/40 ring-1"
                          : "bg-card text-muted-foreground",
                      )}
                    >
                      {LETRAS[idx] ?? idx + 1}
                    </span>
                    <Entrada
                      id={id}
                      aria-label={t("admin-preguntas-respuesta-n", { n: idx + 1 })}
                      placeholder={t("admin-preguntas-respuesta-n", { n: idx + 1 })}
                      value={resp.texto}
                      onChange={(e) => onTextoRespuesta(idx, e.target.value)}
                      required
                      maxLength={LIMITES.respuesta.texto}
                      invalido={excede(resp.texto, LIMITES.respuesta.texto)}
                      className="min-w-0 flex-1 basis-48"
                    />
                    <Casilla
                      id={`${id}-correcta`}
                      etiqueta={t("admin-preguntas-correcta")}
                      checked={resp.es_correcta}
                      onCheckedChange={() => onAlternarCorrecta(idx)}
                      className="shrink-0 px-1"
                    />
                    {onMoverRespuesta ? (
                      <div className="flex shrink-0 items-center">
                        <Button
                          type="button"
                          variant="ghost"
                          size="icon"
                          aria-label={t("admin-preguntas-subir-respuesta")}
                          title={t("admin-preguntas-subir-respuesta")}
                          disabled={idx === 0}
                          onClick={() => onMoverRespuesta(idx, -1)}
                          className="text-muted-foreground hover:text-foreground"
                        >
                          <ArrowUpIcon weight="bold" />
                        </Button>
                        <Button
                          type="button"
                          variant="ghost"
                          size="icon"
                          aria-label={t("admin-preguntas-bajar-respuesta")}
                          title={t("admin-preguntas-bajar-respuesta")}
                          disabled={idx === respuestas.length - 1}
                          onClick={() => onMoverRespuesta(idx, 1)}
                          className="text-muted-foreground hover:text-foreground"
                        >
                          <ArrowDownIcon weight="bold" />
                        </Button>
                      </div>
                    ) : null}
                    {respuestas.length > MIN_RESPUESTAS ? (
                      <Button
                        type="button"
                        variant="ghost"
                        size="icon"
                        aria-label={t("admin-preguntas-quitar-respuesta", { n: idx + 1 })}
                        title={t("admin-preguntas-quitar-respuesta", { n: idx + 1 })}
                        onClick={() => onQuitarRespuesta(idx)}
                        className="text-muted-foreground hover:text-destructive-texto shrink-0"
                      >
                        <XIcon weight="bold" />
                      </Button>
                    ) : null}
                  </li>
                );
              })}
            </ol>
            {respuestaLarga >= 0 ? (
              <p role="alert" className="text-control text-destructive">
                {t("admin-preguntas-respuesta-maximo", {
                  letra: LETRAS[respuestaLarga] ?? respuestaLarga + 1,
                  max: LIMITES.respuesta.texto,
                })}
              </p>
            ) : null}
            {respuestas.length < MAX_RESPUESTAS ? (
              <div>
                <button type="button" onClick={onAgregarRespuesta} className={ACCION_SECUNDARIA}>
                  <PlusIcon weight="bold" className="size-4" />
                  {t("admin-preguntas-agregar-respuesta")}
                </button>
              </div>
            ) : null}
          </Seccion>

          <Seccion
            icon={CheckCircleIcon}
            titulo={t("admin-preguntas-retro-titulo")}
            subtitulo={t("admin-preguntas-retro-subtitulo")}
          >
            <Campo id="pregunta-retro-explicacion" etiqueta={t("admin-preguntas-explicacion")}>
              <AreaTexto
                id="pregunta-retro-explicacion"
                rows={3}
                minLength={retroMinimo ? 10 : undefined}
                maxLength={retroMinimo ? 2000 : undefined}
                value={retroExplicacion}
                onChange={(e) => onRetroExplicacion(e.target.value)}
              />
            </Campo>
            <Campo id="pregunta-retro-referencia" etiqueta={t("admin-preguntas-referencia")} opcional>
              <Entrada
                id="pregunta-retro-referencia"
                value={retroReferencia}
                onChange={(e) => onRetroReferencia(e.target.value)}
              />
            </Campo>
          </Seccion>
        </div>

        {/* Columna lateral: clasificación */}
        <div className="flex min-w-0 flex-col gap-3 lg:sticky lg:top-18">
          <Seccion icon={SlidersHorizontalIcon} titulo={t("admin-preguntas-clasificacion")}>
            {proceso.modo === "elegir" ? (
              <Campo
                id="pregunta-proceso"
                etiqueta={t("admin-preguntas-proceso")}
                error={faltaProceso ? t("admin-preguntas-selecciona-proceso") : undefined}
              >
                <SelectCampo
                  id="pregunta-proceso"
                  placeholder={t("admin-preguntas-seleccionar-proceso")}
                  valor={proceso.valor ? String(proceso.valor) : ""}
                  onCambio={(v) => proceso.onCambio(Number(v) || 0)}
                  opciones={proceso.opciones.map((p) => ({ value: String(p.id), label: p.nombre }))}
                  invalido={faltaProceso}
                  describidoPor={faltaProceso ? "pregunta-proceso-error" : undefined}
                  required
                />
              </Campo>
            ) : (
              <Campo
                id="pregunta-proceso"
                etiqueta={t("admin-preguntas-proceso")}
                ayuda={t("admin-preguntas-no-modificable")}
              >
                <Entrada id="pregunta-proceso" value={proceso.nombre} disabled readOnly />
              </Campo>
            )}

            <Campo
              id="pregunta-asignatura"
              etiqueta={t("admin-preguntas-asignatura")}
              error={faltaAsignatura ? t("admin-preguntas-selecciona-asignatura") : undefined}
            >
              <SelectCampo
                id="pregunta-asignatura"
                placeholder={t("admin-seleccionar-asignatura")}
                valor={asignaturaID ? String(asignaturaID) : ""}
                onCambio={(v) => onAsignatura(Number(v) || 0)}
                opciones={asignaturas.map((a) => ({ value: String(a.asignatura_id), label: a.nombre }))}
                disabled={!procesoID}
                invalido={faltaAsignatura}
                describidoPor={faltaAsignatura ? "pregunta-asignatura-error" : undefined}
                required
              />
            </Campo>

            <div className="grid grid-cols-2 gap-3">
              <Campo id="pregunta-dificultad" etiqueta={t("admin-preguntas-dificultad")}>
                <SelectCampo
                  id="pregunta-dificultad"
                  valor={dificultad}
                  onCambio={(v) => onDificultad(v as DificultadPregunta)}
                  opciones={opcionesDificultad}
                  required
                />
              </Campo>
              <Campo id="pregunta-tipo" etiqueta={t("admin-preguntas-tipo")}>
                <SelectCampo
                  id="pregunta-tipo"
                  valor={tipo}
                  onCambio={(v) => onTipo(v as TipoPregunta)}
                  opciones={opcionesTipo}
                  required
                />
              </Campo>
            </div>

            <Campo id="pregunta-grado" etiqueta={t("admin-preguntas-grado")} opcional>
              <SelectCampo
                id="pregunta-grado"
                valor={String(gradoID)}
                onCambio={(v) => onGrado(Number(v) || 0)}
                opciones={[
                  { value: "0", label: t("admin-preguntas-grado-ninguno") },
                  ...grados.map((g) => ({ value: String(g.id), label: g.nombre })),
                ]}
              />
            </Campo>

            <Campo id="pregunta-especialidad" etiqueta={t("admin-preguntas-especialidad")} opcional>
              <SelectCampo
                id="pregunta-especialidad"
                valor={String(especialidadID)}
                onCambio={(v) => onEspecialidad(Number(v) || 0)}
                opciones={[
                  { value: "0", label: t("admin-preguntas-especialidad-ninguna") },
                  ...especialidades.map((e) => ({ value: String(e.id), label: e.nombre })),
                ]}
              />
            </Campo>
          </Seccion>

          <div className="flex flex-wrap justify-end gap-2">
            <button type="button" onClick={onCancelar} className={BOTON_SECUNDARIO}>
              {t("comun-cancelar")}
            </button>
            <BotonAsync type="submit" pendiente={enviando} textoPendiente={textoEnviando} className={BOTON_PRIMARIO}>
              {textoEnviar}
            </BotonAsync>
          </div>
        </div>
      </div>
    </form>
  );
}
