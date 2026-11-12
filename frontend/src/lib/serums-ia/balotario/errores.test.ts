import en from "../../../i18n/messages/en.json";
import es from "../../../i18n/messages/es.json";
import { claveErrorBalotario, pideRecargar, valoresDeError } from "./errores";
import { describe, expect, it } from "vitest";

const error = (code?: string, detalle?: Record<string, unknown>) =>
  Object.assign(new Error("texto crudo del servidor"), { status: 422, code, detalle });

const tieneEn = (mensajes: object) => (clave: string) =>
  clave
    .split(".")
    .reduce<unknown>((nodo, parte) => (nodo as Record<string, unknown> | undefined)?.[parte], mensajes) !== undefined;

/** Códigos con texto de todo el módulo (docs/api/…balotario.md §7). */
const CODIGOS_DE_MODULO = [
  "version_desactualizada",
  "ya_importado",
  "tipo_fuente_invalido",
  "fuente_no_activa",
  "especialidad_invalida",
  "especialidad_no_corresponde",
  "sin_validas",
  "error_interno",
];
/** Códigos que cambian de sentido según el recurso: texto propio de `balotario`. */
const CODIGOS_DE_RECURSO = ["no_encontrado", "validacion", "referencia_invalida", "limite_preguntas"];

describe.each([
  ["es", es.dashboard],
  ["en", en.dashboard],
])("claveErrorBalotario (%s)", (_, mensajes) => {
  const tiene = tieneEn(mensajes);

  it.each(CODIGOS_DE_MODULO)("uses the module key of %s, which exists", (code) => {
    const clave = claveErrorBalotario(error(code), tiene);
    expect(clave).toBe(`serumsIa.errores.${code}`);
    expect(tiene(clave)).toBe(true);
  });

  it.each(CODIGOS_DE_RECURSO)("uses the balotario key of %s, which exists", (code) => {
    const clave = claveErrorBalotario(error(code), tiene);
    expect(clave).toBe(`serumsIa.balotario.errores.${code}`);
    expect(tiene(clave)).toBe(true);
  });

  it.each(["parametro_invalido", "cuerpo_invalido", "cuerpo_demasiado_grande"])("translates %s", (code) => {
    expect(tiene(claveErrorBalotario(error(code), tiene))).toBe(true);
  });

  it("falls back to the generic text, never to the server message", () => {
    expect(claveErrorBalotario(error("codigo_nuevo"), tiene)).toBe("serumsIa.errores.generico");
    expect(claveErrorBalotario(new TypeError("red"), tiene)).toBe("serumsIa.errores.generico");
  });
});

describe("pideRecargar", () => {
  it.each(["version_desactualizada", "ya_importado"])("is true for %s", (code) => {
    expect(pideRecargar(error(code))).toBe(true);
  });

  it.each(["sin_validas", "limite_preguntas", undefined])("is false for %s", (code) => {
    expect(pideRecargar(error(code))).toBe(false);
  });
});

describe("valoresDeError", () => {
  it("takes only the numbers the texts interpolate", () => {
    expect(valoresDeError(error("limite_preguntas", { maximo: 1000, detectadas: 1200, otro: "x" }))).toEqual({
      maximo: 1000,
      detectadas: 1200,
    });
  });

  it("ignores a detail that is missing or has other types", () => {
    expect(valoresDeError(error("limite_preguntas"))).toEqual({});
    expect(valoresDeError(error("limite_preguntas", { maximo: "1000" }))).toEqual({});
    expect(valoresDeError(null)).toEqual({});
  });
});
