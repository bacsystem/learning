package simulations

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/cbaciliod/learning/internal/catalogs"
	"github.com/cbaciliod/learning/internal/examen"
	"github.com/cbaciliod/learning/internal/grading"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

const (
	estadoEnCurso    = "en_curso"
	estadoCompletado = "completado"

	// uqEnCursoIndex is the unique partial index of migration 049.
	uqEnCursoIndex = "uq_simulacros_usuario_en_curso"
)

// Repository wraps the database connection for the simulations package.
type Repository struct {
	db *sqlx.DB
}

// NewRepository creates a new simulations Repository.
func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

// dbtx is satisfied by both *sqlx.DB and *sqlx.Tx.
type dbtx interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// graciaSeg is GraciaRespuesta in seconds, as passed to SQL.
var graciaSeg = GraciaRespuesta.Seconds()

// ── Internal row types ────────────────────────────────────────────────────────

type simulacroMetaRow struct {
	ID              int64
	UsuarioID       int64
	ProcesoID       int64
	ProcesoNombre   string
	TiempoLimiteSeg int
	FechaInicio     time.Time
	FechaFin        *time.Time
	PuntajeTotal    *float64
	PuntajeMaximo   *float64
	NotaCentesimal  *float64
	Aprobado        *bool
	Estado          string
	InstitucionID   int64
	NotaMinima      float64
	NotaMaxima      float64
	Expirado        bool // NOW() is past fecha_inicio + tiempo_limite + gracia
	TiempoRestante  int  // seconds left (0 when over), computed with the DB clock
}

type configuracionRow struct {
	ID             int64
	TiempoTotalSeg int
	TotalPreguntas int
	NotaMinima     float64
	NotaMaxima     float64
	Estado         string
	Descripcion    string
	ProcesoID      int64
}

// metaSelect is the single source of the simulacro metadata query (H39).
const metaSelect = `
	SELECT s.id, s.usuario_id, s.proceso_id, p.nombre, s.tiempo_limite_seg,
	       s.fecha_inicio, s.fecha_fin, s.puntaje_total, s.puntaje_maximo,
	       s.nota_centesimal, s.aprobado, s.estado, s.institucion_id,
	       s.nota_minima, s.nota_maxima,
	       NOW() > s.fecha_inicio + make_interval(secs => s.tiempo_limite_seg + $2::float8) AS expirado,
	       GREATEST(0, s.tiempo_limite_seg - EXTRACT(EPOCH FROM (NOW() - s.fecha_inicio)))::int AS restante
	FROM simulacros s
	JOIN procesos p ON p.id = s.proceso_id
	WHERE s.id = $1`

// getMeta loads the simulacro metadata and checks ownership (ErrNotFound for
// someone else's simulacro). forUpdate locks the simulacro row (use inside a tx).
func getMeta(ctx context.Context, q dbtx, id, usuarioID int64, forUpdate bool) (simulacroMetaRow, error) {
	query := metaSelect
	if forUpdate {
		query += " FOR UPDATE OF s"
	}
	var m simulacroMetaRow
	err := q.QueryRowContext(ctx, query, id, graciaSeg).Scan(
		&m.ID, &m.UsuarioID, &m.ProcesoID, &m.ProcesoNombre, &m.TiempoLimiteSeg,
		&m.FechaInicio, &m.FechaFin, &m.PuntajeTotal, &m.PuntajeMaximo,
		&m.NotaCentesimal, &m.Aprobado, &m.Estado, &m.InstitucionID,
		&m.NotaMinima, &m.NotaMaxima, &m.Expirado, &m.TiempoRestante,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return m, ErrNotFound
	}
	if err != nil {
		return m, err
	}
	if m.UsuarioID != usuarioID {
		return m, ErrNotFound
	}
	return m, nil
}

// ── Answer rows ───────────────────────────────────────────────────────────────

// preguntaRow is one question of a simulacro with its stored selection rows.
type preguntaRow struct {
	Orden      int
	Enunciado  string
	Tipo       string
	Asignatura string
}

