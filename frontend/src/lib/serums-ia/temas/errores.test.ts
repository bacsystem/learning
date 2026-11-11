import { claveErrorTema, mensajeErrorTema } from "./errores";
import { describe, expect, it } from "vitest";

const t = (clave: string) => `t:${clave}`;

function apiError(status: number, message: string, code?: string) {
  return Object.assign(new Error(message), { status, code });
}

describe("claveErrorTema", () => {
  it("maps known codes to serumsIa.errores keys", () => {
    expect(claveErrorTema(apiError(409, "x", "orden_desactualizado"))).toBe("serumsIa.errores.orden_desactualizado");
    expect(claveErrorTema(apiError(422, "x", "limite_temas"))).toBe("serumsIa.errores.limite_temas");
  });

  it("translates 413 cuerpo_demasiado_grande by code, but not the middleware 413 (plain text)", () => {
    expect(claveErrorTema(apiError(413, "x", "cuerpo_demasiado_grande"))).toBe(
      "serumsIa.errores.cuerpo_demasiado_grande",
    );
    expect(claveErrorTema(apiError(413, "El cuerpo de la solicitud es demasiado grande."))).toBeNull();
  });

  it("returns null for unknown codes and legacy errors", () => {
    expect(claveErrorTema(apiError(422, "x", "codigo_nuevo"))).toBeNull();
    expect(claveErrorTema(apiError(403, "Acceso denegado."))).toBeNull();
  });
});

describe("mensajeErrorTema", () => {
  it("prefers the translation of the code", () => {
    expect(mensajeErrorTema(apiError(422, "Ya existe.", "tema_duplicado"), t, "gen")).toBe(
      "t:serumsIa.errores.tema_duplicado",
    );
  });

  it("falls back to the server message", () => {
    expect(mensajeErrorTema(apiError(403, "Acceso denegado."), t, "gen")).toBe("Acceso denegado.");
    expect(mensajeErrorTema(apiError(422, "Otro error.", "codigo_nuevo"), t, "gen")).toBe("Otro error.");
  });

  it("uses the generic text on network errors", () => {
    expect(mensajeErrorTema(new TypeError("Failed to fetch"), t, "gen")).toBe("gen");
    expect(mensajeErrorTema(null, t, "gen")).toBe("gen");
  });
});
