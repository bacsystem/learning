import { claveErrorApi } from "./errores-api";
import { describe, expect, it } from "vitest";

function apiError(status: number, code?: string) {
  return Object.assign(new Error("texto crudo del servidor"), { status, code });
}

const mapas = { porCodigo: { duplicado: "k-codigo" }, porStatus: { 409: "k-status", 404: "k-404" } };

describe("claveErrorApi", () => {
  it("prefers the translation of the code over the one of the status", () => {
    expect(claveErrorApi(apiError(409, "duplicado"), "guardar", mapas)).toBe("k-codigo");
  });

  it("falls back to the status when the code has no translation or there is none", () => {
    expect(claveErrorApi(apiError(409, "otro"), "guardar", mapas)).toBe("k-status");
    expect(claveErrorApi(apiError(404), "eliminar", mapas)).toBe("k-404");
  });

  it("falls back to the generic text of the action, never to the server message", () => {
    expect(claveErrorApi(apiError(500), "estado", mapas)).toBe("comun-error-estado");
    expect(claveErrorApi(apiError(400), "guardar")).toBe("comun-error-guardar");
    expect(claveErrorApi(new TypeError("Failed to fetch"), "cargar", mapas)).toBe("comun-error-cargar");
    expect(claveErrorApi(null, "eliminar")).toBe("comun-error-eliminar");
  });

  it("does not read inherited keys as codes", () => {
    expect(claveErrorApi(apiError(400, "constructor"), "guardar", mapas)).toBe("comun-error-guardar");
  });
});
