import {
  ERRORES_ASIGNATURA,
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

  it("translates the process errors: weights, already assigned and active process", () => {
    const codigo = (code: string, status: number) => Object.assign(new Error("x"), { status, code });
    expect(claveErrorApi(legado(400), ERRORES_PROCESO.estado)).toBe("admin-error-proceso-pesos-no-suman");
    expect(claveErrorApi(codigo("peso_duplicado", 409), ERRORES_PROCESO.asignar)).toBe(
      "admin-error-asignatura-ya-asignada",
    );
    expect(claveErrorApi(legado(409), ERRORES_PROCESO.asignar)).toBe("admin-error-proceso-activo-pesos");
    expect(claveErrorApi(legado(409), ERRORES_PROCESO.quitar)).toBe("admin-error-proceso-activo-pesos");
    expect(claveErrorApi(legado(500), ERRORES_PROCESO.asignar)).toBe("admin-error-asignar-asignatura");
    expect(claveErrorApi(legado(500), ERRORES_PROCESO.quitar)).toBe("admin-error-eliminar-asignatura");
    // Un código sin traducción no cae en el mensaje del status 409.
    expect(claveErrorApi(codigo("otro", 409), ERRORES_PROCESO.quitar)).toBe("admin-error-eliminar-asignatura");
  });

  it("falls back to the generic text for anything else", () => {
    expect(claveErrorApi(legado(500), ERRORES_GRADO.guardar)).toBe("comun-error-guardar");
    expect(claveErrorApi(legado(409), ERRORES_INSTITUCION.estado)).toBe("comun-error-estado");
  });

  it("only uses keys that exist in both languages", () => {
    esperarClavesEnAmbosIdiomas(
      ...Object.values(ERRORES_INSTITUCION),
      ...Object.values(ERRORES_GRADO),
      ...Object.values(ERRORES_ESPECIALIDAD),
      ...Object.values(ERRORES_ASIGNATURA),
      ...Object.values(ERRORES_PROCESO),
    );
  });
});
