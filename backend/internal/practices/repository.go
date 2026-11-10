package practices

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/cbaciliod/learning/internal/examen"
	"github.com/cbaciliod/learning/internal/grading"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

const (
	estadoEnCurso    = "en_curso"
	estadoCompletada = "completada"

	modoInmediato = "inmediato"

	// validadaCond marks the rows of a question validated in inmediato mode:
	// POST /validar stores es_correcta on the selected rows while the práctica
	// is en_curso (finalize writes it only when closing). It is the persisted
	// "validated" flag that locks the selection (H74).
	validadaCond = "es_correcta IS NOT NULL"

	// uqEnCursoIndex is the unique partial index of migration 049.
	uqEnCursoIndex = "uq_practicas_usuario_en_curso"
)

// Repository wraps the database connection for the practices package.
type Repository struct {
	db *sqlx.DB
}

// NewRepository creates a new practices Repository.
func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

// dbtx is satisfied by both *sqlx.DB and *sqlx.Tx.
type dbtx interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

var graciaSeg = GraciaRespuesta.Seconds()

// ── Internal row types ────────────────────────────────────────────────────────

type practicaMetaRow struct {
	ID               int64
	UsuarioID        int64
	ProcesoNombre    string
	ModoCalificacion string
	TiempoLimiteSeg  *int
	FechaInicio      time.Time
	FechaFin         *time.Time
	TotalPreguntas   int
	Estado           string
	Correctas        int
	Incorrectas      int
	SinResponder     int
	Expirado         bool // timed and NOW() is past fecha_inicio + limite + gracia
	TiempoRestante   *int // seconds left (0 when over, nil when untimed), DB clock
}

type practicaAsignaturaRow struct {
	AsignaturaID int64
	Nombre       string
}

type preguntaInfo struct {
	Orden      int
	Enunciado  string
	Tipo       string
	Asignatura string
}

type practicaHistorialRow struct {
	ID             int64
	TotalPreguntas int
	Correctas      int
	Porcentaje     float64
	TiempoUsadoSeg int
	FechaInicio    time.Time
	Estado         string
}

// metaSelect is the single source of the práctica metadata query.
const metaSelect = `
	SELECT pr.id, pr.usuario_id, p.nombre, pr.modo_calificacion,
	       pr.tiempo_limite_seg, pr.fecha_inicio, pr.fecha_fin,
	       pr.total_preguntas, pr.estado,
	       pr.correctas, pr.incorrectas, pr.sin_responder,
	       (pr.tiempo_limite_seg IS NOT NULL
	        AND NOW() > pr.fecha_inicio + make_interval(secs => pr.tiempo_limite_seg + $2::float8)) AS expirado,
	       CASE WHEN pr.tiempo_limite_seg IS NOT NULL -- GREATEST ignores NULLs
	            THEN GREATEST(0, pr.tiempo_limite_seg - EXTRACT(EPOCH FROM (NOW() - pr.fecha_inicio)))::int
	       END AS restante
	FROM practicas pr
	JOIN procesos p ON p.id = pr.proceso_id
	WHERE pr.id = $1`

