import en from "../../../i18n/messages/en.json";
import es from "../../../i18n/messages/es.json";
import { COEFICIENTE_MAX, COEFICIENTE_MIN, PESO_MAX, PESO_MIN } from "./reglas";
import { createTranslator } from "next-intl";
import { describe, expect, it } from "vitest";

/** Mensajes reales de es.json y en.json, formateados como en la pantalla (QA E1-03 H2). */
const traductor = (locale: "es" | "en") =>
  createTranslator({ locale, messages: locale === "es" ? es : en, namespace: "dashboard.serumsIa.pesos.validacion" });

describe("pesos validation messages", () => {
  it("format the coefficient limits with the locale decimal separator", () => {
    const limites = { min: COEFICIENTE_MIN, max: COEFICIENTE_MAX };
    expect(traductor("es")("coeficiente", limites)).toContain("0,01 a 999,99");
    expect(traductor("en")("coeficiente", limites)).toContain("0.01 to 999.99");
  });

  it("format the weight limits with the locale decimal separator", () => {
    expect(traductor("es")("peso", { min: PESO_MIN, max: PESO_MAX })).toContain("0,01 a 100");
    expect(traductor("en")("peso", { min: PESO_MIN, max: PESO_MAX })).toContain("0.01 to 100");
  });

  it("declare every numeric limit as {x, number} in both locales", () => {
    for (const mensajes of [es, en]) {
      const validacion = mensajes.dashboard.serumsIa.pesos.validacion as Record<string, string>;
      for (const [clave, texto] of Object.entries(validacion)) {
        expect(texto, clave).not.toMatch(/\{(min|max)\}/);
      }
    }
  });
});
