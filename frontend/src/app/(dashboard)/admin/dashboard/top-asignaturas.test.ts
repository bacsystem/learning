import en from "../../../../i18n/messages/en.json";
import es from "../../../../i18n/messages/es.json";
import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

describe("Top asignaturas", () => {
  it("labels the count column as questions, in both languages (D8 of #247)", () => {
    expect(es.dashboard["admin-dashboard-top-preguntas"]).toBe("Preguntas");
    expect(en.dashboard["admin-dashboard-top-preguntas"]).toBe("Questions");
  });

  it("uses that label for the column, not the one of practice sessions", () => {
    const fuente = readFileSync(new URL("./page.tsx", import.meta.url), "utf8");
    const tabla = fuente.slice(fuente.indexOf("admin-dashboard-top-asignaturas"));
    expect(tabla).toContain('t("admin-dashboard-top-preguntas")');
    expect(tabla.slice(0, tabla.indexOf("</TableHeader>"))).not.toContain('t("menu-practicas")');
  });
});
