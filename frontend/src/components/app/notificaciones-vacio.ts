/**
 * Clave i18n del texto del historial de notificaciones vacío. Con la invitación a opinar pendiente (que la campana cuenta
 * en su insignia) se explica; sin ella, o sin saber aún si está pendiente, no se dice nada más.
 */
export function claveDetalleHistorialVacio(recordatorio: { resuelto: boolean; pendiente: boolean }): string | null {
  return recordatorio.resuelto && recordatorio.pendiente ? "notif-historial-vacio-invitacion" : null;
}
