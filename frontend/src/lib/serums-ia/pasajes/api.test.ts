import { pasajesApi } from "./api";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const fetchMock = vi.fn();
beforeEach(() => vi.stubGlobal("fetch", fetchMock));
afterEach(() => {
  vi.unstubAllGlobals();
  fetchMock.mockReset();
});

describe("pasajesApi.buscar", () => {
  it("encodes the query, sends the token and unwraps data", async () => {
    const pasaje = {
      pasaje_id: 1,
      fuente_id: 2,
      fuente_titulo: "NTS",
      version: 1,
      seccion: null,
      linea_inicio: 1,
      linea_fin: 2,
      extracto: "x",
    };
    fetchMock.mockResolvedValue(new Response(JSON.stringify({ data: [pasaje] }), { status: 200 }));
    const signal = new AbortController().signal;

    await expect(pasajesApi.buscar("tok", "hipertensión & más", signal)).resolves.toEqual([pasaje]);

    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toMatch(/\/admin\/pasajes\?q=hipertensi%C3%B3n%20%26%20m%C3%A1s$/);
    expect((init.headers as Record<string, string>).Authorization).toBe("Bearer tok");
    expect(init.signal).toBe(signal);
  });

  it("throws the API error with its code", async () => {
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify({ error: "consulta_invalida", message: "x" }), { status: 422 }),
    );
    await expect(pasajesApi.buscar("tok", "ab")).rejects.toMatchObject({ status: 422, code: "consulta_invalida" });
  });
});