// loadFilas returns the answer rows of a simulacro (one per selected
// alternative, or a single row with nil RespuestaID when unanswered) plus the
// per-question display info, ordered by orden. Rows are grouped by the
// asignatura the question had when the simulacro was created (H73), not by
// its current one.
func loadFilas(ctx context.Context, q dbtx, simulacroID int64) ([]grading.Fila, map[int64]preguntaRow, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT srr.pregunta_id, srr.asignatura_id, sel.respuesta_id, srr.es_correcta,
		       srr.orden, pq.enunciado, pq.tipo, a.nombre
		FROM simulacro_respuestas srr
		JOIN preguntas pq ON pq.id = srr.pregunta_id
		JOIN asignaturas a ON a.id = srr.asignatura_id
		LEFT JOIN simulacro_respuesta_selecciones sel
		       ON sel.simulacro_id = srr.simulacro_id AND sel.pregunta_id = srr.pregunta_id
		WHERE srr.simulacro_id = $1
		ORDER BY srr.orden, sel.respuesta_id`, simulacroID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var filas []grading.Fila
	info := map[int64]preguntaRow{}
	for rows.Next() {
		var f grading.Fila
		var p preguntaRow
		if err := rows.Scan(&f.PreguntaID, &f.AsignaturaID, &f.RespuestaID, &f.EsCorrecta,
			&p.Orden, &p.Enunciado, &p.Tipo, &p.Asignatura); err != nil {
			return nil, nil, err
		}
		filas = append(filas, f)
		info[f.PreguntaID] = p
	}
	return filas, info, rows.Err()
}

// ── GetSimulacroEnCurso ───────────────────────────────────────────────────────

// GetSimulacroEnCurso returns the ID of the user's en_curso simulacro (nil if
// none) and whether its time is already over.
func (r *Repository) GetSimulacroEnCurso(ctx context.Context, usuarioID int64) (*int64, bool, error) {
	var id int64
	var expirado bool
	err := r.db.QueryRowContext(ctx, `
		SELECT id, NOW() > fecha_inicio + make_interval(secs => tiempo_limite_seg + $2::float8)
		FROM simulacros WHERE usuario_id=$1 AND estado='en_curso'
		ORDER BY created_at DESC LIMIT 1`, usuarioID, graciaSeg,
	).Scan(&id, &expirado)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return &id, expirado, nil
}

// ── Configuración ─────────────────────────────────────────────────────────────

const configSelect = `
	SELECT ce.id, ce.proceso_id, p.nombre, ce.tiempo_total_seg, ce.total_preguntas,
	       ce.nota_minima, ce.nota_maxima, COALESCE(ce.descripcion,''), ce.estado
	FROM configuracion_examen ce
	JOIN procesos p ON p.id = ce.proceso_id`

func scanConfig(sc interface{ Scan(...any) error }) (ConfiguracionResponse, error) {
	var c ConfiguracionResponse
	err := sc.Scan(&c.ID, &c.ProcesoID, &c.Proceso, &c.TiempoTotalSeg, &c.TotalPreguntas,
		&c.NotaMinima, &c.NotaMaxima, &c.Descripcion, &c.Estado)
	return c, err
}

// GetConfiguracion returns the active configuracion_examen for the proceso.
func (r *Repository) GetConfiguracion(ctx context.Context, procesoID int64) (*configuracionRow, error) {
	var row configuracionRow
	err := r.db.QueryRowContext(ctx,
		`SELECT id, tiempo_total_seg, total_preguntas, nota_minima, nota_maxima
		 FROM configuracion_examen
		 WHERE proceso_id=$1 AND estado='activo'
		 ORDER BY created_at DESC LIMIT 1`, procesoID,
	).Scan(&row.ID, &row.TiempoTotalSeg, &row.TotalPreguntas, &row.NotaMinima, &row.NotaMaxima)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// GetConfiguracionByID returns one configuracion_examen by id.
func (r *Repository) GetConfiguracionByID(ctx context.Context, id int64) (ConfiguracionResponse, error) {
	c, err := scanConfig(r.db.QueryRowContext(ctx, configSelect+` WHERE ce.id = $1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return ConfiguracionResponse{}, ErrNotFound
	}
	return c, err
}

// CreateConfiguracion inserts a new configuracion_examen and returns the created row.
func (r *Repository) CreateConfiguracion(ctx context.Context, req CreateConfiguracionRequest, notaMinima, notaMaxima float64) (ConfiguracionResponse, error) {
	var id int64
	if err := r.db.QueryRowContext(ctx,
		`INSERT INTO configuracion_examen (proceso_id, tiempo_total_seg, total_preguntas, nota_minima, nota_maxima, descripcion, estado)
		 VALUES ($1, $2, $3, $4, $5, $6, 'activo') RETURNING id`,
		req.ProcesoID, req.TiempoTotalSeg, req.TotalPreguntas, notaMinima, notaMaxima, req.Descripcion,
	).Scan(&id); err != nil {
		return ConfiguracionResponse{}, err
	}
	return r.GetConfiguracionByID(ctx, id)
}

// UpdateConfiguracion overwrites a configuracion_examen with the merged values.
func (r *Repository) UpdateConfiguracion(ctx context.Context, c ConfiguracionResponse) (ConfiguracionResponse, error) {
	res, err := r.db.ExecContext(ctx,
		`UPDATE configuracion_examen
		 SET tiempo_total_seg=$2, total_preguntas=$3, nota_minima=$4, nota_maxima=$5,
		     descripcion=$6, estado=$7, updated_at=NOW()
		 WHERE id=$1`,
		c.ID, c.TiempoTotalSeg, c.TotalPreguntas, c.NotaMinima, c.NotaMaxima, c.Descripcion, c.Estado,
	)
	if err != nil {
		return ConfiguracionResponse{}, err
	}
	if n, err := res.RowsAffected(); err != nil {
		return ConfiguracionResponse{}, err
	} else if n == 0 {
		return ConfiguracionResponse{}, ErrNotFound
	}
	return r.GetConfiguracionByID(ctx, c.ID)
}

