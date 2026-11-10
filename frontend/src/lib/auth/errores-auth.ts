/**
 * Traduce un error de las llamadas de auth a una clave i18n propia, por status HTTP: el `error` del backend
 * viene siempre en español y un fallo de red llega como «Failed to fetch», así que nunca se muestra tal cual.
 *
 * - `porStatus` sobrescribe la clave de un status concreto (los casos de negocio de cada página).
 * - Sin status y `TypeError` (lo que lanza `fetch` sin conexión, CORS o DNS) → error de red.
 * - 5xx → error del servidor; 400/422, 401, 403, 409 y 429 → su clave genérica.
 * - Cualquier otro caso → `fallback`, el mensaje genérico de la página.
 */
export function claveErrorAuth(
  err: unknown,
  fallback: string,
  porStatus: Partial<Record<number, string>> = {},
): string {
  const status = (err as { status?: unknown } | null)?.status;
  if (typeof status !== "number") return err instanceof TypeError ? "auth-error-red" : fallback;
  const propia = porStatus[status];
  if (propia) return propia;
  if (status >= 500) return "auth-error-servidor";
  switch (status) {
    case 400:
    case 422:
      return "auth-error-datos-invalidos";
    case 401:
      return "auth-error-no-autorizado";
    case 403:
      return "auth-error-acceso-denegado";
    case 409:
      return "auth-error-conflicto";
    case 429:
      return "auth-error-demasiados-intentos";
    default:
      return fallback;
  }
}
