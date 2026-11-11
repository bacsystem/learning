package fuentes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/cbaciliod/learning/internal/texto"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// constraintSHA256 is the global duplicate index of migration 062.
const constraintSHA256 = "uq_fuente_versiones_sha256"

// PgSource is the own SourceRepository adapter over PostgreSQL (ADR 0003).
// Errors never carry the query or the passages' text, only ids.
type PgSource struct {
	db *sqlx.DB
}

// NewPgSource creates the adapter over db.
func NewPgSource(db *sqlx.DB) *PgSource {
	return &PgSource{db: db}
}

var _ SourceRepository = (*PgSource)(nil)

// GuardarFuente creates the fuente and its version 1, both 'procesando', and
// writes fuente.crear in the same transaction. The duplicate by hash and the
// fuentes limit are added by E2-03 inside this transaction; a 23505 on the
// hash index still maps to ErrFuenteDuplicada.
func (s *PgSource) GuardarFuente(ctx context.Context, f NuevaFuente, aud Auditor) (int64, int, error) {
	const primera = 1
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("iniciar transacción: %w", err)
	}
	defer tx.Rollback()

	var id int64
	if err := tx.GetContext(ctx, &id, `
		INSERT INTO fuentes (titulo, tipo, anio, proceso_id, creado_por) VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		f.Titulo, f.Tipo, f.Anio, f.ProcesoID, f.CreadoPor); err != nil {
		return 0, 0, fmt.Errorf("crear fuente: %w", err)
	}
	if len(f.EspecialidadIDs) > 0 {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO fuente_especialidades (fuente_id, especialidad_id) SELECT $1, unnest($2::bigint[])`,
			id, pq.Array(f.EspecialidadIDs)); err != nil {
			return 0, 0, fmt.Errorf("especialidades de la fuente %d: %w", id, err)
		}
	}
	sha, tamano, lineas := Huella(f.Contenido)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO fuente_versiones (fuente_id, numero, nombre_archivo, sha256, tamano_bytes, lineas, contenido, creado_por)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		id, primera, f.NombreArchivo, sha, tamano, lineas, string(f.Contenido), f.CreadoPor); err != nil {
		return 0, 0, mapearDuplicada(fmt.Errorf("crear versión 1 de la fuente %d: %w", id, err))
	}
	especialidades := f.EspecialidadIDs
	if especialidades == nil {
		especialidades = []int64{}
	}
	if err := auditar(ctx, tx, aud, Evento{Accion: AccionCrear, Entidad: EntidadFuente, EntidadID: id, Detalle: map[string]any{
		"version": primera, "sha256": sha, "tamano_bytes": tamano, "tipo": f.Tipo,
		"especialidad_ids": especialidades, "estado_nuevo": EstadoProcesando,
	}}); err != nil {
		return 0, 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, 0, mapearDuplicada(fmt.Errorf("confirmar fuente: %w", err))
	}
	return id, primera, nil
}

