"use client";

import { SimulacroNavigationPanel } from "../components/simulacro-navigation-panel";
import { SimulacroQuestionCard } from "../components/simulacro-question-card";
import { SimulacroTimer } from "../components/simulacro-timer";
import { useParams, useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useCallback, useEffect, useRef, useState } from "react";

import { Alerta } from "@/components/feedback/alerta";
import { ProgresoLineal } from "@/components/feedback/progreso";
import { Spinner } from "@/components/feedback/spinner";
import { Button } from "@/components/ui/button";
import { useColaPorPregunta } from "@/hooks/use-cola-por-pregunta";
import { useAuth } from "@/lib/auth/auth-context";
import { CONTENEDOR_PAGINA } from "@/lib/estilos";
import { PreguntaSimulacro, SimulacroEnCurso, simulacrosApi, tiempoAgotado } from "@/lib/simulations/api";

export default function SimulacroPage() {
  const { id } = useParams<{ id: string }>();
  const router = useRouter();
  const { token } = useAuth();
  const t = useTranslations("dashboard");

  const [simulacro, setSimulacro] = useState<SimulacroEnCurso | null>(null);
  const [loading, setLoading] = useState(true);
  const [currentIdx, setCurrentIdx] = useState(0);
  const [markedIds, setMarkedIds] = useState<Set<number>>(new Set());
  const [timeoutMsg, setTimeoutMsg] = useState<string | null>(null);
  const [errorFinalizar, setErrorFinalizar] = useState(false);
  // H28: the polling interval reads the attempt from a ref, so a state update
  // (e.g. after each saved answer) does not restart the 30 s timer.
  const simulacroRef = useRef<SimulacroEnCurso | null>(null);
  useEffect(() => {
    simulacroRef.current = simulacro;
  }, [simulacro]);

  // H84/H123/H37: the PUTs of one question are serialized (latest wins) and
  // «Terminar» waits for them. See hooks/cola-por-pregunta.ts.
  const { cola, selecciones: selections, pendientes } = useColaPorPregunta("reemplazar");

  // beforeunload warning
  useEffect(() => {
    const handler = (e: BeforeUnloadEvent) => {
      e.preventDefault();
      e.returnValue = "";
    };
    window.addEventListener("beforeunload", handler);
    return () => window.removeEventListener("beforeunload", handler);
  }, []);

  useEffect(() => {
    if (!token) return;
    simulacrosApi
      .getSimulacro(token, Number(id))
      .then((s) => {
        setSimulacro(s);
        cola.inicializar(s.preguntas.map((pq: PreguntaSimulacro) => [pq.id, pq.respuesta_ids]));
      })
      .catch((e: unknown) => {
        // Expired: the server already finalized it → go to the result (H82).
        const agotado = tiempoAgotado(e);
        if (agotado?.resultadoId) {
          router.replace(`/simulacros/${agotado.resultadoId}/resultado`);
          return;
        }
        router.push("/simulacros");
      })
      .finally(() => setLoading(false));
  }, [token, id, router, cola]);

  // Polling every 30 seconds to sync time with server
  useEffect(() => {
    if (!token) return;
    const interval = setInterval(async () => {
      if (!simulacroRef.current) return;
      try {
        const updated = await simulacrosApi.getSimulacro(token, Number(id));
        setSimulacro((prev) => {
          if (!prev) return updated;
          // Sync tiempo_restante_seg from server without replacing local selections
          return { ...prev, tiempo_restante_seg: updated.tiempo_restante_seg };
        });
      } catch (e: unknown) {
        // Expired while polling: the server finalized it → show the result (H82).
        const agotado = tiempoAgotado(e);
        if (agotado?.resultadoId) router.replace(`/simulacros/${agotado.resultadoId}/resultado`);
        // other polling errors are transient: ignore
      }
    }, 30000);
    return () => clearInterval(interval);
  }, [token, id, router]);

  const handleFinalizar = useCallback(async () => {
    if (!token || !simulacro) return;
    const simulacroId = simulacro.id;
    if (!cola.finalizando) setErrorFinalizar(false);
    await cola.finalizar(async ({ plazoVencido }) => {
      // H38: the wait for the pending answers expired → the result page warns
      // that the last answer may not have been saved.
      const resultado = `/simulacros/${simulacroId}/resultado${plazoVencido ? "?sin_guardar=1" : ""}`;
      try {
        await simulacrosApi.finalizarSimulacro(token, simulacroId);
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
  }, [token, simulacro, router, cola]);

  const handleSelect = useCallback(
    async (respuestaId: number) => {
      if (!token || !simulacro) return;
      const simulacroId = simulacro.id;
      const pq = simulacro.preguntas[currentIdx];
      // 'unica' replaces the selection; 'multiple' toggles the alternative.
      await cola.click({
        preguntaId: pq.id,
        respuestaId,
        tipo: pq.tipo,
        actuales: pq.respuesta_ids,
        enviar: async (ids) => (await simulacrosApi.setRespuestas(token, simulacroId, pq.id, ids)).respuesta_ids,
        onConfirmado: (ids) =>
          setSimulacro((s) => {
            if (!s) return s;
            return {
              ...s,
              preguntas: s.preguntas.map((q) =>
                q.id === pq.id
                  ? {
                      ...q,
                      respuesta_ids: ids,
                      respuesta_id: ids[0] ?? null,
                      estado: ids.length > 0 ? "respondida" : "sin_responder",
                    }
                  : q,
              ),
            };
          }),
        onError: (e) => {
          // 409: the time limit is over on the server → close the attempt.
          if (tiempoAgotado(e)) {
            setTimeoutMsg(t("simulacros-respuesta-fuera-de-tiempo"));
            handleFinalizar();
          }
        },
      });
    },
    [token, simulacro, currentIdx, cola, handleFinalizar, t],
  );

  const toggleMark = (preguntaId: number) => {
    setMarkedIds((prev) => {
      const next = new Set(prev);
      if (next.has(preguntaId)) {
        next.delete(preguntaId);
      } else {
        next.add(preguntaId);
      }
      return next;
    });
  };

  if (loading)
    return (
      <div className="text-muted-foreground flex justify-center p-10">
        <Spinner tamano="xl" />
      </div>
    );
  if (!simulacro) return null;

  const pq = simulacro.preguntas[currentIdx];
  const selectedIds = pq ? (selections.get(pq.id) ?? pq.respuesta_ids) : [];

  // Build preguntas with local selection state for nav panel
  const preguntasForNav = simulacro.preguntas.map((p: PreguntaSimulacro) => {
    const ids = selections.get(p.id) ?? p.respuesta_ids;
    return { ...p, respuesta_ids: ids, respuesta_id: ids[0] ?? null };
  });

  const respondidas = preguntasForNav.filter((p) => p.respuesta_ids.length > 0).length;

  return (
    <div className={CONTENEDOR_PAGINA}>
      {/* Cabecera: proceso, posición y tiempo */}
      <header className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex min-w-0 flex-1 basis-64 flex-col gap-2">
          <div className="flex min-w-0 flex-col gap-0.5">
            <h1 className="text-titulo-pagina text-foreground truncate">{simulacro.proceso}</h1>
            <p className="text-cuerpo text-muted-foreground tabular-nums">
              {t("simulacros-pregunta")} {currentIdx + 1} / {simulacro.total_preguntas}
            </p>
          </div>
          <ProgresoLineal
            valor={simulacro.total_preguntas > 0 ? (respondidas / simulacro.total_preguntas) * 100 : 0}
            tono="success"
            mostrarValor={false}
            className="max-w-md"
          />
        </div>
        <SimulacroTimer tiempoRestanteInicial={simulacro.tiempo_restante_seg} onExpired={handleFinalizar} />
      </header>

      {timeoutMsg && <Alerta tono="aviso">{timeoutMsg}</Alerta>}

      {errorFinalizar && (
        <Alerta
          tono="error"
          accion={
            <Button variant="outline" size="sm" onClick={handleFinalizar}>
              {t("simulacros-reintentar")}
            </Button>
          }
        >
          {t("simulacros-error-finalizar")}
        </Alerta>
      )}

      <div className="grid grid-cols-1 items-start gap-3 md:grid-cols-3">
        {/* Pregunta */}
        <div className="min-w-0 md:col-span-2">
          {pq && (
            <SimulacroQuestionCard
              pregunta={pq}
              selectedIds={selectedIds}
              onSelect={handleSelect}
              onNext={currentIdx < simulacro.preguntas.length - 1 ? () => setCurrentIdx((i) => i + 1) : undefined}
              onPrev={currentIdx > 0 ? () => setCurrentIdx((i) => i - 1) : undefined}
              onMarcar={() => toggleMark(pq.id)}
              marcada={markedIds.has(pq.id)}
            />
          )}
        </div>

        {/* Panel de navegación */}
        <div className="min-w-0">
          <SimulacroNavigationPanel
            preguntas={preguntasForNav}
            currentIdx={currentIdx}
            onNavigate={setCurrentIdx}
            markedIds={markedIds}
            onFinish={handleFinalizar}
            guardando={pendientes > 0}
          />
        </div>
      </div>
    </div>
  );
}
