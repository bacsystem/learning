"use client";

import { useCallback, useEffect, useState } from "react";

import { useAuth } from "@/lib/auth/auth-context";
import { type CambiarEstadoPayload, type Pregunta, questionsApi } from "@/lib/questions/api";
import { idsTrasAnadir, idsTrasQuitar } from "@/lib/serums-ia/revision/reglas";

type Carga = "cargando" | "lista" | "error" | "no_encontrada";

/**
 * Pregunta con las operaciones que la cambian: citas (E3-07 §4, siempre la lista completa) y estado (§3). Pinta lo
 * que responde el servidor; tras cualquier error, y en particular un 409, recarga la pregunta para no mostrar un
 * estado supuesto.
 */
export function useRevisionPregunta(preguntaId: number) {
  const { token } = useAuth();
  const [pregunta, setPregunta] = useState<Pregunta | null>(null);
  const [carga, setCarga] = useState<Carga>("cargando");
  const [guardando, setGuardando] = useState(false);
  /** Último error de una operación sobre las citas; la pantalla lo traduce. */
  const [error, setError] = useState<unknown>(null);

  const cargar = useCallback(async () => {
    if (!token) return;
    try {
      setPregunta(await questionsApi.getPregunta(token, preguntaId));
      setCarga("lista");
    } catch (e: unknown) {
      setCarga((e as { status?: number }).status === 404 ? "no_encontrada" : "error");
    }
  }, [token, preguntaId]);

  useEffect(() => {
    void cargar();
  }, [cargar]);

  const operar = useCallback(
    async (operacion: (token: string) => Promise<Pregunta>) => {
      if (!token) return;
      setGuardando(true);
      setError(null);
      try {
        setPregunta(await operacion(token));
      } catch (e: unknown) {
        setError(e);
        await cargar();
      } finally {
        setGuardando(false);
      }
    },
    [token, cargar],
  );

  const cambiarCitas = useCallback(
    (ids: number[]) => operar((tk) => questionsApi.reemplazarCitas(tk, preguntaId, ids)),
    [operar, preguntaId],
  );

  const cambiarEstado = useCallback(
    (payload: CambiarEstadoPayload) => operar((tk) => questionsApi.cambiarEstado(tk, preguntaId, payload)),
    [operar, preguntaId],
  );

  const anadir = useCallback(
    (pasajeId: number) => (pregunta ? cambiarCitas(idsTrasAnadir(pregunta.citas ?? [], pasajeId)) : undefined),
    [pregunta, cambiarCitas],
  );
  const quitar = useCallback(
    (pasajeId: number) => (pregunta ? cambiarCitas(idsTrasQuitar(pregunta.citas ?? [], pasajeId)) : undefined),
    [pregunta, cambiarCitas],
  );

  return { pregunta, carga, guardando, error, cargar, anadir, quitar, cambiarEstado };
}