// ListarFuentes returns one page of fuentes by updated_at then id, newest
// first, with their especialidad ids and current version number.
func (s *PgSource) ListarFuentes(ctx context.Context, f FiltroFuentes) (Pagina[FuenteResumen], error) {
	page, perPage := NormalizarPagina(f.Page, f.PerPage)
	const where = `
		WHERE ($1::bigint = 0
		       OR NOT EXISTS (SELECT 1 FROM fuente_especialidades fe WHERE fe.fuente_id = f.id)
		       OR EXISTS (SELECT 1 FROM fuente_especialidades fe WHERE fe.fuente_id = f.id AND fe.especialidad_id = $1))
		  AND ($2 = '' OR f.estado = $2) AND ($3 = '' OR f.tipo = $3)`
	args := []any{f.EspecialidadID, f.Estado, f.Tipo}
	res := Pagina[FuenteResumen]{Page: page, PerPage: perPage, Data: []FuenteResumen{}}
	if err := sqlx.GetContext(ctx, s.db, &res.Total, `SELECT COUNT(*) FROM fuentes f`+where, args...); err != nil {
		return Pagina[FuenteResumen]{}, fmt.Errorf("contar fuentes: %w", err)
	}
	var filas []struct {
		ID              int64         `db:"id"`
		Titulo          string        `db:"titulo"`
		Tipo            string        `db:"tipo"`
		Estado          string        `db:"estado"`
		Anio            *int          `db:"anio"`
		ProcesoID       *int64        `db:"proceso_id"`
		VersionVigente  *int          `db:"version_vigente"`
		EspecialidadIDs pq.Int64Array `db:"especialidad_ids"`
		CreatedAt       time.Time     `db:"created_at"`
		UpdatedAt       time.Time     `db:"updated_at"`
	}
	if err := sqlx.SelectContext(ctx, s.db, &filas, `
		SELECT f.id, f.titulo, f.tipo, f.estado, f.anio, f.proceso_id, v.numero AS version_vigente,
		       COALESCE((SELECT array_agg(fe.especialidad_id ORDER BY fe.especialidad_id)
		                 FROM fuente_especialidades fe WHERE fe.fuente_id = f.id), '{}') AS especialidad_ids,
		       f.created_at, f.updated_at
		FROM fuentes f LEFT JOIN fuente_versiones v ON v.id = f.version_vigente_id`+where+`
		ORDER BY f.updated_at DESC, f.id DESC LIMIT $4 OFFSET $5`,
		append(args, perPage, (page-1)*perPage)...); err != nil {
		return Pagina[FuenteResumen]{}, fmt.Errorf("listar fuentes: %w", err)
	}
	for _, r := range filas {
		res.Data = append(res.Data, FuenteResumen{ID: r.ID, Titulo: r.Titulo, Tipo: r.Tipo, Estado: r.Estado,
			Anio: r.Anio, ProcesoID: r.ProcesoID, EspecialidadIDs: []int64(r.EspecialidadIDs),
			VersionVigente: r.VersionVigente, CreatedAt: r.CreatedAt.UTC(), UpdatedAt: r.UpdatedAt.UTC()})
	}
	return res, nil
}

// ObtenerPasajes returns one page of the passages of a version, by orden.
func (s *PgSource) ObtenerPasajes(ctx context.Context, fuenteID int64, version int, page, perPage int) (Pagina[Pasaje], error) {
	page, perPage = NormalizarPagina(page, perPage)
	versionID, err := idVersion(ctx, s.db, fuenteID, version)
	if err != nil {
		return Pagina[Pasaje]{}, err
	}
	res := Pagina[Pasaje]{Page: page, PerPage: perPage, Data: []Pasaje{}}
	if err := sqlx.GetContext(ctx, s.db, &res.Total, `SELECT COUNT(*) FROM pasajes WHERE fuente_version_id = $1`, versionID); err != nil {
		return Pagina[Pasaje]{}, fmt.Errorf("contar pasajes de la versión %d: %w", versionID, err)
	}
	var filas []filaPasaje
	if err := sqlx.SelectContext(ctx, s.db, &filas, `
		SELECT `+columnasPasaje+` FROM pasajes p
		JOIN fuente_versiones v ON v.id = p.fuente_version_id
		WHERE p.fuente_version_id = $1 ORDER BY p.orden LIMIT $2 OFFSET $3`,
		versionID, perPage, (page-1)*perPage); err != nil {
		return Pagina[Pasaje]{}, fmt.Errorf("leer pasajes de la versión %d: %w", versionID, err)
	}
	for _, f := range filas {
		res.Data = append(res.Data, f.pasaje())
	}
	return res, nil
}

