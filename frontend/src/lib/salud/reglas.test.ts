import type { RespuestaSaludApi } from "./api";
import {
  desglosarDuracion,
  normalizarSalud,
  RespuestaSaludInvalida,
  ultimoEnvio,
  versionBackend,
  versionesDistintas,
} from "./reglas";
import { describe, expect, it } from "vitest";

const respuesta = (cambio: Partial<RespuestaSaludApi> = {}): RespuestaSaludApi => ({
  estado: "degradado",
  comprobado_en: "2026-11-11T15:04:05Z",
  servicios: [
    { id: "api", estado: "operativo", categoria: null, datos: { version: "1.4.0", en_marcha_s: 86400 } },
    { id: "base_datos", estado: "operativo", categoria: null, datos: { latencia_ms: 20 } },
    {
      id: "correo",
      estado: "degradado",
      categoria: "atrasado",
      datos: { sin_enviar: 2, antiguedad_max_min: 12, fallidos_24h: 0, ultimo_enviado_en: "2026-11-11T14:50:00Z" },
    },
  ],
  ...cambio,
});

describe("normalizarSalud", () => {
  it("keeps the contract response as is, with the global state from the API", () => {
    const salud = normalizarSalud(respuesta());
    expect(salud.estado).toBe("degradado");
    expect(salud.comprobadoEn).toBe("2026-11-11T15:04:05Z");
    expect(salud.servicios.map((s) => s.id)).toEqual(["api", "base_datos", "correo"]);
    expect(salud.servicios[2]).toEqual({
      id: "correo",
      estado: "degradado",
      categoria: "atrasado",
      datos: { sin_enviar: 2, antiguedad_max_min: 12, fallidos_24h: 0, ultimo_enviado_en: "2026-11-11T14:50:00Z" },
    });
  });

  it("ignores unknown service ids (PLT-01 regla 7), including the v2 ones", () => {
    const salud = normalizarSalud(
      respuesta({
        servicios: [
          { id: "copia_r2", estado: "caido", categoria: "atrasado", datos: {} },
          { id: "toString", estado: "operativo", categoria: null, datos: {} },
          "basura",
          null,
          { id: "base_datos", estado: "operativo", categoria: null, datos: { latencia_ms: 5 } },
        ],
      }),
    );
    expect(salud.servicios.map((s) => s.id)).toEqual(["base_datos"]);
  });

  it("shows a state outside the closed set as unknown and drops an unknown category", () => {
    const salud = normalizarSalud(
      respuesta({
        estado: "excelente",
        servicios: [{ id: "base_datos", estado: "<b>", categoria: "dial tcp 10.0.0.1:5432", datos: {} }],
      }),
    );
    expect(salud.estado).toBe("desconocido");
    expect(salud.servicios[0]).toMatchObject({ estado: "desconocido", categoria: null });
  });

  it("turns missing or wrongly typed data into null", () => {
    const salud = normalizarSalud(
      respuesta({
        servicios: [
          { id: "api", estado: "operativo", datos: { version: 140, en_marcha_s: "1 día" } },
          { id: "base_datos", estado: "caido", categoria: "sin_conexion", datos: null },
          { id: "correo", estado: "no_configurado", datos: { sin_enviar: null, ultimo_enviado_en: "ayer" } },
        ],
      }),
    );
    expect(salud.servicios.map((s) => s.datos)).toEqual([
      { version: null, en_marcha_s: null },
      { latencia_ms: null },
      { sin_enviar: null, antiguedad_max_min: null, fallidos_24h: null, ultimo_enviado_en: null },
    ]);
  });

  it.each([
    ["without a check time", { comprobado_en: undefined }],
    ["with an invalid check time", { comprobado_en: "hace un rato" }],
    ["without a service list", { servicios: { api: {} } }],
  ])("rejects a response %s", (_, cambio) => {
    expect(() => normalizarSalud(respuesta(cambio))).toThrow(RespuestaSaludInvalida);
  });

  it("rejects a body that is not an object", () => {
    expect(() => normalizarSalud(null as unknown as RespuestaSaludApi)).toThrow(RespuestaSaludInvalida);
  });
});

describe("versiones (PLT-03 regla 4)", () => {
  it("reads the backend version from the api service", () => {
    expect(versionBackend(normalizarSalud(respuesta()))).toBe("1.4.0");
    expect(versionBackend(normalizarSalud(respuesta({ servicios: [] })))).toBeNull();
  });

  it("flags different versions only when the backend one is known", () => {
    expect(versionesDistintas("1.4.0", "1.4.0")).toBe(false);
    expect(versionesDistintas("1.4.0", "1.5.0")).toBe(true);
    expect(versionesDistintas("1.4.0", null)).toBe(false);
  });
});

describe("desglosarDuracion", () => {
  it.each([
    [0, { dias: 0, horas: 0, minutos: 0 }],
    [59, { dias: 0, horas: 0, minutos: 0 }],
    [3_660, { dias: 0, horas: 1, minutos: 1 }],
    [93_784, { dias: 1, horas: 2, minutos: 3 }],
    [-5, { dias: 0, horas: 0, minutos: 0 }],
  ])("splits %i s", (segundos, esperado) => {
    expect(desglosarDuracion(segundos)).toEqual(esperado);
  });
});

describe("ultimoEnvio (hallazgo H2 del #129)", () => {
  const correo = { sin_enviar: 0, antiguedad_max_min: null, fallidos_24h: 0, ultimo_enviado_en: null };

  it("is the date when one was sent", () => {
    expect(ultimoEnvio({ ...correo, ultimo_enviado_en: "2026-11-11T14:50:00Z" })).toEqual({
      tipo: "fecha",
      iso: "2026-11-11T14:50:00Z",
    });
  });

  it("is never when the measurement came but nothing was sent", () => {
    expect(ultimoEnvio(correo)).toEqual({ tipo: "nunca" });
  });

  it("has no data when the measurement failed (all data null)", () => {
    expect(ultimoEnvio({ ...correo, sin_enviar: null, fallidos_24h: null })).toEqual({ tipo: "sin_dato" });
  });
});
