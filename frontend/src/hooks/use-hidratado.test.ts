import { useHidratado } from "./use-hidratado";
import { createElement } from "react";
import { renderToString } from "react-dom/server";
import { describe, expect, it } from "vitest";

function Sonda() {
  return createElement("span", null, useHidratado() ? "cliente" : "servidor");
}

describe("useHidratado", () => {
  it("is false while rendering on the server, so the HTML matches the hydration render", () => {
    expect(renderToString(createElement(Sonda))).toBe("<span>servidor</span>");
  });
});
