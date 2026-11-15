import en from "../../i18n/messages/en.json";
import es from "../../i18n/messages/es.json";
import { destinoNotificacion, textoNotificacion } from "./notificaciones-plantilla";
import { describe, expect, it } from "vitest";

import type { Traductor } from "@/components/app/textos-khipu";

/** `t` mínimo con la interpolación `{nombre}` de los mensajes reales, para probar el texto final. */
const traductor =
  (mensajes: Record<string, string>): Traductor =>
  (clave, valores = {}) =>
    (mensajes[clave] ?? clave).replace(/\{(\w+)\}/g, (_, k: string) => String(valores[k] ?? `{${k}}`));

const tEs = traductor(es.dashboard as unknown as Record<string, string>);
const tEn = traductor(en.dashboard as unknown as Record<string, string>);

const guardada = { titulo: "Guardado", cuerpo: "Texto guardado" };
const rechazada = {
  ...guardada,
  clave: "inscripcion_rechazada",
  parametros: { proceso: "Proceso 2026", institucion: "Institución A", motivo: "Documento ilegible" },
};

describe("textoNotificacion", () => {
  it("translates by key into the current language, with the parameters", () => {
    const es = textoNotificacion(rechazada, tEs);
    expect(es.titulo).toBe("Inscripción rechazada");
    expect(es.cuerpo).toContain("Proceso 2026");
    expect(es.cuerpo).toContain("Documento ilegible");
    const en = textoNotificacion(rechazada, tEn);
    expect(en.titulo).toBe("Enrollment rejected");
    expect(en.cuerpo).toContain("Documento ilegible");
    expect(en.cuerpo).not.toBe(es.cuerpo);
  });

  it("translates an approval, which has no reason", () => {
    const aprobada = {
      ...guardada,
      clave: "inscripcion_aprobada",
      parametros: { proceso: "Proceso 2026", institucion: "Institución A" },
    };
    expect(textoNotificacion(aprobada, tEn)).toEqual({
      titulo: "Enrollment approved",
      cuerpo: expect.stringContaining("Proceso 2026"),
    });
  });

  it("keeps the reason as plain text", () => {
    const n = { ...rechazada, parametros: { ...rechazada.parametros, motivo: "<b>x</b> {proceso}" } };
    expect(textoNotificacion(n, tEs).cuerpo).toContain("<b>x</b> {proceso}");
  });

  it("shows the saved text without a key, with an unknown key or with null", () => {
    expect(textoNotificacion({ ...guardada, clave: null, parametros: null }, tEs)).toEqual(guardada);
    expect(textoNotificacion(guardada, tEs)).toEqual(guardada);
    expect(textoNotificacion({ ...guardada, clave: "otra_clave", parametros: { a: "b" } }, tEs)).toEqual(guardada);
    expect(textoNotificacion({ ...guardada, clave: "constructor" }, tEs)).toEqual(guardada);
  });

  it("shows the saved text when a parameter the template needs is missing", () => {
    const sinMotivo = { ...rechazada, parametros: { proceso: "P", institucion: "I" } };
    expect(textoNotificacion(sinMotivo, tEs)).toEqual(guardada);
    expect(textoNotificacion({ ...rechazada, parametros: null }, tEs)).toEqual(guardada);
  });
});

describe("destinoNotificacion", () => {
  it("takes the enrollment notifications to /procesos", () => {
    expect(destinoNotificacion({ clave: "inscripcion_aprobada" })).toBe("/procesos");
    expect(destinoNotificacion({ clave: "inscripcion_rechazada" })).toBe("/procesos");
  });

  it("goes nowhere without a template", () => {
    expect(destinoNotificacion({ clave: null })).toBeNull();
    expect(destinoNotificacion({})).toBeNull();
    expect(destinoNotificacion({ clave: "otra" })).toBeNull();
  });
});

describe("textos de las plantillas", () => {
  it("exist in es and en, with the same placeholders", () => {
    for (const k of ["aprobada", "rechazada"]) {
      for (const parte of ["titulo", "cuerpo"]) {
        const clave = `notif-plantilla-inscripcion-${k}-${parte}`;
        const a = (es.dashboard as unknown as Record<string, string>)[clave];
        const b = (en.dashboard as unknown as Record<string, string>)[clave];
        expect(a, clave).toBeTruthy();
        expect(b, clave).toBeTruthy();
        expect([...a.matchAll(/\{(\w+)\}/g)].map((m) => m[1]).sort()).toEqual(
          [...b.matchAll(/\{(\w+)\}/g)].map((m) => m[1]).sort(),
        );
      }
    }
  });
});
