package questions

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// Citantes tells the fuentes module which preguntas cite it (contract E3-02
// §5). It satisfies fuentes.Citantes without importing that package.
type Citantes struct{}

// ContarPorFuente counts the aprobada or activo preguntas with a citation to
// the fuente: exactly the ones its retirement sends to en_revision.
func (Citantes) ContarPorFuente(ctx context.Context, q sqlx.QueryerContext, fuenteID int64) (int, error) {
	var n int
	err := sqlx.GetContext(ctx, q, &n, `
		SELECT COUNT(*) FROM preguntas p
		WHERE `+estadosCitantes+`
		  AND EXISTS (`+citaAFuente+`)`, fuenteID)
	if err != nil {
		return 0, fmt.Errorf("contar preguntas citantes de la fuente %d: %w", fuenteID, err)
	}
	return n, nil
}

// VersionCitada reports whether any pregunta, in any state, cites a pasaje of
// the version: its pasajes cannot be deleted while a citation points to them.
func (Citantes) VersionCitada(ctx context.Context, q sqlx.QueryerContext, versionID int64) (bool, error) {
	var citada bool
	err := sqlx.GetContext(ctx, q, &citada, `
		SELECT EXISTS (SELECT 1 FROM pregunta_citas pc
		               JOIN pasajes pa ON pa.id = pc.pasaje_id
		               WHERE pa.fuente_version_id = $1)`, versionID)
	if err != nil {
		return false, fmt.Errorf("comprobar citas de la versión %d: %w", versionID, err)
	}
	return citada, nil
}

// SuscriptorRetiro sends the aprobada and activo preguntas that cite a retired
// fuente to en_revision (contract E3-02 §5). It satisfies
// fuentes.SuscriptorRetiro without importing that package.
type SuscriptorRetiro struct{}

// Nombre is the subscriber's name in the logs.
func (SuscriptorRetiro) Nombre() string { return "preguntas" }

// FuenteRetirada runs inside the retirement's transaction, with the fuente
// already locked. Two statements per set, never a loop per pregunta; the
// audit rows keep the state before the change (the CTE reads it before the
// UPDATE) and the actor of the retirement. It returns how many preguntas it moved.
func (SuscriptorRetiro) FuenteRetirada(ctx context.Context, tx sqlx.ExtContext, fuenteID, actorID int64) (int, error) {
	res, err := tx.ExecContext(ctx, `
		WITH objetivo AS (
		    SELECT p.id, p.estado FROM preguntas p
		    WHERE `+estadosCitantes+`
		      AND EXISTS (`+citaAFuente+`)
		    ORDER BY p.id
		    FOR UPDATE OF p
		), afectadas AS (
		    UPDATE preguntas p SET estado = 'en_revision', updated_at = NOW()
		    FROM objetivo o WHERE p.id = o.id
		    RETURNING p.id, o.estado AS estado_anterior
		)
		INSERT INTO audit_log (usuario_id, accion, entidad, entidad_id, detalle)
		SELECT $2, 'pregunta.estado', 'pregunta', a.id,
		       jsonb_build_object('estado_anterior', a.estado_anterior, 'estado_nuevo', 'en_revision',
		                          'motivo', 'fuente retirada', 'fuente_id', $1::bigint)
		FROM afectadas a`, fuenteID, actorID)
	if err != nil {
		return 0, fmt.Errorf("enviar a revisión las preguntas de la fuente %d: %w", fuenteID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

// estadosCitantes is the filter of the preguntas a retirement affects:
// aprobada and the inherited activo (contract E3-02 §5).
const estadosCitantes = "p.estado IN ('aprobada', 'activo')"

// citaAFuente matches a pregunta p that cites any pasaje of the fuente $1.
const citaAFuente = `SELECT 1 FROM pregunta_citas pc
		JOIN pasajes pa ON pa.id = pc.pasaje_id
		JOIN fuente_versiones fv ON fv.id = pa.fuente_version_id
		WHERE pc.pregunta_id = p.id AND fv.fuente_id = $1`
