/**
 * Rutas que se abren sin sesión: la landing (`/`) y todo `/auth/*` (sign-in, sign-up, enlaces de
 * verify-email y password-new, documentos legales…). Una sesión caducada no debe sacar al usuario de
 * ellas: en `/auth/password-new?token=…` la página ya quitó el token de la URL y redirigir perdería el enlace.
 */
export function esRutaPublica(pathname: string): boolean {
  return pathname === "/" || pathname === "/auth" || pathname.startsWith("/auth/");
}
