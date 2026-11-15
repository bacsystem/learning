import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";
import { describe, expect, it } from "vitest";

const RAIZ = join(__dirname);

function archivos(dir: string): string[] {
  return readdirSync(dir).flatMap((nombre) => {
    const ruta = join(dir, nombre);
    if (statSync(ruta).isDirectory()) return archivos(ruta);
    return /\.tsx?$/.test(nombre) && !/\.test\.tsx?$/.test(nombre) ? [ruta] : [];
  });
}

/** Un `message` solo puede usarse para decidir un caso, nunca para pintarlo. */
const esComparacion = (linea: string) => /\.message\?\.includes\(/.test(linea);

/** El informe de filas inválidas de una importación no es un `ApiError`: son filas del resultado del servidor. */
const EXCEPCIONES_DE_PINTADO = [{ archivo: "(dashboard)/admin/preguntas/import/page.tsx", linea: "{err.message}" }];

describe("las pantallas no muestran el message crudo de la API (#250)", () => {
  it("no usa `.message` salvo para decidir un caso o en las excepciones justificadas", () => {
    const infracciones = archivos(RAIZ).flatMap((ruta) => {
      const rel = relative(RAIZ, ruta);
      return readFileSync(ruta, "utf8")
        .split("\n")
        .flatMap((linea, i) => {
          if (!/\.message\b/.test(linea) || esComparacion(linea)) return [];
          if (EXCEPCIONES_DE_PINTADO.some((e) => rel.endsWith(e.archivo) && linea.includes(e.linea))) return [];
          return [`${rel}:${i + 1}: ${linea.trim()}`];
        });
    });
    expect(infracciones, "usa claveErrorApi (src/lib/errores-api.ts) en vez de .message").toEqual([]);
  });
});
