import { formatoFecha } from "./use-formato-fecha";
import { describe, expect, it } from "vitest";

describe("formatoFecha", () => {
  it("writes dates in Spanish", () => {
    const f = formatoFecha("es");
    expect(f.fecha("2026-09-15")).toBe("15 Set 2026");
    expect(f.fechaHora("2026-11-15T05:49:00Z")).toBe("15 Nov 2026, 00:49");
    expect(f.fechaCorta("2026-03-05")).toBe("5 Mar");
    expect(f.mesAnio("2026-03")).toBe("Mar 2026");
  });

  it("writes dates in English, with the same Lima time", () => {
    const f = formatoFecha("en");
    expect(f.fecha("2026-09-15")).toBe("Sep 15, 2026");
    expect(f.fechaHora("2026-11-15T05:49:00Z")).toBe("Nov 15, 2026, 00:49");
    expect(f.fechaCorta("2026-03-05")).toBe("Mar 5");
    expect(f.mesAnio("2026-03")).toBe("Mar 2026");
  });
});
