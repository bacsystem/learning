import { ERRORES_CORREO_USUARIO } from "./errores-usuario";
import { describe, expect, it } from "vitest";

import { claveErrorApi } from "@/lib/errores-api";
import { esperarClavesEnAmbosIdiomas } from "@/lib/errores-api.test-utils";

const legado = (status: number) => Object.assign(new Error("texto crudo"), { status });

describe("ERRORES_CORREO_USUARIO", () => {
  it("translates 409 as in use and 422 as invalid", () => {
    expect(claveErrorApi(legado(409), ERRORES_CORREO_USUARIO)).toBe("admin-error-correo-en-uso");
    expect(claveErrorApi(legado(422), ERRORES_CORREO_USUARIO)).toBe("admin-error-correo-invalido");
  });

  it("falls back to its generic text", () => {
    expect(claveErrorApi(legado(500), ERRORES_CORREO_USUARIO)).toBe("admin-error-actualizar-correo");
  });

  it("only uses keys that exist in both languages", () => {
    esperarClavesEnAmbosIdiomas(ERRORES_CORREO_USUARIO);
  });
});
