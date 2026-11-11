/**
 * Visibilidad de la pestaña sin React, con el documento inyectable para probarla. En el servidor (sin `document`)
 * se considera visible: el sondeo arranca recién en el cliente.
 */
export interface DocumentoVisible {
  visibilityState: DocumentVisibilityState;
  addEventListener: (tipo: "visibilitychange", oyente: () => void) => void;
  removeEventListener: (tipo: "visibilitychange", oyente: () => void) => void;
}

const documentoActual = (): DocumentoVisible | undefined => (typeof document === "undefined" ? undefined : document);

export function pestanaVisible(documento: DocumentoVisible | undefined = documentoActual()): boolean {
  return documento === undefined || documento.visibilityState === "visible";
}

export function escucharVisibilidad(
  alCambiar: () => void,
  documento: DocumentoVisible | undefined = documentoActual(),
): () => void {
  documento?.addEventListener("visibilitychange", alCambiar);
  return () => documento?.removeEventListener("visibilitychange", alCambiar);
}
