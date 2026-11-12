import { pesosApi } from "./api";
import { mensajeErrorPesos } from "./errores";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const fetchMock = vi.fn();

beforeEach(() => {
  fetchMock.mockImplementation(async () => new Response(JSON.stringify({}), { status: 200 }));
  vi.stubGlobal("fetch", fetchMock);
});

afterEach(() => {
  vi.unstubAllGlobals();
  fetchMock.mockReset();
});

function ultimaLlamada() {
  const [url, init] = fetchMock.mock.calls[fetchMock.mock.calls.length - 1] as [string, RequestInit | undefined];
  return { url, method: init?.method, body: init?.body ? JSON.parse(String(init.body)) : undefined, init };
}

describe("pesosApi", () => {
  it("PE1 reads the summary of the process", async () => {
    await pesosApi.getResumen("tk", 12);
    expect(ultimaLlamada().url).toMatch(/\/admin\/procesos\/12\/pesos$/);
    expect((ultimaLlamada().init?.headers as Record<string, string>).Authorization).toBe("Bearer tk");
  });

  it("PE2 reads the configuration of (process, specialty)", async () => {
    await pesosApi.getConfiguracion("tk", 12, 2);
    expect(ultimaLlamada().url).toMatch(/\/admin\/procesos\/12\/pesos\/2$/);
  });

  it("PE3 replaces the whole configuration with PUT", async () => {
    const areas = [{ asignatura_id: 7, peso_porcentual: 100, numero_preguntas: 20, coeficiente: 1 }];
    await pesosApi.guardar("tk", 12, 2, areas);
    expect(ultimaLlamada()).toMatchObject({ method: "PUT", body: { areas } });
    expect(ultimaLlamada().url).toMatch(/\/admin\/procesos\/12\/pesos\/2$/);
  });

  it("rejects with the error code and detail", async () => {
    fetchMock.mockResolvedValueOnce(
      new Response(
        JSON.stringify({ error: "pesos_no_suman_100", message: "No suma 100.", detalle: { suma_pesos: 95 } }),
        {
          status: 422,
        },
      ),
    );
    await expect(pesosApi.guardar("tk", 12, 2, [])).rejects.toMatchObject({
      status: 422,
      code: "pesos_no_suman_100",
      detalle: { suma_pesos: 95 },
    });
  });
});

describe("mensajeErrorPesos", () => {
  const t = (clave: string) => `t:${clave}`;
  const err = (status: number, message: string, code?: string) => Object.assign(new Error(message), { status, code });

  it("translates the codes of PE3, with its own key for no_encontrado", () => {
    expect(mensajeErrorPesos(err(409, "x", "peso_duplicado"), t, "gen")).toBe(
      "t:serumsIa.pesos.errores.peso_duplicado",
    );
    expect(mensajeErrorPesos(err(404, "x", "no_encontrado"), t, "gen")).toBe("t:serumsIa.pesos.errores.no_encontrado");
    expect(mensajeErrorPesos(err(500, "x", "error_interno"), t, "gen")).toBe("t:serumsIa.errores.error_interno");
    expect(mensajeErrorPesos(err(413, "x", "cuerpo_demasiado_grande"), t, "gen")).toBe(
      "t:serumsIa.errores.cuerpo_demasiado_grande",
    );
  });

  it("shows the translated generic text for a code without translation, never the server message", () => {
    expect(mensajeErrorPesos(err(422, "Otro error.", "codigo_nuevo"), t, "gen")).toBe("t:serumsIa.errores.generico");
  });

  it("falls back to the server message without a code, and to the generic text without a response", () => {
    expect(mensajeErrorPesos(err(403, "Acceso denegado."), t, "gen")).toBe("Acceso denegado.");
    expect(mensajeErrorPesos(new TypeError("Failed to fetch"), t, "gen")).toBe("gen");
  });
});
