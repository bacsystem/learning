import { avisarSiSesionExpirada, escucharSesionExpirada, EVENTO_SESION_EXPIRADA, sesionExpirada } from "./sesion";
import { describe, expect, it, vi } from "vitest";

describe("sesionExpirada", () => {
  it("only a 401 closes the session; 403 and 503 keep it", () => {
    expect(sesionExpirada(401)).toBe(true);
    for (const status of [400, 403, 404, 409, 422, 429, 500, 502, 503])
      expect(sesionExpirada(status), String(status)).toBe(false);
  });
});

describe("avisarSiSesionExpirada / escucharSesionExpirada", () => {
  it("fires the event on 401 and not on 403 or 503", () => {
    const destino = new EventTarget();
    const alExpirar = vi.fn();
    const dejar = escucharSesionExpirada(alExpirar, destino);
    avisarSiSesionExpirada(403, destino);
    avisarSiSesionExpirada(503, destino);
    expect(alExpirar).not.toHaveBeenCalled();
    avisarSiSesionExpirada(401, destino);
    expect(alExpirar).toHaveBeenCalledTimes(1);
    dejar();
    avisarSiSesionExpirada(401, destino);
    expect(alExpirar).toHaveBeenCalledTimes(1);
  });

  it("does nothing without a window (server or tests in node)", () => {
    expect(() => avisarSiSesionExpirada(401, undefined)).not.toThrow();
  });

  it("uses the event name the app already listens to", () => {
    expect(EVENTO_SESION_EXPIRADA).toBe("auth:expired");
  });
});
