/**
 * Sesión expirada: el único lugar que decide cuándo una respuesta de la API cierra la sesión.
 *
 * Solo un **401** significa que la sesión no existe, expiró o fue revocada. Un **403** (sin permiso) o un **503**
 * (p. ej. la BD caída al comprobar la sesión) se muestran como error y la sesión se conserva
 * (docs/api/plataforma-salud.md, «Autenticación con la BD caída»; ADR 0005).
 */
export const EVENTO_SESION_EXPIRADA = "auth:expired";

export function sesionExpirada(status: number): boolean {
  return status === 401;
}

const ventana = (): EventTarget | undefined => (typeof window === "undefined" ? undefined : window);

/** Avisa a la app (`auth-context`) que la sesión expiró, solo si el status lo indica. */
export function avisarSiSesionExpirada(status: number, destino: EventTarget | undefined = ventana()): void {
  if (sesionExpirada(status)) destino?.dispatchEvent(new Event(EVENTO_SESION_EXPIRADA));
}

/** Suscribe `alExpirar` al aviso de sesión expirada; devuelve la función para desuscribirse. */
export function escucharSesionExpirada(
  alExpirar: () => void,
  destino: EventTarget | undefined = ventana(),
): () => void {
  destino?.addEventListener(EVENTO_SESION_EXPIRADA, alExpirar);
  return () => destino?.removeEventListener(EVENTO_SESION_EXPIRADA, alExpirar);
}
