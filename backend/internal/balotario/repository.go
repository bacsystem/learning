package balotario

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/cbaciliod/learning/internal/fuentes"
	"github.com/jmoiron/sqlx"
)

// Repository reads from Postgres what the balotario import needs. The
// content of a fuente and its passages come from internal/fuentes (outside of
// its port, ADR 0007); the imports of balotario are its own.
type Repository struct{ db *sqlx.DB }

func NewRepository(db *sqlx.DB) *Repository { return &Repository{db: db} }

// registro is one row of balotario_importaciones with the number of its version.
type registro struct {
	ID        int64     `db:"id"`
	Version   int       `db:"version"`
	Creadas   int       `db:"creadas"`
	CreatedAt time.Time `db:"created_at"`
}

// lectura is a consistent snapshot of what the preview needs.
type lectura struct {
	Version       fuentes.VersionBalotario
	Rangos        []fuentes.RangoPasaje
	Importaciones []registro // from the most recent version to the oldest
}

// Leer reads the fuente, its current version, the ranges of its passages and
// its imports in one read-only snapshot. The ranges are only read when the
// fuente can be previewed (an active balotario): any other is refused before.
func (r *Repository) Leer(ctx context.Context, fuenteID int64) (lectura, error) {
	tx, err := r.db.BeginTxx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return lectura{}, err
	}
	defer tx.Rollback()

	v, err := fuentes.VersionVigenteBalotario(ctx, tx, fuenteID)
	if err != nil {
		return lectura{}, err
	}
	l := lectura{Version: v}
	if v.Tipo == "balotario" && v.Estado == "activa" && v.VersionID != 0 {
		if l.Rangos, err = fuentes.RangosPasajes(ctx, tx, v.VersionID); err != nil {
			return lectura{}, err
		}
	}
	if err := tx.SelectContext(ctx, &l.Importaciones, `
		SELECT bi.id, fv.numero AS version, bi.creadas, bi.created_at
		FROM balotario_importaciones bi
		JOIN fuente_versiones fv ON fv.id = bi.fuente_version_id
		WHERE bi.fuente_id = $1
		ORDER BY fv.numero DESC`, fuenteID); err != nil {
		return lectura{}, fmt.Errorf("importaciones de la fuente %d: %w", fuenteID, err)
	}
	return l, nil
}
