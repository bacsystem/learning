package fuentes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

// reclamacionVencida is how long a claim protects a version: after it, another
// worker reclaims it and the attempt counts (ADR 0003 § Worker de ingesta).
const reclamacionVencida = 10 * time.Minute

// VersionReclamada is a 'procesando' version claimed by one worker. Intentos
// already counts this attempt.
type VersionReclamada struct {
	FuenteID  int64  `db:"fuente_id"`
	Version   int    `db:"numero"`
	Contenido []byte `db:"contenido"`
	Intentos  int    `db:"intentos"`
}

// PgCola is the ingest queue over fuente_versiones.
type PgCola struct {
	db *sqlx.DB
}

// NewPgCola creates the queue over db.
func NewPgCola(db *sqlx.DB) *PgCola {
	return &PgCola{db: db}
}

// Reclamar atomically claims the oldest 'procesando' version that is not
// claimed or whose claim expired, and counts the attempt. FOR UPDATE SKIP
// LOCKED keeps two replicas from claiming the same version. ok is false when
// there is nothing to process.
func (c *PgCola) Reclamar(ctx context.Context) (v VersionReclamada, ok bool, err error) {
	err = c.db.GetContext(ctx, &v, `
		UPDATE fuente_versiones SET claimed_at = clock_timestamp(), intentos = intentos + 1
		WHERE id = (SELECT id FROM fuente_versiones
		            WHERE estado = 'procesando'
		              AND (claimed_at IS NULL OR claimed_at < NOW() - $1 * INTERVAL '1 second')
		            ORDER BY id FOR UPDATE SKIP LOCKED LIMIT 1)
		RETURNING fuente_id, numero, contenido, intentos`, int64(reclamacionVencida/time.Second))
	if errors.Is(err, sql.ErrNoRows) {
		return VersionReclamada{}, false, nil
	}
	if err != nil {
		return VersionReclamada{}, false, fmt.Errorf("reclamar versión en proceso: %w", err)
	}
	return v, true, nil
}
