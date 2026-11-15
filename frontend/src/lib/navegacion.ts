import type { MouseEvent } from "react";

/**
 * Clic izquierdo sin modificadores: el enlace navega en cliente (router). Con ⌘/Ctrl/Shift/botón central
 * se deja al navegador abrir el href en otra pestaña o ventana, como con cualquier <a>.
 */
export function esClickSimple(e: MouseEvent): boolean {
  return e.button === 0 && !e.metaKey && !e.ctrlKey && !e.shiftKey && !e.altKey;
}

/** Qué grupos del sidebar están abiertos, por id de grupo. */
export type GruposAbiertos = Record<string, boolean>;

/** `elegidos`: lo que decidió el usuario (es lo único que se guarda); `porRuta`: lo que abrió la ruta activa (no se guarda). */
export type EstadoGrupos = { elegidos: GruposAbiertos; porRuta: GruposAbiertos };

export type AccionGrupos =
  | { tipo: "recordado"; guardado: GruposAbiertos }
  | { tipo: "ruta"; id: string }
  | { tipo: "alternar"; id: string; abierto: boolean };

export const ESTADO_GRUPOS_INICIAL: EstadoGrupos = { elegidos: {}, porRuta: {} };

export function reducirGrupos(estado: EstadoGrupos, accion: AccionGrupos): EstadoGrupos {
  switch (accion.tipo) {
    case "recordado":
      return { ...estado, elegidos: { ...accion.guardado, ...estado.elegidos } };
    case "ruta":
      return { ...estado, porRuta: { ...estado.porRuta, [accion.id]: true } };
    case "alternar": {
      // Una decisión del usuario manda sobre lo que abrió la ruta.
      const porRuta = { ...estado.porRuta };
      delete porRuta[accion.id];
      return { elegidos: { ...estado.elegidos, [accion.id]: !accion.abierto }, porRuta };
    }
  }
}

/** Si un grupo está abierto: lo abierto por la ruta gana al estado recordado; si nadie lo fijó, `porOmision`. */
export const grupoAbierto = (estado: EstadoGrupos, id: string, porOmision: boolean): boolean =>
  estado.porRuta[id] ?? estado.elegidos[id] ?? porOmision;

/**
 * Lee de `localStorage` el estado recordado de los grupos del sidebar. Nunca lanza: sin almacenamiento (SSR, modo privado,
 * acceso bloqueado), con JSON roto o con valores que no son booleanos devuelve lo que sea válido (o `{}`).
 */
export function leerGruposAbiertos(clave: string): GruposAbiertos {
  try {
    const crudo = localStorage.getItem(clave);
    const dato: unknown = crudo ? JSON.parse(crudo) : null;
    if (typeof dato !== "object" || dato === null || Array.isArray(dato)) return {};
    return Object.fromEntries(
      Object.entries(dato).filter((par): par is [string, boolean] => typeof par[1] === "boolean"),
    );
  } catch {
    return {};
  }
}

/** Guarda el estado de los grupos del sidebar; si el almacenamiento no está disponible, no hace nada. */
export function guardarGruposAbiertos(clave: string, estado: GruposAbiertos): void {
  try {
    localStorage.setItem(clave, JSON.stringify(estado));
  } catch {
    // Sin almacenamiento el estado vale solo para esta visita.
  }
}
