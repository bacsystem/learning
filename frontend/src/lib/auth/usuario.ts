/** Mismo formato que el backend (VerifyEmailRequest): alfanumérico, 4 a 30 caracteres. */
export const USUARIO_LONGITUD_MIN = 4;
export const USUARIO_LONGITUD_MAX = 30;
export const FORMATO_USUARIO = /^[a-zA-Z0-9]+$/;

/** Solo un usuario con formato válido se consulta y muestra como disponible. */
export function usuarioConsultable(usuario: string): boolean {
  return usuario.length >= USUARIO_LONGITUD_MIN && FORMATO_USUARIO.test(usuario);
}
