import type { PreguntaBalotario } from "./api";
import {
  cuerpoConfirmacion,
  esquemaImportacion,
  letrasCorrectas,
  sePuedeImportar,
  ultimaPaginaProblemas,
  VALORES_INICIALES,
} from "./reglas";
import { describe, expect, it } from "vitest";

describe("esquemaImportacion", () => {
  const esquema = esquemaImportacion("obligatorio");

  it("requires the three fields", async () => {
    await expect(esquema.validate(VALORES_INICIALES, { abortEarly: false })).rejects.toMatchObject({
      errors: ["obligatorio", "obligatorio", "obligatorio"],
    });
  });

  it("accepts a complete form", async () => {
    await expect(esquema.isValid({ proceso_id: "3", especialidad_id: "2", asignatura_id: "7" })).resolves.toBe(true);
  });
});

describe("cuerpoConfirmacion", () => {
  it("sends the version the admin saw and the ids as numbers", () => {
    expect(cuerpoConfirmacion(2, { proceso_id: "3", especialidad_id: "2", asignatura_id: "7" })).toEqual({
      version: 2,
      proceso_id: 3,
      especialidad_id: 2,
      asignatura_id: 7,
    });
  });
});

describe("sePuedeImportar", () => {
  it("follows the server flag", () => {
    expect(sePuedeImportar({ puede_confirmar: true })).toBe(true);
    expect(sePuedeImportar({ puede_confirmar: false })).toBe(false);
  });
});

describe("letrasCorrectas", () => {
  const alternativas = (marcas: boolean[]): PreguntaBalotario["alternativas"] =>
    marcas.map((correcta, i) => ({ letra: "ABCDEF"[i], texto: "x", correcta }));

  it("lists the marked letters in order", () => {
    expect(letrasCorrectas({ alternativas: alternativas([false, true, false, true]) })).toBe("B, D");
    expect(letrasCorrectas({ alternativas: alternativas([true, false]) })).toBe("A");
  });

  it("is empty when nothing is marked", () => {
    expect(letrasCorrectas({ alternativas: alternativas([false, false]) })).toBe("");
  });
});

describe("ultimaPaginaProblemas", () => {
  it.each([
    [0, 20, 1],
    [12, 20, 1],
    [20, 20, 1],
    [21, 20, 2],
    [250, 100, 3],
  ])("%i problems at %i per page is %i pages", (total, per_page, esperado) => {
    expect(ultimaPaginaProblemas({ total, per_page })).toBe(esperado);
  });
});