// BuscarPasajes runs the full-text search of ADR 0003 on the folded query:
// only the current version of 'activa' fuentes of the especialidad or of
// every especialidad, by ts_rank_cd then id. A query that folds to nothing
// searchable (only stopwords) returns an empty list.
func (s *PgSource) BuscarPasajes(ctx context.Context, b Busqueda) ([]PasajeEncontrado, error) {
	consulta, limite, err := NormalizarBusqueda(b)
	if err != nil {
		return nil, err
	}
	var filas []filaPasaje
	if err := sqlx.SelectContext(ctx, s.db, &filas, `
		WITH q AS (SELECT websearch_to_tsquery('spanish', $1) AS q)
		SELECT `+columnasPasaje+`, ts_rank_cd(p.tsv, q.q) AS rank
		FROM q
		JOIN pasajes p          ON p.tsv @@ q.q
		JOIN fuente_versiones v ON v.id = p.fuente_version_id
		JOIN fuentes f          ON f.version_vigente_id = v.id
		WHERE f.estado = 'activa'
		  AND (NOT EXISTS (SELECT 1 FROM fuente_especialidades fe WHERE fe.fuente_id = f.id)
		       OR EXISTS (SELECT 1 FROM fuente_especialidades fe
		                  WHERE fe.fuente_id = f.id AND fe.especialidad_id = $2))
		ORDER BY rank DESC, p.id ASC
		LIMIT $3`, texto.Plegar(consulta), b.EspecialidadID, limite); err != nil {
		return nil, fmt.Errorf("buscar pasajes (especialidad %d): %w", b.EspecialidadID, err)
	}
	encontrados := make([]PasajeEncontrado, 0, len(filas))
	for _, f := range filas {
		encontrados = append(encontrados, f.pasaje())
	}
	return encontrados, nil
}

// ResolverCita returns the passage of (fuente, versión, pasaje) whatever the
// fuente's state; a tuple that does not match is ErrCitaInvalida.
func (s *PgSource) ResolverCita(ctx context.Context, c Cita) (PasajeCitado, error) {
	var fila struct {
		filaPasaje
		TituloFuente      string `db:"titulo_fuente"`
		EstadoFuente      string `db:"estado_fuente"`
		OficialVerificada bool   `db:"oficial_verificada"`
	}
	err := sqlx.GetContext(ctx, s.db, &fila, `
		SELECT `+columnasPasaje+`, f.titulo AS titulo_fuente, f.estado AS estado_fuente, v.oficial_verificada
		FROM pasajes p
		JOIN fuente_versiones v ON v.id = p.fuente_version_id
		JOIN fuentes f          ON f.id = v.fuente_id
		WHERE p.id = $1 AND v.numero = $2 AND v.fuente_id = $3`, c.PasajeID, c.Version, c.FuenteID)
	if errors.Is(err, sql.ErrNoRows) {
		return PasajeCitado{}, ErrCitaInvalida
	}
	if err != nil {
		return PasajeCitado{}, fmt.Errorf("resolver cita (fuente %d, versión %d, pasaje %d): %w", c.FuenteID, c.Version, c.PasajeID, err)
	}
	return PasajeCitado{Pasaje: fila.pasaje(), TituloFuente: fila.TituloFuente,
		EstadoFuente: fila.EstadoFuente, OficialVerificada: fila.OficialVerificada}, nil
}

