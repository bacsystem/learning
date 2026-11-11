import { ConsultaSalud, ESTADO_INICIAL_SALUD, faseSalud } from "./consulta";
import type { Salud } from "./reglas";
import { describe, expect, it } from "vitest";

const salud = (comprobadoEn: string): Salud => ({ estado: "operativo", comprobadoEn, servicios: [] });

function diferida<T>() {
  let resolver!: (v: T) => void;
  let rechazar!: (e: unknown) => void;
  const promesa = new Promise<T>((res, rej) => {
    resolver = res;
    rechazar = rej;
  });
  return { promesa, resolver, rechazar };
}

describe("ConsultaSalud", () => {
  it("marks the query in progress and stores the result", async () => {
    const consulta = new ConsultaSalud();
    const pedido = diferida<Salud>();
    const hecho = consulta.consultar(() => pedido.promesa);
    expect(consulta.obtenerEstado().enCurso).toBe(true);
    pedido.resolver(salud("10:00"));
    await hecho;
    expect(consulta.obtenerEstado()).toEqual({ salud: salud("10:00"), fallo: null, enCurso: false });
  });

  it("keeps the last valid result when a later query fails (PLT-02 regla 5)", async () => {
    const consulta = new ConsultaSalud();
    await consulta.consultar(async () => salud("10:00"));
    const error = Object.assign(new Error("Servicio no disponible."), { status: 503 });
    await consulta.consultar(async () => Promise.reject(error));
    expect(consulta.obtenerEstado()).toEqual({ salud: salud("10:00"), fallo: error, enCurso: false });
  });

  it("clears the failure with the next valid result", async () => {
    const consulta = new ConsultaSalud();
    await consulta.consultar(async () => Promise.reject(new Error("red")));
    await consulta.consultar(async () => salud("10:30"));
    expect(consulta.obtenerEstado()).toEqual({ salud: salud("10:30"), fallo: null, enCurso: false });
  });

  it("does not start a second query while one is in progress", async () => {
    const consulta = new ConsultaSalud();
    const pedido = diferida<Salud>();
    let pedidos = 0;
    const primera = consulta.consultar(() => (pedidos++, pedido.promesa));
    await consulta.consultar(async () => (pedidos++, salud("otra")));
    pedido.resolver(salud("10:00"));
    await primera;
    expect(pedidos).toBe(1);
    expect(consulta.obtenerEstado().salud).toEqual(salud("10:00"));
  });

  it("notifies its listeners until they unsubscribe", async () => {
    const consulta = new ConsultaSalud();
    let avisos = 0;
    const cancelar = consulta.suscribir(() => avisos++);
    await consulta.consultar(async () => salud("10:00"));
    expect(avisos).toBe(2);
    cancelar();
    await consulta.consultar(async () => salud("10:30"));
    expect(avisos).toBe(2);
  });
});

describe("faseSalud (PLT-02 regla 6)", () => {
  it.each([
    ["cargando", ESTADO_INICIAL_SALUD],
    ["error", { salud: null, fallo: new Error("red"), enCurso: false }],
    ["datos", { salud: salud("10:00"), fallo: null, enCurso: true }],
    ["desactualizado", { salud: salud("10:00"), fallo: new Error("red"), enCurso: false }],
  ] as const)("is %s", (fase, estado) => {
    expect(faseSalud(estado)).toBe(fase);
  });
});
