import en from "../../i18n/messages/en.json";
import es from "../../i18n/messages/es.json";
import { textosPieTabla, textosTablaDatos, textosTablaListado, type Traductor } from "./textos-khipu";
import { describe, expect, it } from "vitest";

/** `t` simulado: devuelve la clave y sus valores, y registra las claves pedidas. */
function traductor() {
  const claves: string[] = [];
  const t: Traductor = (clave, valores) => {
    claves.push(clave);
    return valores ? `${clave}:${JSON.stringify(valores)}` : clave;
  };
  return { t, claves };
}

describe("textos-khipu", () => {
  it("maps the table footer texts to their keys", () => {
    const { t } = traductor();
    expect(textosPieTabla(t)).toEqual({
      mostrando: "tabla-mostrando",
      de: "tabla-de",
      anterior: "tabla-anterior",
      siguiente: "tabla-siguiente",
      filas: "tabla-filas",
      filasPorPagina: "tabla-filas-por-pagina",
    });
  });

  it("maps the selection texts, with the row id and singular or plural", () => {
    const { t } = traductor();
    const textos = textosTablaDatos(t);
    expect(textos).toMatchObject({
      accionesSeleccion: "tabla-acciones-seleccion",
      quitarSeleccion: "tabla-quitar-seleccion",
      seleccionarTodo: "tabla-seleccionar-todo",
      mostrando: "tabla-mostrando",
    });
    expect(textos.seleccionarFila("41")).toBe('tabla-seleccionar-fila:{"id":"41"}');
    expect(textos.seleccionados(1)).toBe("tabla-seleccionado");
    expect(textos.seleccionados(0)).toBe("tabla-seleccionados");
    expect(textos.seleccionados(3)).toBe("tabla-seleccionados");
  });

  it("maps the list bar texts on top of the table ones", () => {
    const { t } = traductor();
    expect(textosTablaListado(t)).toMatchObject({
      buscar: "tabla-buscar",
      columnas: "tabla-columnas",
      columnasVisibles: "tabla-columnas-visibles",
      exportar: "tabla-exportar",
      exportarCsv: "tabla-exportar-csv",
      imprimir: "tabla-imprimir",
      siguiente: "tabla-siguiente",
      quitarSeleccion: "tabla-quitar-seleccion",
    });
  });

  it("uses only keys that exist in es.json and en.json, with the {id} placeholder", () => {
    const { t, claves } = traductor();
    const textos = textosTablaListado(t);
    textos.seleccionarFila("1");
    textos.seleccionados(1);
    textos.seleccionados(2);
    const dashboardEs = es.dashboard as Record<string, unknown>;
    const dashboardEn = en.dashboard as Record<string, unknown>;
    for (const clave of new Set(claves)) {
      expect(typeof dashboardEs[clave], `es: ${clave}`).toBe("string");
      expect(typeof dashboardEn[clave], `en: ${clave}`).toBe("string");
    }
    expect(dashboardEs["tabla-seleccionar-fila"]).toContain("{id}");
    expect(dashboardEn["tabla-seleccionar-fila"]).toContain("{id}");
  });
});
