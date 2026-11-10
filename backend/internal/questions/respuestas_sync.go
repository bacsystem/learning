package questions

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// syncRespuestas updates the alternatives of a question IN PLACE so that the
// respuesta ids referenced by attempts (practica_respuestas,
// simulacro_respuesta_selecciones) survive an edit:
//
//   - items with an id update that alternative (it must belong to the question);
//   - if NO item carries an id (legacy payload), items are matched by position
//     with the existing alternatives ordered by orden;
//   - remaining items are inserted as new alternatives;
//   - existing alternatives left unmatched are deleted, unless an attempt
//     references them: then the whole update is rejected with ErrConflict (409).
//     Rejecting is safer than a soft delete: it needs no "deleted alternative"
//     filtering anywhere else. The ON DELETE RESTRICT FKs (migration 048) back
//     this up at the DB level;
//   - the key (which alternatives are correct) cannot change once an attempt
//     was graded with it: the stored verdict and the review, which reads the
//     current key, would contradict each other (H133). That is also ErrConflict.
//     Texts and order can still be edited.
func syncRespuestas(ctx context.Context, tx *sqlx.Tx, preguntaID int64, items []CreateRespuestaItem) error {
	rows, err := tx.QueryContext(ctx,
		`SELECT id, es_correcta FROM respuestas WHERE pregunta_id = $1 ORDER BY orden, id FOR UPDATE`, preguntaID)
	if err != nil {
		return err
	}
	var existing []int64
	oldKey := map[int64]bool{}
	for rows.Next() {
		var id int64
		var correcta bool
		if err := rows.Scan(&id, &correcta); err != nil {
			rows.Close()
			return err
		}
		existing = append(existing, id)
		if correcta {
			oldKey[id] = true
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	belongs := map[int64]bool{}
	for _, id := range existing {
		belongs[id] = true
	}
	anyID := false
	for _, it := range items {
		if it.ID != nil {
			anyID = true
			break
		}
	}

	target := make([]*int64, len(items)) // nil → insert
	used := map[int64]bool{}
	for i, it := range items {
		switch {
		case it.ID != nil:
			if !belongs[*it.ID] {
				return fmt.Errorf("%w: la respuesta %d no pertenece a la pregunta", ErrValidation, *it.ID)
			}
			if used[*it.ID] {
				return fmt.Errorf("%w: la respuesta %d está repetida", ErrValidation, *it.ID)
			}
			id := *it.ID
			target[i] = &id
			used[id] = true
		case !anyID && i < len(existing):
			id := existing[i]
			target[i] = &id
			used[id] = true
		}
	}

	if keyChanged(oldKey, items, target) {
		var graded bool
		if err := tx.QueryRowContext(ctx, `
			SELECT EXISTS (SELECT 1 FROM practica_respuestas WHERE pregunta_id = $1 AND es_correcta IS NOT NULL)
			    OR EXISTS (SELECT 1 FROM simulacro_respuestas WHERE pregunta_id = $1 AND es_correcta IS NOT NULL)`,
			preguntaID,
		).Scan(&graded); err != nil {
			return err
		}
		if graded {
			return fmt.Errorf("%w: %s", ErrConflict, msgClaveConIntentos)
		}
	}

	var removed []int64
	for _, id := range existing {
		if !used[id] {
			removed = append(removed, id)
		}
	}
	if len(removed) > 0 {
		var referenced bool
		if err := tx.QueryRowContext(ctx, `
			SELECT EXISTS (SELECT 1 FROM practica_respuestas WHERE respuesta_id = ANY($1))
			    OR EXISTS (SELECT 1 FROM simulacro_respuesta_selecciones WHERE respuesta_id = ANY($1))`,
			pq.Array(removed),
		).Scan(&referenced); err != nil {
			return err
		}
		if referenced {
			return fmt.Errorf("%w: %s", ErrConflict, msgAlternativaConIntentos)
		}
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM respuestas WHERE pregunta_id = $1 AND id = ANY($2)`, preguntaID, pq.Array(removed)); err != nil {
			return err
		}
	}

	for i, it := range items {
		if target[i] != nil {
			if _, err := tx.ExecContext(ctx,
				`UPDATE respuestas SET texto = $1, es_correcta = $2, orden = $3 WHERE id = $4 AND pregunta_id = $5`,
				it.Texto, it.EsCorrecta, it.Orden, *target[i], preguntaID); err != nil {
				return err
			}
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO respuestas (pregunta_id, texto, es_correcta, orden) VALUES ($1, $2, $3, $4)`,
			preguntaID, it.Texto, it.EsCorrecta, it.Orden); err != nil {
			return err
		}
	}
	return nil
}

// keyChanged reports whether applying items changes which alternatives are
// correct: an existing one flips, a correct one is removed, or a new correct
// one is inserted.
func keyChanged(oldKey map[int64]bool, items []CreateRespuestaItem, target []*int64) bool {
	newKey := 0
	for i, it := range items {
		if !it.EsCorrecta {
			continue
		}
		if target[i] == nil || !oldKey[*target[i]] {
			return true
		}
		newKey++
	}
	return newKey != len(oldKey)
}
