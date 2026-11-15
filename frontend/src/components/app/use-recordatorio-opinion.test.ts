import {
  almacenRecordatorio,
  cargarRecordatorio,
  descartarRecordatorio,
  marcarOpinionEnviada,
  recordatorioPara,
  reiniciarRecordatorio,
  suscribirRecordatorio,
} from "./use-recordatorio-opinion";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { fetchMiTestimonio } from "@/lib/testimonios/api";

vi.mock("@/lib/testimonios/api", () => ({ fetchMiTestimonio: vi.fn() }));

const memoria = new Map<string, string>();
const esperar = () => new Promise((r) => setTimeout(r, 0));
const SIN_RESOLVER = { resuelto: false, pendiente: false };

beforeEach(() => {
  memoria.clear();
  vi.stubGlobal("localStorage", {
    getItem: (k: string) => memoria.get(k) ?? null,
    setItem: (k: string, v: string) => void memoria.set(k, v),
  });
  vi.mocked(fetchMiTestimonio).mockReset();
  reiniciarRecordatorio();
});

describe("recordatorio de opinión compartido", () => {
  it("is unresolved until the query answers, then pending without a testimonial", async () => {
    vi.mocked(fetchMiTestimonio).mockResolvedValue(null);
    cargarRecordatorio("t1");
    expect(recordatorioPara("t1")).toEqual(SIN_RESOLVER);
    await esperar();
    expect(recordatorioPara("t1")).toEqual({ resuelto: true, pendiente: true });
  });

  it("queries once per token", async () => {
    vi.mocked(fetchMiTestimonio).mockResolvedValue(null);
    cargarRecordatorio("t1");
    cargarRecordatorio("t1");
    await esperar();
    expect(fetchMiTestimonio).toHaveBeenCalledTimes(1);
  });

  it("is not pending with a testimonial, nor after a failed query", async () => {
    vi.mocked(fetchMiTestimonio).mockResolvedValueOnce({} as never);
    cargarRecordatorio("t1");
    await esperar();
    expect(recordatorioPara("t1")).toEqual({ resuelto: true, pendiente: false });
    reiniciarRecordatorio();
    vi.mocked(fetchMiTestimonio).mockRejectedValueOnce(new Error("red"));
    cargarRecordatorio("t2");
    await esperar();
    expect(recordatorioPara("t2")).toEqual({ resuelto: true, pendiente: false });
  });

  it("does not query when the invitation was dismissed", () => {
    memoria.set("opinion_reminder_dismissed", "true");
    cargarRecordatorio("t3");
    expect(fetchMiTestimonio).not.toHaveBeenCalled();
    expect(recordatorioPara("t3")).toEqual({ resuelto: true, pendiente: false });
  });

  it("dismissing updates the shared state, notifies the listeners and stores the choice", async () => {
    vi.mocked(fetchMiTestimonio).mockResolvedValue(null);
    cargarRecordatorio("t4");
    await esperar();
    const oyente = vi.fn();
    const baja = suscribirRecordatorio(oyente);
    descartarRecordatorio();
    expect(oyente).toHaveBeenCalledTimes(1);
    expect(recordatorioPara("t4").pendiente).toBe(false);
    expect(memoria.get("opinion_reminder_dismissed")).toBe("true");
    baja();
  });

  it("sending the opinion resolves the invitation without storing a dismissal", async () => {
    vi.mocked(fetchMiTestimonio).mockResolvedValue(null);
    cargarRecordatorio("t5");
    await esperar();
    marcarOpinionEnviada();
    expect(recordatorioPara("t5")).toEqual({ resuelto: true, pendiente: false });
    expect(memoria.has("opinion_reminder_dismissed")).toBe(false);
  });
});

describe("el estado es de una sola sesión", () => {
  it("another session never sees the invitation of the previous one", async () => {
    vi.mocked(fetchMiTestimonio).mockResolvedValue(null);
    cargarRecordatorio("A");
    await esperar();
    expect(recordatorioPara("A").pendiente).toBe(true);
    expect(recordatorioPara("B")).toEqual(SIN_RESOLVER);
    expect(recordatorioPara(null)).toEqual(SIN_RESOLVER);
  });

  it("a late answer of session A does not overwrite session B", async () => {
    let responderA: (v: null) => void = () => {};
    vi.mocked(fetchMiTestimonio).mockImplementationOnce(() => new Promise((r) => (responderA = r)));
    cargarRecordatorio("A");
    vi.mocked(fetchMiTestimonio).mockResolvedValueOnce({} as never); // B ya opinó
    cargarRecordatorio("B");
    await esperar();
    expect(recordatorioPara("B")).toEqual({ resuelto: true, pendiente: false });
    responderA(null); // A contesta tarde: «sin testimonio»
    await esperar();
    expect(almacenRecordatorio().token).toBe("B");
    expect(recordatorioPara("B")).toEqual({ resuelto: true, pendiente: false });
  });
});
