package questions

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// enTx runs fn in a transaction: commit on nil, rollback otherwise.
func (r *Repository) enTx(ctx context.Context, fn func(tx *sqlx.Tx) error) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("iniciar transacción: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("confirmar transacción: %w", err)
	}
	return nil
}

// bloquear takes the locks of contract E3-02 §4.1/§4.2 in the fixed order
// fuente → pregunta (the same as the retirement): the fuentes of the current
// citations plus extraFuentes, FOR SHARE ORDER BY id; then the pregunta FOR
// UPDATE; then it re-reads the citations. It answers ErrNotFound when the
// pregunta does not exist, and estado_invalido when its citations moved to a
// fuente outside the locked set. The pregunta may be 'eliminada' after the
// lock: each caller decides what that means.
func (r *Repository) bloquear(ctx context.Context, tx *sqlx.Tx, id int64, extraFuentes []int64) (preguntaBloqueada, error) {
	var estado string
	if err := tx.GetContext(ctx, &estado, `SELECT estado FROM preguntas WHERE id = $1`, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return preguntaBloqueada{}, ErrNotFound
		}
		return preguntaBloqueada{}, err
	}
	if estado == EstadoEliminada {
		return preguntaBloqueada{}, ErrNotFound
	}

	antes, err := r.citasDe(ctx, tx, id)
	if err != nil {
		return preguntaBloqueada{}, err
	}
	bloqueadas := unirFuentes(antes, extraFuentes)
	if _, err := tx.ExecContext(ctx,
		`SELECT id FROM fuentes WHERE id = ANY($1) ORDER BY id FOR SHARE`, pq.Array(bloqueadas)); err != nil {
		return preguntaBloqueada{}, err
	}

	b := preguntaBloqueada{}
	if err := tx.QueryRowContext(ctx,
		`SELECT estado, origen FROM preguntas WHERE id = $1 FOR UPDATE`, id).Scan(&b.Estado, &b.Origen); err != nil {
		return preguntaBloqueada{}, err
	}
	if b.Citas, err = r.citasDe(ctx, tx, id); err != nil {
		return preguntaBloqueada{}, err
	}
	for _, c := range b.Citas {
		if !contiene(bloqueadas, c.FuenteID) {
			return preguntaBloqueada{}, regla(CodigoEstadoInvalido, map[string]any{"estado_actual": b.Estado})
		}
	}
	return b, nil
}

// citasDe reads the citations of a pregunta with the state of their fuente.
func (r *Repository) citasDe(ctx context.Context, q sqlx.QueryerContext, id int64) ([]citaFila, error) {
	rows, err := q.QueryxContext(ctx, `
		SELECT pc.pasaje_id, fv.fuente_id, f.estado = 'activa'
		FROM pregunta_citas pc
		JOIN pasajes pa          ON pa.id = pc.pasaje_id
		JOIN fuente_versiones fv ON fv.id = pa.fuente_version_id
		JOIN fuentes f           ON f.id = fv.fuente_id
		WHERE pc.pregunta_id = $1
		ORDER BY pc.pasaje_id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var citas []citaFila
	for rows.Next() {
		var c citaFila
		if err := rows.Scan(&c.PasajeID, &c.FuenteID, &c.FuenteActiva); err != nil {
			return nil, err
		}
		citas = append(citas, c)
	}
	return citas, rows.Err()
}

// CambiarEstado locks the pregunta, lets decidir validate the transition
// against the state read under the lock, and writes the new state and its
// audit row in the same transaction.
func (r *Repository) CambiarEstado(ctx context.Context, id int64, decidir func(preguntaBloqueada) (cambioEstado, error), aud Auditor) error {
	return r.enTx(ctx, func(tx *sqlx.Tx) error {
		b, err := r.bloquear(ctx, tx, id, nil)
		if err != nil {
			return err
		}
		c, err := decidir(b)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE preguntas SET estado = $2, motivo_rechazo = $3, updated_at = NOW() WHERE id = $1`,
			id, c.Hacia, c.MotivoRechazo); err != nil {
			return err
		}
		return auditarEstado(ctx, tx, aud, id, c)
	})
}

// auditarEstado writes the pregunta.estado row of a transition (E3-07 §3.3).
func auditarEstado(ctx context.Context, tx *sqlx.Tx, aud Auditor, id int64, c cambioEstado) error {
	if aud == nil {
		return nil
	}
	detalle := map[string]any{"estado_anterior": c.Desde, "estado_nuevo": c.Hacia}
	for k, v := range c.Detalle {
		detalle[k] = v
	}
	return aud(ctx, tx, Evento{Accion: accionEstado, EntidadID: id, Detalle: detalle})
}

