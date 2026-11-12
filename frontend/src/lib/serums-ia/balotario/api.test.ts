import { balotarioApi, PROBLEMAS_POR_PAGINA } from "./api";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const fetchMock = vi.fn();

beforeEach(() => {
  fetchMock.mockImplementation(async () => new Response(JSON.stringify({ ok: true }), { status: 200 }));
  vi.stubGlobal("fetch", fetchMock);
});

afterEach(() => {
  vi.unstubAllGlobals();
  fetchMock.mockReset();
});

function ultima() {
  const [url, init] = fetchMock.mock.calls[fetchMock.mock.calls.length - 1] as [string, RequestInit];
  return { url, init, headers: (init.headers ?? {}) as Record<string, string> };
}

describe("vistaPrevia", () => {
  it("asks for the page of problems with the default size and sends the token", async () => {
    await balotarioApi.vistaPrevia("tok", 9);
    const { url, init, headers } = ultima();
    expect(url).toMatch(/\/admin\/fuentes\/9\/balotario\?page=1&per_page=20$/);
    expect(init.method).toBeUndefined();
    expect(headers.Authorization).toBe("Bearer tok");
    expect(PROBLEMAS_POR_PAGINA).toBe(20);
  });

  it("passes the requested page and size", async () => {
    await balotarioApi.vistaPrevia("tok", 9, 3, 50);
    expect(ultima().url).toMatch(/balotario\?page=3&per_page=50$/);
  });

  it("raises an error that keeps the code and the detail of the API", async () => {
    fetchMock.mockResolvedValueOnce(
      new Response(JSON.stringify({ error: "limite_preguntas", message: "x", detalle: { maximo: 1000 } }), {
        status: 422,
      }),
    );
    await expect(balotarioApi.vistaPrevia("tok", 9)).rejects.toMatchObject({
      status: 422,
      code: "limite_preguntas",
      detalle: { maximo: 1000 },
    });
  });
});

describe("confirmar", () => {
  it("posts exactly the four fields as JSON", async () => {
    const cuerpo = { version: 2, proceso_id: 3, especialidad_id: 2, asignatura_id: 7 };
    await balotarioApi.confirmar("tok", 9, cuerpo);
    const { url, init, headers } = ultima();
    expect(url).toMatch(/\/admin\/fuentes\/9\/balotario\/importaciones$/);
    expect(init.method).toBe("POST");
    expect(headers["Content-Type"]).toBe("application/json");
    expect(JSON.parse(String(init.body))).toEqual(cuerpo);
  });

  it("raises the 409 of an outdated version with the current one", async () => {
    fetchMock.mockResolvedValueOnce(
      new Response(JSON.stringify({ error: "version_desactualizada", message: "x", detalle: { version_vigente: 3 } }), {
        status: 409,
      }),
    );
    await expect(
      balotarioApi.confirmar("tok", 9, { version: 2, proceso_id: 3, especialidad_id: 2, asignatura_id: 7 }),
    ).rejects.toMatchObject({ status: 409, code: "version_desactualizada", detalle: { version_vigente: 3 } });
  });
});
