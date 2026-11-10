/**
 * H37: per-question answer queue shared by simulacros and prácticas (no React).
 *
 * The requests of one question are serialized. Two modes:
 * - "reemplazar" (simulacros, PUT of the whole set): a click whose set was
 *   already superseded by a later click is skipped; only the latest set is sent.
 * - "conmutar" (prácticas, POST that toggles on the server): every click is
 *   sent, in order, and the screen keeps the selection the server returned.
 *
 * In both modes the screen shows the optimistic selection while a newer click
 * is pending; on error it reverts to the last selection the server confirmed,
 * only when the failed click is the latest one.
 *
 * `finalizar` closes the attempt once: it blocks new clicks and waits for the
 * pending requests before calling `cerrar`, at most `plazoFinalizarMs` (H38:
 * fetch has no timeout, a hung request must not block the close forever).
 */

export type ModoCola = "reemplazar" | "conmutar";

export type Seleccion = number[];

export interface Click {
  preguntaId: number;
  respuestaId: number;
  tipo: "unica" | "multiple";
  /** Selection known for the question when the queue has none yet. */
  actuales: Seleccion;
  /** Sends the click; resolves with the selection the server keeps. */
  enviar: (seleccion: Seleccion) => Promise<Seleccion>;
  /** Server confirmed the latest click of the question. */
  onConfirmado?: (seleccion: Seleccion) => void;
  /** The request failed. `esUltimo`: no newer click of the question is pending. */
  onError?: (e: unknown, info: { esUltimo: boolean }) => void;
}

export interface EstadoCola {
  selecciones: ReadonlyMap<number, Seleccion>;
  /** Requests not settled yet (all questions). */
  pendientes: number;
}

export type ResultadoCierre = "ok" | "reintentar";

export interface OpcionesCola {
  /** Max wait for the pending requests before closing anyway (H38). */
  plazoFinalizarMs?: number;
}

export const PLAZO_FINALIZAR_MS = 15_000;

export function siguienteSeleccion(prev: Seleccion, respuestaId: number, tipo: "unica" | "multiple"): Seleccion {
  // Same rule as the server: 'unica' replaces, 'multiple' toggles.
  if (tipo !== "multiple") return [respuestaId];
  return prev.includes(respuestaId) ? prev.filter((x) => x !== respuestaId) : [...prev, respuestaId];
}

export class ColaPorPregunta {
  private estado: EstadoCola = { selecciones: new Map(), pendientes: 0 };
  private readonly confirmadas = new Map<number, Seleccion>();
  private readonly seq = new Map<number, number>();
  private readonly colas = new Map<number, Promise<void>>();
  private readonly oyentes = new Set<() => void>();
  private finalizandoFlag = false;

  private readonly plazoFinalizarMs: number;

  constructor(
    private readonly modo: ModoCola,
    opciones: OpcionesCola = {},
  ) {
    this.plazoFinalizarMs = opciones.plazoFinalizarMs ?? PLAZO_FINALIZAR_MS;
  }

  // ── store (useSyncExternalStore) ────────────────────────────────────────────

  suscribir = (oyente: () => void): (() => void) => {
    this.oyentes.add(oyente);
    return () => this.oyentes.delete(oyente);
  };

  obtenerEstado = (): EstadoCola => this.estado;

  private actualizar(parcial: Partial<EstadoCola>) {
    this.estado = { ...this.estado, ...parcial };
    this.oyentes.forEach((o) => o());
  }

  private setSeleccion(preguntaId: number, ids: Seleccion) {
    this.actualizar({ selecciones: new Map(this.estado.selecciones).set(preguntaId, ids) });
  }

  // ── API ──────────────────────────────────────────────────────────────────────

  get finalizando(): boolean {
    return this.finalizandoFlag;
  }

  /** Loads the selections of the server (first load or resync). */
  inicializar(selecciones: Iterable<[number, Seleccion]>) {
    const sel = new Map(selecciones);
    this.confirmadas.clear();
    sel.forEach((ids, id) => this.confirmadas.set(id, ids));
    this.actualizar({ selecciones: sel });
  }

  seleccion(preguntaId: number): Seleccion | undefined {
    return this.estado.selecciones.get(preguntaId);
  }

  /** Queues a click. Ignored once finalizing started. Resolves when it settles. */
  click(c: Click): Promise<void> {
    // No new answers once finalizing started (they would land after the close).
    if (this.finalizandoFlag) return Promise.resolve();
    const { preguntaId } = c;
    const prev = this.estado.selecciones.get(preguntaId) ?? c.actuales;
    const next = siguienteSeleccion(prev, c.respuestaId, c.tipo);
    // Optimistic update
    this.setSeleccion(preguntaId, next);

    const seq = (this.seq.get(preguntaId) ?? 0) + 1;
    this.seq.set(preguntaId, seq);
    const esUltimo = () => this.seq.get(preguntaId) === seq;

    const send = async () => {
      // reemplazar: a later click already queued the newer full set.
      if (this.modo === "reemplazar" && !esUltimo()) return;
      try {
        const ids = await c.enviar(next);
        this.confirmadas.set(preguntaId, ids);
        if (!esUltimo()) return; // a newer click is pending; keep it on screen
        this.setSeleccion(preguntaId, ids);
        c.onConfirmado?.(ids);
      } catch (e: unknown) {
        const ultimo = esUltimo();
        // Revert only to the last confirmed state, and only if no newer click
        // is pending (its response will bring the final state).
        if (ultimo) this.setSeleccion(preguntaId, this.confirmadas.get(preguntaId) ?? c.actuales);
        c.onError?.(e, { esUltimo: ultimo });
      }
    };

    this.actualizar({ pendientes: this.estado.pendientes + 1 });
    const queued = (this.colas.get(preguntaId) ?? Promise.resolve())
      .then(send)
      .finally(() => this.actualizar({ pendientes: this.estado.pendientes - 1 }));
    this.colas.set(preguntaId, queued);
    return queued;
  }

  /** Waits for the pending requests of one question. */
  async esperarPregunta(preguntaId: number): Promise<void> {
    await this.colas.get(preguntaId);
  }

  /** Waits for the pending requests of every question, at most `plazoMs`. */
  async esperar(plazoMs?: number): Promise<"ok" | "plazo_vencido"> {
    const todas = Promise.allSettled(Array.from(this.colas.values())).then(() => "ok" as const);
    if (plazoMs === undefined) return todas;
    let timer: ReturnType<typeof setTimeout> | undefined;
    const plazo = new Promise<"plazo_vencido">((resolve) => {
      timer = setTimeout(() => resolve("plazo_vencido"), plazoMs);
    });
    try {
      return await Promise.race([todas, plazo]);
    } finally {
      clearTimeout(timer);
    }
  }

  /**
   * Closes the attempt once. Concurrent calls while one is running are
   * ignored. `cerrar` returns "reintentar" (or throws) to allow a new try;
   * `plazoVencido` tells it some answer may not have been saved.
   */
  async finalizar(cerrar: (info: { plazoVencido: boolean }) => Promise<ResultadoCierre>): Promise<void> {
    if (this.finalizandoFlag) return;
    this.finalizandoFlag = true;
    // H123: wait for the answers still in flight before closing the attempt (H38: with a deadline).
    const plazoVencido = (await this.esperar(this.plazoFinalizarMs)) === "plazo_vencido";
    let r: ResultadoCierre;
    try {
      r = await cerrar({ plazoVencido });
    } catch {
      r = "reintentar";
    }
    if (r === "reintentar") this.finalizandoFlag = false;
  }
}