// getMeta loads the práctica metadata and checks ownership: someone else's
// práctica is ErrNotFound, indistinguishable from a missing one (H137).
// forUpdate locks the práctica row (use inside a tx).
func getMeta(ctx context.Context, q dbtx, id, usuarioID int64, forUpdate bool) (practicaMetaRow, error) {
	query := metaSelect
	if forUpdate {
		query += " FOR UPDATE OF pr"
	}
	var m practicaMetaRow
	err := q.QueryRowContext(ctx, query, id, graciaSeg).Scan(
		&m.ID, &m.UsuarioID, &m.ProcesoNombre, &m.ModoCalificacion,
		&m.TiempoLimiteSeg, &m.FechaInicio, &m.FechaFin,
		&m.TotalPreguntas, &m.Estado,
		&m.Correctas, &m.Incorrectas, &m.SinResponder, &m.Expirado, &m.TiempoRestante,
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

// loadFilas returns the answer rows of a práctica (one per selected
// alternative or a NULL placeholder) and per-question display info, by orden.
// Rows are grouped by the asignatura the question had when the práctica was
// created (H73), not by its current one.
func loadFilas(ctx context.Context, q dbtx, practicaID int64) ([]grading.Fila, map[int64]preguntaInfo, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT prr.pregunta_id, prr.asignatura_id, prr.respuesta_id, prr.es_correcta,
		       prr.orden, pq.enunciado, pq.tipo, a.nombre
		FROM practica_respuestas prr
		JOIN preguntas pq ON pq.id = prr.pregunta_id
		JOIN asignaturas a ON a.id = prr.asignatura_id
		WHERE prr.practica_id = $1
		ORDER BY prr.orden, prr.respuesta_id NULLS FIRST`, practicaID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var filas []grading.Fila
	info := map[int64]preguntaInfo{}
	for rows.Next() {
		var f grading.Fila
		var p preguntaInfo
		if err := rows.Scan(&f.PreguntaID, &f.AsignaturaID, &f.RespuestaID, &f.EsCorrecta,
			&p.Orden, &p.Enunciado, &p.Tipo, &p.Asignatura); err != nil {
			return nil, nil, err
		}
		filas = append(filas, f)
		if _, ok := info[f.PreguntaID]; !ok {
			info[f.PreguntaID] = p
		}
	}
	return filas, info, rows.Err()
}

// ── En curso ──────────────────────────────────────────────────────────────────

// GetPracticaEnCurso returns the ID of the user's en_curso practice (nil if
// none) and whether its time limit is over.
func (r *Repository) GetPracticaEnCurso(ctx context.Context, usuarioID int64) (*int64, bool, error) {
	var id int64
	var expirado bool
	err := r.db.QueryRowContext(ctx, `
		SELECT id, (tiempo_limite_seg IS NOT NULL
		            AND NOW() > fecha_inicio + make_interval(secs => tiempo_limite_seg + $2::float8))
		FROM practicas WHERE usuario_id=$1 AND estado='en_curso'
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

// ── Access checks (same rule as simulacros) ──────────────────────────────────

// GetProcesoNivel returns the nivel assigned to the proceso.
func (r *Repository) GetProcesoNivel(ctx context.Context, procesoID int64) (string, error) {
	nivel, err := examen.ProcesoNivel(ctx, r.db, procesoID)
	if errors.Is(err, examen.ErrProcesoNoEncontrado) {
		return "", ErrNotFound
	}
	return nivel, err
}

// HasInscripcionAprobada returns true if the user has an approved inscripción for the proceso.
func (r *Repository) HasInscripcionAprobada(ctx context.Context, usuarioID, procesoID int64) (bool, error) {
	return examen.InscripcionAprobada(ctx, r.db, usuarioID, procesoID)
}

// ── Create ────────────────────────────────────────────────────────────────────

// CreatePractica inserts the práctica, its asignaturas and one placeholder row
// per question in one transaction with batch inserts (H59). Returns
// errEnCursoDuplicado when the user already has one en_curso (H10).
func (r *Repository) CreatePractica(
	ctx context.Context,
	req CreatePracticaRequest,
	usuarioID, institucionID int64,
	preguntas []preguntaExamenItem,
) (int64, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var practicaID int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO practicas (usuario_id, institucion_id, proceso_id, modo_calificacion, tiempo_limite_seg, total_preguntas)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`,
		usuarioID, institucionID, req.ProcesoID, req.ModoCalificacion, req.TiempoLimiteSeg, len(preguntas),
	).Scan(&practicaID)
	if examen.IsUniqueViolation(err, uqEnCursoIndex) {
		return 0, errEnCursoDuplicado
	}
	if err != nil {
		return 0, err
	}

	asigIDs := make([]int64, len(req.Asignaturas))
	nums := make([]int64, len(req.Asignaturas))
	for i, a := range req.Asignaturas {
		asigIDs[i], nums[i] = a.AsignaturaID, int64(a.NumeroPreguntas)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO practica_asignaturas (practica_id, asignatura_id, numero_preguntas)
		SELECT $1, t.asignatura_id, t.n FROM unnest($2::bigint[], $3::int[]) AS t(asignatura_id, n)`,
		practicaID, pq.Array(asigIDs), pq.Array(nums)); err != nil {
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
		INSERT INTO practica_respuestas (practica_id, pregunta_id, asignatura_id, orden)
		SELECT $1, t.pregunta_id, t.asignatura_id, t.orden
		FROM unnest($2::bigint[], $3::bigint[], $4::int[]) AS t(pregunta_id, asignatura_id, orden)`,
		practicaID, pq.Array(ids), pq.Array(qAsigs), pq.Array(ordenes)); err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return practicaID, nil
}

// ── Get ───────────────────────────────────────────────────────────────────────

// GetPractica returns the practice (any estado) for its owner. An en_curso
// práctica whose time limit is over yields errExpirado.
func (r *Repository) GetPractica(ctx context.Context, id, usuarioID int64) (PracticaEnCursoResponse, error) {
	meta, err := getMeta(ctx, r.db, id, usuarioID, false)
	if err != nil {
		return PracticaEnCursoResponse{}, err
	}
	if meta.Estado == estadoEnCurso && meta.Expirado {
		return PracticaEnCursoResponse{}, errExpirado
	}

	filas, info, err := loadFilas(ctx, r.db, id)
	if err != nil {
		return PracticaEnCursoResponse{}, err
	}
	preguntas := grading.Agrupar(filas)
	alts, err := examen.LoadAlternativas(ctx, r.db, id, examen.PreguntaIDs(preguntas))
	if err != nil {
		return PracticaEnCursoResponse{}, err
	}
	// Questions validated in inmediato mode (H74): their key was already
	// revealed, so it is returned to restore the locked state on reload.
	inmediatoEnCurso := meta.Estado == estadoEnCurso && meta.ModoCalificacion == modoInmediato
	var validadas []int64
	if inmediatoEnCurso {
		for _, p := range preguntas {
			if p.Calificada {
				validadas = append(validadas, p.PreguntaID)
			}
		}
	}
	var clave map[int64][]int64
	var retros map[int64]examen.Retro
	if len(validadas) > 0 {
		if clave, err = examen.LoadClave(ctx, r.db, validadas); err != nil {
			return PracticaEnCursoResponse{}, err
		}
		if retros, err = examen.LoadRetro(ctx, r.db, validadas); err != nil {
			return PracticaEnCursoResponse{}, err
		}
	}

	items := make([]PreguntaPracticaItem, 0, len(preguntas))
	for _, p := range preguntas {
		pi := info[p.PreguntaID]
		resps := make([]RespuestaPracticaItem, 0, len(alts[p.PreguntaID]))
		for j, a := range alts[p.PreguntaID] {
			resps = append(resps, RespuestaPracticaItem{ID: a.ID, Texto: a.Texto, Letra: examen.Letra(j + 1), Orden: j + 1})
		}
		estado := "sin_responder"
		switch {
		case p.Respondida && p.Calificada && p.Correcta:
			estado = "correcta"
		case p.Respondida && p.Calificada:
			estado = "incorrecta"
		case p.Respondida:
			estado = "respondida"
		}
		item := PreguntaPracticaItem{
			ID:           p.PreguntaID,
			Orden:        pi.Orden,
			Enunciado:    pi.Enunciado,
			Tipo:         pi.Tipo,
			Asignatura:   pi.Asignatura,
			Respuestas:   resps,
			Estado:       estado,
			RespuestaIDs: append([]int64{}, p.Seleccion...),
		}
		if inmediatoEnCurso && p.Calificada {
			item.Validada = true
			item.RespuestaCorrectaIDs = append([]int64{}, clave[p.PreguntaID]...)
			item.Explicacion = retros[p.PreguntaID].Explicacion
		}
		items = append(items, item)
	}

	return PracticaEnCursoResponse{
		ID:                meta.ID,
		Proceso:           meta.ProcesoNombre,
		ModoCalificacion:  meta.ModoCalificacion,
		TiempoLimiteSeg:   meta.TiempoLimiteSeg,
		FechaInicio:       meta.FechaInicio.Format(time.RFC3339),
		TiempoRestanteSeg: meta.TiempoRestante,
		TotalPreguntas:    meta.TotalPreguntas,
		Preguntas:         items,
	}, nil
}

// ── Toggle ────────────────────────────────────────────────────────────────────

// lockEnCurso locks the práctica row and checks owner, estado and time limit.
func lockEnCurso(ctx context.Context, tx *sqlx.Tx, practicaID, usuarioID int64) (practicaMetaRow, error) {
	meta, err := getMeta(ctx, tx, practicaID, usuarioID, true)
	if err != nil {
		return meta, err
	}
	if meta.Estado != estadoEnCurso {
		return meta, ErrPracticaEstado
	}
	if meta.Expirado {
		return meta, ErrTiempoAgotado
	}
	return meta, nil
}

// ToggleRespuesta selects or deselects an answer for a question in a practice.
// For tipo=unica, replaces any prior selection. For tipo=multiple, toggles.
// The práctica row is locked for the whole transaction (H37), so concurrent
// toggles are serialized. Returns the updated selection.
func (r *Repository) ToggleRespuesta(ctx context.Context, practicaID, preguntaID, respuestaID, usuarioID int64) ([]int64, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	meta, err := lockEnCurso(ctx, tx, practicaID, usuarioID)
	if err != nil {
		return nil, err
	}

	var tipo string
	var respExists, inPractica, validada bool
	err = tx.QueryRowContext(ctx, `
		SELECT pq.tipo,
		       EXISTS (SELECT 1 FROM respuestas r WHERE r.id=$1 AND r.pregunta_id=$2),
		       EXISTS (SELECT 1 FROM practica_respuestas WHERE practica_id=$3 AND pregunta_id=$2),
		       EXISTS (SELECT 1 FROM practica_respuestas WHERE practica_id=$3 AND pregunta_id=$2 AND `+validadaCond+`)
		FROM preguntas pq
		WHERE pq.id=$2`, respuestaID, preguntaID, practicaID,
	).Scan(&tipo, &respExists, &inPractica, &validada)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && !inPractica) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	// H74: once validated in inmediato mode the key is known; the selection
	// stays as it was at validation (and so does the grade).
	if meta.ModoCalificacion == modoInmediato && validada {
		return nil, ErrPreguntaValidada
	}
	if !respExists {
		return nil, fmt.Errorf("%w: respuesta_id %d no pertenece a la pregunta", ErrValidation, respuestaID)
	}

	actual, err := querySeleccion(ctx, tx, practicaID, preguntaID)
	if err != nil {
		return nil, err
	}
	var next []int64
	if tipo == "unica" {
		next = []int64{respuestaID}
	} else {
		found := false
		for _, id := range actual {
			if id == respuestaID {
				found = true
				continue
			}
			next = append(next, id)
		}
		if !found {
			next = append(next, respuestaID)
		}
	}
	if err := replaceSeleccion(ctx, tx, practicaID, preguntaID, next); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	sort.Slice(next, func(i, j int) bool { return next[i] < next[j] })
	if next == nil {
		next = []int64{}
	}
	return next, nil
}

func querySeleccion(ctx context.Context, q dbtx, practicaID, preguntaID int64) ([]int64, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT respuesta_id FROM practica_respuestas
		WHERE practica_id=$1 AND pregunta_id=$2 AND respuesta_id IS NOT NULL
		ORDER BY respuesta_id`, practicaID, preguntaID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// replaceSeleccion stores the full selection of a question: one row per
// selected alternative, or a single NULL placeholder when empty, keeping the
// question's orden and its asignatura snapshot (H73).
func replaceSeleccion(ctx context.Context, tx *sqlx.Tx, practicaID, preguntaID int64, ids []int64) error {
	var orden int
	var asignaturaID int64
	if err := tx.QueryRowContext(ctx,
		`SELECT MIN(orden), MIN(asignatura_id) FROM practica_respuestas WHERE practica_id=$1 AND pregunta_id=$2`,
		practicaID, preguntaID).Scan(&orden, &asignaturaID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM practica_respuestas WHERE practica_id=$1 AND pregunta_id=$2`, practicaID, preguntaID); err != nil {
		return err
	}
	if len(ids) == 0 {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO practica_respuestas (practica_id, pregunta_id, asignatura_id, orden) VALUES ($1, $2, $3, $4)`,
			practicaID, preguntaID, asignaturaID, orden)
		return err
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO practica_respuestas (practica_id, pregunta_id, asignatura_id, respuesta_id, respondida_at, orden)
		SELECT $1, $2, $3, unnest($4::bigint[]), NOW(), $5`,
		practicaID, preguntaID, asignaturaID, pq.Array(ids), orden)
	return err
}

// ── Finalize ──────────────────────────────────────────────────────────────────

// FinalizarPractica finalizes (or abandons) a practice in ONE transaction with
// the práctica row locked and a conditional UPDATE (H9): only one finalize can
// succeed. For timed prácticas fecha_fin is capped at the limit and answers
// saved after limit + gracia are discarded (H8). Returns the graded attempts
// (nil when abandoned) for question statistics.
func (r *Repository) FinalizarPractica(ctx context.Context, id, usuarioID int64, abandonar bool) ([]grading.Intento, error) {
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
		return nil, ErrPracticaEstado
	}

	const fechaFinExpr = `CASE WHEN tiempo_limite_seg IS NULL THEN NOW()
		ELSE LEAST(NOW(), fecha_inicio + make_interval(secs => tiempo_limite_seg)) END`

	if abandonar {
		res, err := tx.ExecContext(ctx,
			`UPDATE practicas SET estado='abandonada', fecha_fin=`+fechaFinExpr+` WHERE id=$1 AND estado='en_curso'`, id)
		if err != nil {
			return nil, err
		}
		if err := mustAffect(res); err != nil {
			return nil, err
		}
		return nil, tx.Commit()
	}

	// Discard answers saved after the time limit (+ gracia).
	if _, err := tx.ExecContext(ctx, `
		UPDATE practica_respuestas prr
		SET respuesta_id = NULL, es_correcta = NULL, respondida_at = NULL
		FROM practicas p
		WHERE p.id = prr.practica_id AND prr.practica_id = $1
		  AND p.tiempo_limite_seg IS NOT NULL
		  AND prr.respondida_at > p.fecha_inicio + make_interval(secs => p.tiempo_limite_seg + $2::float8)`,
		id, graciaSeg); err != nil {
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
	c := grading.Contar(preguntas)

	res, err := tx.ExecContext(ctx, `
		UPDATE practicas
		SET estado='completada', fecha_fin=`+fechaFinExpr+`,
		    correctas=$1, incorrectas=$2, sin_responder=$3
		WHERE id=$4 AND estado='en_curso'`,
		c.Correctas, c.Incorrectas, c.SinResponder, id)
	if err != nil {
		return nil, err
	}
	if err := mustAffect(res); err != nil {
		return nil, err
	}

	// Store question-level correctness on the selected rows.
	if err := storeCorreccion(ctx, tx, id, preguntas); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return grading.Intentos(preguntas), nil
}

func mustAffect(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrPracticaEstado
	}
	return nil
}

// storeCorreccion writes the question-level es_correcta on every selected row
// of the answered questions.
func storeCorreccion(ctx context.Context, q dbtx, practicaID int64, preguntas []grading.Pregunta) error {
	var pids []int64
	var oks []bool
	for _, p := range preguntas {
		if p.Respondida {
			pids = append(pids, p.PreguntaID)
			oks = append(oks, p.Correcta)
		}
	}
	_, err := q.ExecContext(ctx, `
		UPDATE practica_respuestas prr SET es_correcta = v.correcta
		FROM unnest($2::bigint[], $3::bool[]) AS v(pregunta_id, correcta)
		WHERE prr.practica_id = $1 AND prr.pregunta_id = v.pregunta_id AND prr.respuesta_id IS NOT NULL`,
		practicaID, pq.Array(pids), pq.Array(oks))
	return err
}

// ── Results & Revision ────────────────────────────────────────────────────────

func (r *Repository) loadAsignaturas(ctx context.Context, practicaID int64) ([]practicaAsignaturaRow, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT pa.asignatura_id, a.nombre
		FROM practica_asignaturas pa
		JOIN asignaturas a ON a.id = pa.asignatura_id
		WHERE pa.practica_id = $1
		ORDER BY pa.id`, practicaID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []practicaAsignaturaRow
	for rows.Next() {
		var a practicaAsignaturaRow
		if err := rows.Scan(&a.AsignaturaID, &a.Nombre); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// GetResultado returns the results for a completed practice. Totals are the
// values stored at finalize; the per-asignatura breakdown aggregates the stored
// rows PER QUESTION with the shared grading rules (H35).
func (r *Repository) GetResultado(ctx context.Context, id, usuarioID int64) (PracticaResultadoResponse, error) {
	meta, err := getMeta(ctx, r.db, id, usuarioID, false)
	if err != nil {
		return PracticaResultadoResponse{}, err
	}
	if meta.Estado != estadoCompletada {
		return PracticaResultadoResponse{}, ErrPracticaEstado
	}

	asignaturas, err := r.loadAsignaturas(ctx, id)
	if err != nil {
		return PracticaResultadoResponse{}, err
	}
	filas, _, err := loadFilas(ctx, r.db, id)
	if err != nil {
		return PracticaResultadoResponse{}, err
	}
	porAsig := grading.ContarPorAsignatura(grading.Agrupar(filas))

	porAsignatura := make([]ResultadoAsignaturaItem, 0, len(asignaturas))
	for _, a := range asignaturas {
		c := porAsig[a.AsignaturaID]
		porAsignatura = append(porAsignatura, ResultadoAsignaturaItem{
			Asignatura:     a.Nombre,
			TotalPreguntas: c.Total,
			Correctas:      c.Correctas,
			Incorrectas:    c.Incorrectas,
			SinResponder:   c.SinResponder,
			Porcentaje:     grading.Porcentaje(c.Correctas, c.Total),
		})
	}

	var tiempoUsado int
	var fechaFin string
	if meta.FechaFin != nil {
		tiempoUsado = int(meta.FechaFin.Sub(meta.FechaInicio).Seconds())
		fechaFin = meta.FechaFin.Format(time.RFC3339)
	}
	total := meta.Correctas + meta.Incorrectas + meta.SinResponder

	return PracticaResultadoResponse{
		ID:              meta.ID,
		Estado:          meta.Estado,
		Proceso:         meta.ProcesoNombre,
		TotalPreguntas:  meta.TotalPreguntas,
		Correctas:       meta.Correctas,
		Incorrectas:     meta.Incorrectas,
		SinResponder:    meta.SinResponder,
		Porcentaje:      grading.Porcentaje(meta.Correctas, total),
		TiempoUsadoSeg:  tiempoUsado,
		TiempoLimiteSeg: meta.TiempoLimiteSeg,
		FechaInicio:     meta.FechaInicio.Format(time.RFC3339),
		FechaFin:        fechaFin,
		PorAsignatura:   porAsignatura,
	}, nil
}

// GetRevision returns the full revision for a completed practice.
func (r *Repository) GetRevision(ctx context.Context, id, usuarioID int64) (RevisionPracticaResponse, error) {
	meta, err := getMeta(ctx, r.db, id, usuarioID, false)
	if err != nil {
		return RevisionPracticaResponse{}, err
	}
	if meta.Estado != estadoCompletada {
		return RevisionPracticaResponse{}, ErrPracticaEstado
	}

	filas, info, err := loadFilas(ctx, r.db, id)
	if err != nil {
		return RevisionPracticaResponse{}, err
	}
	preguntas := grading.Agrupar(filas)
	if len(preguntas) == 0 {
		return RevisionPracticaResponse{ID: id, Preguntas: []RevisionPreguntaItem{}}, nil
	}
	ids := examen.PreguntaIDs(preguntas)
	alts, err := examen.LoadAlternativas(ctx, r.db, id, ids)
	if err != nil {
		return RevisionPracticaResponse{}, err
	}
	retros, err := examen.LoadRetro(ctx, r.db, ids)
	if err != nil {
		return RevisionPracticaResponse{}, err
	}

	out := make([]RevisionPreguntaItem, 0, len(preguntas))
	for _, p := range preguntas {
		pi := info[p.PreguntaID]
		item := RevisionPreguntaItem{
			Orden:                pi.Orden,
			Enunciado:            pi.Enunciado,
			Asignatura:           pi.Asignatura,
			RespuestaUsuarioIDs:  append([]int64{}, p.Seleccion...),
			RespuestaCorrectaIDs: []int64{},
			Respuestas:           []RespuestaRevisionItem{},
			Explicacion:          retros[p.PreguntaID].Explicacion,
			Referencia:           retros[p.PreguntaID].Referencia,
		}
		for j, a := range alts[p.PreguntaID] {
			item.Respuestas = append(item.Respuestas, RespuestaRevisionItem{
				ID: a.ID, Texto: a.Texto, Letra: examen.Letra(j + 1), EsCorrecta: a.EsCorrecta,
			})
			if a.EsCorrecta {
				item.RespuestaCorrectaIDs = append(item.RespuestaCorrectaIDs, a.ID)
			}
		}
		if p.Respondida && p.Calificada {
			c := p.Correcta
			item.EsCorrecta = &c
		}
		out = append(out, item)
	}
	return RevisionPracticaResponse{ID: id, Preguntas: out}, nil
}

// ── Inmediato Validation ──────────────────────────────────────────────────────

// ValidarPregunta checks the selected answers for a question in inmediato mode.
// The question must belong to the práctica (else ErrNotFound — H6: it must
// never reveal the key of an arbitrary question) and must have a selection
// (else ErrValidation). Correctness uses the shared exact-set rule.
func (r *Repository) ValidarPregunta(ctx context.Context, practicaID, preguntaID, usuarioID int64) (ValidarPreguntaResponse, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return ValidarPreguntaResponse{}, err
	}
	defer tx.Rollback()

	meta, err := lockEnCurso(ctx, tx, practicaID, usuarioID)
	if err != nil {
		return ValidarPreguntaResponse{}, err
	}
	if meta.ModoCalificacion != modoInmediato {
		return ValidarPreguntaResponse{}, ErrPracticaEstado
	}

	var tipo string
	err = tx.QueryRowContext(ctx, `
		SELECT pq.tipo FROM preguntas pq
		WHERE pq.id = $2
		  AND EXISTS (SELECT 1 FROM practica_respuestas WHERE practica_id = $1 AND pregunta_id = $2)`,
		practicaID, preguntaID).Scan(&tipo)
	if errors.Is(err, sql.ErrNoRows) {
		return ValidarPreguntaResponse{}, ErrNotFound
	}
	if err != nil {
		return ValidarPreguntaResponse{}, err
	}

	seleccion, err := querySeleccion(ctx, tx, practicaID, preguntaID)
	if err != nil {
		return ValidarPreguntaResponse{}, err
	}
	if len(seleccion) == 0 {
		return ValidarPreguntaResponse{}, fmt.Errorf("%w: selecciona al menos una respuesta antes de comprobar", ErrValidation)
	}

	clave, err := examen.LoadClave(ctx, tx, []int64{preguntaID})
	if err != nil {
		return ValidarPreguntaResponse{}, err
	}
	esCorrecta := grading.Evaluar(seleccion, clave[preguntaID])

	p := grading.Pregunta{PreguntaID: preguntaID, Seleccion: seleccion, Respondida: true, Correcta: esCorrecta}
	if err := storeCorreccion(ctx, tx, practicaID, []grading.Pregunta{p}); err != nil {
		return ValidarPreguntaResponse{}, err
	}

	retro, err := examen.LoadRetro(ctx, tx, []int64{preguntaID})
	if err != nil {
		return ValidarPreguntaResponse{}, err
	}
	if err := tx.Commit(); err != nil {
		return ValidarPreguntaResponse{}, err
	}

	resp := ValidarPreguntaResponse{
		EsCorrecta:           esCorrecta,
		RespuestaCorrectaIDs: clave[preguntaID],
		Explicacion:          retro[preguntaID].Explicacion,
	}
	if tipo == "unica" && len(clave[preguntaID]) > 0 {
		resp.RespuestaCorrectaID = clave[preguntaID][0]
	}
	if resp.RespuestaCorrectaIDs == nil {
		resp.RespuestaCorrectaIDs = []int64{}
	}
	return resp, nil
}

// ── History & Deletion ────────────────────────────────────────────────────────

// ListHistorial returns paginated history for a user.
func (r *Repository) ListHistorial(ctx context.Context, usuarioID int64, page, perPage int, estado string) (HistorialResponse, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}
	offset := (page - 1) * perPage

	whereArgs := []any{usuarioID}
	where := `pr.usuario_id=$1 AND pr.estado NOT IN ('en_curso','eliminada')`
	if estado != "" {
		whereArgs = append(whereArgs, estado)
		where += fmt.Sprintf(" AND pr.estado=$%d", len(whereArgs))
	}

	var total int
	if err := r.db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT COUNT(*) FROM practicas pr WHERE %s`, where),
		whereArgs...,
	).Scan(&total); err != nil {
		return HistorialResponse{}, err
	}

	whereArgs = append(whereArgs, perPage, offset)
	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT pr.id,
		       pr.total_preguntas,
		       pr.correctas,
		       CASE WHEN pr.total_preguntas > 0
		            THEN ROUND(pr.correctas::numeric / pr.total_preguntas * 100, 2)
		            ELSE 0 END AS porcentaje,
		       COALESCE(EXTRACT(EPOCH FROM (pr.fecha_fin - pr.fecha_inicio))::int, 0) AS tiempo_usado_seg,
		       pr.fecha_inicio,
		       pr.estado
		FROM practicas pr
		WHERE %s
		ORDER BY pr.created_at DESC
		LIMIT $%d OFFSET $%d`, where, len(whereArgs)-1, len(whereArgs)),
		whereArgs...,
	)
	if err != nil {
		return HistorialResponse{}, err
	}
	defer rows.Close()

	items := []HistorialPracticaItem{}
	numero := offset + 1
	for rows.Next() {
		var row practicaHistorialRow
		if err := rows.Scan(
			&row.ID, &row.TotalPreguntas, &row.Correctas, &row.Porcentaje,
			&row.TiempoUsadoSeg, &row.FechaInicio, &row.Estado,
		); err != nil {
			return HistorialResponse{}, err
		}
		items = append(items, HistorialPracticaItem{
			ID:             row.ID,
			Numero:         numero,
			TotalPreguntas: row.TotalPreguntas,
			Correctas:      row.Correctas,
			Porcentaje:     row.Porcentaje,
			TiempoUsadoSeg: row.TiempoUsadoSeg,
			Fecha:          row.FechaInicio.Format(time.RFC3339),
			Estado:         row.Estado,
		})
		numero++
	}
	if err := rows.Err(); err != nil {
		return HistorialResponse{}, err
	}

	if len(items) > 0 {
		ids := make([]int64, len(items))
		for i, it := range items {
			ids[i] = it.ID
		}
		asgRows, err := r.db.QueryContext(ctx, `
			SELECT pa.practica_id, STRING_AGG(a.nombre, ', ' ORDER BY a.nombre)
			FROM practica_asignaturas pa JOIN asignaturas a ON a.id=pa.asignatura_id
			WHERE pa.practica_id = ANY($1)
			GROUP BY pa.practica_id`, pq.Array(ids))
		if err != nil {
			return HistorialResponse{}, err
		}
		defer asgRows.Close()
		asgMap := map[int64]string{}
		for asgRows.Next() {
			var pid int64
			var names string
			if err := asgRows.Scan(&pid, &names); err != nil {
				return HistorialResponse{}, err
			}
			asgMap[pid] = names
		}
		if err := asgRows.Err(); err != nil {
			return HistorialResponse{}, err
		}
		for i := range items {
			items[i].Asignaturas = asgMap[items[i].ID]
		}
	}

	return HistorialResponse{
		Data:    items,
		Total:   total,
		Page:    page,
		PerPage: perPage,
	}, nil
}

// DeletePractica soft-deletes a practice by setting its state to 'eliminada'.
func (r *Repository) DeletePractica(ctx context.Context, id, usuarioID int64) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE practicas SET estado='eliminada' WHERE id=$1 AND usuario_id=$2 AND estado!='eliminada'`,
		id, usuarioID,
	)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		// Missing and someone else's are both ErrNotFound (H137); the owner's
		// already-eliminated práctica is a no-op.
		var existe bool
		if err := r.db.QueryRowContext(ctx,
			`SELECT EXISTS (SELECT 1 FROM practicas WHERE id=$1 AND usuario_id=$2)`, id, usuarioID).Scan(&existe); err != nil {
			return err
		}
		if !existe {
			return ErrNotFound
		}
		return nil // already eliminated
	}
	return nil
}

// BatchDeletePracticas soft-deletes multiple practices owned by the user.
// ids must already be de-duplicated (see Service.BatchDeletePracticas).
func (r *Repository) BatchDeletePracticas(ctx context.Context, ids []int64, usuarioID int64) error {
	if len(ids) == 0 {
		return nil
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var count int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM practicas WHERE id = ANY($1) AND usuario_id=$2`,
		pq.Array(ids), usuarioID).Scan(&count); err != nil {
		return err
	}
	if count != len(ids) {
		return ErrForbidden
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE practicas SET estado='eliminada' WHERE id = ANY($1) AND usuario_id=$2`,
		pq.Array(ids), usuarioID); err != nil {
		return err
	}
	return tx.Commit()
}
