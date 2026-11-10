"use client";
import {
  type DificultadPregunta,
  EditorPregunta,
  MAX_RESPUESTAS,
  MIN_RESPUESTAS,
  type RespuestaRow,
  type TipoPregunta,
} from "../../_componentes/editor-pregunta";
import { useParams, useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import React, { useCallback, useEffect, useRef, useState } from "react";

import { Alerta } from "@/components/feedback/alerta";
import { Skeleton } from "@/components/feedback/skeleton";
import { CabeceraPagina, EnlaceVolver } from "@/components/patrones/cabecera-pagina";
import { useAuth } from "@/lib/auth/auth-context";
import { catalogsApi, Especialidad, Grado, ProcesoAsignaturaItem } from "@/lib/catalogs/api";
import { CONTENEDOR_PAGINA, TARJETA } from "@/lib/estilos";
import { Etiqueta, Pregunta, questionsApi, UpdatePreguntaPayload } from "@/lib/questions/api";

/** Renumbers orden after a move/removal; ids stay attached to their text. */
const renumber = (rows: RespuestaRow[]) => rows.map((r, i) => ({ ...r, orden: i + 1 }));

/** Esqueleto del editor mientras llega la pregunta: mismas dos columnas que el formulario. */
function EditorCargando() {
  const t = useTranslations("dashboard");
  return (
    <div
      className="grid items-start gap-3 lg:grid-cols-[minmax(0,1fr)_340px]"
      aria-busy="true"
      aria-label={t("admin-preguntas-cargando")}
    >
      <div className="flex flex-col gap-3">
        {["h-36", "h-56", "h-28"].map((alto) => (
          <div key={alto} className={`${TARJETA} grid gap-3 p-4`}>
            <Skeleton className="h-4 w-32" />
            <Skeleton className={`${alto} w-full rounded-lg`} />
          </div>
        ))}
      </div>
      <div className={`${TARJETA} grid gap-3 p-4`}>
        <Skeleton className="h-4 w-28" />
        {Array.from({ length: 5 }).map((_, i) => (
          <div key={i} className="grid gap-1.5">
            <Skeleton className="h-3 w-20" />
            <Skeleton className="h-control w-full rounded-lg" />
          </div>
        ))}
      </div>
    </div>
  );
}

export default function EditPreguntaPage() {
  const { token } = useAuth();
  const t = useTranslations("dashboard");
  const router = useRouter();
  const params = useParams<{ id: string }>();
  const preguntaID = Number(params.id);

  const [loading, setLoading] = useState(true);
  const [pregunta, setPregunta] = useState<Pregunta | null>(null);
  const [asignaturas, setAsignaturas] = useState<ProcesoAsignaturaItem[]>([]);
  const [grados, setGrados] = useState<Grado[]>([]);
  const [especialidades, setEspecialidades] = useState<Especialidad[]>([]);
  const [etiquetas, setEtiquetas] = useState<Etiqueta[]>([]);

  const [asignaturaID, setAsignaturaID] = useState<number>(0);
  const [gradoID, setGradoID] = useState<number>(0);
  const [especialidadID, setEspecialidadID] = useState<number>(0);
  const [enunciado, setEnunciado] = useState("");
  const [tipo, setTipo] = useState<TipoPregunta>("unica");
  const [dificultad, setDificultad] = useState<DificultadPregunta>("medio");
  const [selectedEtiquetas, setSelectedEtiquetas] = useState<number[]>([]);
  const [respuestas, setRespuestas] = useState<RespuestaRow[]>([]);
  const [retroExplicacion, setRetroExplicacion] = useState("");
  const [retroReferencia, setRetroReferencia] = useState("");

  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!token || !preguntaID) return;
    Promise.all([
      questionsApi.getPregunta(token, preguntaID),
      catalogsApi.listGrados(token, {}),
      catalogsApi.listEspecialidades(token, {}),
    ])
      .then(async ([pData, gData, eData]) => {
        const aData = await catalogsApi.listProcesoAsignaturas(token, pData.proceso_id);
        setPregunta(pData);
        setAsignaturas(aData);
        setGrados(gData);
        setEspecialidades(eData);

        setAsignaturaID(pData.asignatura_id);
        setEnunciado(pData.enunciado);
        setTipo(pData.tipo);
        setDificultad(pData.dificultad);
        setRespuestas(
          [...pData.respuestas]
            .sort((a, b) => a.orden - b.orden)
            .map((r) => ({
              id: r.id,
              key: `id-${r.id}`,
              texto: r.texto,
              es_correcta: r.es_correcta,
              orden: r.orden,
            })),
        );
        // Preselect the current tags: saving sends etiqueta_ids, which replaces
        // them, so starting empty would delete every tag.
        setSelectedEtiquetas(pData.etiqueta_ids ?? []);
        setRetroExplicacion(pData.retroalimentacion?.explicacion ?? "");
        setRetroReferencia(pData.retroalimentacion?.referencia ?? "");

        // H20: el backend manda los ids; si no llegan (backend anterior), se busca por nombre.
        if (pData.grado_id !== undefined) setGradoID(pData.grado_id ?? 0);
        else {
          const matchedG = gData.find((g) => g.nombre === pData.grado);
          if (matchedG) setGradoID(matchedG.id);
        }

        if (pData.especialidad_id !== undefined) setEspecialidadID(pData.especialidad_id ?? 0);
        else {
          const matchedE = eData.find((e) => e.nombre === pData.especialidad);
          if (matchedE) setEspecialidadID(matchedE.id);
        }

        setLoading(false);
      })
      .catch(() => {
        setError(t("admin-error-cargar-pregunta"));
        setLoading(false);
      });
  }, [token, preguntaID, t]);

  // Solo se aplica la respuesta de la última petición de etiquetas (cambiar rápido de asignatura no deja las de otra).
  const etiquetasReq = useRef(0);
  useEffect(() => {
    const reqId = ++etiquetasReq.current;
    if (!token || !asignaturaID) {
      setEtiquetas([]);
      return;
    }
    questionsApi
      .listEtiquetas(token, { asignatura_id: asignaturaID })
      .then((data) => {
        if (reqId === etiquetasReq.current) setEtiquetas(data);
      })
      .catch(() => {});
  }, [token, asignaturaID]);

  const newKey = useRef(0);
  const addRespuesta = useCallback(() => {
    if (respuestas.length >= MAX_RESPUESTAS) return;
    newKey.current += 1;
    const key = `new-${newKey.current}`;
    setRespuestas((prev) => [...prev, { key, texto: "", es_correcta: false, orden: prev.length + 1 }]);
  }, [respuestas.length]);

  const removeRespuesta = useCallback(
    (index: number) => {
      if (respuestas.length <= MIN_RESPUESTAS) return;
      setRespuestas((prev) => renumber(prev.filter((_, i) => i !== index)));
    },
    [respuestas.length],
  );

  /** Moves a whole row (id, text, correctness) up (-1) or down (+1). */
  const moveRespuesta = useCallback((index: number, delta: -1 | 1) => {
    setRespuestas((prev) => {
      const to = index + delta;
      if (to < 0 || to >= prev.length) return prev;
      const next = [...prev];
      [next[index], next[to]] = [next[to], next[index]];
      return renumber(next);
    });
  }, []);

  const updateRespuesta = useCallback((index: number, field: keyof RespuestaRow, value: string | boolean) => {
    setRespuestas((prev) => prev.map((r, i) => (i === index ? { ...r, [field]: value } : r)));
  }, []);

  const toggleCorrecta = useCallback(
    (index: number) => {
      if (tipo === "unica") {
        setRespuestas((prev) => prev.map((r, i) => ({ ...r, es_correcta: i === index })));
      } else {
        setRespuestas((prev) => prev.map((r, i) => (i === index ? { ...r, es_correcta: !r.es_correcta } : r)));
      }
    },
    [tipo],
  );

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!token || !pregunta) return;
    setSubmitting(true);
    setError(null);

    const payload: UpdatePreguntaPayload = {
      asignatura_id: asignaturaID,
      grado_id: gradoID || null,
      especialidad_id: especialidadID || null,
      enunciado,
      tipo,
      dificultad,
      etiqueta_ids: selectedEtiquetas,
      // Send each alternative's id so the server updates it in place (and keeps
      // its attempts); rows without id are new and get inserted.
      respuestas: respuestas.map((r) => ({
        ...(r.id !== undefined && { id: r.id }),
        texto: r.texto,
        es_correcta: r.es_correcta,
        orden: r.orden,
      })),
      retroalimentacion: retroExplicacion
        ? { explicacion: retroExplicacion, referencia: retroReferencia || null }
        : null,
    };

    try {
      await questionsApi.updatePregunta(token, preguntaID, payload);
      router.push("/admin/preguntas");
    } catch (e: unknown) {
      const err = e as { message?: string; status?: number };
      // The backend sends a specific message for every 409 (alternative with
      // attempts, duplicated statement…) and 400 (invalid subject/tags). The
      // key change on a question that already graded attempts (H133) gets its
      // translated text.
      setError(
        err.status === 409 && err.message?.includes("respuesta correcta")
          ? t("admin-preguntas-clave-con-intentos")
          : err.message || t("admin-error-actualizar-pregunta"),
      );
      setSubmitting(false);
    }
  };

  const volver = <EnlaceVolver href="/admin/preguntas">{t("menu-admin-preguntas")}</EnlaceVolver>;

  if (loading) {
    return (
      <div className={CONTENEDOR_PAGINA}>
        <CabeceraPagina
          volver={volver}
          titulo={t("admin-preguntas-editar-titulo", { id: preguntaID })}
          descripcion={t("admin-preguntas-cargando")}
        />
        <EditorCargando />
      </div>
    );
  }

  if (!pregunta) {
    return (
      <div className={CONTENEDOR_PAGINA}>
        <CabeceraPagina volver={volver} titulo={t("admin-preguntas-editar-titulo", { id: preguntaID })} />
        <Alerta tono="error" titulo={t("admin-preguntas-no-encontrada")}>
          {error ?? t("admin-preguntas-no-encontrada-detalle")}
        </Alerta>
      </div>
    );
  }

  return (
    <div className={CONTENEDOR_PAGINA}>
      <CabeceraPagina
        volver={volver}
        titulo={
          <>
            {t("admin-preguntas-editar")} <span className="text-muted-foreground font-mono">#{preguntaID}</span>
          </>
        }
        descripcion={t.rich("admin-preguntas-proceso-fijo", {
          proceso: pregunta.proceso,
          b: (c) => <strong className="text-foreground font-semibold">{c}</strong>,
        })}
      />

      <EditorPregunta
        proceso={{ modo: "fijo", nombre: pregunta.proceso }}
        asignaturas={asignaturas}
        asignaturaID={asignaturaID}
        onAsignatura={(id) => {
          setAsignaturaID(id);
          // Tags belong to an asignatura: the old ones cannot stay selected.
          setSelectedEtiquetas([]);
        }}
        grados={grados}
        gradoID={gradoID}
        onGrado={setGradoID}
        especialidades={especialidades}
        especialidadID={especialidadID}
        onEspecialidad={setEspecialidadID}
        dificultad={dificultad}
        onDificultad={setDificultad}
        tipo={tipo}
        onTipo={setTipo}
        enunciado={enunciado}
        onEnunciado={setEnunciado}
        mostrarEtiquetas
        etiquetas={etiquetas}
        etiquetasSeleccionadas={selectedEtiquetas}
        onAlternarEtiqueta={(id) =>
          setSelectedEtiquetas((prev) => (prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]))
        }
        respuestas={respuestas}
        onAgregarRespuesta={addRespuesta}
        onQuitarRespuesta={removeRespuesta}
        onTextoRespuesta={(idx, texto) => updateRespuesta(idx, "texto", texto)}
        onAlternarCorrecta={toggleCorrecta}
        onMoverRespuesta={moveRespuesta}
        retroExplicacion={retroExplicacion}
        onRetroExplicacion={setRetroExplicacion}
        retroReferencia={retroReferencia}
        onRetroReferencia={setRetroReferencia}
        retroMinimo
        error={error}
        enviando={submitting}
        textoEnviar={t("admin-actualizar-pregunta")}
        textoEnviando={t("comun-guardando")}
        onCancelar={() => router.push("/admin/preguntas")}
        onEnviar={handleSubmit}
      />
    </div>
  );
}
