import { aFila, aNumero, aPayload, esquemaPesos, filaNueva, type FilaPeso, totales } from "./reglas";
import { describe, expect, it } from "vitest";

const esquema = esquemaPesos({
  areasMin: "areasMin",
  areasMax: "areasMax",
  peso: "peso",
  preguntas: "preguntas",
  coeficiente: "coeficiente",
  repetidas: "repetidas",
  limitePreguntas: "limite",
  sumaPesos: "suma",
});

const fila = (id: number, peso: string, preguntas = "10", coeficiente = "1"): FilaPeso => ({
  asignatura_id: id,
  nombre: `Área ${id}`,
  peso_porcentual: peso,
  numero_preguntas: preguntas,
  coeficiente,
});

/** Todos los mensajes, como los recoge formik (`abortEarly: false`). */
async function erroresDe(areas: FilaPeso[]): Promise<string[]> {
  try {
    await esquema.validate({ areas }, { abortEarly: false });
    return [];
  } catch (e) {
    return (e as { errors: string[] }).errors;
  }
}

describe("aNumero", () => {
  it("parses dot and comma decimals and rejects empty or invalid text", () => {
    expect(aNumero("16.67")).toBe(16.67);
    expect(aNumero(" 16,67 ")).toBe(16.67);
    expect(aNumero("")).toBeNull();
    expect(aNumero("abc")).toBeNull();
  });
});

describe("totales", () => {
  it("adds weights in hundredths: 3 × 33.33 + 0.01 is exactly 100", () => {
    const t = totales([fila(1, "33.33"), fila(2, "33.33"), fila(3, "33.34")]);
    expect(t.sumaPesos).toBe(100);
    expect(t.sumaValida).toBe(true);
  });

  it.each([
    ["99.98", false],
    ["99.99", true],
    ["100.01", true],
    ["100.02", false],
  ])("a sum of %s is valid: %s (100 ± 0.01)", (ultimo, valida) => {
    const t = totales([fila(1, "50"), fila(2, String(Math.round((Number(ultimo) - 50) * 100) / 100))]);
    expect(t.sumaPesos).toBe(Number(ultimo));
    expect(t.sumaValida).toBe(valida);
  });

  it("counts empty fields as 0 instead of failing", () => {
    expect(totales([fila(1, "", ""), fila(2, "40", "5")])).toMatchObject({ sumaPesos: 40, sumaPreguntas: 5 });
  });

  it("allows up to 200 questions in total", () => {
    expect(totales([fila(1, "50", "100"), fila(2, "50", "100")]).preguntasValidas).toBe(true);
    expect(totales([fila(1, "50", "100"), fila(2, "50", "101")]).preguntasValidas).toBe(false);
  });

  it("reports repeated areas", () => {
    expect(totales([fila(7, "50"), fila(8, "25"), fila(7, "25")]).repetidas).toEqual([7]);
  });
});

describe("esquemaPesos", () => {
  it("accepts a configuration that sums 100 with up to 200 questions", async () => {
    expect(await erroresDe([fila(1, "60", "120"), fila(2, "40", "80")])).toEqual([]);
  });

  it("requires at least one area", async () => {
    expect(await erroresDe([])).toContain("areasMin");
  });

  it("reports each set rule with its own message", async () => {
    expect(await erroresDe([fila(1, "50"), fila(1, "50")])).toEqual(["repetidas"]);
    expect(await erroresDe([fila(1, "50", "150"), fila(2, "50", "60")])).toEqual(["limite"]);
    expect(await erroresDe([fila(1, "50"), fila(2, "45")])).toEqual(["suma"]);
  });

  it.each([
    ["peso", fila(1, "0"), "peso"],
    ["peso with 3 decimals", fila(1, "10.123"), "peso"],
    ["preguntas 0", fila(1, "100", "0"), "preguntas"],
    ["preguntas not an integer", fila(1, "100", "1.5"), "preguntas"],
    ["preguntas 201", fila(1, "100", "201"), "preguntas"],
    ["coeficiente 0", fila(1, "100", "10", "0"), "coeficiente"],
    ["coeficiente 1000", fila(1, "100", "10", "1000"), "coeficiente"],
    ["empty peso", fila(1, ""), "peso"],
  ])("rejects a row with %s", async (_, f, mensaje) => {
    expect(await erroresDe([f])).toContain(mensaje);
  });
});

describe("conversions", () => {
  it("aFila keeps server values as text and aPayload converts them back", () => {
    const f = aFila({
      asignatura_id: 7,
      nombre: "Salud pública",
      peso_porcentual: 16.67,
      numero_preguntas: 17,
      coeficiente: 1,
    });
    expect(f).toEqual({
      asignatura_id: 7,
      nombre: "Salud pública",
      peso_porcentual: "16.67",
      numero_preguntas: "17",
      coeficiente: "1",
    });
    expect(aPayload([{ ...f, peso_porcentual: "16,67" }])).toEqual([
      { asignatura_id: 7, peso_porcentual: 16.67, numero_preguntas: 17, coeficiente: 1 },
    ]);
  });

  it("filaNueva starts empty with coefficient 1", () => {
    expect(filaNueva(9, "Ética")).toEqual({
      asignatura_id: 9,
      nombre: "Ética",
      peso_porcentual: "",
      numero_preguntas: "",
      coeficiente: "1",
    });
  });
});
