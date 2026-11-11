import { EVENTO_SESION_EXPIRADA } from "./sesion";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { authApi } from "@/lib/auth/auth-api";
import { catalogsApi } from "@/lib/catalogs/api";
import { saludApi } from "@/lib/salud/api";
import { temasApi } from "@/lib/serums-ia/temas/api";

/** Los clientes reales, con `fetch` simulado y una ventana falsa: solo un 401 cierra la sesión. */
const ventana = new EventTarget();
const alExpirar = vi.fn();
const respuesta = (status: number, error: string) => async () => new Response(JSON.stringify({ error }), { status });

beforeEach(() => {
  vi.stubGlobal("window", ventana);
  ventana.addEventListener(EVENTO_SESION_EXPIRADA, alExpirar);
});
afterEach(() => {
  ventana.removeEventListener(EVENTO_SESION_EXPIRADA, alExpirar);
  vi.unstubAllGlobals();
  alExpirar.mockReset();
});

const llamadas = {
  catalogs: () => catalogsApi.listAsignaturas("tk"),
  temas: () => temasApi.getTema("tk", 1),
  "auth/me": () => authApi.getMe("tk"),
  salud: () => saludApi.consultar("tk"),
};

describe.each(Object.entries(llamadas))("cliente %s", (_, llamar) => {
  it("503 (BD caída al comprobar la sesión) keeps the session and rejects with the status", async () => {
    vi.stubGlobal("fetch", vi.fn(respuesta(503, "Servicio no disponible.")));
    await expect(llamar()).rejects.toMatchObject({ status: 503 });
    expect(alExpirar).not.toHaveBeenCalled();
  });

  it("403 keeps the session", async () => {
    vi.stubGlobal("fetch", vi.fn(respuesta(403, "Acceso denegado.")));
    await expect(llamar()).rejects.toMatchObject({ status: 403 });
    expect(alExpirar).not.toHaveBeenCalled();
  });

  it("401 closes the session", async () => {
    vi.stubGlobal("fetch", vi.fn(respuesta(401, "Sesión revocada.")));
    await expect(llamar()).rejects.toMatchObject({ status: 401 });
    expect(alExpirar).toHaveBeenCalledTimes(1);
  });
});
