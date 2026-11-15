import { instanteDeFechaServidor } from "./fechas";
import { describe, expect, it } from "vitest";

import { formatearFechaHora } from "@/lib/formato";

describe("instanteDeFechaServidor", () => {
  it("reads the server date as UTC, so Lima shows five hours earlier", () => {
    const instante = instanteDeFechaServidor("15/11/2026 05:49");
    expect(instante).toBe("2026-11-15T05:49:00Z");
    expect(formatearFechaHora(instante ?? "")).toBe("15 Nov 2026, 00:49");
  });

  it("returns null for anything else", () => {
    expect(instanteDeFechaServidor(null)).toBeNull();
    expect(instanteDeFechaServidor("")).toBeNull();
    expect(instanteDeFechaServidor("2026-11-15T05:49:00Z")).toBeNull();
    expect(instanteDeFechaServidor("15/11/2026")).toBeNull();
  });
});
