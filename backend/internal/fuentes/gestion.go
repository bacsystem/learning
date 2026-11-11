package fuentes

import (
	"context"
	"database/sql"
	"fmt"
	"slices"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// Audit actions of S5–S7 (contract S5, S6, S7).
const (
	AccionRetirar          = "fuente.retirar"
	AccionRestaurar        = "fuente.restaurar"
	AccionReintentar       = "fuente.reintentar"
	AccionMarcarOficial    = "fuente.marcar_oficial"
	AccionDesmarcarOficial = "fuente.desmarcar_oficial"
	AccionEditar           = "fuente.editar"
)

// Every change below runs under the fuentes lock and then locks the fuente
// (bloquearParaCambio): the same order as S1/S2, so admin changes never
// deadlock among themselves or with the worker (version → fuente).

// Retirar retires an active fuente without a version in process (S5). It
// stores the motivo, notifies every subscriber once inside the transaction
// (an error rolls everything back, named after the subscriber) and returns
// how many contents they marked. A failing subscriber is a SuscriptorError.
func (r *Repository) Retirar(ctx context.Context, id int64, motivo string, actor int64,
	suscriptores []SuscriptorRetiro, aud Auditor) (int, error) {
	marcados := 0
	err := r.enTxBloqueada(ctx, func(tx *sqlx.Tx) error {
		f, err := bloquearParaCambio(ctx, tx, id)
		if err != nil {
			return err
		}
		if f.Estado != EstadoActiva || f.Procesando {
			return ErrEstadoInvalido
		}
		if _, err := tx.ExecContext(ctx, `UPDATE fuentes SET estado = 'retirada', motivo_retiro = $2, updated_at = NOW() WHERE id = $1`,
			id, motivo); err != nil {
			return fmt.Errorf("retirar fuente %d: %w", id, err)
		}
		for _, s := range suscriptores {
			n, err := s.FuenteRetirada(ctx, tx, id, actor)
			if err != nil {
				return &SuscriptorError{Nombre: s.Nombre(), FuenteID: id, Causa: err}
			}
			marcados += n
		}
		return auditar(ctx, tx, aud, Evento{Accion: AccionRetirar, Entidad: EntidadFuente, EntidadID: id, Detalle: map[string]any{
			"estado_anterior": f.Estado, "estado_nuevo": EstadoRetirada, "motivo": motivo, "contenidos_marcados": marcados,
		}})
	})
	return marcados, err
}

// Restaurar makes a retired fuente active again, revalidating the fuentes
// limit; it approves nothing back (S5, regla 5).
func (r *Repository) Restaurar(ctx context.Context, id int64, aud Auditor) error {
	return r.enTxBloqueada(ctx, func(tx *sqlx.Tx) error {
		f, err := bloquearParaCambio(ctx, tx, id)
		if err != nil {
			return err
		}
		if f.Estado != EstadoRetirada {
			return ErrEstadoInvalido
		}
		if err := comprobarLimiteDe(ctx, tx, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE fuentes SET estado = 'activa', motivo_retiro = NULL, updated_at = NOW() WHERE id = $1`,
			id); err != nil {
			return fmt.Errorf("restaurar fuente %d: %w", id, err)
		}
		return auditar(ctx, tx, aud, Evento{Accion: AccionRestaurar, Entidad: EntidadFuente, EntidadID: id, Detalle: map[string]any{
			"estado_anterior": f.Estado, "estado_nuevo": EstadoActiva,
		}})
	})
}

// Reintentar sends the last version (the highest number; S2 only archives
// right before inserting a newer one, so it is never archived), when it
// failed, back to the worker as new: 'procesando' with no failure, attempts,
// claim or processing date. A
// 'fallida' fuente becomes 'procesando' after the fuentes limit; an 'activa'
// one keeps its state (it already counts) and its current version (S5).
func (r *Repository) Reintentar(ctx context.Context, id int64, aud Auditor) error {
	return r.enTxBloqueada(ctx, func(tx *sqlx.Tx) error {
		f, err := bloquearParaCambio(ctx, tx, id)
		if err != nil {
			return err
		}
		if f.Estado != EstadoFallida && f.Estado != EstadoActiva {
			return ErrEstadoInvalido
		}
		var ultima struct {
			ID     int64  `db:"id"`
			Numero int    `db:"numero"`
			Estado string `db:"estado"`
		}
		if err := tx.GetContext(ctx, &ultima, `
			SELECT id, numero, estado FROM fuente_versiones WHERE fuente_id = $1 ORDER BY numero DESC LIMIT 1`, id); err != nil {
			return fmt.Errorf("última versión de la fuente %d: %w", id, err)
		}
		if ultima.Estado != EstadoFallida {
			return ErrEstadoInvalido
		}
		estadoNuevo := f.Estado
		if f.Estado == EstadoFallida {
			if err := comprobarLimiteDe(ctx, tx, id); err != nil {
				return err
			}
			estadoNuevo = EstadoProcesando
		}
		// estado and motivo_fallo change together (ck_fuente_versiones_fallo).
		if _, err := tx.ExecContext(ctx, `
			UPDATE fuente_versiones SET estado = 'procesando', motivo_fallo = NULL, intentos = 0, claimed_at = NULL, procesada_at = NULL
			WHERE id = $1`, ultima.ID); err != nil {
			return mapearVersion(fmt.Errorf("reintentar la versión %d de la fuente %d: %w", ultima.Numero, id, err))
		}
		if _, err := tx.ExecContext(ctx, `UPDATE fuentes SET estado = $2, updated_at = NOW() WHERE id = $1`, id, estadoNuevo); err != nil {
			return fmt.Errorf("reintentar fuente %d: %w", id, err)
		}
		return auditar(ctx, tx, aud, Evento{Accion: AccionReintentar, Entidad: EntidadFuente, EntidadID: id, Detalle: map[string]any{
			"estado_anterior": f.Estado, "estado_nuevo": estadoNuevo, "version": ultima.Numero,
		}})
	})
}

// marcaVigente is the «oficial verificada» mark of the current version.
type marcaVigente struct {
	ID      int64          `db:"id"`
	Numero  int            `db:"numero"`
	Marcada bool           `db:"oficial_verificada"`
	Entidad sql.NullString `db:"oficial_entidad"`
}

func leerMarcaVigente(ctx context.Context, tx *sqlx.Tx, vigenteID int64) (marcaVigente, error) {
	var m marcaVigente
	if err := tx.GetContext(ctx, &m, `SELECT id, numero, oficial_verificada, oficial_entidad FROM fuente_versiones WHERE id = $1`,
		vigenteID); err != nil {
		return marcaVigente{}, fmt.Errorf("versión vigente %d: %w", vigenteID, err)
	}
	return m, nil
}

// MarcarOficial marks the current version of an active fuente as «oficial
// verificada» with its entidad and nota; a marked one is ErrEstadoInvalido (S6).
func (r *Repository) MarcarOficial(ctx context.Context, id int64, entidad, nota string, actor int64, aud Auditor) error {
	return r.enTxBloqueada(ctx, func(tx *sqlx.Tx) error {
		f, err := bloquearParaCambio(ctx, tx, id)
		if err != nil {
			return err
		}
		if f.Estado != EstadoActiva {
			return ErrEstadoInvalido
		}
		m, err := leerMarcaVigente(ctx, tx, f.VigenteID.Int64)
		if err != nil {
			return err
		}
		if m.Marcada {
			return ErrEstadoInvalido
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE fuente_versiones SET oficial_verificada = true, oficial_entidad = $2, oficial_nota = $3, oficial_por = $4, oficial_at = NOW()
			WHERE id = $1`, m.ID, entidad, nota, actor); err != nil {
			return fmt.Errorf("marcar la versión %d de la fuente %d: %w", m.Numero, id, err)
		}
		if err := tocarFuente(ctx, tx, id); err != nil {
			return err
		}
		return auditar(ctx, tx, aud, Evento{Accion: AccionMarcarOficial, Entidad: EntidadFuente, EntidadID: id, Detalle: map[string]any{
			"version": m.Numero, "entidad_emisora": entidad, "nota": nota,
		}})
	})
}

// DesmarcarOficial removes the mark of the current version of an active or
// retired fuente; the history stays in audit_log, with the entidad it had (S6).
func (r *Repository) DesmarcarOficial(ctx context.Context, id int64, motivo string, aud Auditor) error {
	return r.enTxBloqueada(ctx, func(tx *sqlx.Tx) error {
		f, err := bloquearParaCambio(ctx, tx, id)
		if err != nil {
			return err
		}
		if f.Estado != EstadoActiva && f.Estado != EstadoRetirada {
			return ErrEstadoInvalido
		}
		m, err := leerMarcaVigente(ctx, tx, f.VigenteID.Int64)
		if err != nil {
			return err
		}
		if !m.Marcada {
			return ErrEstadoInvalido
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE fuente_versiones SET oficial_verificada = false, oficial_entidad = NULL, oficial_nota = NULL, oficial_por = NULL, oficial_at = NULL
			WHERE id = $1`, m.ID); err != nil {
			return fmt.Errorf("desmarcar la versión %d de la fuente %d: %w", m.Numero, id, err)
		}
		if err := tocarFuente(ctx, tx, id); err != nil {
			return err
		}
		return auditar(ctx, tx, aud, Evento{Accion: AccionDesmarcarOficial, Entidad: EntidadFuente, EntidadID: id, Detalle: map[string]any{
			"version": m.Numero, "entidad_emisora": m.Entidad.String, "motivo": motivo,
		}})
	})
}

// Editar replaces the title and the especialidades without a new version
// (S7), in the contract's order after the 404: references (an inactive one
// is kept only if the fuente already had it), state, and the fuentes limit
// when the fuente counts (activa), counting it once with its new set.
func (r *Repository) Editar(ctx context.Context, id int64, titulo string, especialidades []int64, aud Auditor) error {
	return r.enTxBloqueada(ctx, func(tx *sqlx.Tx) error {
		f, err := bloquearParaCambio(ctx, tx, id)
		if err != nil {
			return err
		}
		var antes struct {
			Titulo string `db:"titulo"`
		}
		if err := tx.GetContext(ctx, &antes, `SELECT titulo FROM fuentes WHERE id = $1`, id); err != nil {
			return fmt.Errorf("leer fuente %d: %w", id, err)
		}
		actuales, err := especialidadesDe(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := comprobarEspecialidadesEditables(ctx, tx, especialidades, actuales); err != nil {
			return err
		}
		if f.Estado == EstadoProcesando {
			return ErrEstadoInvalido
		}
		if f.Estado == EstadoActiva {
			if err := comprobarLimite(ctx, tx, especialidades, id); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE fuentes SET titulo = $2, updated_at = NOW() WHERE id = $1`, id, titulo); err != nil {
			return fmt.Errorf("editar fuente %d: %w", id, err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM fuente_especialidades WHERE fuente_id = $1`, id); err != nil {
			return fmt.Errorf("borrar especialidades de la fuente %d: %w", id, err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO fuente_especialidades (fuente_id, especialidad_id) SELECT $1, unnest($2::bigint[])`,
			id, pq.Array(especialidades)); err != nil {
			return fmt.Errorf("especialidades de la fuente %d: %w", id, err)
		}
		// Sorted like especialidadesDe, so antes and despues compare; Clone
		// keeps an empty set as [] in the audit, not null.
		despues := slices.Clone(especialidades)
		slices.Sort(despues)
		return auditar(ctx, tx, aud, Evento{Accion: AccionEditar, Entidad: EntidadFuente, EntidadID: id, Detalle: map[string]any{
			"antes":   map[string]any{"titulo": antes.Titulo, "especialidad_ids": actuales},
			"despues": map[string]any{"titulo": titulo, "especialidad_ids": despues},
		}})
	})
}

// comprobarEspecialidadesEditables: each one exists and is active, or the
// fuente already had it (contract S7 regla 2, as temas.Editar).
func comprobarEspecialidadesEditables(ctx context.Context, tx *sqlx.Tx, especialidades, actuales []int64) error {
	if len(especialidades) == 0 {
		return nil
	}
	var validas int
	if err := tx.GetContext(ctx, &validas, `
		SELECT COUNT(*) FROM especialidades WHERE id = ANY($1::bigint[]) AND (estado = 'activo' OR id = ANY($2::bigint[]))`,
		pq.Array(especialidades), pq.Array(actuales)); err != nil {
		return fmt.Errorf("validar especialidades: %w", err)
	}
	if validas != len(especialidades) {
		return ErrReferenciaInvalida
	}
	return nil
}

// comprobarLimiteDe checks the fuentes limit for a fuente that does not count
// yet (retirada, fallida), with its own especialidades.
func comprobarLimiteDe(ctx context.Context, tx *sqlx.Tx, id int64) error {
	especialidades, err := especialidadesDe(ctx, tx, id)
	if err != nil {
		return err
	}
	return comprobarLimite(ctx, tx, especialidades, sinExcluir)
}

// tocarFuente moves the fuente up in the listing (updated_at), which also
// tells the UI's refresh that its row changed.
func tocarFuente(ctx context.Context, tx *sqlx.Tx, id int64) error {
	if _, err := tx.ExecContext(ctx, `UPDATE fuentes SET updated_at = NOW() WHERE id = $1`, id); err != nil {
		return fmt.Errorf("actualizar fuente %d: %w", id, err)
	}
	return nil
}
