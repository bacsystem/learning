// Package examen holds the data access shared by the two kinds of exam
// attempt, simulacros and prácticas (H39/H86): reading the alternatives of a
// question in a stable per-attempt order, the answer key, the feedback, and
// the access checks on a proceso. The grading rules themselves live in
// package grading; each attempt package keeps its own tables and queries.
package examen

import (
	"context"
	"database/sql"
	"errors"

	"github.com/cbaciliod/learning/internal/grading"
	"github.com/lib/pq"
)

// Querier is satisfied by *sqlx.DB, *sqlx.Tx, *sql.DB and *sql.Tx.
type Querier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// ErrProcesoNoEncontrado is returned by ProcesoNivel for an unknown proceso.
var ErrProcesoNoEncontrado = errors.New("proceso no encontrado")

// Alternativa is one alternative of a question.
type Alternativa struct {
	ID         int64
	Texto      string
	EsCorrecta bool
}

// LoadAlternativas returns the alternatives of the given questions in an order
// that is random-looking but STABLE for one attempt (H58): it is derived from
// md5(attempt id, respuesta id), so reloading never reshuffles a/b/c/d.
func LoadAlternativas(ctx context.Context, q Querier, attemptID int64, preguntaIDs []int64) (map[int64][]Alternativa, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT r.pregunta_id, r.id, r.texto, r.es_correcta
		FROM respuestas r
		WHERE r.pregunta_id = ANY($2)
		ORDER BY r.pregunta_id, md5($1::text || ':' || r.id::text), r.id`,
		attemptID, pq.Array(preguntaIDs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64][]Alternativa{}
	for rows.Next() {
		var pid int64
		var a Alternativa
		if err := rows.Scan(&pid, &a.ID, &a.Texto, &a.EsCorrecta); err != nil {
			return nil, err
		}
		out[pid] = append(out[pid], a)
	}
	return out, rows.Err()
}

// LoadClave returns pregunta_id → correct respuesta ids (ascending).
func LoadClave(ctx context.Context, q Querier, preguntaIDs []int64) (map[int64][]int64, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT pregunta_id, id FROM respuestas WHERE es_correcta AND pregunta_id = ANY($1) ORDER BY id`,
		pq.Array(preguntaIDs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	clave := map[int64][]int64{}
	for rows.Next() {
		var pid, rid int64
		if err := rows.Scan(&pid, &rid); err != nil {
			return nil, err
		}
		clave[pid] = append(clave[pid], rid)
	}
	return clave, rows.Err()
}

// Retro is the feedback of a question.
type Retro struct {
	Explicacion string
	Referencia  string
}

// LoadRetro returns pregunta_id → feedback (questions without one are absent).
func LoadRetro(ctx context.Context, q Querier, preguntaIDs []int64) (map[int64]Retro, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT pregunta_id, COALESCE(explicacion, ''), COALESCE(referencia, '')
		FROM pregunta_retroalimentacion WHERE pregunta_id = ANY($1)`, pq.Array(preguntaIDs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]Retro{}
	for rows.Next() {
		var pid int64
		var r Retro
		if err := rows.Scan(&pid, &r.Explicacion, &r.Referencia); err != nil {
			return nil, err
		}
		out[pid] = r
	}
	return out, rows.Err()
}

// Letra converts a 1-based order to "a", "b", "c", … ("?" out of range).
func Letra(orden int) string {
	if orden < 1 || orden > 26 {
		return "?"
	}
	return string(rune('a' + orden - 1))
}

// PreguntaIDs returns the ids of the grouped questions, in order.
func PreguntaIDs(ps []grading.Pregunta) []int64 {
	ids := make([]int64, len(ps))
	for i, p := range ps {
		ids[i] = p.PreguntaID
	}
	return ids
}

// IsUniqueViolation reports whether err is a unique violation of constraint.
func IsUniqueViolation(err error, constraint string) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "23505" && pqErr.Constraint == constraint
}

// ProcesoNivel returns the nivel assigned to the proceso ("" when none), or
// ErrProcesoNoEncontrado.
func ProcesoNivel(ctx context.Context, q Querier, procesoID int64) (string, error) {
	var nivel string
	err := q.QueryRowContext(ctx,
		`SELECT COALESCE(nivel, '') FROM procesos WHERE id = $1`, procesoID).Scan(&nivel)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrProcesoNoEncontrado
	}
	return nivel, err
}

// InscripcionAprobada reports whether the user has an approved inscripción
// for the proceso (required to start a simulacro or a práctica).
func InscripcionAprobada(ctx context.Context, q Querier, usuarioID, procesoID int64) (bool, error) {
	var ok bool
	err := q.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM inscripciones WHERE usuario_id=$1 AND proceso_id=$2 AND estado='aprobado')`,
		usuarioID, procesoID).Scan(&ok)
	return ok, err
}
