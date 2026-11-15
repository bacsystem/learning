import { comoLocale } from "./constants";
import { describe, expect, it } from "vitest";

describe("comoLocale", () => {
  it("accepts the app languages and rejects anything else", () => {
    expect(comoLocale("es")).toBe("es");
    expect(comoLocale("en")).toBe("en");
    expect(comoLocale("en-US")).toBeUndefined();
    expect(comoLocale("")).toBeUndefined();
  });
});
