package fuentes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// Admin queries of the balotario import (SIA-E3-03 I3). They are outside of
// the Source port on purpose, like the admin search (ADR 0007): the port is
// what the IA consumes and must not expose the original content.

// VersionBalotario is a fuente with its current version, as the balotario
// import reads it. When the fuente has no current version yet (it is still
// 'procesando' or failed), VersionID is 0 and the version fields are empty.
type VersionBalotario struct {
	FuenteID          int64
	Titulo            string
	Tipo              string
	Estado            string
	VersionID         int64
	Numero            int
	Contenido         string
	OficialVerificada bool
	OficialEntidad    *string
	EspecialidadIDs   []int64 // empty = the fuente applies to every especialidad
}

// VersionVigenteBalotario reads the fuente, its current version with the
// original content and its especialidades. It answers ErrNoEncontrado if the
// fuente does not exist. Inside a transaction it sees what the transaction
// has locked.
//
// It does not filter by tipo or by estado: a guía or a retired fuente is
// returned as it is, and the caller (the preview and the confirmation of
// SIA-E3-03) validates both before using the content. The current version is
// the one the fuente points to, not the latest: while a newer version is
// 'procesando', the previous one stays current.
func VersionVigenteBalotario(ctx context.Context, q sqlx.QueryerContext, fuenteID int64) (VersionBalotario, error) {
	var fila struct {
		FuenteID          int64          `db:"id"`
		Titulo            string         `db:"titulo"`
		Tipo              string         `db:"tipo"`
		Estado            string         `db:"estado"`
		VersionID         sql.NullInt64  `db:"version_id"`
		Numero            sql.NullInt64  `db:"numero"`
		Contenido         sql.NullString `db:"contenido"`
		OficialVerificada sql.NullBool   `db:"oficial_verificada"`
		OficialEntidad    sql.NullString `db:"oficial_entidad"`
	}
	err := sqlx.GetContext(ctx, q, &fila, `
		SELECT f.id, f.titulo, f.tipo, f.estado, v.id AS version_id, v.numero, v.contenido,
		       v.oficial_verificada, v.oficial_entidad
		FROM fuentes f
		LEFT JOIN fuente_versiones v ON v.id = f.version_vigente_id
		WHERE f.id = $1`, fuenteID)
	if errors.Is(err, sql.ErrNoRows) {
		return VersionBalotario{}, ErrNoEncontrado
	}
	if err != nil {
		return VersionBalotario{}, fmt.Errorf("versión vigente de la fuente %d: %w", fuenteID, err)
	}
	especialidades, err := especialidadesDe(ctx, q, fuenteID)
	if err != nil {
		return VersionBalotario{}, err
	}
	out := VersionBalotario{FuenteID: fila.FuenteID, Titulo: fila.Titulo, Tipo: fila.Tipo, Estado: fila.Estado,
		VersionID: fila.VersionID.Int64, Numero: int(fila.Numero.Int64), Contenido: fila.Contenido.String,
		OficialVerificada: fila.OficialVerificada.Bool, EspecialidadIDs: especialidades}
	if fila.OficialEntidad.Valid {
		out.OficialEntidad = &fila.OficialEntidad.String
	}
	return out, nil
}

// RangoPasaje is the range of lines of a passage; the balotario import cites
// the passages that overlap the lines of each question (D8).
type RangoPasaje struct {
	PasajeID    int64 `db:"pasaje_id"`
	LineaInicio int   `db:"linea_inicio"`
	LineaFin    int   `db:"linea_fin"`
}

// RangosPasajes lists the passages of a version by orden, without their text.
func RangosPasajes(ctx context.Context, q sqlx.QueryerContext, fuenteVersionID int64) ([]RangoPasaje, error) {
	rangos := []RangoPasaje{}
	if err := sqlx.SelectContext(ctx, q, &rangos, `
		SELECT id AS pasaje_id, linea_inicio, linea_fin
		FROM pasajes WHERE fuente_version_id = $1 ORDER BY orden`, fuenteVersionID); err != nil {
		return nil, fmt.Errorf("rangos de pasajes de la versión %d: %w", fuenteVersionID, err)
	}
	return rangos, nil
}
