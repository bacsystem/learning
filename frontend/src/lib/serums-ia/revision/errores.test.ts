import en from "../../../i18n/messages/en.json";
import es from "../../../i18n/messages/es.json";
import { CLAVE_ERROR_GENERICO, claveErrorRevision } from "./errores";
import { describe, expect, it } from "vitest";

const error = (code?: string) => Object.assign(new Error("texto crudo del servidor"), { status: 422, code });

const existe = (mensajes: object, clave: string) =>
  clave
    .split(".")
    .reduce<unknown>((nodo, parte) => (nodo as Record<string, unknown> | undefined)?.[parte], mensajes) !== undefined;

describe("claveErrorRevision", () => {
  it.each(["fuente_no_activa", "limite_citas", "ultima_cita_activa"])("translates %s", (code) => {
    expect(claveErrorRevision(error(code))).toBe(`serumsIa.errores.${code}`);
  });

  it("uses the generic text for unknown codes, errors without code and network failures", () => {
    expect(claveErrorRevision(error("codigo_nuevo"))).toBe(CLAVE_ERROR_GENERICO);
    expect(claveErrorRevision(error())).toBe(CLAVE_ERROR_GENERICO);
    expect(claveErrorRevision(new TypeError("red"))).toBe(CLAVE_ERROR_GENERICO);
  });

  it("does not map inherited object keys", () => {
    expect(claveErrorRevision(error("constructor"))).toBe(CLAVE_ERROR_GENERICO);
  });

  it.each([
    ["es", es.dashboard],
    ["en", en.dashboard],
  ])("has every key in %s", (_, mensajes) => {
    const claves = ["fuente_no_activa", "limite_citas", "ultima_cita_activa"].map((c) => claveErrorRevision(error(c)));
    expect([...claves, CLAVE_ERROR_GENERICO].filter((c) => !existe(mensajes, c))).toEqual([]);
  });
});
