import DashboardLayout from "./layout";
import { createElement } from "react";
import { renderToString } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";

import Loading from "@/app/loading";

/**
 * Issue #143: en la hidratación el layout tiene que dar el mismo HTML que el servidor (el indicador de carga), aunque
 * `AuthProvider` ya haya resuelto la sesión. Aquí la sesión ya está resuelta con el perfil pendiente (backend caído).
 */
vi.mock("@/lib/auth/auth-context", () => ({ useAuth: () => ({ token: "tk", loading: false, user: null }) }));
vi.mock("next-intl", () => ({ useTranslations: () => (clave: string) => clave }));
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace: vi.fn() }), usePathname: () => "/admin/salud" }));

describe("DashboardLayout en el servidor y en la hidratación (issue #143)", () => {
  it("renders the loading indicator, not the server-down notice", () => {
    const html = renderToString(createElement(DashboardLayout, null, "contenido"));
    expect(html).toBe(renderToString(createElement(Loading)));
    expect(html).not.toContain("sesion-servidor-no-disponible-titulo");
  });
});
