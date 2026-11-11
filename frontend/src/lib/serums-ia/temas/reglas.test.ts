import { aPayload, esquemaTema, mover, normalizarNombre, opcionesEspecialidad } from "./reglas";
import { describe, expect, it } from "vitest";

const esquema = esquemaTema({
  nombreRequerido: "requerido",
  nombreLongitud: "longitud",
  descripcionLongitud: "descripcion",
  especialidadesMax: "max",
});

const valido = { nombre: "Vigilancia epidemiológica", descripcion: "", especialidad_ids: [2, 5] };

async function errorDe(valores: object): Promise<string | null> {
  try {
    await esquema.validate(valores);
    return null;
  } catch (e) {
    return (e as Error).message;
  }
}

describe("normalizarNombre", () => {
  it("trims and collapses inner whitespace", () => {
    expect(normalizarNombre("  Salud   pública \t rural ")).toBe("Salud pública rural");
  });
});

describe("esquemaTema", () => {
  it("accepts a valid tema", async () => {
    expect(await errorDe(valido)).toBeNull();
  });

  it("rejects a name with 2 characters", async () => {
    expect(await errorDe({ ...valido, nombre: "ab" })).toBe("longitud");
  });

  it("counts the name after normalizing whitespace", async () => {
    expect(await errorDe({ ...valido, nombre: "  a    " })).toBe("longitud");
    expect(await errorDe({ ...valido, nombre: " abc " })).toBeNull();
  });

  it("counts code points, not UTF-16 units", async () => {
    expect(await errorDe({ ...valido, nombre: "😀".repeat(120) })).toBeNull();
    expect(await errorDe({ ...valido, nombre: "a".repeat(121) })).toBe("longitud");
  });

  it("requires the name", async () => {
    expect(await errorDe({ ...valido, nombre: "" })).toBe("requerido");
  });

  it("limits the description to 500 characters", async () => {
    expect(await errorDe({ ...valido, descripcion: "x".repeat(500) })).toBeNull();
    expect(await errorDe({ ...valido, descripcion: "x".repeat(501) })).toBe("descripcion");
  });

  it("limits especialidades to 50", async () => {
    const ids = Array.from({ length: 51 }, (_, i) => i + 1);
    expect(await errorDe({ ...valido, especialidad_ids: ids })).toBe("max");
  });
});

describe("aPayload", () => {
  it("normalizes the name, sends an empty description as null and removes repeated ids", () => {
    expect(aPayload({ nombre: " Salud  rural ", descripcion: "  ", especialidad_ids: [2, 2, 5] })).toEqual({
      nombre: "Salud rural",
      descripcion: null,
      especialidad_ids: [2, 5],
    });
  });
});

describe("mover", () => {
  it("moves an item up and down", () => {
    expect(mover(["A", "B", "C"], 2, "subir")).toEqual(["A", "C", "B"]);
    expect(mover(["A", "B", "C"], 0, "bajar")).toEqual(["B", "A", "C"]);
  });

  it("C up twice gives C, A, B", () => {
    expect(mover(mover(["A", "B", "C"], 2, "subir"), 1, "subir")).toEqual(["C", "A", "B"]);
  });

  it("keeps the list at the edges and does not mutate the input", () => {
    const lista = ["A", "B"];
    expect(mover(lista, 0, "subir")).toEqual(["A", "B"]);
    expect(mover(lista, 1, "bajar")).toEqual(["A", "B"]);
    expect(mover(lista, 5, "subir")).toEqual(["A", "B"]);
    expect(lista).toEqual(["A", "B"]);
  });
});

describe("opcionesEspecialidad", () => {
  const activas = [
    { id: 2, nombre: "Medicina" },
    { id: 5, nombre: "Enfermería" },
  ];

  it("lists the active ones and appends associated ones that are no longer active, marked as inactive", () => {
    expect(
      opcionesEspecialidad(activas, [
        { id: 5, nombre: "Enfermería" },
        { id: 9, nombre: "Odontología" },
      ]),
    ).toEqual([
      { id: 2, nombre: "Medicina", inactiva: false },
      { id: 5, nombre: "Enfermería", inactiva: false },
      { id: 9, nombre: "Odontología", inactiva: true },
    ]);
  });

  it("has no inactive options when creating", () => {
    expect(opcionesEspecialidad(activas, []).some((o) => o.inactiva)).toBe(false);
  });
});

describe("esquemaTema with inactive especialidades", () => {
  it("accepts an associated inactive especialidad checked (T4: the server keeps the ones the tema already had)", async () => {
    await expect(esquema.validate({ ...valido, especialidad_ids: [2, 9] })).resolves.toBeTruthy();
  });
});
