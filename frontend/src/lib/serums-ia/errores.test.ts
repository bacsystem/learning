import en from "../../i18n/messages/en.json";
import es from "../../i18n/messages/es.json";
import { mensajeErrorFuente } from "./fuentes/errores";
import { mensajeErrorPesos } from "./pesos/errores";
import { mensajeErrorProfesion } from "./profesion/errores";
import { mensajeErrorTema } from "./temas/errores";
import { createTranslator } from "next-intl";
import { describe, expect, it } from "vitest";

const MENSAJE_DEL_SERVIDOR = "Mensaje crudo del servidor en español.";
const apiError = (code?: string) => Object.assign(new Error(MENSAJE_DEL_SERVIDOR), { status: 422, code });

/** Todas las pantallas de SERUMS IA (E1 y E2) resuelven sus errores con el mismo fallback (docs/api/README.md §8). */
const PANTALLAS = [
  ["temas", mensajeErrorTema],
  ["pesos", mensajeErrorPesos],
  ["profesion", mensajeErrorProfesion],
  ["fuentes", mensajeErrorFuente],
] as const;

describe.each([
  ["es", es],
  ["en", en],
] as const)("fallback of an unknown error code (%s)", (locale, messages) => {
  const t = createTranslator({ locale, messages, namespace: "dashboard" });
  const generico = t("serumsIa.errores.generico");

  it.each(PANTALLAS)("%s shows the translated generic text, never the server message", (_, mensaje) => {
    const texto = mensaje(apiError("codigo_que_nadie_conoce"), t as (clave: string) => string, "texto de la pantalla");
    expect(texto).toBe(generico);
    expect(texto).not.toContain(MENSAJE_DEL_SERVIDOR);
  });

  it("the generic text exists and is not the key", () => {
    expect(generico).not.toBe("serumsIa.errores.generico");
    expect(generico.length).toBeGreaterThan(0);
  });
});
