import { CargaPorClave, vistaDeCarga } from "./carga-por-clave";
import { describe, expect, it, vi } from "vitest";

function diferido<T>() {
  let resolve!: (v: T) => void;
  let reject!: (e: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

describe("CargaPorClave", () => {
  it("loads the data of a key", async () => {
    const carga = new CargaPorClave<string>();
    await carga.cargar("a", async () => "datos de a");
    expect(vistaDeCarga(carga.obtenerEstado(), "a")).toEqual({ datos: "datos de a", error: null, cargando: false });
  });

  it("applies only the last load: a slow answer of a previous key does not overwrite the current one", async () => {
    const carga = new CargaPorClave<string>();
    const lenta = diferido<string>();
    const primera = carga.cargar("a", () => lenta.promise);
    await carga.cargar("b", async () => "datos de b");
    lenta.resolve("datos de a");
    await primera;
    expect(carga.obtenerEstado()).toMatchObject({ clave: "b", datos: "datos de b", cargando: false });
  });

  it("keeps the error of the key and no data", async () => {
    const carga = new CargaPorClave<string>();
    const error = new Error("500");
    await carga.cargar("a", async () => Promise.reject(error));
    expect(vistaDeCarga(carga.obtenerEstado(), "a")).toEqual({ datos: null, error, cargando: false });
  });

  it("ignores the error of a superseded load", async () => {
    const carga = new CargaPorClave<string>();
    const lenta = diferido<string>();
    const primera = carga.cargar("a", () => lenta.promise);
    await carga.cargar("b", async () => "datos de b");
    lenta.reject(new Error("tarde"));
    await primera;
    expect(vistaDeCarga(carga.obtenerEstado(), "b")).toEqual({ datos: "datos de b", error: null, cargando: false });
  });

  it("does not request anything without a key", async () => {
    const carga = new CargaPorClave<string>();
    const pedir = vi.fn(async () => "x");
    await carga.cargar(null, pedir);
    expect(pedir).not.toHaveBeenCalled();
    expect(vistaDeCarga(carga.obtenerEstado(), null)).toEqual({ datos: null, error: null, cargando: false });
  });

  it("replaces the data of a key and notifies the subscribers", async () => {
    const carga = new CargaPorClave<string>();
    const oyente = vi.fn();
    const desuscribir = carga.suscribir(oyente);
    await carga.cargar("a", async () => "viejo");
    carga.reemplazar("a", "guardado");
    expect(vistaDeCarga(carga.obtenerEstado(), "a").datos).toBe("guardado");
    expect(oyente).toHaveBeenCalled();
    desuscribir();
    oyente.mockClear();
    carga.reemplazar("a", "otro");
    expect(oyente).not.toHaveBeenCalled();
  });
});

describe("vistaDeCarga", () => {
  it("never shows data or errors of another key and counts as loading until the current key arrives", async () => {
    const carga = new CargaPorClave<string>();
    await carga.cargar("a", async () => "datos de a");
    expect(vistaDeCarga(carga.obtenerEstado(), "b")).toEqual({ datos: null, error: null, cargando: true });
  });

  it("keeps showing the current data while reloading the same key", async () => {
    const carga = new CargaPorClave<string>();
    await carga.cargar("a", async () => "datos de a");
    const recarga = diferido<string>();
    const pendiente = carga.cargar("a", () => recarga.promise);
    expect(vistaDeCarga(carga.obtenerEstado(), "a")).toEqual({ datos: "datos de a", error: null, cargando: true });
    recarga.resolve("nuevos");
    await pendiente;
  });
});
