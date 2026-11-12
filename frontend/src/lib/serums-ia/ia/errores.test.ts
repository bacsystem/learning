import en from "../../../i18n/messages/en.json";
import es from "../../../i18n/messages/es.json";
import { claveErrorIa } from "./errores";
import { describe, expect, it } from "vitest";

const error = (code?: string) => Object.assign(new Error("texto crudo del servidor"), { status: 429, code });

const tieneEn = (mensajes: object) => (clave: string) =>
  clave
    .split(".")
    .reduce<unknown>((nodo, parte) => (nodo as Record<string, unknown> | undefined)?.[parte], mensajes) !== undefined;

describe.each([
  ["es", es.dashboard],
  ["en", en.dashboard],
])("claveErrorIa (%s)", (_, mensajes) => {
  const tiene = tieneEn(mensajes);

  it.each(["presupuesto_agotado", "limite_ia", "limite_ia_mensual"])("uses the module key of the 429 %s", (code) => {
    const clave = claveErrorIa(error(code), tiene);
    expect(clave).toBe(`serumsIa.errores.${code}`);
    expect(tiene(clave)).toBe(true);
  });

  it("uses the ia key of validacion", () => {
    const clave = claveErrorIa(error("validacion"), tiene);
    expect(clave).toBe("serumsIa.ia.errores.validacion");
    expect(tiene(clave)).toBe(true);
  });

  it("falls back to the generic text, never to the server message", () => {
    expect(claveErrorIa(error("codigo_nuevo"), tiene)).toBe("serumsIa.errores.generico");
  });
});