// PublicarVersion inserts every passage with its folded tsv, marks the
// version 'lista', makes it the fuente's current version (a retired fuente
// stays retired) and writes fuente.procesar, all in one transaction.
func (s *PgSource) PublicarVersion(ctx context.Context, fuenteID int64, version int, pasajes []PasajeNuevo, aud Auditor) error {
	if err := ValidarPasajes(pasajes); err != nil {
		return err
	}
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("iniciar transacción: %w", err)
	}
	defer tx.Rollback()
	versionID, err := versionEnProceso(ctx, tx, fuenteID, version)
	if err != nil {
		return err
	}
	if err := insertarPasajes(ctx, tx, versionID, pasajes); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE fuente_versiones SET estado = 'lista', pasajes_count = $2, procesada_at = NOW() WHERE id = $1`,
		versionID, len(pasajes)); err != nil {
		return fmt.Errorf("marcar lista la versión %d: %w", versionID, err)
	}
	var cambio struct {
		Anterior string `db:"anterior"`
		Nuevo    string `db:"nuevo"`
	}
	if err := tx.GetContext(ctx, &cambio, `
		UPDATE fuentes f SET version_vigente_id = $2, updated_at = NOW(),
		       estado = CASE WHEN f.estado = 'retirada' THEN 'retirada' ELSE 'activa' END
		FROM (SELECT estado FROM fuentes WHERE id = $1) AS antes
		WHERE f.id = $1
		RETURNING antes.estado AS anterior, f.estado AS nuevo`, fuenteID, versionID); err != nil {
		return fmt.Errorf("publicar la versión %d de la fuente %d: %w", version, fuenteID, err)
	}
	if err := auditar(ctx, tx, aud, Evento{Accion: AccionProcesar, Entidad: EntidadFuente, EntidadID: fuenteID, Detalle: map[string]any{
		"version": version, "pasajes_count": len(pasajes), "estado_anterior": cambio.Anterior, "estado_nuevo": cambio.Nuevo,
	}}); err != nil {
		return err
	}
	return tx.Commit()
}

// MarcarVersionFallida marks a 'procesando' version failed and writes
// fuente.fallar. A fuente without a current version becomes 'fallida'; one
// with a current version keeps its state.
func (s *PgSource) MarcarVersionFallida(ctx context.Context, fuenteID int64, version int, motivo string, aud Auditor) error {
	if !MotivoValido(motivo) {
		return ErrPublicacionInvalida
	}
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("iniciar transacción: %w", err)
	}
	defer tx.Rollback()
	versionID, err := versionEnProceso(ctx, tx, fuenteID, version)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE fuente_versiones SET estado = 'fallida', motivo_fallo = $2, procesada_at = NOW() WHERE id = $1`,
		versionID, motivo); err != nil {
		return fmt.Errorf("marcar fallida la versión %d: %w", versionID, err)
	}
	var cambio struct {
		Anterior string `db:"anterior"`
		Nuevo    string `db:"nuevo"`
	}
	if err := tx.GetContext(ctx, &cambio, `
		UPDATE fuentes f SET updated_at = NOW(),
		       estado = CASE WHEN f.version_vigente_id IS NULL THEN 'fallida' ELSE f.estado END
		FROM (SELECT estado FROM fuentes WHERE id = $1) AS antes
		WHERE f.id = $1
		RETURNING antes.estado AS anterior, f.estado AS nuevo`, fuenteID); err != nil {
		return fmt.Errorf("marcar fallida la fuente %d: %w", fuenteID, err)
	}
	if err := auditar(ctx, tx, aud, Evento{Accion: AccionFallar, Entidad: EntidadFuente, EntidadID: fuenteID, Detalle: map[string]any{
		"version": version, "motivo_fallo": motivo, "estado_anterior": cambio.Anterior, "estado_nuevo": cambio.Nuevo,
	}}); err != nil {
		return err
	}
	return tx.Commit()
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// auditar calls aud when there is one (a nil Auditor audits nothing).
func auditar(ctx context.Context, tx *sqlx.Tx, aud Auditor, ev Evento) error {
	if aud == nil {
		return nil
	}
	if err := aud(ctx, tx, ev); err != nil {
		return fmt.Errorf("auditar %s de la fuente %d: %w", ev.Accion, ev.EntidadID, err)
	}
	return nil
}

const columnasPasaje = `v.fuente_id, v.numero, p.id, p.orden, p.linea_inicio, p.linea_fin, p.seccion, p.texto`

