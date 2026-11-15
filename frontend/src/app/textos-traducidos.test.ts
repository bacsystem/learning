import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";
import { describe, expect, it } from "vitest";

const SRC = join(__dirname, "..");
// Las pantallas y la capa propia de la app (khipu, en components/{ui,datos,…}, no se edita aquí).
const CARPETAS = [join(SRC, "app"), join(SRC, "components", "app")];

/** Componentes de khipu con un texto fijo en castellano que la app tiene que traducir, y la prop que lo recibe. */
const PROPS_DE_TEXTO = [
  { etiqueta: "ThemeToggle", prop: "textos" },
  { etiqueta: "Buscador", prop: "etiquetaLimpiar" },
  { etiqueta: "SidebarContent", prop: "textosNav" },
  { etiqueta: "SidebarNav", prop: "textos" },
  { etiqueta: "SidebarContent", prop: "textosNav" },
  { etiqueta: "SidebarNav", prop: "textos" },
];

function archivos(dir: string): string[] {
  return readdirSync(dir).flatMap((nombre) => {
    const ruta = join(dir, nombre);
    if (statSync(ruta).isDirectory()) return archivos(ruta);
    return /\.tsx$/.test(nombre) ? [ruta] : [];
  });
}

/** Cada uso `<Etiqueta …>` de un archivo con su texto de apertura (hasta el `>` que cierra la etiqueta, con JSX anidado en las props). */
export function usosDe(fuente: string, etiqueta: string): { linea: number; texto: string }[] {
  const usos: { linea: number; texto: string }[] = [];
  const inicio = new RegExp(`<${etiqueta}(?=[\\s/>])`, "g");
  for (const m of fuente.matchAll(inicio)) {
    let profundidad = 0;
    let i = m.index + m[0].length;
    for (; i < fuente.length; i++) {
      const c = fuente[i];
      if (c === "{") profundidad++;
      else if (c === "}") profundidad--;
      else if (c === ">" && profundidad === 0) break;
    }
    usos.push({ linea: fuente.slice(0, m.index).split("\n").length, texto: fuente.slice(m.index, i + 1) });
  }
  return usos;
}

const tieneProp = (texto: string, prop: string) => new RegExp(`\\s${prop}(=|\\s|/|>)`).test(texto);

describe("usosDe", () => {
  it("reads the whole opening tag, also with nested JSX in the props", () => {
    const fuente = '<Buscador a={<b>x</b>} etiquetaLimpiar={t("k")} />\n<Buscador id="x" />';
    const usos = usosDe(fuente, "Buscador");
    expect(usos).toHaveLength(2);
    expect(tieneProp(usos[0].texto, "etiquetaLimpiar")).toBe(true);
    expect(tieneProp(usos[1].texto, "etiquetaLimpiar")).toBe(false);
    expect(usos[1].linea).toBe(2);
  });
});

describe("los componentes de khipu con texto fijo reciben su texto traducido (#242)", () => {
  it("todo <ThemeToggle> y <SidebarNav> llevan `textos`, <SidebarContent> `textosNav` y <Buscador> `etiquetaLimpiar`", () => {
    const faltan = CARPETAS.flatMap(archivos).flatMap((ruta) => {
      const fuente = readFileSync(ruta, "utf8");
      return PROPS_DE_TEXTO.flatMap(({ etiqueta, prop }) =>
        usosDe(fuente, etiqueta)
          .filter((uso) => !tieneProp(uso.texto, prop))
          .map((uso) => `${relative(SRC, ruta)}:${uso.linea}: <${etiqueta}> sin \`${prop}\``),
      );
    });
    expect(faltan, "pasa el texto traducido (textos-khipu.ts o t(...))").toEqual([]);
  });
});
