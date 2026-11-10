/**
 * Diseños disponibles. Un diseño es un bloque de tokens en `src/styles/tokens.css` que se activa con `<html data-diseno="valor">`;
 * el modo claro/oscuro sigue siendo aparte (clase `.dark`), así que cualquier diseño existe en los dos.
 * Para sumar uno: agrega su bloque en `src/styles/tokens.css` y su entrada aquí (ver docs/design-system.md, «Diseños»).
 */
export type Diseno = { valor: string; etiqueta: string };

/** Clave de localStorage donde se guarda el diseño elegido. */
export const CLAVE_DISENO = "khipu-diseno";

/** Diseño base de khipu: no define bloque en `src/styles/tokens.css`, son los tokens de `:root` y `.dark`. */
export const DISENO_POR_DEFECTO = "khipu";

export const DISENOS: Diseno[] = [{ valor: "khipu", etiqueta: "khipu" }];
