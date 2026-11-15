import { guardarIdiomaDelUsuario } from "./idioma";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const fetchMock = vi.fn();

beforeEach(() => {
  fetchMock.mockResolvedValue(new Response(JSON.stringify({ idioma: "en" }), { status: 200 }));
  vi.stubGlobal("fetch", fetchMock);
});
afterEach(() => {
  fetchMock.mockReset();
  vi.unstubAllGlobals();
});

const siguienteTurno = () => new Promise((r) => setTimeout(r, 0));

describe("guardarIdiomaDelUsuario", () => {
  it("sends PUT /auth/idioma with the token and the chosen language", () => {
    guardarIdiomaDelUsuario("tok", "en");
    const [url, init] = fetchMock.mock.calls[0];
    expect(String(url)).toMatch(/\/auth\/idioma$/);
    expect(init.method).toBe("PUT");
    expect(new Headers(init.headers).get("Authorization")).toBe("Bearer tok");
    expect(JSON.parse(init.body as string)).toEqual({ idioma: "en" });
  });

  it("does not call without a session", () => {
    guardarIdiomaDelUsuario(null, "en");
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("returns at once and swallows an error response", async () => {
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify({ error: "idioma_invalido", message: "x" }), { status: 422 }),
    );
    expect(guardarIdiomaDelUsuario("tok", "en")).toBeUndefined();
    await siguienteTurno();
  });

  it("swallows a network failure", async () => {
    fetchMock.mockRejectedValue(new TypeError("network"));
    expect(() => guardarIdiomaDelUsuario("tok", "es")).not.toThrow();
    await siguienteTurno();
  });
});
