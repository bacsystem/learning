"use client";
import {
  type DificultadPregunta,
  EditorPregunta,
  MAX_RESPUESTAS,
  MIN_RESPUESTAS,
  type RespuestaRow,
  type TipoPregunta,
} from "../_componentes/editor-pregunta";
import { useMensajesValidacionPregunta } from "../_componentes/use-mensajes-validacion-pregunta";
import { useFormik } from "formik";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useCallback, useEffect, useRef, useState } from "react";

import { useToast } from "@/components/feedback/toast";
import { CabeceraPagina, EnlaceVolver } from "@/components/patrones/cabecera-pagina";
import { useAuth } from "@/lib/auth/auth-context";
import { catalogsApi, Especialidad, Grado, Proceso, ProcesoAsignaturaItem } from "@/lib/catalogs/api";
import { CONTENEDOR_PAGINA } from "@/lib/estilos";
import { CreateRespuestaPayload, Etiqueta, questionsApi } from "@/lib/questions/api";
import { validarPregunta } from "@/lib/questions/pregunta";

interface ValoresNuevaPregunta {
  proceso_id: number;
  asignatura_id: number;
  grado_id: number;
  especialidad_id: number;
  enunciado: string;
  tipo: TipoPregunta;
  dificultad: DificultadPregunta;
  etiqueta_ids: number[];
  respuestas: RespuestaRow[];
  retro_explicacion: string;
  retro_referencia: string;
}

const VALORES_INICIALES: ValoresNuevaPregunta = {
  proceso_id: 0,
  asignatura_id: 0,
  grado_id: 0,
  especialidad_id: 0,
  enunciado: "",
  tipo: "unica",
  dificultad: "medio",
  etiqueta_ids: [],
  respuestas: [
    { texto: "", es_correcta: false, orden: 1 },
    { texto: "", es_correcta: false, orden: 2 },
  ],
  retro_explicacion: "",
  retro_referencia: "",
};

