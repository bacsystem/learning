import Home from "./page";
import { createElement } from "react";
import { renderToString } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";

// En el servidor la sesión aún no se conoce: `loading` es true (issue #134).
vi.mock("@/lib/auth/auth-context", () => ({ useAuth: () => ({ isAuthenticated: false, loading: true }) }));
vi.mock("next-intl", () => {
  const t = Object.assign((clave: string) => clave, { rich: (clave: string) => clave, raw: (clave: string) => clave });
  return { useTranslations: () => t };
});

describe("landing en el servidor (issue #134)", () => {
  const html = renderToString(createElement(Home));

  it("renders the landing even while the session is loading", () => {
    expect(html).toContain("landing-pasos-titulo");
  });

  it("leaves out the session link until hydration", () => {
    expect(html).not.toContain("landing-registrate-opinar");
    expect(html).not.toContain("landing-dejar-opinion");
  });
});
