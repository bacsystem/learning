import { ERROR_CREAR_PRACTICA, ERROR_CREAR_SIMULACRO, ERROR_INSCRIBIR } from "./errores";
import { describe, expect, it } from "vitest";

import { claveErrorApi } from "@/lib/errores-api";
import { esperarClavesEnAmbosIdiomas } from "@/lib/errores-api.test-utils";

const conCodigo = (status: number, code: string) => Object.assign(new Error("texto crudo"), { status, code });
const legado = (status: number) => Object.assign(new Error("texto crudo"), { status });

describe("errores de acceso a un proceso (PLT-06)", () => {
  it.each([
    ["institucion_no_permitida", "acceso-proceso-institucion-no-permitida"],
    ["nivel_no_permitido", "acceso-proceso-nivel-no-permitido"],
  ])("translates the 403 %s in the three screens", (code, clave) => {
    for (const opciones of [ERROR_INSCRIBIR, ERROR_CREAR_PRACTICA, ERROR_CREAR_SIMULACRO]) {
      expect(claveErrorApi(conCodigo(403, code), opciones)).toBe(clave);
    }
  });

  it("translates inscripcion_requerida in practices and simulations", () => {
    for (const opciones of [ERROR_CREAR_PRACTICA, ERROR_CREAR_SIMULACRO]) {
      expect(claveErrorApi(conCodigo(403, "inscripcion_requerida"), opciones)).toBe(
        "acceso-proceso-inscripcion-requerida",
      );
    }
  });

  it("translates the codes of POST /inscripciones", () => {
    expect(claveErrorApi(conCodigo(409, "ya_inscrito"), ERROR_INSCRIBIR)).toBe("acceso-proceso-ya-inscrito");
    expect(claveErrorApi(conCodigo(409, "sin_monto_configurado"), ERROR_INSCRIBIR)).toBe("acceso-proceso-sin-monto");
    expect(claveErrorApi(conCodigo(404, "no_encontrado"), ERROR_INSCRIBIR)).toBe("acceso-proceso-no-encontrado");
  });

  it("keeps the texts the screens had for errors without a code", () => {
    expect(claveErrorApi(legado(403), ERROR_CREAR_PRACTICA)).toBe("practicas-error-sin-acceso");
    expect(claveErrorApi(legado(422), ERROR_CREAR_PRACTICA)).toBe("practicas-error-preguntas");
    expect(claveErrorApi(legado(500), ERROR_CREAR_PRACTICA)).toBe("practicas-error-crear");
    expect(claveErrorApi(legado(403), ERROR_CREAR_SIMULACRO)).toBe("simulacros-error-sin-acceso");
    expect(claveErrorApi(legado(400), ERROR_CREAR_SIMULACRO)).toBe("simulacros-error-configuracion");
    expect(claveErrorApi(legado(500), ERROR_INSCRIBIR)).toBe("procesos-error-inscribir");
  });

  it("falls back to the generic text for an unknown code", () => {
    expect(claveErrorApi(conCodigo(403, "otro"), ERROR_CREAR_PRACTICA)).toBe("practicas-error-crear");
  });

  it("has every text in es and en", () => {
    esperarClavesEnAmbosIdiomas(ERROR_INSCRIBIR, ERROR_CREAR_PRACTICA, ERROR_CREAR_SIMULACRO);
  });
});
