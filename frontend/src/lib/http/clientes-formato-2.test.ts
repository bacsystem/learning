import { codigoDeConflicto } from "./api-error";
import { afterEach, describe, expect, it, vi } from "vitest";

import { adminGetDashboard } from "@/lib/admin/api";
import { authApi, personasApi } from "@/lib/auth/auth-api";
import { inscripcionesApi } from "@/lib/inscripciones/api";
import { notificacionesApi } from "@/lib/notifications/api";
import { errorCode, practicasApi, tiempoAgotado as tiempoAgotadoPractica } from "@/lib/practices/api";
import { simulacrosApi, tiempoAgotado as tiempoAgotadoSimulacro } from "@/lib/simulations/api";

/**
 * Los clientes anteriores al formato 2 (docs/api/README.md §3) tienen que mostrar el `message` y guardar el código
 * aparte: el 413 del middleware responde `{"error": "<codigo>", "message": "<texto>"}` en todas las rutas.
 */
const TEXTO = "El cuerpo de la solicitud es demasiado grande.";
const responder = (status: number, cuerpo: unknown) =>
  vi.stubGlobal(
    "fetch",
    vi.fn(async () => new Response(JSON.stringify(cuerpo), { status })),
  );

afterEach(() => vi.unstubAllGlobals());

const llamadas = {
  "auth (JSON)": () => authApi.getMe("tk"),
  "auth (importar personas)": () => personasApi.import("tk", new File(["x"], "personas.csv")),
  admin: () => adminGetDashboard("tk"),
  inscripciones: () => inscripcionesApi.listProcesos("tk"),
  notifications: () => notificacionesApi.list("tk"),
  practices: () => practicasApi.getPractica("tk", 1),
  simulations: () => simulacrosApi.getSimulacro("tk", 1),
};

describe.each(Object.entries(llamadas))("cliente %s", (_, llamar) => {
  it("shows the message of a 413 in format 2, and keeps its code apart", async () => {
    responder(413, { error: "archivo_demasiado_grande", message: TEXTO });
    await expect(llamar()).rejects.toMatchObject({ status: 413, message: TEXTO, code: "archivo_demasiado_grande" });
  });

  it("still shows the text of the legacy format, without a code", async () => {
    responder(403, { error: "Acceso denegado." });
    const error = await llamar().catch((e: unknown) => e as Error & { code?: string });
    expect(error).toMatchObject({ status: 403, message: "Acceso denegado." });
    expect((error as { code?: string }).code).toBeUndefined();
  });
});

describe.each([
  ["practices", () => practicasApi.getPractica("tk", 1), tiempoAgotadoPractica, "practica_id", "practica_en_curso"],
  [
    "simulations",
    () => simulacrosApi.getSimulacro("tk", 1),
    tiempoAgotadoSimulacro,
    "simulacro_id",
    "simulacro_en_curso",
  ],
] as const)("409 of %s", (_, llamar, tiempoAgotado, campoId, enCurso) => {
  it("tiempo_agotado carries a message: the code is read from `code`, with the result id", async () => {
    responder(409, { error: "tiempo_agotado", message: "El tiempo terminó.", [campoId]: 5, resultado_id: 9 });
    const error = await llamar().catch((e: unknown) => e);
    expect(tiempoAgotado(error)).toEqual({ resultadoId: 9 });
    expect(error).toMatchObject({ code: "tiempo_agotado", [campoId]: 5, resultado_id: 9 });
  });

  it("the in-progress code arrives without message: the code is read from the message", async () => {
    responder(409, { error: enCurso, [campoId]: 5 });
    const error = await llamar().catch((e: unknown) => e);
    expect(codigoDeConflicto(error)).toBe(enCurso);
    expect(error).toMatchObject({ [campoId]: 5 });
    expect(tiempoAgotado(error)).toBeNull();
  });

  it("tiempo_agotado is not another 409 or another status", async () => {
    responder(409, { error: enCurso, message: "Ya hay uno." });
    expect(tiempoAgotado(await llamar().catch((e: unknown) => e))).toBeNull();
    responder(500, { error: "tiempo_agotado", message: "x" });
    expect(tiempoAgotado(await llamar().catch((e: unknown) => e))).toBeNull();
  });
});

describe("codigoDeConflicto", () => {
  it.each([
    [{ status: 409, message: "simulacro_en_curso" }, "simulacro_en_curso"],
    [{ status: 409, code: "simulacro_en_curso", message: "Ya hay uno." }, "simulacro_en_curso"],
    [{ status: 422, code: "validacion", message: "x" }, undefined],
    [null, undefined],
  ])("%j gives %s", (err, esperado) => {
    expect(codigoDeConflicto(err)).toBe(esperado);
    expect(errorCode(err)).toBe(esperado);
  });
});
