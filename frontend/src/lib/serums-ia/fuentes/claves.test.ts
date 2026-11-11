import en from "../../../i18n/messages/en.json";
import es from "../../../i18n/messages/es.json";
import type { AccionFuente } from "./reglas";
import { TIPOS } from "./reglas";
import { describe, expect, it } from "vitest";

/** Cada valor de los enums del contrato tiene su texto en es y en (`dashboard.serumsIa.fuentes.*`). */
const ENUMS: Record<string, readonly string[]> = {
  estado: ["procesando", "activa", "fallida", "retirada"],
  tipo: TIPOS,
  motivos: ["sin_contenido", "limite_pasajes", "error_interno"],
  versionEstado: ["procesando", "lista", "fallida", "archivada"],
  versionEnProceso: ["procesando", "fallida"],
  acciones: [
    "nuevaVersion",
    "editar",
    "retirar",
    "restaurar",
    "reintentar",
    "marcarOficial",
    "desmarcarOficial",
  ] satisfies AccionFuente[],
};

describe("fuentes i18n enum keys", () => {
  for (const [locale, mensajes] of [
    ["es", es],
    ["en", en],
  ] as const) {
    it(`exist in ${locale}.json`, () => {
      const fuentes = mensajes.dashboard.serumsIa.fuentes as unknown as Record<string, Record<string, unknown>>;
      for (const [grupo, valores] of Object.entries(ENUMS)) {
        for (const valor of valores) expect(typeof fuentes[grupo]?.[valor], `${grupo}.${valor}`).toBe("string");
      }
    });
  }
});
