/**
 * Error de la API según docs/api/README.md §3. Convive con dos formatos de cuerpo:
 * - heredado `{"error": "<texto>"}`: `error` es el texto y no hay `code`;
 * - nuevo `{"error": "<codigo>", "message": "<texto>", "detalle"?: {…}}`: `error` es el código estable.
 */
export interface ApiError extends Error {
  status: number;
  code?: string;
  detalle?: Record<string, unknown>;
}

interface ErrorBody {
  error?: string;
  message?: string;
  detalle?: Record<string, unknown>;
}

/** Solo un texto no vacío cuenta: `""` se trata igual que un campo ausente. */
function stringOrUndefined(value: unknown): string | undefined {
  return typeof value === "string" && value !== "" ? value : undefined;
}

function parseErrorBody(raw: unknown): ErrorBody {
  if (typeof raw !== "object" || raw === null) return {};
  const record = raw as Record<string, unknown>;
  const detalle = record.detalle;
  return {
    error: stringOrUndefined(record.error),
    message: stringOrUndefined(record.message),
    detalle: typeof detalle === "object" && detalle !== null ? (detalle as Record<string, unknown>) : undefined,
  };
}

/** Construye el `ApiError` de una respuesta no exitosa leyendo su cuerpo JSON (si lo hay). */
export async function apiErrorFrom(res: Response): Promise<ApiError> {
  const body = parseErrorBody(await res.json().catch(() => null));
  return Object.assign(new Error(body.message ?? body.error ?? res.statusText), {
    status: res.status,
    code: body.message !== undefined ? body.error : undefined,
    detalle: body.detalle,
  });
}

/** Código estable del error (`tema_duplicado`, …) o `undefined` si la respuesta usa el formato heredado. */
export function apiErrorCode(err: unknown): string | undefined {
  const code = (err as { code?: unknown } | null)?.code;
  return typeof code === "string" ? code : undefined;
}
