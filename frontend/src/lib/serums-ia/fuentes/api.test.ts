import { formularioNuevaFuente, formularioNuevaVersion, fuentesApi } from "./api";
import { fuenteDuplicada, mensajeErrorFuente } from "./errores";
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

function ultima() {
  const [url, init] = fetchMock.mock.calls[fetchMock.mock.calls.length - 1] as [string, RequestInit];
  return { url, init, headers: (init.headers ?? {}) as Record<string, string> };
}

const md = new File(["# Balotario"], "balotario.md", { type: "" });

describe("multipart forms", () => {
  it("S1 sends the file as text/markdown with its name, repeated especialidad_ids and no empty optionals", async () => {
    const form = formularioNuevaFuente({
      archivo: md,
      titulo: "  Balotario  ",
      tipo: "balotario",
      especialidadIds: [2, 5],
    });
    const file = form.get("file") as File;
    expect(file.type).toBe("text/markdown");
    expect(file.name).toBe("balotario.md");
    expect(await file.text()).toBe("# Balotario");
    expect(form.get("titulo")).toBe("Balotario");
    expect(form.get("tipo")).toBe("balotario");
    expect(form.getAll("especialidad_ids")).toEqual(["2", "5"]);
    expect(form.has("anio")).toBe(false);
    expect(form.has("proceso_id")).toBe(false);
  });

  it("S1 includes year and process when given", () => {
    const form = formularioNuevaFuente({
      archivo: md,
      titulo: "B",
      tipo: "guia",
      especialidadIds: [],
      anio: 2025,
      procesoId: 12,
    });
    expect(form.get("anio")).toBe("2025");
    expect(form.get("proceso_id")).toBe("12");
    expect(form.getAll("especialidad_ids")).toEqual([]);
  });

  it("S2 sends fuente_id in the form", () => {
    const form = formularioNuevaVersion(9, md);
    expect(form.get("fuente_id")).toBe("9");
    expect((form.get("file") as File).type).toBe("text/markdown");
  });
});

describe("fuentesApi", () => {
  it("S1 and S2 POST multipart without Content-Type (the browser sets the boundary)", async () => {
    await fuentesApi.crear("tk", { archivo: md, titulo: "B", tipo: "otro", especialidadIds: [] });
    expect(ultima().url).toMatch(/\/admin\/fuentes$/);
    expect(ultima().init.body).toBeInstanceOf(FormData);
    expect(ultima().headers["Content-Type"]).toBeUndefined();
    expect(ultima().headers.Authorization).toBe("Bearer tk");
    await fuentesApi.nuevaVersion("tk", 9, md);
    expect(ultima().url).toMatch(/\/admin\/fuentes\/versiones$/);
  });

  it("S3 lists with the filters, omitting empty ones", async () => {
    await fuentesApi.listar("tk", { estado: "activa", oficial: true, q: "", page: 2, per_page: 20 });
    expect(ultima().url).toMatch(/\/admin\/fuentes\?estado=activa&oficial=true&page=2&per_page=20$/);
  });

  it("S5, S6 and S7 send JSON with trimmed texts", async () => {
    await fuentesApi.cambiarEstado("tk", 9, { estado: "retirada", motivo: "Obsoleta" });
    expect(ultima()).toMatchObject({ url: expect.stringMatching(/\/admin\/fuentes\/9\/estado$/) });
    expect(JSON.parse(String(ultima().init.body))).toEqual({ estado: "retirada", motivo: "Obsoleta" });
    expect(ultima().headers["Content-Type"]).toBe("application/json");
    await fuentesApi.marcarOficial("tk", 9, " MINSA ", " Nota de verificación. ");
    expect(JSON.parse(String(ultima().init.body))).toEqual({
      oficial_verificada: true,
      entidad_emisora: "MINSA",
      nota: "Nota de verificación.",
    });
    await fuentesApi.desmarcarOficial("tk", 9, " Error de carga ");
    expect(JSON.parse(String(ultima().init.body))).toEqual({ oficial_verificada: false, motivo: "Error de carga" });
    await fuentesApi.editar("tk", 9, " Título ", [2]);
    expect(ultima().init.method).toBe("PUT");
    expect(JSON.parse(String(ultima().init.body))).toEqual({ titulo: "Título", especialidad_ids: [2] });
  });
});

describe("errores", () => {
  const t = (clave: string, v?: Record<string, string | number>) => (v ? `${clave}:${JSON.stringify(v)}` : clave);
  const err = (status: number, code: string, detalle?: object) =>
    Object.assign(new Error("msg"), { status, code, detalle });

  it("translates archivo_invalido by its motivo and limite_fuentes by scope", () => {
    expect(mensajeErrorFuente(err(422, "archivo_invalido", { motivo: "codificacion" }), t, "g")).toBe(
      "serumsIa.fuentes.errores.archivo_invalido.codificacion",
    );
    expect(mensajeErrorFuente(err(422, "limite_fuentes", { limite: 100, especialidad_id: 2 }), t, "g")).toBe(
      'serumsIa.fuentes.errores.limite_fuentes.especialidad:{"limite":100}',
    );
    expect(mensajeErrorFuente(err(422, "limite_fuentes", { limite: 500 }), t, "g")).toBe(
      'serumsIa.fuentes.errores.limite_fuentes.total:{"limite":500}',
    );
    expect(mensajeErrorFuente(err(413, "archivo_demasiado_grande"), t, "g")).toBe(
      "serumsIa.fuentes.errores.archivo_demasiado_grande",
    );
  });

  it("falls back to the server message for unknown motivos and to the generic text on network errors", () => {
    expect(mensajeErrorFuente(err(422, "archivo_invalido", { motivo: "nuevo" }), t, "g")).toBe("msg");
    expect(mensajeErrorFuente(new TypeError("Failed to fetch"), t, "g")).toBe("g");
  });

  it("reads the existing source of a 409 fuente_duplicada", () => {
    expect(
      fuenteDuplicada(err(409, "fuente_duplicada", { fuente_id: 9, titulo: "B", estado: "retirada", version: 2 })),
    ).toEqual({
      fuenteId: 9,
      titulo: "B",
      estado: "retirada",
      version: 2,
    });
    expect(fuenteDuplicada(err(409, "estado_invalido"))).toBeNull();
  });
});
