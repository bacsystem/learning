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

describe("CargaPorClave.refrescar", () => {
  it("replaces the data without marking loading", async () => {
    const carga = new CargaPorClave<string>();
    await carga.cargar("a", async () => "v1");
    const pendiente = carga.refrescar("a", async () => "v2");
    expect(carga.obtenerEstado().cargando).toBe(false);
    await pendiente;
    expect(vistaDeCarga(carga.obtenerEstado(), "a")).toEqual({ datos: "v2", error: null, cargando: false });
  });

  it("keeps the previous data and shows no error when it fails", async () => {
    const carga = new CargaPorClave<string>();
    await carga.cargar("a", async () => "v1");
    await carga.refrescar("a", async () => Promise.reject(new Error("500")));
    expect(vistaDeCarga(carga.obtenerEstado(), "a")).toEqual({ datos: "v1", error: null, cargando: false });
  });

  it("does not overwrite a newer load of another key", async () => {
    const carga = new CargaPorClave<string>();
    await carga.cargar("a", async () => "v1");
    let terminar!: (v: string) => void;
    const refresco = carga.refrescar("a", () => new Promise((r) => (terminar = r)));
    await carga.cargar("b", async () => "datos de b");
    terminar("tarde");
    await refresco;
    expect(carga.obtenerEstado()).toMatchObject({ clave: "b", datos: "datos de b" });
  });
});

describe("CargaPorClave.refrescar durante un cargar en curso (review #99 H1)", () => {
  it("a failed refresh does not leave the screen loading forever", async () => {
    const c = new CargaPorClave<string>();
    let resolver!: (v: string) => void;
    const carga = c.cargar("p1", () => new Promise<string>((r) => (resolver = r)));
    await c.refrescar("p1", () => Promise.reject(new Error("red")));
    resolver("datos");
    await carga;
    expect(c.obtenerEstado()).toMatchObject({ datos: "datos", cargando: false });
  });

  it("a refresh of the previous key never overwrites the new key's load", async () => {
    const c = new CargaPorClave<string>();
    await c.cargar("p1", async () => "datos p1");
    let terminarRefresco!: (v: string) => void;
    const refresco = c.refrescar("p1", () => new Promise<string>((r) => (terminarRefresco = r)));
    const carga = c.cargar("p2", async () => "datos p2");
    terminarRefresco("p1 tarde");
    await Promise.all([refresco, carga]);
    expect(vistaDeCarga(c.obtenerEstado(), "p2")).toEqual({ datos: "datos p2", error: null, cargando: false });
  });
});

describe("CargaPorClave.reemplazar con un refresco en vuelo (seguimiento del review #99)", () => {
  it("a refresh that started before a save does not bring back the old data", async () => {
    const c = new CargaPorClave<string>();
    await c.cargar("f1", async () => "antes");
    let terminarRefresco!: (v: string) => void;
    const refresco = c.refrescar("f1", () => new Promise<string>((r) => (terminarRefresco = r)));
    c.reemplazar("f1", "guardado");
    terminarRefresco("antes (respuesta vieja)");
    await refresco;
    expect(vistaDeCarga(c.obtenerEstado(), "f1").datos).toBe("guardado");
  });
});
