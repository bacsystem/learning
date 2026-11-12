import en from "../../i18n/messages/en.json";
import es from "../../i18n/messages/es.json";
import { CLAVES_BARRA_SUPERIOR } from "./barra-superior-textos";
import { describe, expect, it } from "vitest";

const dashboard = { es: es.dashboard, en: en.dashboard } as unknown as Record<"es" | "en", Record<string, string>>;

describe("textos accesibles de la barra superior", () => {
  it.each(Object.entries(CLAVES_BARRA_SUPERIOR))(
    "%s has a text in es and in en, and the English is not the Spanish",
    (_, clave) => {
      expect(typeof dashboard.es[clave]).toBe("string");
      expect(typeof dashboard.en[clave]).toBe("string");
      expect(dashboard.en[clave]).not.toBe(dashboard.es[clave]);
    },
  );

  it("keeps the Spanish text khipu used before, so Spanish does not change", () => {
    expect(dashboard.es[CLAVES_BARRA_SUPERIOR.abrirMenu]).toBe("Abrir menú");
    expect(dashboard.es[CLAVES_BARRA_SUPERIOR.ubicacion]).toBe("Ubicación");
  });
});
