import en from "../i18n/messages/en.json";
import es from "../i18n/messages/es.json";
import type { OpcionesError } from "./errores-api";
import { expect } from "vitest";

/** Todas las claves que `claveErrorApi` puede devolver con esas opciones: códigos, status y el genérico. */
export function clavesDeOpciones(o: OpcionesError): string[] {
  return [
    ...Object.values(o.porCodigo ?? {}),
    ...Object.values(o.porStatus ?? {}),
    o.generico ?? `comun-error-${o.accion}`,
  ];
}

/** Falla si alguna clave de las opciones no existe (o está vacía) en es.json o en en.json. */
export function esperarClavesEnAmbosIdiomas(...opciones: OpcionesError[]): void {
  const idiomas = { es: es.dashboard, en: en.dashboard } as unknown as Record<string, Record<string, string>>;
  for (const [idioma, mensajes] of Object.entries(idiomas)) {
    for (const clave of opciones.flatMap(clavesDeOpciones)) expect(mensajes[clave], `${idioma}: ${clave}`).toBeTruthy();
  }
}
