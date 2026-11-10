import type { MouseEvent } from "react";

/**
 * Clic izquierdo sin modificadores: el enlace navega en cliente (router). Con ⌘/Ctrl/Shift/botón central
 * se deja al navegador abrir el href en otra pestaña o ventana, como con cualquier <a>.
 */
export function esClickSimple(e: MouseEvent): boolean {
  return e.button === 0 && !e.metaKey && !e.ctrlKey && !e.shiftKey && !e.altKey;
}