type filaPasaje struct {
	FuenteID    int64          `db:"fuente_id"`
	Numero      int            `db:"numero"`
	ID          int64          `db:"id"`
	Orden       int            `db:"orden"`
	LineaInicio int            `db:"linea_inicio"`
	LineaFin    int            `db:"linea_fin"`
	Seccion     sql.NullString `db:"seccion"`
	Texto       string         `db:"texto"`
	Rank        float64        `db:"rank"`
}

func (f filaPasaje) pasaje() Pasaje {
	p := Pasaje{FuenteID: f.FuenteID, Version: f.Numero, PasajeID: f.ID, Orden: f.Orden,
		LineaInicio: f.LineaInicio, LineaFin: f.LineaFin, Texto: f.Texto}
	if f.Seccion.Valid {
		s := f.Seccion.String
		p.Seccion = &s
	}
	return p
}

func idVersion(ctx context.Context, q sqlx.QueryerContext, fuenteID int64, version int) (int64, error) {
	var id int64
	err := sqlx.GetContext(ctx, q, &id, `SELECT id FROM fuente_versiones WHERE fuente_id = $1 AND numero = $2`, fuenteID, version)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNoEncontrado
	}
	if err != nil {
		return 0, fmt.Errorf("buscar versión %d de la fuente %d: %w", version, fuenteID, err)
	}
	return id, nil
}

// versionEnProceso locks the version if it is 'procesando'; otherwise
// ErrNoEncontrado (there is nothing to publish or fail).
func versionEnProceso(ctx context.Context, tx *sqlx.Tx, fuenteID int64, version int) (int64, error) {
	var id int64
	err := tx.GetContext(ctx, &id, `
		SELECT id FROM fuente_versiones WHERE fuente_id = $1 AND numero = $2 AND estado = 'procesando' FOR UPDATE`,
		fuenteID, version)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNoEncontrado
	}
	if err != nil {
		return 0, fmt.Errorf("bloquear versión %d de la fuente %d: %w", version, fuenteID, err)
	}
	return id, nil
}

// insertarPasajes writes every passage in one statement; tsv is computed from
// the text folded in Go (ADR 0003 § Recuperación, paso 2).
func insertarPasajes(ctx context.Context, tx *sqlx.Tx, versionID int64, pasajes []PasajeNuevo) error {
	n := len(pasajes)
	ordenes, inicios, fines, palabras := make([]int64, n), make([]int64, n), make([]int64, n), make([]int64, n)
	secciones := make([]sql.NullString, n)
	textos, plegados := make([]string, n), make([]string, n)
	for i, p := range pasajes {
		ordenes[i], inicios[i], fines[i], palabras[i] = int64(p.Orden), int64(p.LineaInicio), int64(p.LineaFin), int64(p.Palabras)
		if p.Seccion != nil {
			secciones[i] = sql.NullString{String: *p.Seccion, Valid: true}
		}
		textos[i], plegados[i] = p.Texto, texto.Plegar(p.Texto)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO pasajes (fuente_version_id, orden, linea_inicio, linea_fin, seccion, texto, palabras, tsv)
		SELECT $1, t.orden, t.inicio, t.fin, t.seccion, t.texto, t.palabras, to_tsvector('spanish', t.plegado)
		FROM unnest($2::int[], $3::int[], $4::int[], $5::text[], $6::text[], $7::int[], $8::text[])
		     AS t(orden, inicio, fin, seccion, texto, palabras, plegado)
		ORDER BY t.orden`,
		versionID, pq.Array(ordenes), pq.Array(inicios), pq.Array(fines), pq.Array(secciones),
		pq.Array(textos), pq.Array(palabras), pq.Array(plegados)); err != nil {
		return fmt.Errorf("insertar %d pasajes de la versión %d: %w", n, versionID, err)
	}
	return nil
}

func mapearDuplicada(err error) error {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23505" && pqErr.Constraint == constraintSHA256 {
		return ErrFuenteDuplicada
	}
	return err
}
