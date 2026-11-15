import { claveErrorRespuesta, validarPregunta, type ValoresValidablesPregunta } from "./pregunta";
import { describe, expect, it } from "vitest";

const m = {
  procesoRequerido: "P",
  asignaturaRequerida: "A",
  enunciadoRequerido: "ER",
  enunciadoLongitud: "EL",
  respuestaRequerida: "RR",
  respuestaMaximo: "RM",
  respuestasMinimo: "RMIN",
  retroLongitud: "RL",
};

const valido: ValoresValidablesPregunta = {
  proceso_id: 1,
  asignatura_id: 2,
  enunciado: "Un enunciado de más de diez",
  respuestas: [{ texto: "a" }, { texto: "b" }],
  retro_explicacion: "",
};

describe("validarPregunta", () => {
  it("accepts a valid form", () => {
    expect(validarPregunta(valido, m, true)).toEqual({});
  });

  it("reports every missing required field at once", () => {
    const errores = validarPregunta(
      { ...valido, proceso_id: 0, asignatura_id: 0, enunciado: "  ", respuestas: [{ texto: "" }, { texto: "b" }] },
      m,
      true,
    );
    expect(errores).toEqual({
      proceso_id: "P",
      asignatura_id: "A",
      enunciado: "ER",
      [claveErrorRespuesta(0)]: "RR",
    });
  });

  it("does not require the process when it is fixed (edit)", () => {
    expect(validarPregunta({ ...valido, proceso_id: 0 }, m, false)).toEqual({});
  });

  it("checks statement, answer and feedback lengths", () => {
    const errores = validarPregunta(
      {
        ...valido,
        enunciado: "corto",
        respuestas: [{ texto: "x".repeat(501) }, { texto: "b" }],
        retro_explicacion: "breve",
      },
      m,
      true,
    );
    expect(errores).toEqual({ enunciado: "EL", [claveErrorRespuesta(0)]: "RM", retro_explicacion: "RL" });
  });

  it("requires at least two answers", () => {
    expect(validarPregunta({ ...valido, respuestas: [{ texto: "a" }] }, m, true)).toEqual({ respuestas: "RMIN" });
  });
});
