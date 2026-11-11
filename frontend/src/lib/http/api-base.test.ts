import { API_BASE_DESARROLLO, resolverApiBase } from "./api-base";
import { describe, expect, it } from "vitest";

describe("resolverApiBase", () => {
  it("uses the configured URL, trimmed and without trailing slashes", () => {
    expect(resolverApiBase("https://api.ejemplo.com", "production")).toBe("https://api.ejemplo.com");
    expect(resolverApiBase("  https://api.ejemplo.com/  ", "production")).toBe("https://api.ejemplo.com");
    expect(resolverApiBase("http://localhost:8090//", "development")).toBe("http://localhost:8090");
  });

  it("fails clearly in production when the variable is missing or empty (?? did not cover the empty string)", () => {
    expect(() => resolverApiBase(undefined, "production")).toThrow(/NEXT_PUBLIC_API_URL no está definida/);
    expect(() => resolverApiBase("", "production")).toThrow(/NEXT_PUBLIC_API_URL no está definida/);
    expect(() => resolverApiBase("   ", "production")).toThrow(/NEXT_PUBLIC_API_URL no está definida/);
  });

  it("falls back to localhost only outside production", () => {
    expect(resolverApiBase(undefined, "development")).toBe(API_BASE_DESARROLLO);
    expect(resolverApiBase("", "test")).toBe(API_BASE_DESARROLLO);
  });

  it("rejects invalid URLs and non-http protocols", () => {
    expect(() => resolverApiBase("api.ejemplo.com", "development")).toThrow(/no es una URL válida/);
    expect(() => resolverApiBase("ftp://api.ejemplo.com", "production")).toThrow(/http o https/);
  });
});
