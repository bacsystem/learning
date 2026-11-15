import type { OpcionesError } from "@/lib/errores-api";

/**
 * Cambio de correo de un usuario (`PATCH /admin/usuarios/{id}/email`, auth/handler_admin.go): responde en el formato
 * heredado, sin `code`; 409 si el correo ya lo usa otra persona y 422 si no es válido.
 */
export const ERRORES_CORREO_USUARIO = {
  generico: "admin-error-actualizar-correo",
  porStatus: { 409: "admin-error-correo-en-uso", 422: "admin-error-correo-invalido" },
} satisfies OpcionesError;
