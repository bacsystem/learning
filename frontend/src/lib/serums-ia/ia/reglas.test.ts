import {
  aCambio,
  aValores,
  CAMPOS_CONFIGURACION,
  camposRechazados,
  esquemaConfiguracion,
  estadoGasto,
  parseNumero,
  porcentajeBarra,
  porcentajeConsumo,
  RANGOS,
  type ValoresConfiguracion,
} from "./reglas";
import { describe, expect, it } from "vitest";

const BASE: ValoresConfiguracion = {
  limite_usuario_dia: "5",
  limite_usuario_mes: "60",
  limite_admin_dia: "200",
  max_preguntas_solicitud: "20",
  lote_clasificacion: "20",
  presupuesto_mensual_usd: "40",
};

const esquema = esquemaConfiguracion({
  rango: (campo) => `rango:${campo}`,
  decimales: "decimales",
  diaMayorQueMes: "dia>mes",
});

const errores = async (v: Partial<ValoresConfiguracion>) =>
  esquema.validate({ ...BASE, ...v }, { abortEarly: false }).then(
    () => [] as string[],
    (e: { errors: string[] }) => e.errors,
  );

describe("esquemaConfiguracion", () => {
  it("accepts the values of the story", async () => {
    expect(await errores({})).toEqual([]);
  });

  it.each(
    CAMPOS_CONFIGURACION.flatMap((campo) => [
      [campo, String(RANGOS[campo].min), true],
      [campo, String(RANGOS[campo].max), true],
      [campo, String(RANGOS[campo].min - 1), false],
      [campo, String(RANGOS[campo].max + 1), false],
    ]),
  )("%s = %s is valid: %s", async (campo, valor, valido) => {
    // El día no puede superar al mes: se acomoda el otro para aislar el rango.
    const base =
      campo === "limite_usuario_dia"
        ? { limite_usuario_mes: "1000" }
        : campo === "limite_usuario_mes"
          ? { limite_usuario_dia: "0" }
          : {};
    const resultado = await errores({ ...base, [campo]: valor });
    if (valido) expect(resultado).toEqual([]);
    else expect(resultado).toContain(`rango:${campo}`);
  });

  it("requires the day limit not to exceed the month limit", async () => {
    expect(await errores({ limite_usuario_dia: "10", limite_usuario_mes: "9" })).toContain("dia>mes");
    expect(await errores({ limite_usuario_dia: "9", limite_usuario_mes: "9" })).toEqual([]);
  });

  it("accepts at most 2 decimals in the budget and integers elsewhere", async () => {
    expect(await errores({ presupuesto_mensual_usd: "40.25" })).toEqual([]);
    expect(await errores({ presupuesto_mensual_usd: "40.1" })).toEqual([]);
    expect(await errores({ presupuesto_mensual_usd: "40.255" })).toContain("decimales");
    expect(await errores({ limite_admin_dia: "10.5" })).toContain("rango:limite_admin_dia");
  });

  it("rejects an empty or non-numeric field", async () => {
    expect(await errores({ limite_admin_dia: "" })).toContain("rango:limite_admin_dia");
    expect(await errores({ presupuesto_mensual_usd: "abc" })).toContain("rango:presupuesto_mensual_usd");
  });
});

describe("conversion between the API and the form", () => {
  const config = {
    limite_usuario_dia: 5,
    limite_usuario_mes: 60,
    limite_admin_dia: 200,
    max_preguntas_solicitud: 20,
    lote_clasificacion: 20,
    presupuesto_mensual_usd: 40.5,
    updated_at: "2026-11-12T15:04:05Z",
  };

  it("keeps the six fields as text and drops updated_at", () => {
    expect(aValores(config)).toEqual({ ...BASE, presupuesto_mensual_usd: "40.5" });
  });

  it("sends numbers and never updated_at", () => {
    const cambio = aCambio(aValores(config));
    expect(cambio).toEqual({ ...config, updated_at: undefined });
    expect("updated_at" in cambio).toBe(false);
  });
});

describe("estadoGasto y porcentajeBarra", () => {
  it("follows the server flags", () => {
    expect(estadoGasto({ agotado: true, alerta_80_at: "x" })).toBe("agotado");
    expect(estadoGasto({ agotado: false, alerta_80_at: "2026-11-20T18:00:00Z" })).toBe("alerta");
    expect(estadoGasto({ agotado: false, alerta_80_at: null })).toBe("normal");
  });

  it("caps the bar at 100 although the spending can exceed the budget", () => {
    expect(porcentajeBarra({ porcentaje: 80.25 })).toBe(80.25);
    expect(porcentajeBarra({ porcentaje: 104.5 })).toBe(100);
    expect(porcentajeBarra({ porcentaje: -1 })).toBe(0);
  });
});

describe("decimal comma", () => {
  it.each([
    ["40,5", 40.5],
    ["40.5", 40.5],
    [" 12 ", 12],
  ])("parseNumero(%j) is %d", (texto, esperado) => {
    expect(parseNumero(texto)).toBe(esperado);
  });

  it.each(["", "  ", "abc"])("parseNumero(%j) is NaN", (texto) => {
    expect(parseNumero(texto)).toBeNaN();
  });

  it("the budget accepts a comma and still limits the decimals", async () => {
    expect(await errores({ presupuesto_mensual_usd: "40,5" })).toEqual([]);
    expect(await errores({ presupuesto_mensual_usd: "40,255" })).toContain("decimales");
  });

  it("an empty field is rejected, not read as 0", async () => {
    expect(await errores({ limite_usuario_dia: "" })).toContain("rango:limite_usuario_dia");
  });

  it("sends the comma value as a number", () => {
    expect(aCambio({ ...BASE, presupuesto_mensual_usd: "40,5" }).presupuesto_mensual_usd).toBe(40.5);
  });
});

describe("camposRechazados", () => {
  const error = (detalle?: unknown) => Object.assign(new Error("x"), { status: 422, code: "validacion", detalle });

  it("keeps only the form fields named by the server", () => {
    expect(
      camposRechazados(
        error({ campos: { limite_admin_dia: "max", desconocido: "x", presupuesto_mensual_usd: "dec2" } }),
      ),
    ).toEqual(["limite_admin_dia", "presupuesto_mensual_usd"]);
  });

  it("is empty without detail or with another shape", () => {
    expect(camposRechazados(error())).toEqual([]);
    expect(camposRechazados(error({ campos: "x" }))).toEqual([]);
    expect(camposRechazados(null)).toEqual([]);
  });
});

describe("porcentajeConsumo", () => {
  it.each([
    [0, 5, 0],
    [3, 5, 60],
    [5, 5, 100],
    [7, 5, 100],
    [0, 0, 100],
  ])("%i of %i is %i %%", (usados, limite, esperado) => {
    expect(porcentajeConsumo({ usados, limite })).toBe(esperado);
  });
});
