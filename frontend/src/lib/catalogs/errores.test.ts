import {
  ERRORES_ASIGNATURA,
  ERRORES_CORREO_USUARIO,
  ERRORES_ESPECIALIDAD,
  ERRORES_GRADO,
  ERRORES_INSTITUCION,
  ERRORES_PROCESO,
} from "./errores";
import { describe, expect, it } from "vitest";

import { claveErrorApi } from "@/lib/errores-api";
import { esperarClavesEnAmbosIdiomas } from "@/lib/errores-api.test-utils";

const legado = (status: number) => Object.assign(new Error("texto crudo"), { status });

describe("errores de los catálogos", () => {
  it("translates a 409 on save as a duplicate, per catalog", () => {
    expect(claveErrorApi(legado(409), ERRORES_INSTITUCION.guardar)).toBe("admin-error-institucion-duplicada");
    expect(claveErrorApi(legado(409), ERRORES_GRADO.guardar)).toBe("admin-error-grado-duplicado");
    expect(claveErrorApi(legado(409), ERRORES_ESPECIALIDAD.guardar)).toBe("admin-error-especialidad-duplicada");
    expect(claveErrorApi(legado(409), ERRORES_ASIGNATURA.guardar)).toBe("admin-error-asignatura-duplicada");
    expect(claveErrorApi(legado(409), ERRORES_PROCESO.guardar)).toBe("admin-error-proceso-duplicado");
  });

  it("translates a 409 when deactivating a grade or a specialty as linked active users", () => {
    expect(claveErrorApi(legado(409), ERRORES_GRADO.estado)).toBe("admin-error-grado-con-usuarios");
    expect(claveErrorApi(legado(409), ERRORES_ESPECIALIDAD.estado)).toBe("admin-error-especialidad-con-usuarios");
  });

  it("falls back to the generic text for anything else", () => {
    expect(claveErrorApi(legado(500), ERRORES_GRADO.guardar)).toBe("comun-error-guardar");
    expect(claveErrorApi(legado(409), ERRORES_INSTITUCION.estado)).toBe("comun-error-estado");
  });

  it("translates the e-mail change: 409 in use, 422 invalid", () => {
    expect(claveErrorApi(legado(409), ERRORES_CORREO_USUARIO)).toBe("admin-error-correo-en-uso");
    expect(claveErrorApi(legado(422), ERRORES_CORREO_USUARIO)).toBe("admin-error-correo-invalido");
    expect(claveErrorApi(legado(500), ERRORES_CORREO_USUARIO)).toBe("admin-error-actualizar-correo");
  });

  it("only uses keys that exist in both languages", () => {
    esperarClavesEnAmbosIdiomas(
      ...Object.values(ERRORES_INSTITUCION),
      ...Object.values(ERRORES_GRADO),
      ...Object.values(ERRORES_ESPECIALIDAD),
      ...Object.values(ERRORES_ASIGNATURA),
      ...Object.values(ERRORES_PROCESO),
      ERRORES_CORREO_USUARIO,
    );
  });
});
