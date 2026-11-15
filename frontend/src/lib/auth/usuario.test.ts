import { usuarioConsultable } from "./usuario";
import { describe, expect, it } from "vitest";

describe("usuarioConsultable", () => {
  it("acepta un usuario alfanumérico de 4 o más caracteres", () => {
    expect(usuarioConsultable("qauno")).toBe(true);
  });

  it("rechaza el guion bajo aunque tenga longitud suficiente", () => {
    expect(usuarioConsultable("qa_uno")).toBe(false);
  });

  it("rechaza un usuario demasiado corto", () => {
    expect(usuarioConsultable("qa1")).toBe(false);
  });
});