// unirFuentes returns the distinct fuente ids of citas and extra.
func unirFuentes(citas []citaFila, extra []int64) []int64 {
	vistas := map[int64]bool{}
	var out []int64
	add := func(id int64) {
		if !vistas[id] {
			vistas[id] = true
			out = append(out, id)
		}
	}
	for _, c := range citas {
		add(c.FuenteID)
	}
	for _, id := range extra {
		add(id)
	}
	return out
}

func contiene(ids []int64, id int64) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

// ReemplazarCitas replaces the citations of a pregunta (E3-07 §4). The fuentes
// of the requested pasajes are locked together with those of the current
// citations, so a fuente cannot be retired, nor its version archived, while the
// citation is inserted. decidir validates against the state read under the lock.
func (r *Repository) ReemplazarCitas(ctx context.Context, id int64, pedidos []int64, decidir func(preguntaBloqueada, map[int64]pasajeInfo) (planCitas, error), aud Auditor) error {
	return r.enTx(ctx, func(tx *sqlx.Tx) error {
		previas, err := r.infoPasajes(ctx, tx, pedidos)
		if err != nil {
			return err
		}
		extra := make([]int64, 0, len(previas))
		for _, p := range previas {
			extra = append(extra, p.FuenteID)
		}
		b, err := r.bloquear(ctx, tx, id, extra)
		if err != nil {
			return err
		}
		if b.Estado == EstadoEliminada {
			return ErrNotFound
		}
		// Read again under the lock: a version can be archived, and its pasajes
		// deleted, between the first read and the lock.
		info, err := r.infoPasajes(ctx, tx, pedidos)
		if err != nil {
			return err
		}
		plan, err := decidir(b, info)
		if err != nil {
			return err
		}
		if plan.sinCambios() {
			return nil
		}
		return r.escribirCitas(ctx, tx, id, plan, aud)
	})
}

// infoPasajes returns, for the pasajes that exist, their fuente and whether it is 'activa'.
func (r *Repository) infoPasajes(ctx context.Context, tx *sqlx.Tx, ids []int64) (map[int64]pasajeInfo, error) {
	rows, err := tx.QueryxContext(ctx, `
		SELECT pa.id, fv.fuente_id, f.estado = 'activa'
		FROM pasajes pa
		JOIN fuente_versiones fv ON fv.id = pa.fuente_version_id
		JOIN fuentes f           ON f.id = fv.fuente_id
		WHERE pa.id = ANY($1)`, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	info := make(map[int64]pasajeInfo, len(ids))
	for rows.Next() {
		var id int64
		var p pasajeInfo
		if err := rows.Scan(&id, &p.FuenteID, &p.FuenteActiva); err != nil {
			return nil, err
		}
		info[id] = p
	}
	return info, rows.Err()
}

func (r *Repository) escribirCitas(ctx context.Context, tx *sqlx.Tx, id int64, plan planCitas, aud Auditor) error {
	if len(plan.Quitar) > 0 {
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM pregunta_citas WHERE pregunta_id = $1 AND pasaje_id = ANY($2)`, id, pq.Array(plan.Quitar)); err != nil {
			return err
		}
	}
	if len(plan.Agregar) > 0 {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO pregunta_citas (pregunta_id, pasaje_id) SELECT $1, UNNEST($2::bigint[])`, id, pq.Array(plan.Agregar)); err != nil {
			return err
		}
	}
	if aud == nil {
		return nil
	}
	return aud(ctx, tx, Evento{Accion: accionCitas, EntidadID: id,
		Detalle: map[string]any{"antes": nonNil(plan.Antes), "despues": nonNil(plan.Despues)}})
}

// nonNil makes an empty list marshal as [] instead of null.
func nonNil(ids []int64) []int64 {
	if ids == nil {
		return []int64{}
	}
	return ids
}

// EliminarPregunta soft-deletes a pregunta from the states the state machine
// allows (E3-02 §3, P2), under the same lock as the other transitions, and
// audits it in the same transaction.
func (r *Repository) EliminarPregunta(ctx context.Context, id int64, aud Auditor) error {
	return r.enTx(ctx, func(tx *sqlx.Tx) error {
		b, err := r.bloquear(ctx, tx, id, nil)
		if err != nil {
			return err
		}
		if !transicionPermitida(b.Estado, EstadoEliminada) {
			return regla(CodigoEstadoInvalido, map[string]any{"estado_actual": b.Estado})
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE preguntas SET estado = 'eliminada', updated_at = NOW() WHERE id = $1`, id); err != nil {
			return err
		}
		return auditarEstado(ctx, tx, aud, id, cambioEstado{Desde: b.Estado, Hacia: EstadoEliminada})
	})
}
