import { claveDetalleHistorialVacio } from "./notificaciones-vacio";
import { describe, expect, it } from "vitest";

describe("claveDetalleHistorialVacio", () => {
  it("explains the pending invitation once it is known", () => {
    expect(claveDetalleHistorialVacio({ resuelto: true, pendiente: true })).toBe("notif-historial-vacio-invitacion");
  });

  it("says nothing without a pending invitation or while it is still unknown", () => {
    expect(claveDetalleHistorialVacio({ resuelto: true, pendiente: false })).toBeNull();
    expect(claveDetalleHistorialVacio({ resuelto: false, pendiente: false })).toBeNull();
  });
});
