import { ERROR_ESTADO_INSCRIPCION } from "./errores-admin";
import { describe, expect, it } from "vitest";

import { claveErrorApi } from "@/lib/errores-api";
import { esperarClavesEnAmbosIdiomas } from "@/lib/errores-api.test-utils";

const conCodigo = (status: number, code: string) => Object.assign(new Error("texto crudo"), { status, code });

describe("errores del cambio de estado de una inscripción", () => {
  it("translates motivo_requerido", () => {
    expect(claveErrorApi(conCodigo(422, "motivo_requerido"), ERROR_ESTADO_INSCRIPCION)).toBe(
      "admin-inscripciones-motivo-requerido",
    );
  });

  it("falls back to the generic text for any other error", () => {
    expect(claveErrorApi(conCodigo(500, "error_interno"), ERROR_ESTADO_INSCRIPCION)).toBe("admin-error-actualizar");
  });

  it("has its texts in es and en", () => {
    esperarClavesEnAmbosIdiomas(ERROR_ESTADO_INSCRIPCION);
  });
});
