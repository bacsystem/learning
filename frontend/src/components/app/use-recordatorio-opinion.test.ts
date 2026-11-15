import {
  cargarRecordatorio,
  descartarRecordatorio,
  estadoRecordatorio,
  reiniciarRecordatorio,
} from "./use-recordatorio-opinion";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { fetchMiTestimonio } from "@/lib/testimonios/api";

vi.mock("@/lib/testimonios/api", () => ({ fetchMiTestimonio: vi.fn() }));

const memoria = new Map<string, string>();
const esperar = () => new Promise((r) => setTimeout(r, 0));

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
    expect(estadoRecordatorio()).toEqual({ resuelto: false, pendiente: false });
    await esperar();
    expect(estadoRecordatorio()).toEqual({ resuelto: true, pendiente: true });
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
    expect(estadoRecordatorio()).toEqual({ resuelto: true, pendiente: false });
    reiniciarRecordatorio();
    vi.mocked(fetchMiTestimonio).mockRejectedValueOnce(new Error("red"));
    cargarRecordatorio("t2");
    await esperar();
    expect(estadoRecordatorio()).toEqual({ resuelto: true, pendiente: false });
  });

  it("does not query when the invitation was dismissed", () => {
    memoria.set("opinion_reminder_dismissed", "true");
    cargarRecordatorio("t3");
    expect(fetchMiTestimonio).not.toHaveBeenCalled();
    expect(estadoRecordatorio()).toEqual({ resuelto: true, pendiente: false });
  });

  it("dismissing updates the shared state, so the bell and the page agree", async () => {
    vi.mocked(fetchMiTestimonio).mockResolvedValue(null);
    cargarRecordatorio("t4");
    await esperar();
    descartarRecordatorio();
    expect(estadoRecordatorio().pendiente).toBe(false);
    expect(memoria.get("opinion_reminder_dismissed")).toBe("true");
  });
});