export default function NuevaPreguntaPage() {
  const { token } = useAuth();
  const t = useTranslations("dashboard");
  const toast = useToast();
  const router = useRouter();

  const [procesos, setProcesos] = useState<Proceso[]>([]);
  const [asignaturas, setAsignaturas] = useState<ProcesoAsignaturaItem[]>([]);
  const [grados, setGrados] = useState<Grado[]>([]);
  const [especialidades, setEspecialidades] = useState<Especialidad[]>([]);
  const [etiquetas, setEtiquetas] = useState<Etiqueta[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [redirigiendo, setRedirigiendo] = useState(false);

  const mensajes = useMensajesValidacionPregunta();

  const formik = useFormik<ValoresNuevaPregunta>({
    initialValues: VALORES_INICIALES,
    // Sin validación nativa del navegador: los mensajes salen del selector de idioma de la app.
    validate: (valores) => validarPregunta(valores, mensajes, true),
    onSubmit: async (valores) => {
      if (!token) return;
      setError(null);
      const respuestas: CreateRespuestaPayload[] = valores.respuestas.map((r) => ({
        texto: r.texto,
        es_correcta: r.es_correcta,
        orden: r.orden,
      }));
      try {
        await questionsApi.createPregunta(token, {
          proceso_id: valores.proceso_id,
          asignatura_id: valores.asignatura_id,
          grado_id: valores.grado_id || null,
          especialidad_id: valores.especialidad_id || null,
          enunciado: valores.enunciado,
          tipo: valores.tipo,
          dificultad: valores.dificultad,
          etiqueta_ids: valores.etiqueta_ids,
          respuestas,
          retroalimentacion: valores.retro_explicacion.trim()
            ? { explicacion: valores.retro_explicacion.trim(), referencia: valores.retro_referencia.trim() || null }
            : null,
        });
        // P1 (SIA-E3-02): toda pregunta nueva nace en borrador y no se sirve hasta aprobarla.
        toast.ok(t("serumsIa.avisosPregunta.creadaBorrador"));
        // formik apaga `isSubmitting` al terminar, antes de que la navegación acabe: se mantiene pendiente hasta salir.
        setRedirigiendo(true);
        router.push("/admin/preguntas");
      } catch (e: unknown) {
        const err = e as { message?: string };
        setError(err.message ?? t("admin-error-crear-pregunta"));
      }
    },
  });
  const { values, setFieldValue } = formik;
  const procesoID = values.proceso_id;
  const asignaturaID = values.asignatura_id;
  // Los errores se muestran desde el primer envío; antes, el formulario no regaña a quien aún no terminó de escribir.
  const errores = formik.submitCount > 0 ? formik.errors : {};

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

  const setRespuestas = useCallback(
    (actualizar: (prev: RespuestaRow[]) => RespuestaRow[]) =>
      setFieldValue("respuestas", actualizar(values.respuestas)),
    [setFieldValue, values.respuestas],
  );

  const addRespuesta = () => {
    if (values.respuestas.length >= MAX_RESPUESTAS) return;
    setRespuestas((prev) => [...prev, { texto: "", es_correcta: false, orden: prev.length + 1 }]);
  };

  const removeRespuesta = (index: number) => {
    if (values.respuestas.length <= MIN_RESPUESTAS) return;
    setRespuestas((prev) => prev.filter((_, i) => i !== index).map((r, i) => ({ ...r, orden: i + 1 })));
  };

  const updateTexto = (index: number, texto: string) =>
    setRespuestas((prev) => prev.map((r, i) => (i === index ? { ...r, texto } : r)));

  const toggleCorrecta = (index: number) =>
    setRespuestas((prev) =>
      // Única: solo una correcta; múltiple: cada una se alterna.
      prev.map((r, i) =>
        values.tipo === "unica"
          ? { ...r, es_correcta: i === index }
          : i === index
            ? { ...r, es_correcta: !r.es_correcta }
            : r,
      ),
    );

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
            setFieldValue("proceso_id", id);
            setFieldValue("asignatura_id", 0);
            setFieldValue("etiqueta_ids", []);
          },
        }}
        asignaturas={asignaturas}
        asignaturaID={asignaturaID}
        onAsignatura={(id) => {
          // H19: las etiquetas pertenecen a la asignatura.
          setFieldValue("asignatura_id", id);
          setFieldValue("etiqueta_ids", []);
        }}
        grados={grados}
        gradoID={values.grado_id}
        onGrado={(id) => setFieldValue("grado_id", id)}
        especialidades={especialidades}
        especialidadID={values.especialidad_id}
        onEspecialidad={(id) => setFieldValue("especialidad_id", id)}
        dificultad={values.dificultad}
        onDificultad={(v) => setFieldValue("dificultad", v)}
        tipo={values.tipo}
        onTipo={(v) => setFieldValue("tipo", v)}
        enunciado={values.enunciado}
        onEnunciado={(v) => setFieldValue("enunciado", v)}
        mostrarEtiquetas={asignaturaID > 0}
        etiquetas={etiquetas}
        etiquetasSeleccionadas={values.etiqueta_ids}
        onAlternarEtiqueta={(id) =>
          setFieldValue(
            "etiqueta_ids",
            values.etiqueta_ids.includes(id)
              ? values.etiqueta_ids.filter((x) => x !== id)
              : [...values.etiqueta_ids, id],
          )
        }
        respuestas={values.respuestas}
        onAgregarRespuesta={addRespuesta}
        onQuitarRespuesta={removeRespuesta}
        onTextoRespuesta={updateTexto}
        onAlternarCorrecta={toggleCorrecta}
        retroExplicacion={values.retro_explicacion}
        onRetroExplicacion={(v) => setFieldValue("retro_explicacion", v)}
        retroReferencia={values.retro_referencia}
        onRetroReferencia={(v) => setFieldValue("retro_referencia", v)}
        retroMinimo
        error={error}
        errores={errores as Record<string, string>}
        intentosEnvio={formik.submitCount}
        enviando={formik.isSubmitting || redirigiendo}
        textoEnviar={t("admin-crear-pregunta")}
        textoEnviando={t("comun-guardando")}
        onCancelar={() => router.push("/admin/preguntas")}
        onEnviar={formik.handleSubmit}
      />
    </div>
  );
}
