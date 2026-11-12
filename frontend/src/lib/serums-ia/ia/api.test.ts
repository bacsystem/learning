import { iaApi } from "./api";
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

describe("iaApi", () => {
  it.each([
    ["consumo", "/serums-ia/ia/consumo"],
    ["configuracion", "/admin/ia/configuracion"],
    ["gasto", "/admin/ia/gasto"],
  ] as const)("%s reads %s with the token", async (metodo, ruta) => {
    await iaApi[metodo]("tok");
    const { url, init, headers } = ultima();
    expect(url.endsWith(ruta)).toBe(true);
    expect(init.method).toBeUndefined();
    expect(headers.Authorization).toBe("Bearer tok");
  });

  it("guardarConfiguracion replaces the whole configuration with PUT", async () => {
    const cambio = {
      limite_usuario_dia: 5,
      limite_usuario_mes: 60,
      limite_admin_dia: 200,
      max_preguntas_solicitud: 20,
      lote_clasificacion: 20,
      presupuesto_mensual_usd: 40,
    };
    await iaApi.guardarConfiguracion("tok", cambio);
    const { url, init, headers } = ultima();
    expect(url.endsWith("/admin/ia/configuracion")).toBe(true);
    expect(init.method).toBe("PUT");
    expect(headers["Content-Type"]).toBe("application/json");
    expect(JSON.parse(String(init.body))).toEqual(cambio);
  });

  it("raises an error that keeps the code and the detail of a 422 validacion", async () => {
    fetchMock.mockResolvedValueOnce(
      new Response(
        JSON.stringify({ error: "validacion", message: "x", detalle: { campos: { limite_usuario_dia: "max" } } }),
        { status: 422 },
      ),
    );
    await expect(iaApi.guardarConfiguracion("tok", {} as never)).rejects.toMatchObject({
      status: 422,
      code: "validacion",
      detalle: { campos: { limite_usuario_dia: "max" } },
    });
  });
});
