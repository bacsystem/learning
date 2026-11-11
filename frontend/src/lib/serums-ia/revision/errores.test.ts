import en from "../../../i18n/messages/en.json";
import es from "../../../i18n/messages/es.json";
import { claveErrorPregunta } from "./errores";
import { describe, expect, it } from "vitest";

const error = (code?: string) => Object.assign(new Error("texto crudo del servidor"), { status: 409, code });

const tieneEn = (mensajes: object) => (clave: string) =>
  clave
    .split(".")
    .reduce<unknown>((nodo, parte) => (nodo as Record<string, unknown> | undefined)?.[parte], mensajes) !== undefined;

const CODIGOS_DE_PREGUNTAS = [
  "fuente_no_activa",
  "limite_citas",
  "ultima_cita_activa",
  "clave_oficial",
];

describe.each([
  ["es", es.dashboard],
  ["en", en.dashboard],
])("claveErrorPregunta (%s)", (_, mensajes) => {
  const tiene = tieneEn(mensajes);

  it.each(CODIGOS_DE_PREGUNTAS)("uses the module key of %s, which exists", (code) => {
    const clave = claveErrorPregunta(error(code), tiene);
    expect(clave).toBe(`serumsIa.errores.${code}`);
    expect(tiene(clave)).toBe(true);
  });

  it.each(["estado_invalido", "no_encontrado", "validacion", "referencia_invalida"])(
    "uses the resource key of the common code %s",
    (code) => {
      expect(claveErrorPregunta(error(code), tiene)).toBe(`serumsIa.preguntas.errores.${code}`);
    },
  );

  it("falls back to the generic text, never to the server message", () => {
    expect(claveErrorPregunta(error("codigo_nuevo"), tiene)).toBe("serumsIa.errores.generico");
    expect(claveErrorPregunta(error(), tiene)).toBe("serumsIa.errores.generico");
    expect(claveErrorPregunta(new TypeError("red"), tiene)).toBe("serumsIa.errores.generico");
  });

  it("does not look up inherited object keys", () => {
    expect(claveErrorPregunta(error("constructor"), () => false)).toBe("serumsIa.errores.generico");
  });
});

describe("claveErrorModulo", () => {
  it("falls back to the neutral module text when the resource has no own key", () => {
    const claves = new Set(["serumsIa.errores.estado_invalido"]);
    expect(claveErrorPregunta(error("estado_invalido"), (c) => claves.has(c))).toBe("serumsIa.errores.estado_invalido");
  });
});
