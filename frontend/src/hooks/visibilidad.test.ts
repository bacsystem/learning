import { type DocumentoVisible, escucharVisibilidad, pestanaVisible } from "./visibilidad";
import { describe, expect, it, vi } from "vitest";

function documentoFalso(inicial: DocumentVisibilityState) {
  const eventos = new EventTarget();
  const documento: DocumentoVisible & { cambiar: (v: DocumentVisibilityState) => void } = {
    visibilityState: inicial,
    addEventListener: (tipo, oyente) => eventos.addEventListener(tipo, oyente),
    removeEventListener: (tipo, oyente) => eventos.removeEventListener(tipo, oyente),
    cambiar(v) {
      this.visibilityState = v;
      eventos.dispatchEvent(new Event("visibilitychange"));
    },
  };
  return documento;
}

describe("visibilidad de la pestaña", () => {
  it("reads the current state", () => {
    expect(pestanaVisible(documentoFalso("visible"))).toBe(true);
    expect(pestanaVisible(documentoFalso("hidden"))).toBe(false);
  });

  it("counts as visible on the server, where there is no document", () => {
    expect(pestanaVisible(undefined)).toBe(true);
    expect(() => escucharVisibilidad(() => {}, undefined)()).not.toThrow();
  });

  it("notifies each change until unsubscribed", () => {
    const documento = documentoFalso("visible");
    const alCambiar = vi.fn(() => pestanaVisible(documento));
    const cancelar = escucharVisibilidad(alCambiar, documento);
    documento.cambiar("hidden");
    documento.cambiar("visible");
    expect(alCambiar.mock.results.map((r) => r.value)).toEqual([false, true]);
    cancelar();
    documento.cambiar("hidden");
    expect(alCambiar).toHaveBeenCalledTimes(2);
  });
});
