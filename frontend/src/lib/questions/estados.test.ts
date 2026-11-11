import en from "../../i18n/messages/en.json";
import es from "../../i18n/messages/es.json";
import type { EstadoPregunta, Pregunta } from "./api";
import {
  cambiaElContenido,
  claveErrorPregunta,
  claveEstadoPregunta,
  claveOrigenPregunta,
  contenidoDePregunta,
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
    expect(claveErrorPregunta(error("estado_invalido"))).toBe("serumsIa.avisosPregunta.eliminarAyuda");
    expect(claveErrorPregunta(error("clave_oficial"))).toBe("serumsIa.errores.clave_oficial");
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

  it("use kebab case for multi-word states", () => {
    expect(claveEstadoPregunta("en_revision")).toBe("admin-preguntas-estado-en-revision");
  });
});

describe("cambiaElContenido (aviso de edición, E3-02 §4.5)", () => {
  const guardada = {
    enunciado: "¿Cuál es la capital?",
    tipo: "unica",
    respuestas: [
      { id: 1, texto: "Lima", es_correcta: true, orden: 1 },
      { id: 2, texto: "Cusco", es_correcta: false, orden: 2 },
    ],
    retroalimentacion: { explicacion: "Es Lima.", referencia: null },
  } as Pregunta;
  const base = () => structuredClone(contenidoDePregunta(guardada));

  it("is false when nothing changed, ignoring surrounding spaces", () => {
    const editado = { ...base(), enunciado: "  ¿Cuál es la capital? " };
    expect(cambiaElContenido(contenidoDePregunta(guardada), editado)).toBe(false);
  });

  it("is false when the alternatives are only reordered", () => {
    const editado = { ...base(), respuestas: [...base().respuestas].reverse() };
    expect(cambiaElContenido(contenidoDePregunta(guardada), editado)).toBe(false);
  });

  it.each<[string, (c: ReturnType<typeof base>) => ReturnType<typeof base>]>([
    ["the statement", (c) => ({ ...c, enunciado: "Otra" })],
    ["the type", (c) => ({ ...c, tipo: "multiple" })],
    ["an alternative text", (c) => ({ ...c, respuestas: [{ texto: "Arequipa", es_correcta: true }, c.respuestas[1]] })],
    ["the answer key", (c) => ({ ...c, respuestas: c.respuestas.map((r) => ({ ...r, es_correcta: !r.es_correcta })) })],
    ["the explanation", (c) => ({ ...c, explicacion: "Es Cusco." })],
  ])("is true when %s changes", (_, cambiar) => {
    expect(cambiaElContenido(contenidoDePregunta(guardada), cambiar(base()))).toBe(true);
  });
});
