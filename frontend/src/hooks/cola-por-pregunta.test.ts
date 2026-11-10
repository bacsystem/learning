import { ColaPorPregunta, ResultadoCierre, Seleccion, siguienteSeleccion } from "./cola-por-pregunta";
import { afterEach, describe, expect, it, vi } from "vitest";

interface Diferido<T> {
  promise: Promise<T>;
  resolve: (v: T) => void;
  reject: (e: unknown) => void;
}

function diferido<T>(): Diferido<T> {
  let resolve!: (v: T) => void;
  let reject!: (e: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

/** Fake server: each request waits until the test settles it. */
function servidor() {
  const llamadas: { seleccion: Seleccion; d: Diferido<Seleccion> }[] = [];
  const enviar = vi.fn((seleccion: Seleccion) => {
    const d = diferido<Seleccion>();
    llamadas.push({ seleccion, d });
    return d.promise;
  });
  return { llamadas, enviar };
}

const flush = () => new Promise((r) => setTimeout(r, 0));

const P = 1;
const err409 = Object.assign(new Error("tiempo_agotado"), { status: 409 });

describe("siguienteSeleccion", () => {
  it("unica replaces, multiple toggles", () => {
    expect(siguienteSeleccion([1], 2, "unica")).toEqual([2]);
    expect(siguienteSeleccion([1], 2, "multiple")).toEqual([1, 2]);
    expect(siguienteSeleccion([1, 2], 1, "multiple")).toEqual([2]);
  });
});

describe("ColaPorPregunta — reemplazar (simulacros)", () => {
  it("double click: the first is in flight, the intermediate set is skipped, the latest is sent", async () => {
    const cola = new ColaPorPregunta("reemplazar");
    cola.inicializar([[P, []]]);
    const srv = servidor();
    const base = { preguntaId: P, tipo: "multiple" as const, actuales: [], enviar: srv.enviar };

    const c1 = cola.click({ ...base, respuestaId: 10 });
    await flush();
    expect(srv.llamadas).toHaveLength(1);
    const c2 = cola.click({ ...base, respuestaId: 11 });
    const c3 = cola.click({ ...base, respuestaId: 12 });
    expect(cola.seleccion(P)).toEqual([10, 11, 12]);
    expect(cola.obtenerEstado().pendientes).toBe(3);
    srv.llamadas[0].d.resolve([10]);
    await flush();
    // The first response does not overwrite the newer optimistic selection.
    expect(cola.seleccion(P)).toEqual([10, 11, 12]);
    // The intermediate set ([10, 11]) is skipped: only the latest one is sent.
    expect(srv.llamadas).toHaveLength(2);
    expect(srv.llamadas[1].seleccion).toEqual([10, 11, 12]);
    srv.llamadas[1].d.resolve([10, 11, 12]);
    await Promise.all([c1, c2, c3]);
    expect(srv.llamadas).toHaveLength(2);
    expect(cola.seleccion(P)).toEqual([10, 11, 12]);
    expect(cola.obtenerEstado().pendientes).toBe(0);
  });

  it("clicks queued before any request is sent collapse into one PUT", async () => {
    const cola = new ColaPorPregunta("reemplazar");
    cola.inicializar([[P, []]]);
    const srv = servidor();
    const base = { preguntaId: P, tipo: "unica" as const, actuales: [], enviar: srv.enviar };
    const clicks = [cola.click({ ...base, respuestaId: 10 }), cola.click({ ...base, respuestaId: 11 })];
    await flush();
    expect(srv.llamadas.map((l) => l.seleccion)).toEqual([[11]]);
    srv.llamadas[0].d.resolve([11]);
    await Promise.all(clicks);
    expect(cola.seleccion(P)).toEqual([11]);
  });

  it("error on the last click reverts to the last confirmed selection", async () => {
    const cola = new ColaPorPregunta("reemplazar");
    cola.inicializar([[P, [5]]]);
    const srv = servidor();
    const onError = vi.fn();
    const c = cola.click({ preguntaId: P, respuestaId: 6, tipo: "unica", actuales: [5], enviar: srv.enviar, onError });
    expect(cola.seleccion(P)).toEqual([6]);
    await flush();
    srv.llamadas[0].d.reject(new Error("boom"));
    await c;
    expect(cola.seleccion(P)).toEqual([5]);
    expect(onError).toHaveBeenCalledWith(expect.any(Error), { esUltimo: true });
  });
});

describe("ColaPorPregunta — conmutar (prácticas)", () => {
  it("double click: sends every click in order and keeps the server truth", async () => {
    const cola = new ColaPorPregunta("conmutar");
    cola.inicializar([[P, []]]);
    const srv = servidor();
    const base = { preguntaId: P, tipo: "multiple" as const, actuales: [], enviar: srv.enviar };
    const onConfirmado = vi.fn();

    const c1 = cola.click({ ...base, respuestaId: 10, onConfirmado });
    const c2 = cola.click({ ...base, respuestaId: 10, onConfirmado });
    expect(cola.seleccion(P)).toEqual([]);
    await flush();
    expect(srv.llamadas).toHaveLength(1);
    srv.llamadas[0].d.resolve([10]);
    await flush();
    // The first response does not overwrite the newer optimistic selection.
    expect(cola.seleccion(P)).toEqual([]);
    expect(srv.llamadas).toHaveLength(2);
    // The server is the truth: it says [10] is still selected.
    srv.llamadas[1].d.resolve([10]);
    await Promise.all([c1, c2]);
    expect(cola.seleccion(P)).toEqual([10]);
    expect(onConfirmado).toHaveBeenCalledTimes(1);
  });

  it("error on an intermediate toggle keeps the newer click and the next response wins", async () => {
    const cola = new ColaPorPregunta("conmutar");
    cola.inicializar([[P, []]]);
    const srv = servidor();
    const onError = vi.fn();
    const base = { preguntaId: P, tipo: "multiple" as const, actuales: [], enviar: srv.enviar, onError };

    const c1 = cola.click({ ...base, respuestaId: 10 });
    const c2 = cola.click({ ...base, respuestaId: 11 });
    await flush();
    srv.llamadas[0].d.reject(new Error("boom"));
    await flush();
    // Not reverted: a newer click is pending.
    expect(cola.seleccion(P)).toEqual([10, 11]);
    expect(onError).toHaveBeenCalledWith(expect.any(Error), { esUltimo: false });
    srv.llamadas[1].d.resolve([11]);
    await Promise.all([c1, c2]);
    expect(cola.seleccion(P)).toEqual([11]);
  });

  it("error on the last toggle reverts to the last confirmed selection", async () => {
    const cola = new ColaPorPregunta("conmutar");
    cola.inicializar([[P, []]]);
    const srv = servidor();
    const base = { preguntaId: P, tipo: "multiple" as const, actuales: [], enviar: srv.enviar };

    const c1 = cola.click({ ...base, respuestaId: 10 });
    const c2 = cola.click({ ...base, respuestaId: 11 });
    await flush();
    srv.llamadas[0].d.resolve([10]);
    await flush();
    srv.llamadas[1].d.reject(new Error("boom"));
    await Promise.all([c1, c2]);
    expect(cola.seleccion(P)).toEqual([10]);
  });
});

describe("ColaPorPregunta — finalizar", () => {
  it("click then immediate finish: finish waits for the queue", async () => {
    const cola = new ColaPorPregunta("reemplazar");
    cola.inicializar([[P, []]]);
    const srv = servidor();
    const cerrar = vi.fn(async (): Promise<ResultadoCierre> => "ok");

    void cola.click({ preguntaId: P, respuestaId: 10, tipo: "unica", actuales: [], enviar: srv.enviar });
    const fin = cola.finalizar(cerrar);
    await flush();
    expect(cerrar).not.toHaveBeenCalled();
    // Clicks after finishing started are ignored.
    void cola.click({ preguntaId: P, respuestaId: 11, tipo: "unica", actuales: [], enviar: srv.enviar });
    srv.llamadas[0].d.resolve([10]);
    await fin;
    expect(cerrar).toHaveBeenCalledTimes(1);
    expect(srv.llamadas).toHaveLength(1);
    expect(cola.seleccion(P)).toEqual([10]);
  });

  it("several 409 call finish only once", async () => {
    const cola = new ColaPorPregunta("conmutar");
    cola.inicializar([]);
    const srv = servidor();
    const cerrar = vi.fn(async (): Promise<ResultadoCierre> => "ok");
    const onError = () => void cola.finalizar(cerrar);

    const clicks = [1, 2, 3].map((p) =>
      cola.click({ preguntaId: p, respuestaId: 10, tipo: "unica", actuales: [], enviar: srv.enviar, onError }),
    );
    await flush();
    srv.llamadas.forEach((l) => l.d.reject(err409));
    await Promise.all(clicks);
    await flush();
    expect(cerrar).toHaveBeenCalledTimes(1);
  });

  it("a failed close allows a retry", async () => {
    const cola = new ColaPorPregunta("reemplazar");
    const cerrar = vi.fn(async (): Promise<ResultadoCierre> => "reintentar");
    await cola.finalizar(cerrar);
    expect(cola.finalizando).toBe(false);
    await cola.finalizar(cerrar);
    expect(cerrar).toHaveBeenCalledTimes(2);
  });
});

describe("ColaPorPregunta — finalizar con plazo (H38)", () => {
  afterEach(() => vi.useRealTimers());

  it("a hung request does not block finish forever: after the deadline it closes and reports it", async () => {
    vi.useFakeTimers();
    const cola = new ColaPorPregunta("reemplazar", { plazoFinalizarMs: 15_000 });
    const cerrar = vi.fn<(info: { plazoVencido: boolean }) => Promise<ResultadoCierre>>(async () => "ok");
    // The PUT never settles.
    void cola.click({
      preguntaId: P,
      respuestaId: 10,
      tipo: "unica",
      actuales: [],
      enviar: () => new Promise<Seleccion>(() => {}),
    });
    const fin = cola.finalizar(cerrar);
    await vi.advanceTimersByTimeAsync(14_999);
    expect(cerrar).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(1);
    await fin;
    expect(cerrar).toHaveBeenCalledTimes(1);
    expect(cerrar).toHaveBeenCalledWith({ plazoVencido: true });
  });

  it("when the queue settles in time the deadline is not reported", async () => {
    vi.useFakeTimers();
    const cola = new ColaPorPregunta("reemplazar", { plazoFinalizarMs: 15_000 });
    const srv = servidor();
    const cerrar = vi.fn<(info: { plazoVencido: boolean }) => Promise<ResultadoCierre>>(async () => "ok");
    void cola.click({ preguntaId: P, respuestaId: 10, tipo: "unica", actuales: [], enviar: srv.enviar });
    const fin = cola.finalizar(cerrar);
    await vi.advanceTimersByTimeAsync(1_000);
    srv.llamadas[0].d.resolve([10]);
    await fin;
    expect(cerrar).toHaveBeenCalledWith({ plazoVencido: false });
    // No timer left behind.
    expect(vi.getTimerCount()).toBe(0);
  });
});
