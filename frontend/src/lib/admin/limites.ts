/**
 * Largo máximo de los textos que el backend valida con `validate:"max=…"` (go-playground/validator cuenta runas del
 * texto tal como llega, sin recortar). Fuente: `backend/internal/catalogs/dto.go` (Create*Request, que también usan
 * los PUT), `backend/internal/questions/dto.go` (CreateRespuestaItem) y `backend/internal/auth/dto.go`
 * (UpdateUsuarioEstadoRequest). Si cambia allí, cambia aquí.
 */
export const LIMITES = {
  grado: { nombre: 100, abreviatura: 20 },
  especialidad: { nombre: 150, codigo: 20 },
  asignatura: { nombre: 100, codigo: 20 },
  proceso: { nombre: 150, descripcion: 500 },
  respuesta: { texto: 500 },
  observacionEstado: 500,
} as const;

/** Largo en runas (como `utf8.RuneCountInString` en Go), no en unidades UTF-16 como `String.length`. */
export const runas = (s: string | null | undefined) => Array.from(s ?? "").length;

/** El texto supera `max` runas y el backend lo rechazaría. */
export const excede = (s: string | null | undefined, max: number) => runas(s) > max;