// GetConfiguracionDetalle returns the active config for a proceso with asignaturas and available
// question counts filtered by the user's grado/especialidad (pass 0 to skip filtering).
func (r *Repository) GetConfiguracionDetalle(ctx context.Context, procesoID, gradoID, especialidadID int64) (*ConfiguracionResponse, error) {
	resp, err := scanConfig(r.db.QueryRowContext(ctx,
		configSelect+` WHERE ce.proceso_id=$1 AND ce.estado='activo' ORDER BY ce.created_at DESC LIMIT 1`, procesoID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	aRows, err := r.db.QueryContext(ctx, `
		SELECT pa.asignatura_id, a.nombre, pa.numero_preguntas, pa.peso_porcentual,
		       (SELECT COUNT(*) FROM preguntas p
		        WHERE p.proceso_id = pa.proceso_id
		          AND p.asignatura_id = pa.asignatura_id
		          AND p.estado = 'activo'
		          AND ($2 = 0 OR p.grado_id = $2 OR p.grado_id IS NULL)
		          AND ($3::bigint = 0 OR p.especialidad_id = $3 OR p.especialidad_id IS NULL)
		       ) AS preguntas_disponibles
		FROM proceso_asignaturas pa
		JOIN asignaturas a ON a.id = pa.asignatura_id
		WHERE `+catalogs.ConjuntoDePesos("pa", 1, 3)+`
		ORDER BY a.nombre ASC`, procesoID, gradoID, especialidadID,
	)
	if err != nil {
		return nil, err
	}
	defer aRows.Close()

	resp.Asignaturas = []AsignaturaConfigItem{}
	for aRows.Next() {
		var item AsignaturaConfigItem
		if err := aRows.Scan(&item.AsignaturaID, &item.Nombre, &item.NumeroPreguntas,
			&item.PesoPorcentual, &item.PreguntasDisponibles); err != nil {
			return nil, err
		}
		resp.Asignaturas = append(resp.Asignaturas, item)
	}
	if err := aRows.Err(); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListConfiguraciones returns all active configuraciones with proceso names.
func (r *Repository) ListConfiguraciones(ctx context.Context) ([]ConfiguracionResponse, error) {
	rows, err := r.db.QueryContext(ctx, configSelect+` WHERE ce.estado='activo' ORDER BY p.nombre, ce.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []ConfiguracionResponse{}
	for rows.Next() {
		c, err := scanConfig(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

// ── GetProcesoAsignaturas ─────────────────────────────────────────────────────

// GetProcesoAsignaturas returns the asignaturas of the set that applies to
// the proceso and especialidad (0 = none): the especialidad's own set or the
// compatibility rows, never mixed, so each asignatura appears once (ADR 0001, L7).
func (r *Repository) GetProcesoAsignaturas(ctx context.Context, procesoID, especialidadID int64) ([]simulacroAsignaturaRow, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT pa.asignatura_id, a.nombre, pa.numero_preguntas, pa.peso_porcentual, pa.coeficiente
		 FROM proceso_asignaturas pa
		 JOIN asignaturas a ON a.id = pa.asignatura_id
		 WHERE `+catalogs.ConjuntoDePesos("pa", 1, 2)+`
		 ORDER BY pa.asignatura_id`, procesoID, especialidadID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []simulacroAsignaturaRow
	for rows.Next() {
		var row simulacroAsignaturaRow
		if err := rows.Scan(&row.AsignaturaID, &row.Nombre, &row.NumeroPreguntas, &row.Peso, &row.Coeficiente); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// ── CreateSimulacro ───────────────────────────────────────────────────────────

// CreateSimulacro inserts the simulacro, its question rows and the grading
// snapshot (asignaturas with the number of questions actually assigned, peso,
// coeficiente; nota_minima / nota_maxima) in one transaction, with batch
// inserts. Returns errEnCursoDuplicado when the user already has one en_curso.
func (r *Repository) CreateSimulacro(
	ctx context.Context,
	procesoID, usuarioID, institucionID int64,
	cfg configuracionRow,
	asignaturas []simulacroAsignaturaRow,
	preguntas []preguntaExamenItem,
) (int64, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var simulacroID int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO simulacros (usuario_id, institucion_id, proceso_id, configuracion_id, tiempo_limite_seg,
		                        nota_minima, nota_maxima)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`,
		usuarioID, institucionID, procesoID, cfg.ID, cfg.TiempoTotalSeg, cfg.NotaMinima, cfg.NotaMaxima,
	).Scan(&simulacroID)
	if examen.IsUniqueViolation(err, uqEnCursoIndex) {
		return 0, errEnCursoDuplicado
	}
	if err != nil {
		return 0, err
	}

	// The asignatura snapshot is the one each question was selected for, not
	// preguntas.asignatura_id: the selection ran outside this transaction and
	// an admin may have moved a question since (H110).
	ids := make([]int64, len(preguntas))
	qAsigs := make([]int64, len(preguntas))
	ordenes := make([]int64, len(preguntas))
	for i, p := range preguntas {
		ids[i], qAsigs[i], ordenes[i] = p.ID, p.AsignaturaID, int64(i+1)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO simulacro_respuestas (simulacro_id, pregunta_id, asignatura_id, orden)
		SELECT $1, t.pregunta_id, t.asignatura_id, t.orden
		FROM unnest($2::bigint[], $3::bigint[], $4::int[]) AS t(pregunta_id, asignatura_id, orden)`,
		simulacroID, pq.Array(ids), pq.Array(qAsigs), pq.Array(ordenes),
	); err != nil {
		return 0, err
	}

	asigIDs := make([]int64, len(asignaturas))
	pesos := make([]float64, len(asignaturas))
	coefs := make([]float64, len(asignaturas))
	for i, a := range asignaturas {
		asigIDs[i], pesos[i], coefs[i] = a.AsignaturaID, a.Peso, effectiveCoef(a.Coeficiente)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO simulacro_asignaturas (simulacro_id, asignatura_id, numero_preguntas, peso, coeficiente)
		SELECT $1, c.asignatura_id,
		       (SELECT COUNT(*) FROM simulacro_respuestas srr
		        WHERE srr.simulacro_id = $1 AND srr.asignatura_id = c.asignatura_id),
		       c.peso, c.coeficiente
		FROM unnest($2::bigint[], $3::numeric[], $4::numeric[]) AS c(asignatura_id, peso, coeficiente)`,
		simulacroID, pq.Array(asigIDs), pq.Array(pesos), pq.Array(coefs),
	); err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return simulacroID, nil
}

// ── GetSimulacro ──────────────────────────────────────────────────────────────

// GetSimulacro returns the en_curso simulacro for the given user.
// Returns ErrNotFound for someone else's simulacro, ErrSimulacroEstado when it
// is not en_curso and errExpirado when its time is over (the service then
// finalizes it).
func (r *Repository) GetSimulacro(ctx context.Context, id, usuarioID int64) (SimulacroEnCursoResponse, error) {
	meta, err := getMeta(ctx, r.db, id, usuarioID, false)
	if err != nil {
		return SimulacroEnCursoResponse{}, err
	}
	if meta.Estado != estadoEnCurso {
		return SimulacroEnCursoResponse{}, ErrSimulacroEstado
	}
	if meta.Expirado {
		return SimulacroEnCursoResponse{}, errExpirado
	}

	filas, info, err := loadFilas(ctx, r.db, id)
	if err != nil {
		return SimulacroEnCursoResponse{}, err
	}
	preguntas := grading.Agrupar(filas)
	alts, err := examen.LoadAlternativas(ctx, r.db, id, examen.PreguntaIDs(preguntas))
	if err != nil {
		return SimulacroEnCursoResponse{}, err
	}

	items := make([]PreguntaSimulacroItem, 0, len(preguntas))
	for _, p := range preguntas {
		pi := info[p.PreguntaID]
		resps := make([]RespuestaSimulacroItem, 0, len(alts[p.PreguntaID]))
		for j, a := range alts[p.PreguntaID] {
			resps = append(resps, RespuestaSimulacroItem{ID: a.ID, Texto: a.Texto, Letra: examen.Letra(j + 1), Orden: j + 1})
		}
		item := PreguntaSimulacroItem{
			ID:           p.PreguntaID,
			Orden:        pi.Orden,
			Enunciado:    pi.Enunciado,
			Tipo:         pi.Tipo,
			Asignatura:   pi.Asignatura,
			Respuestas:   resps,
			Estado:       "sin_responder",
			RespuestaIDs: append([]int64{}, p.Seleccion...),
		}
		if p.Respondida {
			item.Estado = "respondida"
			first := p.Seleccion[0]
			item.RespuestaID = &first
		}
		items = append(items, item)
	}

	return SimulacroEnCursoResponse{
		ID:              meta.ID,
		Proceso:         meta.ProcesoNombre,
		TiempoLimiteSeg: meta.TiempoLimiteSeg,
		FechaInicio:     meta.FechaInicio.Format(time.RFC3339),
		TiempoRestante:  meta.TiempoRestante,
		TotalPreguntas:  len(items),
		Preguntas:       items,
	}, nil
}

// ── Selección de respuestas ───────────────────────────────────────────────────

// selectionFn computes the new selection of a question from its tipo and the
// current selection.
type selectionFn func(tipo string, actual []int64) ([]int64, error)

// guardarSeleccion replaces the selection of one question atomically: the
// simulacro row is locked (serializing with other answers and with
// finalize), the state and time limit are enforced server-side (H8), and every
// selected id must belong to the question (H5).
func (r *Repository) guardarSeleccion(ctx context.Context, simulacroID, preguntaID, usuarioID int64, next selectionFn) ([]int64, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	meta, err := getMeta(ctx, tx, simulacroID, usuarioID, true)
	if err != nil {
		return nil, err
	}
	if meta.Estado != estadoEnCurso {
		return nil, ErrSimulacroEstado
	}
	if meta.Expirado {
		return nil, ErrTiempoAgotado
	}

	var tipo string
	err = tx.QueryRowContext(ctx, `
		SELECT pq.tipo FROM simulacro_respuestas srr JOIN preguntas pq ON pq.id = srr.pregunta_id
		WHERE srr.simulacro_id = $1 AND srr.pregunta_id = $2`, simulacroID, preguntaID,
	).Scan(&tipo)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	actual, err := querySeleccion(ctx, tx, simulacroID, preguntaID)
	if err != nil {
		return nil, err
	}
	ids, err := next(tipo, actual)
	if err != nil {
		return nil, err
	}
	ids = dedupeSorted(ids)
	if tipo == "unica" && len(ids) > 1 {
		return nil, fmt.Errorf("%w: una pregunta de respuesta única admite una sola selección", ErrValidation)
	}
	if len(ids) > 0 {
		var n int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM respuestas WHERE pregunta_id = $1 AND id = ANY($2)`,
			preguntaID, pq.Array(ids)).Scan(&n); err != nil {
			return nil, err
		}
		if n != len(ids) {
			return nil, fmt.Errorf("%w: la respuesta no pertenece a la pregunta", ErrValidation)
		}
	}

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM simulacro_respuesta_selecciones WHERE simulacro_id = $1 AND pregunta_id = $2`,
		simulacroID, preguntaID); err != nil {
		return nil, err
	}
	if len(ids) > 0 {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO simulacro_respuesta_selecciones (simulacro_id, pregunta_id, respuesta_id)
			SELECT $1, $2, unnest($3::bigint[])`, simulacroID, preguntaID, pq.Array(ids)); err != nil {
			return nil, err
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE simulacro_respuestas SET respondida_at = CASE WHEN $3 THEN NOW() END
		WHERE simulacro_id = $1 AND pregunta_id = $2`, simulacroID, preguntaID, len(ids) > 0); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return ids, nil
}

func querySeleccion(ctx context.Context, q dbtx, simulacroID, preguntaID int64) ([]int64, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT respuesta_id FROM simulacro_respuesta_selecciones
		WHERE simulacro_id = $1 AND pregunta_id = $2 ORDER BY respuesta_id`, simulacroID, preguntaID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func dedupeSorted(ids []int64) []int64 {
	out := make([]int64, 0, len(ids))
	seen := map[int64]bool{}
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// SetRespuestas replaces the whole selection of a question (idempotent).
func (r *Repository) SetRespuestas(ctx context.Context, simulacroID, preguntaID int64, ids []int64, usuarioID int64) ([]int64, error) {
	return r.guardarSeleccion(ctx, simulacroID, preguntaID, usuarioID, func(string, []int64) ([]int64, error) {
		return ids, nil
	})
}

// ResponderPregunta records a single respuesta: for 'unica' it replaces the
// selection; for 'multiple' it toggles that alternative.
func (r *Repository) ResponderPregunta(ctx context.Context, simulacroID, preguntaID, respuestaID, usuarioID int64) ([]int64, error) {
	return r.guardarSeleccion(ctx, simulacroID, preguntaID, usuarioID, func(tipo string, actual []int64) ([]int64, error) {
		if tipo == "unica" {
			return []int64{respuestaID}, nil
		}
		out := []int64{}
		found := false
		for _, id := range actual {
			if id == respuestaID {
				found = true
				continue
			}
			out = append(out, id)
		}
		if !found {
			out = append(out, respuestaID)
		}
		return out, nil
	})
}

// ── FinalizarSimulacro ────────────────────────────────────────────────────────

// FinalizarSimulacro grades and closes the simulacro in ONE transaction with the
// simulacro row locked (H9): a concurrent or retried finalize waits, then sees
// estado != en_curso and gets ErrSimulacroEstado, so results and statistics
// are produced exactly once. fecha_fin is capped at fecha_inicio + tiempo
// límite and answers saved after the limit (+ gracia) are discarded (H8).
// Grading uses the snapshot taken at creation (H33).
// Returns the graded attempts for question statistics.
func (r *Repository) FinalizarSimulacro(ctx context.Context, id, usuarioID int64) ([]grading.Intento, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	meta, err := getMeta(ctx, tx, id, usuarioID, true)
	if err != nil {
		return nil, err
	}
	if meta.Estado != estadoEnCurso {
		return nil, ErrSimulacroEstado
	}

	// Discard answers saved after the limit (+ gracia).
	if _, err := tx.ExecContext(ctx, `
		WITH tardias AS (
			UPDATE simulacro_respuestas srr SET respondida_at = NULL
			FROM simulacros s
			WHERE s.id = srr.simulacro_id AND srr.simulacro_id = $1
			  AND srr.respondida_at > s.fecha_inicio + make_interval(secs => s.tiempo_limite_seg + $2::float8)
			RETURNING srr.pregunta_id
		)
		DELETE FROM simulacro_respuesta_selecciones sel
		USING tardias t
		WHERE sel.simulacro_id = $1 AND sel.pregunta_id = t.pregunta_id`, id, graciaSeg); err != nil {
		return nil, err
	}

	asignaturas, err := loadSnapshot(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	filas, _, err := loadFilas(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	preguntas := grading.Agrupar(filas)
	clave, err := examen.LoadClave(ctx, tx, examen.PreguntaIDs(preguntas))
	if err != nil {
		return nil, err
	}
	grading.Calificar(preguntas, clave)
	nota := CalcularNota(preguntas, asignaturas, meta.NotaMinima, meta.NotaMaxima)

	res, err := tx.ExecContext(ctx, `
		UPDATE simulacros
		SET estado='completado',
		    fecha_fin = LEAST(NOW(), fecha_inicio + make_interval(secs => tiempo_limite_seg)),
		    puntaje_total=$1, puntaje_maximo=$2, nota_centesimal=$3, aprobado=$4
		WHERE id=$5 AND estado='en_curso'`,
		nota.PuntajeTotal, nota.PuntajeMaximo, nota.NotaCentesimal, nota.Aprobado, id,
	)
	if err != nil {
		return nil, err
	}
	if n, err := res.RowsAffected(); err != nil {
		return nil, err
	} else if n == 0 {
		return nil, ErrSimulacroEstado
	}

	// Question-level correctness (NULL for unanswered questions).
	var pids []int64
	var oks []bool
	for _, p := range preguntas {
		if p.Respondida {
			pids = append(pids, p.PreguntaID)
			oks = append(oks, p.Correcta)
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE simulacro_respuestas srr SET es_correcta = v.correcta
		FROM unnest($2::bigint[], $3::bool[]) AS v(pregunta_id, correcta)
		WHERE srr.simulacro_id = $1 AND srr.pregunta_id = v.pregunta_id`,
		id, pq.Array(pids), pq.Array(oks)); err != nil {
		return nil, err
	}

	n := len(nota.PorAsignatura)
	asigIDs, totales, correctas, incorrectas, sinResp := make([]int64, n), make([]int64, n), make([]int64, n), make([]int64, n), make([]int64, n)
	puntajes, pesos := make([]float64, n), make([]float64, n)
	for i, a := range nota.PorAsignatura {
		asigIDs[i] = a.AsignaturaID
		totales[i] = int64(a.Conteo.Total)
		correctas[i] = int64(a.Conteo.Correctas)
		incorrectas[i] = int64(a.Conteo.Incorrectas)
		sinResp[i] = int64(a.Conteo.SinResponder)
		puntajes[i] = a.Puntaje
		pesos[i] = a.Peso
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO simulacro_resultados
		  (simulacro_id, asignatura_id, total_preguntas, correctas, incorrectas, sin_responder, puntaje, peso)
		SELECT $1, t.asignatura_id, t.total, t.correctas, t.incorrectas, t.sin_responder, t.puntaje, t.peso
		FROM unnest($2::bigint[], $3::int[], $4::int[], $5::int[], $6::int[], $7::numeric[], $8::numeric[])
		     AS t(asignatura_id, total, correctas, incorrectas, sin_responder, puntaje, peso)
		ON CONFLICT (simulacro_id, asignatura_id) DO UPDATE
		  SET total_preguntas=EXCLUDED.total_preguntas, correctas=EXCLUDED.correctas,
		      incorrectas=EXCLUDED.incorrectas, sin_responder=EXCLUDED.sin_responder,
		      puntaje=EXCLUDED.puntaje, peso=EXCLUDED.peso`,
		id, pq.Array(asigIDs), pq.Array(totales), pq.Array(correctas), pq.Array(incorrectas),
		pq.Array(sinResp), pq.Array(puntajes), pq.Array(pesos)); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return grading.Intentos(preguntas), nil
}

// loadSnapshot returns the asignaturas snapshot of a simulacro (migration 050).
func loadSnapshot(ctx context.Context, q dbtx, simulacroID int64) ([]simulacroAsignaturaRow, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT sa.asignatura_id, a.nombre, sa.numero_preguntas, sa.peso, sa.coeficiente
		FROM simulacro_asignaturas sa
		JOIN asignaturas a ON a.id = sa.asignatura_id
		WHERE sa.simulacro_id = $1
		ORDER BY sa.asignatura_id`, simulacroID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []simulacroAsignaturaRow
	for rows.Next() {
		var a simulacroAsignaturaRow
		if err := rows.Scan(&a.AsignaturaID, &a.Nombre, &a.NumeroPreguntas, &a.Peso, &a.Coeficiente); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// computePosicion ranks the user's nota among DISTINCT users that completed a
// simulacro of the same proceso (H57): nota_centesimal is comparable across
// attempts (puntaje_total is not, its maximum depends on the configuration),
// and a user counts once however many attempts they made.
func (r *Repository) computePosicion(ctx context.Context, procesoID, usuarioID int64, nota float64) (*PosicionEstimada, error) {
	var otros, mejores int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT usuario_id),
		       COUNT(DISTINCT usuario_id) FILTER (WHERE nota_centesimal > $1)
		FROM simulacros
		WHERE proceso_id = $2 AND estado = 'completado' AND usuario_id <> $3
		  AND nota_centesimal IS NOT NULL`,
		nota, procesoID, usuarioID,
	).Scan(&otros, &mejores)
	if err != nil {
		return nil, err
	}
	return &PosicionEstimada{Posicion: mejores + 1, TotalUsuarios: otros + 1}, nil
}

// ── GetResultado ──────────────────────────────────────────────────────────────

// GetResultado returns the result for a completed simulacro (also the response
// of finalize, so both always agree).
func (r *Repository) GetResultado(ctx context.Context, id, usuarioID int64) (SimulacroResultadoResponse, error) {
	meta, err := getMeta(ctx, r.db, id, usuarioID, false)
	if err != nil {
		return SimulacroResultadoResponse{}, err
	}
	if meta.Estado != estadoCompletado {
		return SimulacroResultadoResponse{}, ErrSimulacroEstado
	}

	asgRows, err := r.db.QueryContext(ctx, `
		SELECT a.nombre, sr.total_preguntas, sr.correctas,
		       sr.incorrectas, sr.sin_responder, sr.puntaje, sr.peso
		FROM simulacro_resultados sr
		JOIN asignaturas a ON a.id = sr.asignatura_id
		WHERE sr.simulacro_id = $1
		ORDER BY sr.asignatura_id`, id,
	)
	if err != nil {
		return SimulacroResultadoResponse{}, err
	}
	defer asgRows.Close()

	porAsignatura := []ResultadoAsignaturaSimulacro{}
	var total grading.Conteo
	for asgRows.Next() {
		var item ResultadoAsignaturaSimulacro
		if err := asgRows.Scan(&item.Asignatura, &item.TotalPreguntas,
			&item.Correctas, &item.Incorrectas, &item.SinResponder, &item.Puntaje, &item.Peso); err != nil {
			return SimulacroResultadoResponse{}, err
		}
		item.Porcentaje = grading.Porcentaje(item.Correctas, item.TotalPreguntas)
		total.Total += item.TotalPreguntas
		total.Correctas += item.Correctas
		total.Incorrectas += item.Incorrectas
		total.SinResponder += item.SinResponder
		porAsignatura = append(porAsignatura, item)
	}
	if err := asgRows.Err(); err != nil {
		return SimulacroResultadoResponse{}, err
	}

	deref := func(p *float64) float64 {
		if p == nil {
			return 0
		}
		return *p
	}
	notaCentesimal := deref(meta.NotaCentesimal)

	var tiempoUsado int
	var fechaFin string
	if meta.FechaFin != nil {
		tiempoUsado = int(meta.FechaFin.Sub(meta.FechaInicio).Seconds())
		fechaFin = meta.FechaFin.Format(time.RFC3339)
	}

	posicion, err := r.computePosicion(ctx, meta.ProcesoID, usuarioID, notaCentesimal)
	if err != nil {
		return SimulacroResultadoResponse{}, err
	}

	return SimulacroResultadoResponse{
		ID:              meta.ID,
		Proceso:         meta.ProcesoNombre,
		TotalPreguntas:  total.Total,
		Correctas:       total.Correctas,
		Incorrectas:     total.Incorrectas,
		SinResponder:    total.SinResponder,
		PuntajeTotal:    deref(meta.PuntajeTotal),
		PuntajeMaximo:   deref(meta.PuntajeMaximo),
		Porcentaje:      porcentajePonderado(deref(meta.PuntajeTotal), deref(meta.PuntajeMaximo)),
		NotaCentesimal:  notaCentesimal,
		Aprobado:        meta.Aprobado != nil && *meta.Aprobado,
		TiempoUsadoSeg:  tiempoUsado,
		TiempoLimiteSeg: meta.TiempoLimiteSeg,
		FechaInicio:     meta.FechaInicio.Format(time.RFC3339),
		FechaFin:        fechaFin,
		PorAsignatura:   porAsignatura,
		Posicion:        posicion,
	}, nil
}

// ── GetRevision ───────────────────────────────────────────────────────────────

// GetRevision returns the full revision for a completed simulacro, with every
// selected and every correct alternative (es_correcta IS included here).
func (r *Repository) GetRevision(ctx context.Context, id, usuarioID int64) (RevisionSimulacroResponse, error) {
	meta, err := getMeta(ctx, r.db, id, usuarioID, false)
	if err != nil {
		return RevisionSimulacroResponse{}, err
	}
	if meta.Estado != estadoCompletado {
		return RevisionSimulacroResponse{}, ErrSimulacroEstado
	}

	filas, info, err := loadFilas(ctx, r.db, id)
	if err != nil {
		return RevisionSimulacroResponse{}, err
	}
	preguntas := grading.Agrupar(filas)
	if len(preguntas) == 0 {
		return RevisionSimulacroResponse{ID: id, Preguntas: []RevisionPreguntaSimulacro{}}, nil
	}
	ids := examen.PreguntaIDs(preguntas)
	alts, err := examen.LoadAlternativas(ctx, r.db, id, ids)
	if err != nil {
		return RevisionSimulacroResponse{}, err
	}
	retros, err := examen.LoadRetro(ctx, r.db, ids)
	if err != nil {
		return RevisionSimulacroResponse{}, err
	}

	out := make([]RevisionPreguntaSimulacro, 0, len(preguntas))
	for _, p := range preguntas {
		pi := info[p.PreguntaID]
		item := RevisionPreguntaSimulacro{
			PreguntaID:           p.PreguntaID,
			Tipo:                 pi.Tipo,
			Orden:                pi.Orden,
			Enunciado:            pi.Enunciado,
			Asignatura:           pi.Asignatura,
			RespuestaUsuarioIDs:  append([]int64{}, p.Seleccion...),
			RespuestaCorrectaIDs: []int64{},
			Respuestas:           []RespuestaRevisionSimItem{},
			Explicacion:          retros[p.PreguntaID].Explicacion,
			Referencia:           retros[p.PreguntaID].Referencia,
		}
		for j, a := range alts[p.PreguntaID] {
			item.Respuestas = append(item.Respuestas, RespuestaRevisionSimItem{
				ID: a.ID, Texto: a.Texto, Letra: examen.Letra(j + 1), EsCorrecta: a.EsCorrecta,
			})
			if a.EsCorrecta {
				item.RespuestaCorrectaIDs = append(item.RespuestaCorrectaIDs, a.ID)
			}
		}
		sort.Slice(item.RespuestaCorrectaIDs, func(i, j int) bool { return item.RespuestaCorrectaIDs[i] < item.RespuestaCorrectaIDs[j] })
		if len(item.RespuestaCorrectaIDs) > 0 {
			item.RespuestaCorrectaID = item.RespuestaCorrectaIDs[0]
		}
		if p.Respondida {
			first := p.Seleccion[0]
			item.RespuestaUsuarioID = &first
			c := p.Correcta
			item.EsCorrecta = &c
		}
		out = append(out, item)
	}
	return RevisionSimulacroResponse{ID: id, Preguntas: out}, nil
}

// ── GetHistorial ──────────────────────────────────────────────────────────────

// GetHistorial returns paginated history for a user.
func (r *Repository) GetHistorial(ctx context.Context, usuarioID int64, page, perPage int, filtros HistorialFiltros) (HistorialSimulacroResponse, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}
	offset := (page - 1) * perPage

	whereArgs := []any{usuarioID}
	where := `s.usuario_id=$1 AND s.estado='completado'`

	if filtros.ProcesoID != nil {
		whereArgs = append(whereArgs, *filtros.ProcesoID)
		where += fmt.Sprintf(" AND s.proceso_id=$%d", len(whereArgs))
	}
	if filtros.Aprobado != nil {
		whereArgs = append(whereArgs, *filtros.Aprobado)
		where += fmt.Sprintf(" AND s.aprobado=$%d", len(whereArgs))
	}

	var total int
	if err := r.db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT COUNT(*) FROM simulacros s WHERE %s`, where),
		whereArgs...,
	).Scan(&total); err != nil {
		return HistorialSimulacroResponse{}, err
	}

	whereArgs = append(whereArgs, perPage, offset)
	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT s.id, p.nombre AS proceso, s.fecha_inicio,
		       COALESCE(s.puntaje_total, 0), COALESCE(s.puntaje_maximo, 0),
		       COALESCE(s.nota_centesimal, 0), COALESCE(s.aprobado, false),
		       COALESCE(EXTRACT(EPOCH FROM (s.fecha_fin - s.fecha_inicio))::int, 0),
		       s.estado
		FROM simulacros s
		JOIN procesos p ON p.id = s.proceso_id
		WHERE %s
		ORDER BY s.created_at DESC
		LIMIT $%d OFFSET $%d`, where, len(whereArgs)-1, len(whereArgs)),
		whereArgs...,
	)
	if err != nil {
		return HistorialSimulacroResponse{}, err
	}
	defer rows.Close()

	items := []HistorialSimulacroItem{}
	numero := offset + 1
	for rows.Next() {
		var item HistorialSimulacroItem
		var fechaInicio time.Time
		if err := rows.Scan(
			&item.ID, &item.Proceso, &fechaInicio,
			&item.PuntajeTotal, &item.PuntajeMaximo,
			&item.NotaCentesimal, &item.Aprobado,
			&item.TiempoUsadoSeg, &item.Estado,
		); err != nil {
			return HistorialSimulacroResponse{}, err
		}
		item.Numero = numero
		item.Fecha = fechaInicio.Format(time.RFC3339)
		item.Porcentaje = porcentajePonderado(item.PuntajeTotal, item.PuntajeMaximo)
		items = append(items, item)
		numero++
	}
	if err := rows.Err(); err != nil {
		return HistorialSimulacroResponse{}, err
	}

	return HistorialSimulacroResponse{
		Data:    items,
		Total:   total,
		Page:    page,
		PerPage: perPage,
	}, nil
}

// ── Access checks ─────────────────────────────────────────────────────────────

// GetProcesoNivel returns the nivel assigned to the proceso.
func (r *Repository) GetProcesoNivel(ctx context.Context, procesoID int64) (string, error) {
	nivel, err := examen.ProcesoNivel(ctx, r.db, procesoID)
	if errors.Is(err, examen.ErrProcesoNoEncontrado) {
		return "", ErrNotFound
	}
	return nivel, err
}

// HasInscripcionAprobada returns true if the user has an approved inscription for the proceso.
func (r *Repository) HasInscripcionAprobada(ctx context.Context, usuarioID, procesoID int64) (bool, error) {
	return examen.InscripcionAprobada(ctx, r.db, usuarioID, procesoID)
}

// porcentajePonderado is the simulacro's overall percentage: puntaje_total /
// puntaje_maximo * 100, weighted by peso and coeficiente like the nota. The
// resultado and the historial both use it so they show the same figure (H136);
// the per-asignatura porcentaje stays correctas/total (one weight inside it).
func porcentajePonderado(puntajeTotal, puntajeMaximo float64) float64 {
	if puntajeMaximo <= 0 {
		return 0
	}
	return grading.Round2(puntajeTotal / puntajeMaximo * 100)
}
