"use client";

import { NavigationPanel } from "../components/navigation-panel";
import { QuestionCard } from "../components/question-card";
import { Timer } from "../components/timer";
import { useParams, useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useCallback, useEffect, useState } from "react";

import { Alerta } from "@/components/feedback/alerta";
import { ProgresoLineal } from "@/components/feedback/progreso";
import { Skeleton } from "@/components/feedback/skeleton";
import { Button } from "@/components/ui/button";
import { useColaPorPregunta } from "@/hooks/use-cola-por-pregunta";
import { useAuth } from "@/lib/auth/auth-context";
import { CONTENEDOR_PAGINA, TARJETA } from "@/lib/estilos";
import {
  errorCode,
  PracticaEnCurso,
  practicasApi,
  PreguntaPractica,
  tiempoAgotado,
  ValidarPreguntaResponse,
} from "@/lib/practices/api";
import { cn } from "@/lib/utils";
import { XIcon } from "@phosphor-icons/react";

/** Restores the feedback of questions already checked in inmediato mode (H74). */
function validacionesDe(preguntas: PreguntaPractica[]): Map<number, ValidarPreguntaResponse> {
  const out = new Map<number, ValidarPreguntaResponse>();
  for (const pq of preguntas) {
    if (!pq.validada) continue;
    out.set(pq.id, {
      es_correcta: pq.estado === "correcta",
      respuesta_correcta_ids: pq.respuesta_correcta_ids ?? [],
      explicacion: pq.explicacion ?? "",
    });
  }
  return out;
}

