import { claveErrorEtiqueta, esquemaEtiqueta, NOMBRE_ETIQUETA_MAX } from "./etiquetas";
import { describe, expect, it } from "vitest";

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

describe("claveErrorEtiqueta", () => {
  it("maps 409 on save to the duplicate-name text", () => {
    expect(claveErrorEtiqueta(apiError(409), "guardar")).toBe("admin-etiquetas-error-duplicada");
  });

  it("maps 404 to the missing-tag text on every action", () => {
    expect(claveErrorEtiqueta(apiError(404), "eliminar")).toBe("admin-etiquetas-error-no-existe");
    expect(claveErrorEtiqueta(apiError(404), "estado")).toBe("admin-etiquetas-error-no-existe");
  });

  it("falls back to the generic text of the action, never to the server message", () => {
    expect(claveErrorEtiqueta(apiError(400, "Campos inválidos o faltantes: nombre."), "guardar")).toBe(
      "comun-error-guardar",
    );
    expect(claveErrorEtiqueta(apiError(500), "estado")).toBe("comun-error-estado");
    expect(claveErrorEtiqueta(new TypeError("Failed to fetch"), "eliminar")).toBe("comun-error-eliminar");
    expect(claveErrorEtiqueta(apiError(409), "estado")).toBe("comun-error-estado");
  });

  it("uses the generic text for a coded error without translation", () => {
    expect(claveErrorEtiqueta(apiError(409, "x", "codigo_nuevo"), "guardar")).toBe("comun-error-guardar");
  });
});
