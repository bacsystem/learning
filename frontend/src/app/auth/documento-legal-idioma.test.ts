import en from "../../i18n/messages/en.json";
import es from "../../i18n/messages/es.json";
import { avisarSoloEspanol } from "./documento-legal-idioma";
import { describe, expect, it } from "vitest";

describe("avisarSoloEspanol", () => {
  it("warns English readers and not Spanish ones", () => {
    expect(avisarSoloEspanol("en")).toBe(true);
    expect(avisarSoloEspanol("es")).toBe(false);
  });

  it("has the notice text in both languages", () => {
    expect(en.dashboard["auth-documento-solo-espanol"]).toBe("This document is available in Spanish only.");
    expect(es.dashboard["auth-documento-solo-espanol"]).toBe("Este documento está disponible solo en español.");
  });
});
