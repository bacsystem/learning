import en from "../../i18n/messages/en.json";
import es from "../../i18n/messages/es.json";
import type { EstadoPregunta } from "./api";
import {
  claveErrorPregunta,
  claveEstadoPregunta,
  claveOrigenPregunta,
  editarContenidoLaRetira,
  ESTADOS_FILTRABLES,
  ESTADOS_PREGUNTA,
  ORIGENES_PREGUNTA,
  puedeEliminar,
} from "./estados";
import { describe, expect, it } from "vitest";

describe("estados de la pregunta (SIA-E3-02)", () => {
  it("filters by every state except the deleted one", () => {
    expect(ESTADOS_FILTRABLES).toEqual(["borrador", "en_revision", "aprobada", "rechazada", "activo"]);
  });

  it.each<[EstadoPregunta, boolean]>([
    ["aprobada", true],
    ["rechazada", true],
    ["activo", true],
    ["borrador", false],
    ["en_revision", false],
    ["eliminada", false],
  ])("offers delete for %s: %s (P2)", (estado, esperado) => {
    expect(puedeEliminar(estado)).toBe(esperado);
  });

  it("warns that editing content takes only published questions out of circulation (P3)", () => {
    expect(ESTADOS_PREGUNTA.filter(editarContenidoLaRetira)).toEqual(["aprobada", "activo"]);
  });
});

describe("claveErrorPregunta", () => {
  const error = (code?: string) => Object.assign(new Error("x"), { status: 409, code });

  it("maps the coded errors of E3-02", () => {
    expect(claveErrorPregunta(error("estado_invalido"))).toBe("admin-preguntas-error-estado-invalido");
    expect(claveErrorPregunta(error("clave_oficial"))).toBe("admin-preguntas-error-clave-oficial");
  });

  it("leaves other errors to the screen", () => {
    expect(claveErrorPregunta(error("no_encontrado"))).toBeUndefined();
    expect(claveErrorPregunta(error())).toBeUndefined();
    expect(claveErrorPregunta(new TypeError("red"))).toBeUndefined();
  });
});

describe("claves i18n de estado y origen", () => {
  const claves = [
    ...ESTADOS_PREGUNTA.map(claveEstadoPregunta),
    ...ORIGENES_PREGUNTA.map(claveOrigenPregunta),
    ...["estado_invalido", "clave_oficial"].map(
      (code) => claveErrorPregunta(Object.assign(new Error("x"), { status: 422, code })) ?? "",
    ),
  ];

  it.each([
    ["es", es.dashboard],
    ["en", en.dashboard],
  ])("exist in %s", (_, mensajes) => {
    const existe = (clave: string) =>
      clave
        .split(".")
        .reduce<unknown>((nodo, parte) => (nodo as Record<string, unknown> | undefined)?.[parte], mensajes) !==
      undefined;
    const faltan = claves.filter((c) => !existe(c));
    expect(faltan).toEqual([]);
  });

  it("live under serumsIa.estadosPregunta, owned by E3-02", () => {
    expect(claveEstadoPregunta("en_revision")).toBe("serumsIa.estadosPregunta.en_revision");
  });
});
