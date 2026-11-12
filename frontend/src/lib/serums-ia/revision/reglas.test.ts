import {
  citasLlenas,
  consultaValida,
  estaCitado,
  fuenteRetirada,
  idsTrasAnadir,
  idsTrasQuitar,
  MAX_CITAS,
} from "./reglas";
import { describe, expect, it } from "vitest";

import type { CitaPregunta } from "@/lib/questions/api";

const cita = (pasaje_id: number, fuente_estado = "activa") => ({ pasaje_id, fuente_estado }) as CitaPregunta;

describe("consultaValida", () => {
  it("trims and accepts 3 to 300 characters", () => {
    expect(consultaValida("  abc ")).toBe("abc");
    expect(consultaValida("a".repeat(300))).toHaveLength(300);
  });

  it("rejects shorter and longer queries, counting characters and not UTF-16 units", () => {
    expect(consultaValida("ab")).toBeNull();
    expect(consultaValida("  ab  ")).toBeNull();
    expect(consultaValida("a".repeat(301))).toBeNull();
    expect(consultaValida("😀".repeat(300))).not.toBeNull();
  });
});

describe("lista completa de citas", () => {
  const citas = [cita(3), cita(1)];

  it("appends the new passage keeping the current ones", () => {
    expect(idsTrasAnadir(citas, 7)).toEqual([3, 1, 7]);
  });

  it("removes only the chosen passage", () => {
    expect(idsTrasQuitar(citas, 3)).toEqual([1]);
    expect(idsTrasQuitar([cita(3)], 3)).toEqual([]);
  });

  it("detects already cited passages and the limit", () => {
    expect(estaCitado(citas, 1)).toBe(true);
    expect(estaCitado(citas, 2)).toBe(false);
    expect(citasLlenas(Array.from({ length: MAX_CITAS }, (_, i) => cita(i + 1)))).toBe(true);
    expect(citasLlenas(citas)).toBe(false);
  });

  it("flags citations to a withdrawn source", () => {
    expect(fuenteRetirada(cita(1, "retirada"))).toBe(true);
    expect(fuenteRetirada(cita(1))).toBe(false);
  });
});
