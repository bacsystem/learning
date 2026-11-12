package questions

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// NuevaPregunta is one draft to insert in bulk (SIA-E3-03 §4.3). The lote
// always creates borradores: the review decides what gets served.
type NuevaPregunta struct {
	InstitucionID  int64
	ProcesoID      int64
	AsignaturaID   int64
	EspecialidadID *int64
	Enunciado      string
	Tipo           string
	Dificultad     string
	Origen         string // OrigenManual, OrigenBalotario or OrigenIA
	Respuestas     []CreateRespuestaItem
}

// InsertarLote inserts the preguntas and their respuestas inside the caller's
// transaction, with one statement per table whatever the size of the lote, and
// returns the new ids in the order of the lote. Unlike CreatePregunta it does
// not check the classification (proceso, asignatura, etiquetas): the caller
// has already done it for the whole lote. Each pregunta is validated here
// (validarNueva), so a lote can not leave a pregunta that cannot be served.
func InsertarLote(ctx context.Context, tx *sqlx.Tx, lote []NuevaPregunta) ([]int64, error) {
	if len(lote) == 0 {
		return []int64{}, nil
	}
	for i, p := range lote {
		if err := validarNueva(p); err != nil {
			return nil, fmt.Errorf("pregunta %d del lote: %w", i+1, err)
		}
	}
	ids, err := reservarIDs(ctx, tx, len(lote))
	if err != nil {
		return nil, err
	}
	if err := insertarPreguntas(ctx, tx, ids, lote); err != nil {
		return nil, err
	}
	if err := insertarRespuestas(ctx, tx, ids, lote); err != nil {
		return nil, err
	}
	return ids, nil
}

// Limits of the columns respuestas.texto (VARCHAR(500)).
const maxTextoRespuesta = 500

// validarNueva checks what the schema does not: the statement and the answers
// are not empty, the order is 1, 2, 3… without repeats and the key matches
// the tipo. The caller validates the classification.
func validarNueva(p NuevaPregunta) error {
	if strings.TrimSpace(p.Enunciado) == "" {
		return fmt.Errorf("%w: el enunciado está vacío", ErrValidation)
	}
	if err := ValidateRespuestas(p.Tipo, p.Respuestas); err != nil {
		return err
	}
	ordenes := make(map[int]bool, len(p.Respuestas))
	for _, r := range p.Respuestas {
		if strings.TrimSpace(r.Texto) == "" || utf8.RuneCountInString(r.Texto) > maxTextoRespuesta {
			return fmt.Errorf("%w: el texto de una respuesta está vacío o supera %d caracteres", ErrValidation, maxTextoRespuesta)
		}
		if r.Orden < 1 || r.Orden > len(p.Respuestas) || ordenes[r.Orden] {
			return fmt.Errorf("%w: el orden de las respuestas debe ser 1…n sin repetir", ErrValidation)
		}
		ordenes[r.Orden] = true
	}
	return nil
}

// reservarIDs takes n ids of the sequence at once, in ascending order.
func reservarIDs(ctx context.Context, tx *sqlx.Tx, n int) ([]int64, error) {
	var ids []int64
	err := tx.SelectContext(ctx, &ids,
		`SELECT nextval('preguntas_id_seq') AS id FROM generate_series(1, $1) ORDER BY id`, n)
	if err != nil {
		return nil, fmt.Errorf("reservar ids de preguntas: %w", err)
	}
	return ids, nil
}

func insertarPreguntas(ctx context.Context, tx *sqlx.Tx, ids []int64, lote []NuevaPregunta) error {
	var (
		instituciones, procesos, asignaturas []int64
		especialidades                       []sql.NullInt64
		enunciados, tipos, dificultades      []string
		origenes                             []string
	)
	for _, p := range lote {
		instituciones = append(instituciones, p.InstitucionID)
		procesos = append(procesos, p.ProcesoID)
		asignaturas = append(asignaturas, p.AsignaturaID)
		especialidades = append(especialidades, nullInt(p.EspecialidadID))
		enunciados = append(enunciados, p.Enunciado)
		tipos = append(tipos, p.Tipo)
		dificultades = append(dificultades, p.Dificultad)
		origenes = append(origenes, p.Origen)
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO preguntas (id, institucion_id, proceso_id, asignatura_id, especialidad_id,
			enunciado, tipo, dificultad, estado, origen)
		SELECT id, institucion_id, proceso_id, asignatura_id, especialidad_id,
			enunciado, tipo, dificultad, 'borrador', origen
		FROM unnest($1::bigint[], $2::bigint[], $3::bigint[], $4::bigint[], $5::bigint[],
			$6::text[], $7::text[], $8::text[], $9::text[])
			AS t(id, institucion_id, proceso_id, asignatura_id, especialidad_id,
				enunciado, tipo, dificultad, origen)`,
		pq.Array(ids), pq.Array(instituciones), pq.Array(procesos), pq.Array(asignaturas),
		pq.Array(especialidades), pq.Array(enunciados), pq.Array(tipos), pq.Array(dificultades), pq.Array(origenes))
	if err != nil {
		return fmt.Errorf("insertar preguntas del lote: %w", err)
	}
	return nil
}

func insertarRespuestas(ctx context.Context, tx *sqlx.Tx, ids []int64, lote []NuevaPregunta) error {
	var (
		preguntaIDs []int64
		textos      []string
		correctas   []bool
		ordenes     []int64
	)
	for i, p := range lote {
		for _, r := range p.Respuestas {
			preguntaIDs = append(preguntaIDs, ids[i])
			textos = append(textos, r.Texto)
			correctas = append(correctas, r.EsCorrecta)
			ordenes = append(ordenes, int64(r.Orden))
		}
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO respuestas (pregunta_id, texto, es_correcta, orden)
		SELECT pregunta_id, texto, es_correcta, orden
		FROM unnest($1::bigint[], $2::text[], $3::boolean[], $4::int[])
			AS t(pregunta_id, texto, es_correcta, orden)`,
		pq.Array(preguntaIDs), pq.Array(textos), pq.Array(correctas), pq.Array(ordenes))
	if err != nil {
		return fmt.Errorf("insertar respuestas del lote: %w", err)
	}
	return nil
}

func nullInt(v *int64) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *v, Valid: true}
}