function CargandoPractica() {
  return (
    <div className={CONTENEDOR_PAGINA} aria-busy="true">
      <div className="flex items-center justify-between gap-3">
        <div className="flex flex-col gap-2">
          <Skeleton className="h-5 w-56" />
          <Skeleton className="h-3 w-28" />
        </div>
        <Skeleton className="h-control w-24 rounded-full" />
      </div>
      <div className="grid grid-cols-1 gap-3 md:grid-cols-12">
        <div className={cn(TARJETA, "flex flex-col gap-4 p-6 md:col-span-8")}>
          <Skeleton className="h-3 w-24" />
          <Skeleton className="h-6 w-4/5" />
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} className="h-14 w-full rounded-xl" />
          ))}
        </div>
        <div className={cn(TARJETA, "hidden flex-col gap-3 p-4 md:col-span-4 md:flex")}>
          <Skeleton className="h-3 w-40" />
          <div className="grid grid-cols-6 gap-2">
            {Array.from({ length: 18 }).map((_, i) => (
              <Skeleton key={i} className="h-9 rounded-lg" />
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}

export default function PracticaPage() {
  const { id } = useParams<{ id: string }>();
  const router = useRouter();
  const { token } = useAuth();
  const t = useTranslations("dashboard");

  const [practica, setPractica] = useState<PracticaEnCurso | null>(null);
  const [loading, setLoading] = useState(true);
  const [currentIdx, setCurrentIdx] = useState(0);
  const [markedIds, setMarkedIds] = useState<Set<number>>(new Set());
  const [validaciones, setValidaciones] = useState<Map<number, ValidarPreguntaResponse>>(new Map());
  const [aviso, setAviso] = useState<{ tono: "aviso" | "error"; msg: string } | null>(null);
  const [errorFinalizar, setErrorFinalizar] = useState(false);

  // H18/H124/H37: POST /responder toggles the selection on the server, so the
  // clicks of one question are queued and sent in order; the screen keeps the
  // respuesta_ids of the last processed request. See hooks/cola-por-pregunta.ts.
  const { cola, selecciones: selections, pendientes } = useColaPorPregunta("conmutar");

  // inicial: the first load (any failure leaves the page). A resync (H151)
  // only leaves on 404/403; a transient error keeps the attempt on screen.
  const cargar = useCallback(
    async (inicial = false) => {
      if (!token) return;
      try {
        const p = await practicasApi.getPractica(token, Number(id));
        setPractica(p);
        cola.inicializar(p.preguntas.map((pq) => [pq.id, pq.respuesta_ids]));
        setValidaciones(validacionesDe(p.preguntas));
      } catch (e: unknown) {
        // Expired: the server already finalized it → go to the result (H82).
        const agotado = tiempoAgotado(e);
        if (agotado?.resultadoId) {
          router.replace(`/practicas/${agotado.resultadoId}/resultado`);
          return;
        }
        const status = (e as { status?: number }).status;
        if (inicial || status === 404 || status === 403) {
          router.push("/practicas");
          return;
        }
        setAviso({ tono: "error", msg: t("practicas-error-resincronizar") });
      }
    },
    [token, id, router, t, cola],
  );

  useEffect(() => {
    if (!token) return;
    cargar(true).finally(() => setLoading(false));
  }, [token, cargar]);

  const handleFinalizar = useCallback(async () => {
    if (!token || !practica) return;
    const practicaId = practica.id;
    if (!cola.finalizando) setErrorFinalizar(false);
    await cola.finalizar(async ({ plazoVencido }) => {
      // H38: the wait for the pending answers expired → the result page warns
      // that the last answer may not have been saved.
      const resultado = `/practicas/${practicaId}/resultado${plazoVencido ? "?sin_guardar=1" : ""}`;
      try {
        await practicasApi.finalizarPractica(token, practicaId);
        router.push(resultado);
        return "ok";
      } catch (e: unknown) {
        // 400: already finalized (e.g. expired and closed by the server) → show the result.
        if ((e as { status?: number }).status === 400) {
          router.push(resultado);
          return "ok";
        }
        // H28: network or server error → tell the user and let them retry.
        setErrorFinalizar(true);
        return "reintentar";
      }
    });
  }, [token, practica, router, cola]);

  const handleToggle = useCallback(
    async (respuestaId: number) => {
      if (!token || !practica) return;
      const practicaId = practica.id;
      const pq = practica.preguntas[currentIdx];
      // Every click is sent (the endpoint toggles), one after another.
      await cola.click({
        preguntaId: pq.id,
        respuestaId,
        tipo: pq.tipo,
        actuales: pq.respuesta_ids,
        enviar: async () => (await practicasApi.toggleRespuesta(token, practicaId, pq.id, respuestaId)).respuesta_ids,
        onConfirmado: () => setAviso(null),
        onError: (e, { esUltimo }) => {
          const code = errorCode(e);
          if (code === "tiempo_agotado") {
            // The limit is over on the server: tell the user and close the attempt (H82).
            setAviso({ tono: "aviso", msg: t("practicas-respuesta-fuera-de-tiempo") });
            handleFinalizar();
          } else if (code === "pregunta_validada") {
            // Locked by the server (H74): resync the locked state.
            setAviso({ tono: "aviso", msg: t("practicas-pregunta-validada") });
            if (esUltimo) cargar();
          } else {
            setAviso({ tono: "error", msg: t("practicas-error-guardar") });
          }
        },
      });
    },
    [token, practica, currentIdx, cola, handleFinalizar, cargar, t],
  );

  const handleComprobar = useCallback(async () => {
    if (!token || !practica || cola.finalizando) return;
    const pq = practica.preguntas[currentIdx];
    // H124: check only after the pending answers of this question were saved.
    await cola.esperarPregunta(pq.id);
    try {
      const resp = await practicasApi.validarPregunta(token, practica.id, pq.id);
      setValidaciones((prev) => new Map(prev).set(pq.id, resp));
      setPractica((prev) => {
        if (!prev) return prev;
        return {
          ...prev,
          preguntas: prev.preguntas.map((q) =>
            q.id === pq.id ? { ...q, validada: true, estado: resp.es_correcta ? "correcta" : "incorrecta" } : q,
          ),
        };
      });
      setAviso(null);
    } catch (e: unknown) {
      const code = errorCode(e);
      if (code === "tiempo_agotado") {
        // El plazo terminó en el servidor: se avisa y se cierra el intento (H82, H104).
        setAviso({ tono: "aviso", msg: t("practicas-respuesta-fuera-de-tiempo") });
        handleFinalizar();
      } else {
        setAviso({ tono: "error", msg: t("practicas-error-comprobar") });
      }
    }
  }, [token, practica, currentIdx, cola, handleFinalizar, t]);

  const toggleMark = (id: number) => {
    setMarkedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  };

  if (loading) return <CargandoPractica />;
  if (!practica) return null;

  const pq = practica.preguntas[currentIdx];
  const selectedIds = selections.get(pq?.id ?? 0) ?? [];
  const validacion = validaciones.get(pq?.id ?? 0);
  const avance = practica.total_preguntas > 0 ? ((currentIdx + 1) / practica.total_preguntas) * 100 : 0;

  return (
    <div className={CONTENEDOR_PAGINA}>
      {/* Cabecera: proceso, posición y temporizador */}
      <header className="flex flex-col gap-2.5">
        <div className="flex items-center justify-between gap-3">
          <div className="flex min-w-0 flex-col gap-0.5">
            <h2 className="text-titulo text-foreground truncate">{practica.proceso}</h2>
            <p className="text-control text-muted-foreground tabular-nums">
              {t("practicas-pregunta")} {currentIdx + 1} / {practica.total_preguntas}
            </p>
          </div>
          <Timer tiempoRestanteSeg={practica.tiempo_restante_seg} onExpired={handleFinalizar} />
        </div>
        <ProgresoLineal valor={avance} mostrarValor={false} />
      </header>

      {aviso && (
        <Alerta
          tono={aviso.tono}
          accion={
            <Button variant="ghost" size="icon-sm" aria-label={t("comun-cerrar")} onClick={() => setAviso(null)}>
              <XIcon weight="bold" className="size-4" />
            </Button>
          }
        >
          {aviso.msg}
        </Alerta>
      )}

      {errorFinalizar && (
        <Alerta
          tono="error"
          accion={
            <Button variant="outline" size="sm" onClick={handleFinalizar}>
              {t("practicas-reintentar")}
            </Button>
          }
        >
          {t("practicas-error-finalizar")}
        </Alerta>
      )}

      <div className="grid grid-cols-1 items-start gap-3 md:grid-cols-12">
        {/* Pregunta */}
        <div className="min-w-0 md:col-span-8">
          {pq && (
            <QuestionCard
              pregunta={pq}
              selectedIds={selectedIds}
              onToggle={handleToggle}
              mode={practica.modo_calificacion}
              validacion={validacion}
              onComprobar={practica.modo_calificacion === "inmediato" ? handleComprobar : undefined}
              onNext={currentIdx < practica.preguntas.length - 1 ? () => setCurrentIdx((i) => i + 1) : undefined}
              onPrev={currentIdx > 0 ? () => setCurrentIdx((i) => i - 1) : undefined}
              onMarcar={() => toggleMark(pq.id)}
              marcada={markedIds.has(pq.id)}
            />
          )}
        </div>

        {/* Navegación */}
        <div className="min-w-0 md:col-span-4">
          <NavigationPanel
            preguntas={practica.preguntas.map((p) => ({
              ...p,
              respuesta_ids: selections.get(p.id) ?? p.respuesta_ids,
            }))}
            currentIdx={currentIdx}
            onNavigate={setCurrentIdx}
            markedIds={markedIds}
            onFinish={handleFinalizar}
            guardando={pendientes > 0}
            modoCalificacion={practica.modo_calificacion}
          />
        </div>
      </div>
    </div>
  );
}
