import { claveErrorApi } from "./errores-api";
import { describe, expect, it } from "vitest";

function apiError(status: number, code?: string) {
  return Object.assign(new Error("texto crudo del servidor"), { status, code });
}

const mapas = { porCodigo: { duplicado: "k-codigo" }, porStatus: { 409: "k-status", 404: "k-404" } };

describe("claveErrorApi", () => {
  it("uses the translation of the code", () => {
    expect(claveErrorApi(apiError(409, "duplicado"), { accion: "guardar", ...mapas })).toBe("k-codigo");
  });

  it("does not read the status when the error has a code without translation", () => {
    expect(claveErrorApi(apiError(409, "otro"), { accion: "guardar", ...mapas })).toBe("comun-error-guardar");
  });

  it("reads the status only for legacy errors, without code", () => {
    expect(claveErrorApi(apiError(409), { accion: "guardar", ...mapas })).toBe("k-status");
    expect(claveErrorApi(apiError(404), { accion: "eliminar", ...mapas })).toBe("k-404");
  });

  it("falls back to the generic text of the action, never to the server message", () => {
    expect(claveErrorApi(apiError(500), { accion: "estado", ...mapas })).toBe("comun-error-estado");
    expect(claveErrorApi(apiError(400), { accion: "guardar" })).toBe("comun-error-guardar");
    expect(claveErrorApi(new TypeError("Failed to fetch"), { accion: "cargar", ...mapas })).toBe("comun-error-cargar");
    expect(claveErrorApi(null, { accion: "eliminar" })).toBe("comun-error-eliminar");
  });

  it("prefers the screen's own generic key over the one of the action", () => {
    expect(claveErrorApi(apiError(500), { generico: "admin-ingresos-error-tarifa" })).toBe(
      "admin-ingresos-error-tarifa",
    );
    expect(claveErrorApi(apiError(500), { accion: "guardar", generico: "propia" })).toBe("propia");
  });

  it("does not read inherited keys as codes", () => {
    expect(claveErrorApi(apiError(400, "constructor"), { accion: "guardar", ...mapas })).toBe("comun-error-guardar");
  });
});
