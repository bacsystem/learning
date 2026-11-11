import type { FuenteResumen } from "./api";
import {
  accionesDisponibles,
  ARCHIVO_MAX_BYTES,
  enProceso,
  esquemaEdicion,
  esquemaMarca,
  esquemaMotivo,
  esquemaSubida,
  type MensajesFuente,
  validarArchivo,
} from "./reglas";
import { describe, expect, it } from "vitest";

const archivo = (nombre: string, bytes: number) => {
  const f = new File(["x"], nombre, { type: "" });
  Object.defineProperty(f, "size", { value: bytes });
  return f;
};

const m: MensajesFuente = {
  titulo: "titulo",
  tipo: "tipo",
  anio: "anio",
  especialidadesMax: "max",
  archivo: { requerido: "requerido", extension: "extension", tamano: "tamano" },
  motivo: "motivo",
  entidad: "entidad",
  nota: "nota",
};

async function errores(esquema: { validate: (v: unknown, o: object) => Promise<unknown> }, valores: object) {
  try {
    await esquema.validate(valores, { abortEarly: false });
    return [];
  } catch (e) {
    return (e as { errors: string[] }).errors;
  }
}

describe("validarArchivo", () => {
  it("accepts a .md up to exactly 2 MiB, case-insensitive extension", () => {
    expect(validarArchivo(archivo("balotario.md", ARCHIVO_MAX_BYTES))).toBeNull();
    expect(validarArchivo(archivo("BALOTARIO.MD", 10))).toBeNull();
  });

  it("rejects one byte over 2 MiB (3 MiB too), other extensions and no file", () => {
    expect(validarArchivo(archivo("b.md", ARCHIVO_MAX_BYTES + 1))).toBe("tamano");
    expect(validarArchivo(archivo("b.md", 3 * 1024 * 1024))).toBe("tamano");
    expect(validarArchivo(archivo("b.pdf", 10))).toBe("extension");
    expect(validarArchivo(archivo("b.md.txt", 10))).toBe("extension");
    expect(validarArchivo(null)).toBe("requerido");
  });
});

const fuente = (p: Partial<FuenteResumen>): FuenteResumen => ({
  id: 1,
  titulo: "F",
  tipo: "balotario",
  estado: "activa",
  aplica_a_todas: true,
  especialidades: [],
  version_vigente: { numero: 1, oficial_verificada: false, pasajes_count: 10 },
  version_en_proceso: null,
  created_at: "",
  updated_at: "",
  ...p,
});

describe("enProceso", () => {
  it("is true for a processing source or an active one with a version in process", () => {
    expect(enProceso(fuente({ estado: "procesando", version_vigente: null }))).toBe(true);
    expect(enProceso(fuente({ version_en_proceso: { numero: 2, estado: "procesando", motivo_fallo: null } }))).toBe(
      true,
    );
    expect(
      enProceso(fuente({ version_en_proceso: { numero: 2, estado: "fallida", motivo_fallo: "sin_contenido" } })),
    ).toBe(false);
    expect(enProceso(fuente({}))).toBe(false);
  });
});

describe("accionesDisponibles", () => {
  const acciones = (p: Partial<FuenteResumen>) => [...accionesDisponibles(fuente(p))].sort();

  it("active without mark: new version, edit, retire, mark", () => {
    expect(acciones({})).toEqual(["editar", "marcarOficial", "nuevaVersion", "retirar"]);
  });

  it("active with a version processing: only edit (no new version, no retire)", () => {
    expect(acciones({ version_en_proceso: { numero: 2, estado: "procesando", motivo_fallo: null } })).toEqual([
      "editar",
      "marcarOficial",
    ]);
  });

  it("active whose last version failed: can retry it", () => {
    expect(acciones({ version_en_proceso: { numero: 2, estado: "fallida", motivo_fallo: "error_interno" } })).toContain(
      "reintentar",
    );
  });

  it("active and marked: unmark instead of mark", () => {
    const a = acciones({ version_vigente: { numero: 1, oficial_verificada: true, pasajes_count: 1 } });
    expect(a).toContain("desmarcarOficial");
    expect(a).not.toContain("marcarOficial");
  });

  it("processing: nothing", () => {
    expect(acciones({ estado: "procesando", version_vigente: null })).toEqual([]);
  });

  it("failed: new version, edit, retry", () => {
    expect(acciones({ estado: "fallida", version_vigente: null })).toEqual(["editar", "nuevaVersion", "reintentar"]);
  });

  it("retired: edit, restore, and unmark if marked", () => {
    expect(acciones({ estado: "retirada" })).toEqual(["editar", "restaurar"]);
    expect(
      acciones({ estado: "retirada", version_vigente: { numero: 1, oficial_verificada: true, pasajes_count: 1 } }),
    ).toEqual(["desmarcarOficial", "editar", "restaurar"]);
  });
});

describe("esquemas", () => {
  it("upload: requires a valid file, title 3–150, type, and a year in range if given", async () => {
    const valido = {
      archivo: archivo("b.md", 100),
      titulo: "Balotario",
      tipo: "balotario",
      especialidad_ids: [],
      anio: "",
      proceso_id: "",
    };
    expect(await errores(esquemaSubida(m), valido)).toEqual([]);
    expect(await errores(esquemaSubida(m), { ...valido, archivo: archivo("b.md", ARCHIVO_MAX_BYTES + 1) })).toContain(
      "tamano",
    );
    expect(await errores(esquemaSubida(m), { ...valido, titulo: "  ab  " })).toContain("titulo");
    expect(await errores(esquemaSubida(m), { ...valido, anio: "1899" })).toContain("anio");
    expect(await errores(esquemaSubida(m), { ...valido, anio: "2025" })).toEqual([]);
  });

  it("counts runes, not UTF-16 units: a 150-character title with an emoji is valid (review #99 H2)", async () => {
    const titulo = "😀" + "a".repeat(149);
    expect(titulo.length).toBe(151);
    expect(await errores(esquemaEdicion(m), { titulo, especialidad_ids: [] })).toEqual([]);
    expect(await errores(esquemaEdicion(m), { titulo: titulo + "a", especialidad_ids: [] })).toEqual(["titulo"]);
  });

  it("reason: 5–500 after trimming", async () => {
    expect(await errores(esquemaMotivo(m), { motivo: "   abcd   " })).toEqual(["motivo"]);
    expect(await errores(esquemaMotivo(m), { motivo: "abcde" })).toEqual([]);
  });

  it("official mark: entity 3–100 and note 10–500, both required", async () => {
    expect(await errores(esquemaMarca(m), { entidad_emisora: "", nota: "" })).toEqual(["entidad", "nota"]);
    expect(await errores(esquemaMarca(m), { entidad_emisora: "MINSA", nota: "Descargado de la web oficial." })).toEqual(
      [],
    );
  });
});
