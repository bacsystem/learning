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

/**
 * Construye el `ApiError` de una respuesta no exitosa leyendo su cuerpo JSON (si lo hay). `extras` son campos numéricos
 * de primer nivel del cuerpo que el cliente necesita además (p. ej. `resultado_id` en el 409 `tiempo_agotado`).
 */
export async function apiErrorFrom<K extends string = never>(
  res: Response,
  extras: readonly K[] = [],
): Promise<ApiError & Partial<Record<K, number>>> {
  const raw: unknown = await res.json().catch(() => null);
  const body = parseErrorBody(raw);
  const record = (typeof raw === "object" && raw !== null ? raw : {}) as Record<string, unknown>;
  const campos = Object.fromEntries(extras.flatMap((k) => (typeof record[k] === "number" ? [[k, record[k]]] : [])));
  return Object.assign(
    new Error(body.message ?? body.error ?? res.statusText),
    { status: res.status, code: body.message !== undefined ? body.error : undefined, detalle: body.detalle },
    campos,
  ) as ApiError & Partial<Record<K, number>>;
}

/** Código estable del error (`tema_duplicado`, …) o `undefined` si la respuesta usa el formato heredado. */
export function apiErrorCode(err: unknown): string | undefined {
  const code = (err as { code?: unknown } | null)?.code;
  return typeof code === "string" ? code : undefined;
}

/**
 * Código de un 409 de práctica o simulacro (`tiempo_agotado`, `simulacro_en_curso`, …). Esos endpoints responden
 * `{"error": "<codigo>"}` (el código queda en `message`) y, en el formato 2, `{"error": "<codigo>", "message": "<texto>"}`
 * (el código queda en `code`): se leen los dos.
 */
export function codigoDeConflicto(err: unknown): string | undefined {
  const { status, code, message } = (err ?? {}) as { status?: unknown; code?: unknown; message?: unknown };
  if (status !== 409) return undefined;
  if (typeof code === "string") return code;
  return typeof message === "string" ? message : undefined;
}
