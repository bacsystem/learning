import { profesionApi } from "./api";
import { mensajeErrorProfesion } from "./errores";
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

describe("profesionApi", () => {
  it("F1 reads the options with the session token and no user id", async () => {
    await profesionApi.getOpciones("tk");
    expect(ultimaLlamada().url).toMatch(/\/serums-ia\/profesion\/opciones$/);
    expect((ultimaLlamada().init?.headers as Record<string, string>).Authorization).toBe("Bearer tk");
  });

  it("F2 reads the active profile", async () => {
    await profesionApi.getProfesion("tk");
    expect(ultimaLlamada().url).toMatch(/\/serums-ia\/profesion$/);
  });

  it("F3 saves with PUT and the restart confirmation flag", async () => {
    await profesionApi.guardar("tk", { proceso_id: 12, especialidad_id: 5, confirmar_reinicio: false });
    expect(ultimaLlamada()).toMatchObject({
      method: "PUT",
      body: { proceso_id: 12, especialidad_id: 5, confirmar_reinicio: false },
    });
  });

  it("rejects 409 confirmacion_requerida with the current pair in detalle", async () => {
    fetchMock.mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          error: "confirmacion_requerida",
          message: "Confirma el reinicio.",
          detalle: { actual: { proceso_id: 12, especialidad_id: 2 } },
        }),
        { status: 409 },
      ),
    );
    await expect(
      profesionApi.guardar("tk", { proceso_id: 12, especialidad_id: 5, confirmar_reinicio: false }),
    ).rejects.toMatchObject({
      status: 409,
      code: "confirmacion_requerida",
      detalle: { actual: { proceso_id: 12, especialidad_id: 2 } },
    });
  });
});

describe("mensajeErrorProfesion", () => {
  const t = (clave: string) => `t:${clave}`;
  const err = (status: number, message: string, code?: string) => Object.assign(new Error(message), { status, code });

  it("translates the codes of F1–F3", () => {
    expect(mensajeErrorProfesion(err(403, "x", "inscripcion_requerida"), t, "gen")).toBe(
      "t:serumsIa.profesion.errores.inscripcion_requerida",
    );
    expect(mensajeErrorProfesion(err(422, "x", "especialidad_invalida"), t, "gen")).toBe(
      "t:serumsIa.profesion.errores.especialidad_invalida",
    );
    expect(mensajeErrorProfesion(err(409, "x", "estado_invalido"), t, "gen")).toBe(
      "t:serumsIa.profesion.errores.estado_invalido",
    );
  });

  it("falls back to the server message, and to the generic text on network errors", () => {
    expect(mensajeErrorProfesion(err(400, "Otro."), t, "gen")).toBe("Otro.");
    expect(mensajeErrorProfesion(new TypeError("Failed to fetch"), t, "gen")).toBe("gen");
  });
});
