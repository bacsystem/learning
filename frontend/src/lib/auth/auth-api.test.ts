import { authApi } from "./auth-api";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const fetchMock = vi.fn();

beforeEach(() => {
  fetchMock.mockResolvedValue(new Response(JSON.stringify({ message: "ok" }), { status: 200 }));
  vi.stubGlobal("fetch", fetchMock);
});
afterEach(() => {
  fetchMock.mockReset();
  vi.unstubAllGlobals();
});

const cuerpoEnviado = () => JSON.parse(fetchMock.mock.calls[0][1].body as string) as Record<string, unknown>;

describe("authApi sends the UI locale for the e-mails", () => {
  it("register", async () => {
    await authApi.register({ dni: "12345678", email: "a@b.pe", locale: "en" });
    expect(cuerpoEnviado()).toMatchObject({ locale: "en" });
  });

  it("verifyEmail", async () => {
    await authApi.verifyEmail({ token: "t", username: "usuario", password: "p", confirm_password: "p", locale: "en" });
    expect(cuerpoEnviado()).toMatchObject({ locale: "en" });
  });

  it("forgotPassword", async () => {
    await authApi.forgotPassword("a@b.pe", "en");
    expect(cuerpoEnviado()).toEqual({ email: "a@b.pe", locale: "en" });
  });

  it("resetPassword", async () => {
    await authApi.resetPassword("t", "p", "p", "es");
    expect(cuerpoEnviado()).toMatchObject({ locale: "es" });
  });

  it("omits the field when no locale is given", async () => {
    await authApi.forgotPassword("a@b.pe");
    expect(cuerpoEnviado()).toEqual({ email: "a@b.pe" });
  });
});
