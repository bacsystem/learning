import { crearSondeo } from "./sondeo";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

beforeEach(() => vi.useFakeTimers());
afterEach(() => vi.useRealTimers());

describe("crearSondeo", () => {
  it("runs the task every interval while it should continue", async () => {
    const tarea = vi.fn(async () => {});
    const sondeo = crearSondeo({ tarea, intervalo: 5000, seguir: () => true });
    sondeo.iniciar();
    expect(tarea).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(5000);
    expect(tarea).toHaveBeenCalledTimes(1);
    await vi.advanceTimersByTimeAsync(5000);
    expect(tarea).toHaveBeenCalledTimes(2);
    sondeo.detener();
  });

  it("stops when nothing is left in process", async () => {
    let quedan = 2;
    const tarea = vi.fn(async () => {
      quedan--;
    });
    crearSondeo({ tarea, intervalo: 5000, seguir: () => quedan > 0 }).iniciar();
    await vi.advanceTimersByTimeAsync(30000);
    expect(tarea).toHaveBeenCalledTimes(2);
  });

  it("schedules the next run only after a slow task finishes (no pile-up)", async () => {
    let terminar!: () => void;
    const tarea = vi.fn(() => new Promise<void>((r) => (terminar = r)));
    const sondeo = crearSondeo({ tarea, intervalo: 5000, seguir: () => true });
    sondeo.iniciar();
    await vi.advanceTimersByTimeAsync(5000);
    await vi.advanceTimersByTimeAsync(20000);
    expect(tarea).toHaveBeenCalledTimes(1);
    terminar();
    await vi.advanceTimersByTimeAsync(5000);
    expect(tarea).toHaveBeenCalledTimes(2);
    sondeo.detener();
  });

  it("keeps polling after a failed task", async () => {
    const tarea = vi.fn(async () => Promise.reject(new Error("500")));
    const sondeo = crearSondeo({ tarea, intervalo: 5000, seguir: () => true });
    sondeo.iniciar();
    await vi.advanceTimersByTimeAsync(10000);
    expect(tarea).toHaveBeenCalledTimes(2);
    sondeo.detener();
  });

  it("detener cancels the pending run (unmount)", async () => {
    const tarea = vi.fn(async () => {});
    const sondeo = crearSondeo({ tarea, intervalo: 5000, seguir: () => true });
    sondeo.iniciar();
    sondeo.detener();
    await vi.advanceTimersByTimeAsync(20000);
    expect(tarea).not.toHaveBeenCalled();
  });
});
