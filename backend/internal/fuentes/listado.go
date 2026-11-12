package fuentes

import (
	"context"
	"fmt"
	"time"

	"github.com/cbaciliod/learning/internal/db"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// The admin summary of a fuente (contract E2 § FuenteResumen) comes from one
// SQL shape shared by the listing S3 and the detail S4–S6, so the rule of
// version_en_proceso lives in one place: the newest version after the current
// one that is neither 'lista' nor 'archivada' (that is, 'procesando' or
// 'fallida'). Without a current version every version counts.
const (
	columnasResumen = `
		f.id, f.titulo, f.tipo, f.estado, f.created_at, f.updated_at,
		vig.numero AS vig_numero, vig.oficial_verificada AS vig_oficial, vig.pasajes_count AS vig_pasajes,
		ep.numero AS ep_numero, ep.estado AS ep_estado, ep.motivo_fallo AS ep_motivo`
	desdeResumen = `
		FROM fuentes f
		LEFT JOIN fuente_versiones vig ON vig.id = f.version_vigente_id
		LEFT JOIN LATERAL (
			SELECT v.numero, v.estado, v.motivo_fallo FROM fuente_versiones v
			WHERE v.fuente_id = f.id AND v.numero > COALESCE(vig.numero, 0) AND v.estado IN ('procesando', 'fallida')
			ORDER BY v.numero DESC LIMIT 1) ep ON true`
	// filtrosListado: $1 estado, $2 tipo, $3 especialidad (also the fuentes
	// for all), $4 mark of the current version (none counts as unmarked),
	// $5 title fragment with LIKE wildcards escaped.
	filtrosListado = `
		WHERE ($1 = '' OR f.estado = $1)
		  AND ($2 = '' OR f.tipo = $2)
		  AND ($3::bigint = 0
		       OR NOT EXISTS (SELECT 1 FROM fuente_especialidades fe WHERE fe.fuente_id = f.id)
		       OR EXISTS (SELECT 1 FROM fuente_especialidades fe WHERE fe.fuente_id = f.id AND fe.especialidad_id = $3))
		  AND ($4::boolean IS NULL OR COALESCE(vig.oficial_verificada, false) = $4)
		  AND ($5 = '' OR f.titulo ILIKE '%' || $5 || '%' ESCAPE '\')`
)

// filaResumen is one row of columnasResumen.
type filaResumen struct {
	ID         int64     `db:"id"`
	Titulo     string    `db:"titulo"`
	Tipo       string    `db:"tipo"`
	Estado     string    `db:"estado"`
	CreatedAt  time.Time `db:"created_at"`
	UpdatedAt  time.Time `db:"updated_at"`
	VigNumero  *int      `db:"vig_numero"`
	VigOficial *bool     `db:"vig_oficial"`
	VigPasajes *int      `db:"vig_pasajes"`
	EPNumero   *int      `db:"ep_numero"`
	EPEstado   *string   `db:"ep_estado"`
	EPMotivo   *string   `db:"ep_motivo"`
}

// resumen builds the summary; especialidades are filled by the caller.
func (f filaResumen) resumen(especialidades []EspecialidadRef) ResumenFuente {
	if especialidades == nil {
		especialidades = []EspecialidadRef{}
	}
	r := ResumenFuente{ID: f.ID, Titulo: f.Titulo, Tipo: f.Tipo, Estado: f.Estado, AplicaATodas: len(especialidades) == 0,
		Especialidades: especialidades, CreatedAt: f.CreatedAt.UTC(), UpdatedAt: f.UpdatedAt.UTC()}
	if f.VigNumero != nil {
		r.VersionVigente = &VersionVigenteResumen{Numero: *f.VigNumero, OficialVerificada: *f.VigOficial, PasajesCount: *f.VigPasajes}
	}
	if f.EPNumero != nil {
		r.VersionEnProceso = &VersionEnProceso{Numero: *f.EPNumero, Estado: *f.EPEstado, MotivoFallo: f.EPMotivo}
	}
	return r
}

// Listar returns one page of S3, newest change first: the count, the rows,
// and the especialidades of the whole page in one query (no N+1).
func (r *Repository) Listar(ctx context.Context, f FiltroListado) (ListaFuentes, error) {
	args := []any{f.Estado, f.Tipo, f.EspecialidadID, f.Oficial, db.EscapeLike(f.Q)}
	lista := ListaFuentes{Data: []ResumenFuente{}, Page: f.Page, PerPage: f.PerPage}
	if err := sqlx.GetContext(ctx, r.db, &lista.Total, `SELECT COUNT(*)`+desdeResumen+filtrosListado, args...); err != nil {
		return ListaFuentes{}, fmt.Errorf("contar fuentes: %w", err)
	}
	var filas []filaResumen
	if err := sqlx.SelectContext(ctx, r.db, &filas, `SELECT`+columnasResumen+desdeResumen+filtrosListado+`
		ORDER BY f.updated_at DESC, f.id DESC LIMIT $6 OFFSET $7`,
		append(args, f.PerPage, int64(f.Page-1)*int64(f.PerPage))...); err != nil {
		return ListaFuentes{}, fmt.Errorf("listar fuentes: %w", err)
	}
	ids := make([]int64, len(filas))
	for i, fila := range filas {
		ids[i] = fila.ID
	}
	especialidades, err := especialidadesConNombre(ctx, r.db, ids)
	if err != nil {
		return ListaFuentes{}, err
	}
	for _, fila := range filas {
		lista.Data = append(lista.Data, fila.resumen(especialidades[fila.ID]))
	}
	return lista, nil
}

// especialidadesConNombre returns the especialidades of each fuente, by name.
func especialidadesConNombre(ctx context.Context, q sqlx.QueryerContext, fuenteIDs []int64) (map[int64][]EspecialidadRef, error) {
	por := map[int64][]EspecialidadRef{}
	if len(fuenteIDs) == 0 {
		return por, nil
	}
	var filas []struct {
		FuenteID int64 `db:"fuente_id"`
		EspecialidadRef
	}
	if err := sqlx.SelectContext(ctx, q, &filas, `
		SELECT fe.fuente_id, e.id, e.nombre FROM fuente_especialidades fe JOIN especialidades e ON e.id = fe.especialidad_id
		WHERE fe.fuente_id = ANY($1::bigint[]) ORDER BY e.nombre, e.id`, pq.Array(fuenteIDs)); err != nil {
		return nil, fmt.Errorf("especialidades de %d fuentes: %w", len(fuenteIDs), err)
	}
	for _, f := range filas {
		por[f.FuenteID] = append(por[f.FuenteID], f.EspecialidadRef)
	}
	return por, nil
}
