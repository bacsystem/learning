import { nodosConClave, segmentarMarcas } from "./marcas";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

const claves = (texto: string) =>
  segmentarMarcas(texto)
    .filter((s) => s.clave)
    .map((s) => s.texto);
const unido = (texto: string) =>
  segmentarMarcas(texto)
    .map((s) => (s.clave ? `<mark>${s.texto}</mark>` : s.texto))
    .join("");
const html = (texto: string) => renderToStaticMarkup(createElement("p", null, nodosConClave(texto, "clave oficial")));

describe("segmentarMarcas", () => {
  it("highlights a well paired mark and keeps the text around it", () => {
    expect(segmentarMarcas("A) uno <mark>B) dos</mark> C) tres")).toEqual([
      { texto: "A) uno ", clave: false },
      { texto: "B) dos", clave: true },
      { texto: " C) tres", clave: false },
    ]);
  });

  it("highlights each pair of several marks", () => {
    expect(claves("<mark>uno</mark> y <mark>dos</mark>")).toEqual(["uno", "dos"]);
  });

  it("accepts the tag in any case", () => {
    expect(claves("<MARK>x</Mark>")).toEqual(["x"]);
  });

  it("leaves a mark that is never closed as plain text", () => {
    expect(segmentarMarcas("A) uno <mark>B) dos")).toEqual([{ texto: "A) uno <mark>B) dos", clave: false }]);
    expect(claves("A) uno <mark>B) dos")).toEqual([]);
  });

  it("leaves a closing mark without opening as plain text", () => {
    expect(segmentarMarcas("B) dos</mark> C)")).toEqual([{ texto: "B) dos</mark> C)", clave: false }]);
  });

  it("pairs the last opening with the closing when marks are nested, and leaves the rest as text", () => {
    expect(segmentarMarcas("<mark>a <mark>b</mark> c</mark>")).toEqual([
      { texto: "<mark>a ", clave: false },
      { texto: "b", clave: true },
      { texto: " c</mark>", clave: false },
    ]);
  });

  it("never loses text: putting the paired marks back gives the original text", () => {
    for (const texto of ["sin marcas", "a <mark>b</mark> c", "<mark>x</mark>", "", "a <mark>b", "b</mark> c"]) {
      expect(unido(texto)).toBe(texto);
    }
  });

  it("does not treat other tags, attributes or entities as marks", () => {
    expect(claves('<mark class="x">a</mark>')).toEqual([]);
    expect(claves("<markup>a</markup>")).toEqual([]);
    expect(claves("&lt;mark&gt;a&lt;/mark&gt;")).toEqual([]);
    expect(segmentarMarcas("<b>negrita</b> <i>cursiva</i>")).toEqual([
      { texto: "<b>negrita</b> <i>cursiva</i>", clave: false },
    ]);
  });
});

describe("nodosConClave (markup)", () => {
  it("renders the key as a semantic mark with a screen reader text, not only color", () => {
    const salida = html("A) uno <mark>B) dos</mark>");
    expect(salida).toContain("<mark");
    expect(salida).toContain('<span class="sr-only">clave oficial: </span>B) dos');
    expect(salida).toMatch(/font-semibold/);
    expect(salida).toMatch(/underline/);
  });

  it("shows an unclosed mark as literal text, escaped", () => {
    const salida = html("A) uno <mark>B) dos");
    expect(salida).not.toContain("<mark");
    expect(salida).toContain("&lt;mark&gt;B) dos");
  });

  it("shows other HTML as plain text, escaped, and never as elements", () => {
    const salida = html("<b>negrita</b> <a href='x'>enlace</a>");
    expect(salida).not.toContain("<b>");
    expect(salida).not.toContain("<a ");
    expect(salida).toContain("&lt;b&gt;negrita&lt;/b&gt;");
  });

  it.each([
    "<script>alert(1)</script>",
    '<img src=x onerror="alert(1)">',
    '<svg onload="alert(1)"></svg>',
    '<iframe src="javascript:alert(1)"></iframe>',
  ])("an injection attempt %s is shown as text", (intento) => {
    for (const texto of [`antes ${intento} despues`, `<mark>${intento}</mark>`, `<mark>x</mark>${intento}`]) {
      const salida = html(texto);
      for (const etiqueta of ["<script", "<img", "<svg", "<iframe"]) expect(salida).not.toContain(etiqueta);
      expect(salida).toContain("&lt;");
    }
  });

  it("an injection inside a pair is highlighted as text, never run", () => {
    const salida = html("<mark><script>alert(1)</script></mark>");
    expect(salida).toContain("<mark");
    expect(salida).toContain("&lt;script&gt;alert(1)&lt;/script&gt;");
    expect(salida).not.toContain("<script");
  });
});
