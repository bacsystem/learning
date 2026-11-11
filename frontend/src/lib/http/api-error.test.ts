import { apiErrorCode, apiErrorFrom } from "./api-error";
import { describe, expect, it } from "vitest";

function jsonResponse(body: unknown, status: number, statusText = "Error"): Response {
  return new Response(JSON.stringify(body), { status, statusText });
}

describe("apiErrorFrom", () => {
  it("uses message as text and error as code with the new format", async () => {
    const err = await apiErrorFrom(
      jsonResponse({ error: "tema_duplicado", message: "Ya existe un tema.", detalle: { campo: "nombre" } }, 409),
    );

    expect(err.message).toBe("Ya existe un tema.");
    expect(err.status).toBe(409);
    expect(err.code).toBe("tema_duplicado");
    expect(err.detalle).toEqual({ campo: "nombre" });
    expect(apiErrorCode(err)).toBe("tema_duplicado");
  });

  it("keeps error as text and no code with the legacy format", async () => {
    const err = await apiErrorFrom(jsonResponse({ error: "Acceso denegado." }, 403));

    expect(err.message).toBe("Acceso denegado.");
    expect(err.status).toBe(403);
    expect(err.code).toBeUndefined();
    expect(apiErrorCode(err)).toBeUndefined();
  });

  it("falls back to statusText when the body is not a JSON object", async () => {
    const err = await apiErrorFrom(new Response("<html>", { status: 502, statusText: "Bad Gateway" }));

    expect(err.message).toBe("Bad Gateway");
    expect(err.status).toBe(502);
    expect(err.code).toBeUndefined();
  });

  it("ignores non-string fields", async () => {
    const err = await apiErrorFrom(jsonResponse({ error: 42, message: null, detalle: "x" }, 500, "Internal"));

    expect(err.message).toBe("Internal");
    expect(err.code).toBeUndefined();
    expect(err.detalle).toBeUndefined();
  });
});

describe("apiErrorCode", () => {
  it("returns undefined for errors without a code", () => {
    expect(apiErrorCode(new Error("x"))).toBeUndefined();
    expect(apiErrorCode(null)).toBeUndefined();
  });
});
