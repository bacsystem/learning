import { questionsApi } from "./api";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const fetchMock = vi.fn();
beforeEach(() => {
  fetchMock.mockResolvedValue(new Response(JSON.stringify({ id: 5 }), { status: 200 }));
  vi.stubGlobal("fetch", fetchMock);
});
afterEach(() => {
  vi.unstubAllGlobals();
  fetchMock.mockReset();
});

const ultima = () => fetchMock.mock.calls[fetchMock.mock.calls.length - 1] as [string, RequestInit];

describe("questionsApi (SIA-E3-07)", () => {
  it("reemplazarCitas sends the whole list with PUT", async () => {
    await questionsApi.reemplazarCitas("tok", 5, [9, 4]);
    const [url, init] = ultima();
    expect(url).toMatch(/\/preguntas\/5\/citas$/);
    expect(init.method).toBe("PUT");
    expect(JSON.parse(init.body as string)).toEqual({ pasaje_ids: [9, 4] });
  });

  it("reemplazarCitas can remove every citation", async () => {
    await questionsApi.reemplazarCitas("tok", 5, []);
    expect(JSON.parse(ultima()[1].body as string)).toEqual({ pasaje_ids: [] });
  });

  it("cambiarEstado sends the four-field body with PATCH", async () => {
    await questionsApi.cambiarEstado("tok", 5, { estado: "aprobada", verificado: true, nota: null });
    const [url, init] = ultima();
    expect(url).toMatch(/\/preguntas\/5\/estado$/);
    expect(init.method).toBe("PATCH");
    expect(JSON.parse(init.body as string)).toEqual({ estado: "aprobada", verificado: true, nota: null });
  });
});
