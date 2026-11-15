import en from "../../i18n/messages/en.json";
import es from "../../i18n/messages/es.json";
import { ERRORES_ETIQUETA, esquemaEtiqueta, NOMBRE_ETIQUETA_MAX } from "./etiquetas";
import { describe, expect, it } from "vitest";

import { claveErrorApi } from "@/lib/errores-api";

const mensajes = { asignaturaRequerida: "A", nombreRequerido: "N", nombreLongitud: "L" };

function apiError(status: number, message = "texto del servidor", code?: string) {
  return Object.assign(new Error(message), { status, code });
}

describe("esquemaEtiqueta", () => {
  it("requires subject and name when creating", async () => {
    const r = await esquemaEtiqueta(mensajes, "create")
      .validate({ asignatura_id: 0, nombre: "  " }, { abortEarly: false })
      .catch((e: { errors: string[] }) => e.errors);
    expect(r).toEqual(["A", "N"]);
  });

  it("does not require the subject when editing", async () => {
    await expect(esquemaEtiqueta(mensajes, "edit").validate({ asignatura_id: 0, nombre: "x" })).resolves.toBeTruthy();
  });

  it("rejects a name over the maximum", async () => {
    const nombre = "a".repeat(NOMBRE_ETIQUETA_MAX + 1);
    await expect(esquemaEtiqueta(mensajes, "edit").validate({ asignatura_id: 1, nombre })).rejects.toThrow("L");
  });
});

describe("ERRORES_ETIQUETA", () => {
  it("maps 409 on save to the duplicate-name text", () => {
    expect(claveErrorApi(apiError(409), ERRORES_ETIQUETA.guardar)).toBe("admin-etiquetas-error-duplicada");
  });

  it("maps 404 to the missing-tag text on every action, saving included", () => {
    for (const opciones of Object.values(ERRORES_ETIQUETA)) {
      expect(claveErrorApi(apiError(404), opciones)).toBe("admin-etiquetas-error-no-existe");
    }
  });

  it("does not read a 409 as a duplicate outside saving", () => {
    expect(claveErrorApi(apiError(409), ERRORES_ETIQUETA.estado)).toBe("comun-error-estado");
  });

  it("only uses keys that exist in both languages", () => {
    const claves = Object.values(ERRORES_ETIQUETA).flatMap((o) => Object.values(o.porStatus));
    for (const mensajes of [es.dashboard, en.dashboard] as unknown as Record<string, string>[]) {
      for (const clave of claves) expect(mensajes[clave], clave).toBeTruthy();
    }
  });
});
