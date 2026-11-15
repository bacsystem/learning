import en from "../../i18n/messages/en.json";
import es from "../../i18n/messages/es.json";
import { gruposDelSidebar, NAVEGACION, RUTAS_EXTRA } from "./navegacion";
import { describe, expect, it } from "vitest";

const gruposAdmin = NAVEGACION.filter((g) => g.soloAdmin);
const hrefsAdmin = gruposAdmin.flatMap((g) => g.items.map((i) => i.href));

/** Resuelve claves anidadas como `saludServicios.menu`. */
const tieneClave = (mensajes: unknown, clave: string) =>
  clave
    .split(".")
    .reduce<unknown>((nodo, parte) => (nodo as Record<string, unknown> | undefined)?.[parte], mensajes) !== undefined;

describe("navegacion del administrador", () => {
  it("puts every admin group in a collapsible group with a stable id", () => {
    expect(gruposAdmin.length).toBeGreaterThan(0);
    for (const grupo of gruposAdmin) {
      expect(grupo.colapsable, grupo.titulo).toBe(true);
      expect(grupo.id, grupo.titulo).toMatch(/^[a-z]+$/);
    }
  });

  it("keeps the ids of the groups unique and fixed, since the open state is saved by id", () => {
    expect(gruposAdmin.map((g) => g.id)).toEqual(["personas", "contenido", "catalogos", "sistema"]);
  });

  it("keeps the study block fixed and not collapsible", () => {
    const estudio = NAVEGACION.filter((g) => !g.soloAdmin);
    expect(estudio.length).toBeGreaterThan(0);
    expect(estudio.some((g) => g.colapsable)).toBe(false);
  });

  it("lists every admin route once", () => {
    expect(new Set(hrefsAdmin).size).toBe(hrefsAdmin.length);
    expect(hrefsAdmin).toHaveLength(22);
    expect(hrefsAdmin.every((h) => h.startsWith("/admin/"))).toBe(true);
  });

  it("gives the admin routes without an item a section that is a group", () => {
    const titulos = new Set(NAVEGACION.map((g) => g.titulo));
    for (const r of RUTAS_EXTRA.filter((x) => x.prefijo.startsWith("/admin/"))) {
      expect(titulos.has(r.seccion), r.prefijo).toBe(true);
    }
  });

  it("has every title and label in es and en", () => {
    const claves = NAVEGACION.flatMap((g) => [g.titulo, ...g.items.map((i) => i.etiqueta)]);
    for (const clave of [...claves, ...RUTAS_EXTRA.flatMap((r) => [r.seccion, r.pagina])]) {
      expect(tieneClave(es.dashboard, clave), `es ${clave}`).toBe(true);
      expect(tieneClave(en.dashboard, clave), `en ${clave}`).toBe(true);
    }
  });
});

describe("gruposDelSidebar", () => {
  const t = (clave: string) => `t:${clave}`;

  it("passes id and collapsible to the sidebar for the admin groups and not for the study block", () => {
    const grupos = gruposDelSidebar(true, t);
    expect(grupos).toHaveLength(NAVEGACION.length);
    for (const [i, grupo] of grupos.entries()) {
      expect(grupo.id).toBe(NAVEGACION[i].id);
      expect(grupo.colapsable).toBe(NAVEGACION[i].colapsable);
    }
    expect(grupos.filter((g) => g.colapsable).map((g) => g.id)).toEqual([
      "personas",
      "contenido",
      "catalogos",
      "sistema",
    ]);
  });

  it("hides the admin groups from who is not admin", () => {
    const grupos = gruposDelSidebar(false, t);
    expect(grupos).toHaveLength(NAVEGACION.filter((g) => !g.soloAdmin).length);
    expect(grupos.some((g) => g.colapsable)).toBe(false);
  });

  it("translates titles and labels and keeps href and exact match", () => {
    const grupos = gruposDelSidebar(true, t);
    const configuracion = grupos.flatMap((g) => g.items).find((i) => i.href === "/admin/configuracion");
    expect(configuracion).toMatchObject({ label: "t:menu-admin-configuracion", exacto: true });
    expect(grupos[0].titulo).toBe(`t:${NAVEGACION[0].titulo}`);
  });
});
