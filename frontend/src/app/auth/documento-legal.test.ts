import en from "../../i18n/messages/en.json";
import es from "../../i18n/messages/es.json";
import { DocumentoLegal } from "./documento-legal";
import { type AbstractIntlMessages, NextIntlClientProvider } from "next-intl";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

const pagina = (locale: "es" | "en", messages: AbstractIntlMessages) =>
  renderToStaticMarkup(
    createElement(NextIntlClientProvider, {
      locale,
      messages,
      children: createElement(DocumentoLegal, {
        titulo: "Términos",
        intro: "INTRO-EN-ESPAÑOL",
        children: createElement("p", null, "CUERPO"),
      }),
    }),
  );

describe("DocumentoLegal", () => {
  it("in English the Spanish-only notice goes between the title and the intro, before the body", () => {
    const html = pagina("en", en);
    const aviso = html.indexOf("This document is available in Spanish only.");
    expect(aviso).toBeGreaterThan(html.indexOf("Términos"));
    expect(aviso).toBeLessThan(html.indexOf("INTRO-EN-ESPAÑOL"));
    expect(html.indexOf("INTRO-EN-ESPAÑOL")).toBeLessThan(html.indexOf("CUERPO"));
  });

  it("in Spanish there is no notice and the intro stays in the header", () => {
    const html = pagina("es", es);
    expect(html).not.toContain("disponible solo en español");
    expect(html).not.toContain("Spanish only");
    expect(html.indexOf("INTRO-EN-ESPAÑOL")).toBeLessThan(html.indexOf("</header>"));
    expect(html.indexOf("INTRO-EN-ESPAÑOL")).toBeLessThan(html.indexOf("CUERPO"));
    expect(html.match(/INTRO-EN-ESPAÑOL/g)).toHaveLength(1);
  });
});
