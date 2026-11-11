import type { ProcesoOpcion } from "./api";
import { cambiarProceso, especialidadesDe, intentarGuardar, puedeGuardar, seleccionInicial } from "./seleccion";
import { describe, expect, it, vi } from "vitest";

const proceso = (id: number, especialidades: number[]): ProcesoOpcion => ({
  id,
  nombre: `Proceso ${id}`,
  anio: 2026,
  especialidades: especialidades.map((e) => ({ id: e, nombre: `Esp ${e}`, codigo: `E${e}`, pesos: "estimada" })),
});

describe("seleccionInicial", () => {
  it("uses the server suggestion as is", () => {
    expect(
      seleccionInicial({
        procesos: [proceso(12, [2, 5]), proceso(13, [5])],
        preseleccion: { proceso_id: 12, especialidad_id: 5 },
      }),
    ).toEqual({ procesoId: 12, especialidadId: 5 });
  });

  it("fills the only process when the suggestion has proceso_id null", () => {
    expect(
      seleccionInicial({ procesos: [proceso(12, [5])], preseleccion: { proceso_id: null, especialidad_id: 5 } }),
    ).toEqual({
      procesoId: 12,
      especialidadId: 5,
    });
  });

  it("keeps proceso_id null with several processes", () => {
    expect(
      seleccionInicial({
        procesos: [proceso(12, [5]), proceso(13, [5])],
        preseleccion: { proceso_id: null, especialidad_id: 5 },
      }),
    ).toEqual({ procesoId: null, especialidadId: 5 });
  });

  it("without a suggestion, picks the only process and leaves the specialty empty", () => {
    expect(seleccionInicial({ procesos: [proceso(12, [2])], preseleccion: null })).toEqual({
      procesoId: 12,
      especialidadId: null,
    });
  });

  it("without a suggestion and several processes, starts empty", () => {
    expect(seleccionInicial({ procesos: [proceso(12, [2]), proceso(13, [2])], preseleccion: null })).toEqual({
      procesoId: null,
      especialidadId: null,
    });
  });

  it("with no approved enrollment, starts empty", () => {
    expect(seleccionInicial({ procesos: [], preseleccion: null })).toEqual({ procesoId: null, especialidadId: null });
  });
});

describe("cambiarProceso", () => {
  const procesos = [proceso(12, [2, 5]), proceso(13, [5]), proceso(14, [7])];

  it("keeps the specialty when the new process also has it", () => {
    expect(cambiarProceso(procesos, { procesoId: 12, especialidadId: 5 }, 13)).toEqual({
      procesoId: 13,
      especialidadId: 5,
    });
  });

  it("clears the specialty when the new process does not have it", () => {
    expect(cambiarProceso(procesos, { procesoId: 12, especialidadId: 5 }, 14)).toEqual({
      procesoId: 14,
      especialidadId: null,
    });
    expect(cambiarProceso(procesos, { procesoId: 12, especialidadId: 5 }, null)).toEqual({
      procesoId: null,
      especialidadId: null,
    });
  });
});

describe("especialidadesDe", () => {
  it("returns the specialties of the chosen process, none without one", () => {
    const procesos = [proceso(12, [2, 5])];
    expect(especialidadesDe(procesos, 12).map((e) => e.id)).toEqual([2, 5]);
    expect(especialidadesDe(procesos, null)).toEqual([]);
  });
});

describe("puedeGuardar", () => {
  it("needs a full pair that differs from the active profile", () => {
    expect(puedeGuardar({ procesoId: null, especialidadId: 5 }, null)).toBe(false);
    expect(puedeGuardar({ procesoId: 12, especialidadId: null }, null)).toBe(false);
    expect(puedeGuardar({ procesoId: 12, especialidadId: 5 }, null)).toBe(true);
    expect(puedeGuardar({ procesoId: 12, especialidadId: 5 }, { procesoId: 12, especialidadId: 5 })).toBe(false);
    expect(puedeGuardar({ procesoId: 12, especialidadId: 2 }, { procesoId: 12, especialidadId: 5 })).toBe(true);
  });
});

describe("intentarGuardar (409 confirmacion_requerida → diálogo → reenvío)", () => {
  const confirmacion = () =>
    Object.assign(new Error("Confirma."), {
      status: 409,
      code: "confirmacion_requerida",
      detalle: { actual: { proceso_id: 12, especialidad_id: 2 } },
    });

  it("saves directly when the server accepts the first request", async () => {
    const guardar = vi.fn(async () => {});
    expect(await intentarGuardar(guardar, false)).toEqual({ tipo: "guardado" });
    expect(guardar).toHaveBeenCalledExactlyOnceWith(false);
  });

  it("asks for confirmation on 409 without resending on its own", async () => {
    const guardar = vi.fn(async () => Promise.reject(confirmacion()));
    expect(await intentarGuardar(guardar, false)).toEqual({ tipo: "confirmar" });
    expect(guardar).toHaveBeenCalledExactlyOnceWith(false);
  });

  it("after the user confirms, resends with confirmar_reinicio true", async () => {
    const guardar = vi.fn(async (confirmar: boolean) => (confirmar ? undefined : Promise.reject(confirmacion())));
    expect(await intentarGuardar(guardar, false)).toEqual({ tipo: "confirmar" });
    expect(await intentarGuardar(guardar, true)).toEqual({ tipo: "guardado" });
    expect(guardar.mock.calls).toEqual([[false], [true]]);
  });

  it("treats a 409 on the confirmed request as an error, not as another confirmation", async () => {
    const error = confirmacion();
    expect(await intentarGuardar(async () => Promise.reject(error), true)).toEqual({ tipo: "error", error });
  });

  it("returns any other error as is", async () => {
    const error = Object.assign(new Error("x"), { status: 422, code: "especialidad_invalida" });
    expect(await intentarGuardar(async () => Promise.reject(error), false)).toEqual({ tipo: "error", error });
  });
});
