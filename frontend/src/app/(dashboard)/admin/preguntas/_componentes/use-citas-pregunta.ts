"use client";

import { useCallback, useEffect, useState } from "react";

import { useAuth } from "@/lib/auth/auth-context";
import { type Pregunta, questionsApi } from "@/lib/questions/api";
import { claveErrorRevision } from "@/lib/serums-ia/revision/errores";
import { idsTrasAnadir, idsTrasQuitar } from "@/lib/serums-ia/revision/reglas";

type Carga = "cargando" | "lista" | "error" | "no_encontrada";

/**
 * Pregunta con sus citas y las dos operaciones que las cambian. Cada operación envía la lista completa (E3-07 §4) y
 * pinta lo que responde el servidor; tras un error recarga la pregunta para no mostrar un estado supuesto.
 */
export function useCitasPregunta(preguntaId: number) {
  const { token } = useAuth();
  const [pregunta, setPregunta] = useState<Pregunta | null>(null);
  const [carga, setCarga] = useState<Carga>("cargando");
  const [guardando, setGuardando] = useState(false);
  /** Clave i18n del último error de una operación sobre las citas. */
  const [claveError, setClaveError] = useState<string | null>(null);

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

  const cambiarCitas = useCallback(
    async (ids: number[]) => {
      if (!token) return;
      setGuardando(true);
      setClaveError(null);
      try {
        setPregunta(await questionsApi.reemplazarCitas(token, preguntaId, ids));
      } catch (e: unknown) {
        setClaveError(claveErrorRevision(e));
        await cargar();
      } finally {
        setGuardando(false);
      }
    },
    [token, preguntaId, cargar],
  );

  const anadir = useCallback(
    (pasajeId: number) => (pregunta ? cambiarCitas(idsTrasAnadir(pregunta.citas ?? [], pasajeId)) : undefined),
    [pregunta, cambiarCitas],
  );
  const quitar = useCallback(
    (pasajeId: number) => (pregunta ? cambiarCitas(idsTrasQuitar(pregunta.citas ?? [], pasajeId)) : undefined),
    [pregunta, cambiarCitas],
  );

  return { pregunta, carga, guardando, claveError, cargar, anadir, quitar };
}
