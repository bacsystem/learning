import {
  accionesDisponibles,
  CUERPO_REABRIR,
  cuerpoAprobar,
  cuerpoRechazar,
  motivoValido,
  notaObligatoria,
  notaParaAprobar,
} from "./acciones";
import { describe, expect, it } from "vitest";

import type { EstadoPregunta, OrigenPregunta } from "@/lib/questions/api";

const ORIGENES: OrigenPregunta[] = ["manual", "balotario", "ia"];
const ESTADOS: EstadoPregunta[] = ["borrador", "en_revision", "aprobada", "rechazada", "activo", "eliminada"];

describe("accionesDisponibles", () => {
  it("offers approve only on a manual draft; balotario and IA drafts are reviewed in their queue", () => {
    expect(accionesDisponibles("borrador", "manual")).toMatchObject({ aprobar: true, seRevisaEnCola: false });
    for (const origen of ["balotario", "ia"] as const) {
      expect(accionesDisponibles("borrador", origen)).toMatchObject({ aprobar: false, seRevisaEnCola: true });
    }
  });

  it("offers revalidate on en_revision for every origin", () => {
    for (const origen of ORIGENES) expect(accionesDisponibles("en_revision", origen).revalidar).toBe(true);
    expect(accionesDisponibles("borrador", "manual").revalidar).toBe(false);
  });

  it("offers reject on borrador and en_revision, and reopen only on rechazada", () => {
    expect(ESTADOS.filter((e) => accionesDisponibles(e, "manual").rechazar)).toEqual(["borrador", "en_revision"]);
    expect(ESTADOS.filter((e) => accionesDisponibles(e, "manual").reabrir)).toEqual(["rechazada"]);
  });

  it("offers nothing on aprobada, activo and eliminada", () => {
    for (const estado of ["aprobada", "activo", "eliminada"] as const) {
      for (const origen of ORIGENES) {
        expect(Object.values(accionesDisponibles(estado, origen)).some(Boolean)).toBe(false);
      }
    }
  });

  it("does not guess the origin: without it there is no approve, revalidate or queue", () => {
    expect(accionesDisponibles("borrador")).toMatchObject({ aprobar: false, seRevisaEnCola: false, rechazar: true });
    expect(accionesDisponibles("en_revision").revalidar).toBe(false);
  });
});

describe("motivoValido", () => {
  it("accepts 5 to 500 characters after trimming", () => {
    expect(motivoValido("  Clave incorrecta ")).toBe("Clave incorrecta");
    expect(motivoValido("malo")).toBeNull();
    expect(motivoValido("   ab   ")).toBeNull();
    expect(motivoValido("a".repeat(500))).not.toBeNull();
    expect(motivoValido("a".repeat(501))).toBeNull();
  });
});

describe("nota al aprobar o revalidar", () => {
  it("is required (10 to 500) for balotario and IA", () => {
    expect(notaObligatoria("ia")).toBe(true);
    expect(notaObligatoria("balotario")).toBe(true);
    expect(notaParaAprobar("ia", "")).toBeUndefined();
    expect(notaParaAprobar("ia", "corta")).toBeUndefined();
    expect(notaParaAprobar("ia", " Revisada contra la cita. ")).toBe("Revisada contra la cita.");
    expect(notaParaAprobar("balotario", "a".repeat(501))).toBeUndefined();
  });

  it("is optional in a manual question: empty becomes null, but too long is still rejected", () => {
    expect(notaObligatoria("manual")).toBe(false);
    expect(notaParaAprobar("manual", "  ")).toBeNull();
    expect(notaParaAprobar("manual", "ok")).toBe("ok");
    expect(notaParaAprobar("manual", "a".repeat(501))).toBeUndefined();
  });
});

describe("cuerpos de PATCH …/estado", () => {
  it("approve needs the checkbox and a valid note", () => {
    expect(cuerpoAprobar("manual", false, "")).toBeNull();
    expect(cuerpoAprobar("manual", true, "")).toEqual({ estado: "aprobada", verificado: true, nota: null });
    expect(cuerpoAprobar("ia", true, "")).toBeNull();
    expect(cuerpoAprobar("ia", true, "Nota de diez letras")).toEqual({
      estado: "aprobada",
      verificado: true,
      nota: "Nota de diez letras",
    });
  });

  it("reject needs a valid reason and reopen sends only the state", () => {
    expect(cuerpoRechazar("malo")).toBeNull();
    expect(cuerpoRechazar("Clave incorrecta")).toEqual({ estado: "rechazada", motivo: "Clave incorrecta" });
    expect(CUERPO_REABRIR).toEqual({ estado: "borrador" });
  });
});
