"use client";
import {
  type DificultadPregunta,
  EditorPregunta,
  MAX_RESPUESTAS,
  MIN_RESPUESTAS,
  type RespuestaRow,
  type TipoPregunta,
} from "../_componentes/editor-pregunta";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import React, { useCallback, useEffect, useRef, useState } from "react";

import { CabeceraPagina, EnlaceVolver } from "@/components/patrones/cabecera-pagina";
import { useAuth } from "@/lib/auth/auth-context";
import { catalogsApi, Especialidad, Grado, Proceso, ProcesoAsignaturaItem } from "@/lib/catalogs/api";
import { CONTENEDOR_PAGINA } from "@/lib/estilos";
import { CreateRespuestaPayload, Etiqueta, questionsApi } from "@/lib/questions/api";

const DEFAULT_RESPUESTAS: RespuestaRow[] = [
  { texto: "", es_correcta: false, orden: 1 },
  { texto: "", es_correcta: false, orden: 2 },
];

export default function NuevaPreguntaPage() {
  const { token } = useAuth();
  const t = useTranslations("dashboard");
  const router = useRouter();

  const [procesos, setProcesos] = useState<Proceso[]>([]);
  const [asignaturas, setAsignaturas] = useState<ProcesoAsignaturaItem[]>([]);
  const [grados, setGrados] = useState<Grado[]>([]);
  const [especialidades, setEspecialidades] = useState<Especialidad[]>([]);
  const [etiquetas, setEtiquetas] = useState<Etiqueta[]>([]);

  const [procesoID, setProcesoID] = useState<number>(0);
  const [asignaturaID, setAsignaturaID] = useState<number>(0);
  const [gradoID, setGradoID] = useState<number>(0);
  const [especialidadID, setEspecialidadID] = useState<number>(0);
  const [enunciado, setEnunciado] = useState("");
  const [tipo, setTipo] = useState<TipoPregunta>("unica");
  const [dificultad, setDificultad] = useState<DificultadPregunta>("medio");
  const [selectedEtiquetas, setSelectedEtiquetas] = useState<number[]>([]);
  const [respuestas, setRespuestas] = useState<RespuestaRow[]>(DEFAULT_RESPUESTAS);
  const [retroExplicacion, setRetroExplicacion] = useState("");
  const [retroReferencia, setRetroReferencia] = useState("");

  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // Contadores de petición: solo se aplica la respuesta de la última (cambiar rápido de proceso o asignatura no deja
  // las asignaturas o etiquetas de la anterior).
  const asignaturasReq = useRef(0);
  const etiquetasReq = useRef(0);

  useEffect(() => {
    if (!token) return;
    catalogsApi
      .listProcesos(token, {})
      .then(setProcesos)
      .catch(() => {});
    catalogsApi
      .listGrados(token, {})
      .then(setGrados)
      .catch(() => {});
    catalogsApi
      .listEspecialidades(token, {})
      .then(setEspecialidades)
      .catch(() => {});
  }, [token]);

  useEffect(() => {
    const reqId = ++asignaturasReq.current;
    if (!token || !procesoID) {
      setAsignaturas([]);
      setAsignaturaID(0);
      setEtiquetas([]);
      return;
    }
    catalogsApi
      .listProcesoAsignaturas(token, procesoID)
      .then((data) => {
        if (reqId === asignaturasReq.current) setAsignaturas(data);
      })
      .catch(() => {});
  }, [token, procesoID]);

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

  const addRespuesta = useCallback(() => {
    if (respuestas.length >= MAX_RESPUESTAS) return;
    setRespuestas((prev) => [...prev, { texto: "", es_correcta: false, orden: prev.length + 1 }]);
  }, [respuestas.length]);

  const removeRespuesta = useCallback(
    (index: number) => {
      if (respuestas.length <= MIN_RESPUESTAS) return;
      setRespuestas((prev) => prev.filter((_, i) => i !== index).map((r, i) => ({ ...r, orden: i + 1 })));
    },
    [respuestas.length],
  );

  const updateRespuesta = useCallback((index: number, field: keyof RespuestaRow, value: string | boolean) => {
    setRespuestas((prev) =>
      prev.map((r, i) => {
        if (i !== index) return r;
        return { ...r, [field]: value };
      }),
    );
  }, []);

  const toggleCorrecta = useCallback(
    (index: number) => {
      if (tipo === "unica") {
        // Only one correct
        setRespuestas((prev) => prev.map((r, i) => ({ ...r, es_correcta: i === index })));
      } else {
        setRespuestas((prev) => prev.map((r, i) => (i === index ? { ...r, es_correcta: !r.es_correcta } : r)));
      }
    },
    [tipo],
  );

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!token) return;
    setSubmitting(true);
    setError(null);

    const payload: CreateRespuestaPayload[] = respuestas.map((r) => ({
      texto: r.texto,
      es_correcta: r.es_correcta,
      orden: r.orden,
    }));

    try {
      await questionsApi.createPregunta(token, {
        proceso_id: procesoID,
        asignatura_id: asignaturaID,
        grado_id: gradoID || null,
        especialidad_id: especialidadID || null,
        enunciado,
        tipo,
        dificultad,
        etiqueta_ids: selectedEtiquetas,
        respuestas: payload,
        retroalimentacion: retroExplicacion
          ? { explicacion: retroExplicacion, referencia: retroReferencia || null }
          : null,
      });
      router.push("/admin/preguntas");
    } catch (e: unknown) {
      const err = e as { message?: string };
      setError(err.message ?? t("admin-error-crear-pregunta"));
      setSubmitting(false);
    }
  };

  return (
    <div className={CONTENEDOR_PAGINA}>
      <CabeceraPagina
        volver={<EnlaceVolver href="/admin/preguntas">{t("menu-admin-preguntas")}</EnlaceVolver>}
        titulo={t("admin-preguntas-nueva-titulo")}
        descripcion={t("admin-preguntas-nueva-descripcion")}
      />

      <EditorPregunta
        proceso={{
          modo: "elegir",
          valor: procesoID,
          opciones: procesos,
          onCambio: (id) => {
            // H19: una asignatura o etiqueta de otro proceso ya no es válida.
            setProcesoID(id);
            setAsignaturaID(0);
            setSelectedEtiquetas([]);
          },
        }}
        asignaturas={asignaturas}
        asignaturaID={asignaturaID}
        onAsignatura={(id) => {
          // H19: las etiquetas pertenecen a la asignatura.
          setAsignaturaID(id);
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
        mostrarEtiquetas={asignaturaID > 0}
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
        retroExplicacion={retroExplicacion}
        onRetroExplicacion={setRetroExplicacion}
        retroReferencia={retroReferencia}
        onRetroReferencia={setRetroReferencia}
        retroMinimo
        error={error}
        enviando={submitting}
        textoEnviar={t("admin-crear-pregunta")}
        textoEnviando={t("comun-guardando")}
        onCancelar={() => router.push("/admin/preguntas")}
        onEnviar={handleSubmit}
      />
    </div>
  );
}
