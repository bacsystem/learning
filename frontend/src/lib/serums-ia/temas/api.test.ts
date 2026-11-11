import { temasApi } from "./api";
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

function ultimaLlamada(): { url: string; method: string | undefined; body: unknown } {
  const [url, init] = fetchMock.mock.calls[fetchMock.mock.calls.length - 1] as [string, RequestInit | undefined];
  return { url, method: init?.method, body: init?.body ? JSON.parse(String(init.body)) : undefined };
}

describe("temasApi", () => {
  it("T1 lists with the query, omitting empty filters", async () => {
    await temasApi.listTemas("tk", { asignatura_id: 7, estado: "activo", page: 2, per_page: 100 });
    expect(ultimaLlamada().url).toMatch(/\/admin\/temas\?asignatura_id=7&estado=activo&page=2&per_page=100$/);

    await temasApi.listTemas("tk", { asignatura_id: 7, especialidad_id: undefined });
    expect(ultimaLlamada().url).toMatch(/\/admin\/temas\?asignatura_id=7$/);
  });

  it("sends the bearer token", async () => {
    await temasApi.getTema("tk", 41);
    const [, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect((init.headers as Record<string, string>).Authorization).toBe("Bearer tk");
    expect(ultimaLlamada().url).toMatch(/\/admin\/temas\/41$/);
  });

  it("T3 creates with POST", async () => {
    const payload = { asignatura_id: 7, nombre: "Tema", descripcion: null, especialidad_ids: [2] };
    await temasApi.createTema("tk", payload);
    expect(ultimaLlamada()).toMatchObject({ method: "POST", body: payload });
    expect(ultimaLlamada().url).toMatch(/\/admin\/temas$/);
  });

  it("T4 updates with PUT without asignatura_id", async () => {
    await temasApi.updateTema("tk", 41, { nombre: "Tema", descripcion: "d", especialidad_ids: [] });
    const { url, method, body } = ultimaLlamada();
    expect(url).toMatch(/\/admin\/temas\/41$/);
    expect(method).toBe("PUT");
    expect(body).toEqual({ nombre: "Tema", descripcion: "d", especialidad_ids: [] });
  });

  it("T5 changes the estado with PATCH", async () => {
    await temasApi.updateTemaEstado("tk", 41, "archivado");
    expect(ultimaLlamada()).toMatchObject({ method: "PATCH", body: { estado: "archivado" } });
    expect(ultimaLlamada().url).toMatch(/\/admin\/temas\/41\/estado$/);
  });

  it("T6 sends the full order", async () => {
    await temasApi.reordenarTemas("tk", 7, [41, 38, 40]);
    expect(ultimaLlamada()).toMatchObject({ method: "PUT", body: { tema_ids: [41, 38, 40] } });
    expect(ultimaLlamada().url).toMatch(/\/admin\/asignaturas\/7\/temas\/orden$/);
  });

  it("rejects with the error code of the new format", async () => {
    fetchMock.mockResolvedValueOnce(
      new Response(JSON.stringify({ error: "orden_desactualizado", message: "Recarga." }), { status: 409 }),
    );
    await expect(temasApi.reordenarTemas("tk", 7, [1])).rejects.toMatchObject({
      status: 409,
      code: "orden_desactualizado",
      message: "Recarga.",
    });
  });
});
