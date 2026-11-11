package questions

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/cbaciliod/learning/internal/grading"
	"github.com/cbaciliod/learning/internal/texto"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// Repository wraps the database connection for the questions package.
type Repository struct {
	db *sqlx.DB
}

// NewRepository creates a new questions Repository.
func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

// ── Preguntas ─────────────────────────────────────────────────────────────────

// GetProcesoInstitucionID returns the institucion_id for a given proceso.
func (r *Repository) GetProcesoInstitucionID(ctx context.Context, procesoID int64) (int64, error) {
	var instID int64
	err := r.db.QueryRowContext(ctx,
		`SELECT institucion_id FROM procesos WHERE id = $1`, procesoID,
	).Scan(&instID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	return instID, err
}

// validarClasificacion checks, inside the caller's transaction, that the
// asignatura is assigned to the proceso and that every etiqueta exists and
// belongs to that asignatura (H19). The catalogs otherwise allow any
// combination, and a question filed under a foreign asignatura never enters a
// simulacro of its proceso.
func validarClasificacion(ctx context.Context, q sqlx.QueryerContext, procesoID, asignaturaID int64, etiquetaIDs []int64) error {
	var asignada bool
	if err := sqlx.GetContext(ctx, q, &asignada, `
		SELECT EXISTS (SELECT 1 FROM proceso_asignaturas WHERE proceso_id = $1 AND asignatura_id = $2)`,
		procesoID, asignaturaID,
	); err != nil {
		return err
	}
	if !asignada {
		return fmt.Errorf("%w: la asignatura no está asignada a este proceso", ErrValidation)
	}
	if len(etiquetaIDs) == 0 {
		return nil
	}
	var ajenas int
	if err := sqlx.GetContext(ctx, q, &ajenas, `
		SELECT COUNT(*) FROM unnest($1::bigint[]) AS t(id)
		WHERE NOT EXISTS (SELECT 1 FROM etiquetas_pregunta ep WHERE ep.id = t.id AND ep.asignatura_id = $2)`,
		pq.Array(etiquetaIDs), asignaturaID,
	); err != nil {
		return err
	}
	if ajenas > 0 {
		return fmt.Errorf("%w: hay etiquetas que no existen o no son de esta asignatura", ErrValidation)
	}
	return nil
}

// validarTema checks, inside the caller's transaction, the optional tema of a
// question (T8): it must exist in the same asignatura and be active. The one
// exception is actual, the tema the question already has, which may be sent
// back on update even if it was archived since. FOR SHARE keeps the tema from
// being archived until the question is saved.
func validarTema(ctx context.Context, q sqlx.QueryerContext, temaID *int64, asignaturaID int64, actual *int64) error {
	if temaID == nil {
		return nil
	}
	var estado string
	err := sqlx.GetContext(ctx, q, &estado,
		`SELECT estado FROM temas WHERE id = $1 AND asignatura_id = $2 FOR SHARE`, *temaID, asignaturaID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrTemaInvalido
	}
	if err != nil {
		return err
	}
	conservaElActual := actual != nil && *actual == *temaID
	if estado != "activo" && !conservaElActual {
		return ErrTemaInvalido
	}
	return nil
}

// temaTrasEditar is the tema_id a PUT stores (tech-lead decision on T8, like
// etiqueta_ids in H100): a sent value or null replaces it; an absent field
// keeps the current tema, unless the asignatura changes, since the old tema
// no longer belongs to it.
func temaTrasEditar(enviado OptionalID, actual *int64, asignaturaActual, asignaturaNueva int64) *int64 {
	switch {
	case enviado.Set:
		return enviado.Value
	case asignaturaNueva != asignaturaActual:
		return nil
	default:
		return actual
	}
}

// CreatePregunta inserts a new question with its answers, tags, and optional feedback.
func (r *Repository) CreatePregunta(ctx context.Context, req CreatePreguntaRequest, institucionID int64) (PreguntaResponse, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return PreguntaResponse{}, err
	}
	defer tx.Rollback()

	if err := validarClasificacion(ctx, tx, req.ProcesoID, req.AsignaturaID, req.EtiquetaIDs); err != nil {
		return PreguntaResponse{}, err
	}
	if err := validarTema(ctx, tx, req.TemaID, req.AsignaturaID, nil); err != nil {
		return PreguntaResponse{}, err
	}

	var preguntaID int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO preguntas (institucion_id, proceso_id, asignatura_id, grado_id, especialidad_id,
			tema_id, enunciado, tipo, dificultad, estado, origen)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'borrador', 'manual')
		RETURNING id`,
		institucionID, req.ProcesoID, req.AsignaturaID, req.GradoID, req.EspecialidadID,
		req.TemaID, req.Enunciado, req.Tipo, req.Dificultad,
	).Scan(&preguntaID)
	if err != nil {
		return PreguntaResponse{}, err
	}

	// Insert respuestas
	for _, resp := range req.Respuestas {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO respuestas (pregunta_id, texto, es_correcta, orden)
			VALUES ($1, $2, $3, $4)`,
			preguntaID, resp.Texto, resp.EsCorrecta, resp.Orden,
		)
		if err != nil {
			return PreguntaResponse{}, err
		}
	}

	// Insert etiquetas
	for _, etID := range req.EtiquetaIDs {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO pregunta_etiquetas (pregunta_id, etiqueta_id)
			VALUES ($1, $2)
			ON CONFLICT DO NOTHING`,
			preguntaID, etID,
		)
		if err != nil {
			return PreguntaResponse{}, err
		}
	}

	// Insert retroalimentacion if provided
	if req.Retroalimentacion != nil {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO pregunta_retroalimentacion (pregunta_id, explicacion, referencia)
			VALUES ($1, $2, $3)`,
			preguntaID, req.Retroalimentacion.Explicacion, req.Retroalimentacion.Referencia,
		)
		if err != nil {
			return PreguntaResponse{}, err
		}
	}

	if err = tx.Commit(); err != nil {
		return PreguntaResponse{}, err
	}

	return r.GetPregunta(ctx, preguntaID)
}

// GetPregunta fetches one question by ID in a single query, returning ErrNotFound if not found or deleted.
func (r *Repository) GetPregunta(ctx context.Context, id int64) (PreguntaResponse, error) {
	type row struct {
		ID               int64    `db:"id"`
		ProcesoID        int64    `db:"proceso_id"`
		Proceso          string   `db:"proceso"`
		AsignaturaID     int64    `db:"asignatura_id"`
		Asignatura       string   `db:"asignatura"`
		GradoID          *int64   `db:"grado_id"`
		Grado            *string  `db:"grado"`
		EspecialidadID   *int64   `db:"especialidad_id"`
		Especialidad     *string  `db:"especialidad"`
		TemaID           *int64   `db:"tema_id"`
		Tema             *string  `db:"tema"`
		Enunciado        string   `db:"enunciado"`
		Tipo             string   `db:"tipo"`
		Dificultad       string   `db:"dificultad"`
		Estado           string   `db:"estado"`
		Origen           string   `db:"origen"`
		MotivoRechazo    *string  `db:"motivo_rechazo"`
		CreatedAt        string   `db:"created_at"`
		RetroExplicacion *string  `db:"retro_explicacion"`
		RetroReferencia  *string  `db:"retro_referencia"`
		EstIntentos      *int     `db:"est_intentos"`
		EstAciertos      *int     `db:"est_aciertos"`
		EstIndDif        *float64 `db:"est_ind_dif"`
		EstIndDisc       *float64 `db:"est_ind_disc"`
		RespuestasJSON   []byte   `db:"respuestas_json"`
		EtiquetasJSON    []byte   `db:"etiquetas_json"`
		EtiquetaIDsJSON  []byte   `db:"etiqueta_ids_json"`
		CitasJSON        []byte   `db:"citas_json"`
	}

	var r0 row
	err := r.db.QueryRowContext(ctx, `
		SELECT
			p.id, p.proceso_id,
			pr.nombre AS proceso,
			p.asignatura_id,
			a.nombre  AS asignatura,
			p.grado_id,
			g.nombre  AS grado,
			p.especialidad_id,
			e.nombre  AS especialidad,
			p.tema_id,
			t.nombre  AS tema,
			p.enunciado, p.tipo, p.dificultad, p.estado, p.origen, p.motivo_rechazo,
			p.created_at::text,
			ret.explicacion AS retro_explicacion,
			ret.referencia  AS retro_referencia,
			est.total_intentos        AS est_intentos,
			est.total_aciertos        AS est_aciertos,
			est.indice_dificultad     AS est_ind_dif,
			est.indice_discriminacion AS est_ind_disc,
			COALESCE(
				(SELECT JSON_AGG(r ORDER BY r.orden)
				 FROM (SELECT id, texto, es_correcta, orden FROM respuestas WHERE pregunta_id = p.id) r),
				'[]'::json
			) AS respuestas_json,
			COALESCE(
				(SELECT JSON_AGG(ep.nombre ORDER BY ep.nombre)
				 FROM pregunta_etiquetas pe
				 JOIN etiquetas_pregunta ep ON ep.id = pe.etiqueta_id
				 WHERE pe.pregunta_id = p.id),
				'[]'::json
			) AS etiquetas_json,
			COALESCE(
				(SELECT JSON_AGG(pe.etiqueta_id ORDER BY pe.etiqueta_id)
				 FROM pregunta_etiquetas pe WHERE pe.pregunta_id = p.id),
				'[]'::json
			) AS etiqueta_ids_json,
			COALESCE(
				(SELECT JSON_AGG(c ORDER BY c.fuente_id, c.version, c.pasaje_id)
				 FROM (SELECT pc.pasaje_id, fv.fuente_id, fv.numero AS version, f.titulo AS fuente_titulo, f.estado AS fuente_estado,
				              pa.seccion, pa.linea_inicio, pa.linea_fin, pa.texto
				       FROM pregunta_citas pc
				       JOIN pasajes pa          ON pa.id = pc.pasaje_id
				       JOIN fuente_versiones fv ON fv.id = pa.fuente_version_id
				       JOIN fuentes f           ON f.id = fv.fuente_id
				       WHERE pc.pregunta_id = p.id) c),
				'[]'::json
			) AS citas_json
		FROM preguntas p
		JOIN procesos pr ON pr.id = p.proceso_id
		JOIN asignaturas a  ON a.id  = p.asignatura_id
		LEFT JOIN grados g         ON g.id = p.grado_id
		LEFT JOIN especialidades e ON e.id = p.especialidad_id
		LEFT JOIN temas t          ON t.id = p.tema_id
		LEFT JOIN pregunta_retroalimentacion ret ON ret.pregunta_id = p.id
		LEFT JOIN pregunta_estadisticas      est ON est.pregunta_id = p.id
		WHERE p.id = $1 AND p.estado != 'eliminada'`, id,
	).Scan(
		&r0.ID, &r0.ProcesoID, &r0.Proceso, &r0.AsignaturaID, &r0.Asignatura,
		&r0.GradoID, &r0.Grado, &r0.EspecialidadID, &r0.Especialidad,
		&r0.TemaID, &r0.Tema,
		&r0.Enunciado, &r0.Tipo, &r0.Dificultad, &r0.Estado, &r0.Origen, &r0.MotivoRechazo, &r0.CreatedAt,
		&r0.RetroExplicacion, &r0.RetroReferencia,
		&r0.EstIntentos, &r0.EstAciertos, &r0.EstIndDif, &r0.EstIndDisc,
		&r0.RespuestasJSON, &r0.EtiquetasJSON, &r0.EtiquetaIDsJSON, &r0.CitasJSON,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return PreguntaResponse{}, ErrNotFound
	}
	if err != nil {
		return PreguntaResponse{}, err
	}

	var respuestas []RespuestaResponse
	if err := json.Unmarshal(r0.RespuestasJSON, &respuestas); err != nil {
		return PreguntaResponse{}, err
	}
	if respuestas == nil {
		respuestas = []RespuestaResponse{}
	}

	var etiquetas []string
	if err := json.Unmarshal(r0.EtiquetasJSON, &etiquetas); err != nil {
		return PreguntaResponse{}, err
	}
	if etiquetas == nil {
		etiquetas = []string{}
	}
	etiquetaIDs := []int64{}
	if err := json.Unmarshal(r0.EtiquetaIDsJSON, &etiquetaIDs); err != nil {
		return PreguntaResponse{}, err
	}
	citas, err := decodificarCitas(r0.CitasJSON)
	if err != nil {
		return PreguntaResponse{}, err
	}

	resp := PreguntaResponse{
		ID:             r0.ID,
		ProcesoID:      r0.ProcesoID,
		Proceso:        r0.Proceso,
		AsignaturaID:   r0.AsignaturaID,
		Asignatura:     r0.Asignatura,
		GradoID:        r0.GradoID,
		Grado:          r0.Grado,
		EspecialidadID: r0.EspecialidadID,
		Especialidad:   r0.Especialidad,
		TemaID:         r0.TemaID,
		Tema:           r0.Tema,
		Enunciado:      r0.Enunciado,
		Tipo:           r0.Tipo,
		Dificultad:     r0.Dificultad,
		Estado:         r0.Estado,
		Origen:         r0.Origen,
		MotivoRechazo:  r0.MotivoRechazo,
		Citas:          &citas,
		Etiquetas:      etiquetas,
		EtiquetaIDs:    etiquetaIDs,
		Respuestas:     respuestas,
		CreatedAt:      r0.CreatedAt,
	}

	if r0.RetroExplicacion != nil {
		resp.Retroalimentacion = &RetroalimentacionResponse{
			Explicacion: *r0.RetroExplicacion,
			Referencia:  r0.RetroReferencia,
		}
	}

	if r0.EstIntentos != nil {
		resp.Estadisticas = &EstadisticaResponse{
			TotalIntentos:        *r0.EstIntentos,
			TotalAciertos:        *r0.EstAciertos,
			IndiceDificultad:     *r0.EstIndDif,
			IndiceDiscriminacion: *r0.EstIndDisc,
		}
	}

	return resp, nil
}

// ListPreguntas returns a paginated, filtered list of questions.
func (r *Repository) ListPreguntas(ctx context.Context, q ListQuery) (PreguntaListResponse, error) {
	args := []any{}
	i := 1
	where := []string{"p.estado != 'eliminada'"}

	if q.Estado != "" {
		where = append(where, fmt.Sprintf("p.estado = $%d", i))
		args = append(args, q.Estado)
		i++
	}
	if q.Origen != "" {
		where = append(where, fmt.Sprintf("p.origen = $%d", i))
		args = append(args, q.Origen)
		i++
	}
	if q.ProcesoID > 0 {
		where = append(where, fmt.Sprintf("p.proceso_id = $%d", i))
		args = append(args, q.ProcesoID)
		i++
	}
	if q.AsignaturaID > 0 {
		where = append(where, fmt.Sprintf("p.asignatura_id = $%d", i))
		args = append(args, q.AsignaturaID)
		i++
	}
	if q.GradoID > 0 {
		where = append(where, fmt.Sprintf("p.grado_id = $%d", i))
		args = append(args, q.GradoID)
		i++
	}
	if q.EspecialidadID > 0 {
		where = append(where, fmt.Sprintf("p.especialidad_id = $%d", i))
		args = append(args, q.EspecialidadID)
		i++
	}
	if q.Dificultad != "" {
		where = append(where, fmt.Sprintf("p.dificultad = $%d", i))
		args = append(args, q.Dificultad)
		i++
	}
	if q.EtiquetaID > 0 {
		where = append(where, fmt.Sprintf("EXISTS (SELECT 1 FROM pregunta_etiquetas pe WHERE pe.pregunta_id = p.id AND pe.etiqueta_id = $%d)", i))
		args = append(args, q.EtiquetaID)
		i++
	}
	if q.TemaID > 0 {
		where = append(where, fmt.Sprintf("p.tema_id = $%d", i))
		args = append(args, q.TemaID)
		i++
	}
	if q.Search != "" {
		where = append(where, fmt.Sprintf("p.enunciado ILIKE $%d", i))
		args = append(args, "%"+q.Search+"%")
		i++
	}

	whereClause := strings.Join(where, " AND ")

	page := q.Page
	if page < 1 {
		page = 1
	}
	perPage := q.PerPage
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}
	offset := (page - 1) * perPage

	query := fmt.Sprintf(`
		SELECT
			p.id,
			pr.nombre AS proceso,
			a.nombre  AS asignatura,
			g.nombre  AS grado,
			e.nombre  AS especialidad,
			p.tema_id,
			t.nombre  AS tema,
			p.enunciado, p.tipo, p.dificultad, p.estado, p.origen, p.motivo_rechazo,
			(SELECT COUNT(*) FROM pregunta_citas pc WHERE pc.pregunta_id = p.id) AS citas_count,
			p.created_at::text,
			COUNT(*) OVER() AS total_count
		FROM preguntas p
		JOIN procesos pr ON pr.id = p.proceso_id
		JOIN asignaturas a ON a.id = p.asignatura_id
		LEFT JOIN grados g ON g.id = p.grado_id
		LEFT JOIN especialidades e ON e.id = p.especialidad_id
		LEFT JOIN temas t ON t.id = p.tema_id
		WHERE %s
		ORDER BY p.id DESC
		LIMIT $%d OFFSET $%d`, whereClause, i, i+1)

	args = append(args, perPage, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return PreguntaListResponse{}, err
	}
	defer rows.Close()

	var data []PreguntaResponse
	var total int
	for rows.Next() {
		var p PreguntaResponse
		var citas int
		if err := rows.Scan(
			&p.ID, &p.Proceso, &p.Asignatura, &p.Grado, &p.Especialidad,
			&p.TemaID, &p.Tema,
			&p.Enunciado, &p.Tipo, &p.Dificultad, &p.Estado, &p.Origen, &p.MotivoRechazo, &citas, &p.CreatedAt,
			&total,
		); err != nil {
			return PreguntaListResponse{}, err
		}
		p.CitasCount = &citas
		p.Etiquetas = []string{}
		p.EtiquetaIDs = []int64{}
		p.Respuestas = []RespuestaResponse{}
		data = append(data, p)
	}
	if err := rows.Err(); err != nil {
		return PreguntaListResponse{}, err
	}
	if data == nil {
		data = []PreguntaResponse{}
	}

	return PreguntaListResponse{
		Data:    data,
		Total:   total,
		Page:    page,
		PerPage: perPage,
	}, nil
}

// UpdatePregunta updates a question within a transaction.
// Returns ErrValidation if proceso_id differs from the stored value.
func (r *Repository) UpdatePregunta(ctx context.Context, id int64, req UpdatePreguntaRequest) (PreguntaResponse, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return PreguntaResponse{}, err
	}
	defer tx.Rollback()

	// proceso_id is immutable; the stored one also scopes the asignatura and
	// etiquetas check below (H19). FOR UPDATE keeps the row stable meanwhile.
	var storedProcesoID, storedAsignaturaID int64
	var storedTemaID *int64
	err = tx.QueryRowContext(ctx,
		`SELECT proceso_id, asignatura_id, tema_id FROM preguntas WHERE id = $1 AND estado != 'eliminada' FOR UPDATE`, id,
	).Scan(&storedProcesoID, &storedAsignaturaID, &storedTemaID)
	if errors.Is(err, sql.ErrNoRows) {
		return PreguntaResponse{}, ErrNotFound
	}
	if err != nil {
		return PreguntaResponse{}, err
	}
	if req.ProcesoID != nil && *req.ProcesoID != storedProcesoID {
		return PreguntaResponse{}, fmt.Errorf("%w: proceso_id es inmutable", ErrValidation)
	}
	// An absent etiqueta_ids keeps the current tags, which must still belong to
	// the (possibly new) asignatura: check the stored ones in that case.
	etiquetas := req.EtiquetaIDs
	if etiquetas == nil {
		if err := tx.SelectContext(ctx, &etiquetas,
			`SELECT etiqueta_id FROM pregunta_etiquetas WHERE pregunta_id = $1`, id); err != nil {
			return PreguntaResponse{}, err
		}
	}
	if err := validarClasificacion(ctx, tx, storedProcesoID, req.AsignaturaID, etiquetas); err != nil {
		return PreguntaResponse{}, err
	}
	// The resulting tema goes in the same UPDATE as asignatura_id so the
	// composite FK sees both.
	temaID := temaTrasEditar(req.TemaID, storedTemaID, storedAsignaturaID, req.AsignaturaID)
	if err := validarTema(ctx, tx, temaID, req.AsignaturaID, storedTemaID); err != nil {
		return PreguntaResponse{}, err
	}

	res, err := tx.ExecContext(ctx, `
		UPDATE preguntas
		SET asignatura_id = $1, grado_id = $2, especialidad_id = $3, tema_id = $4,
		    enunciado = $5, tipo = $6, dificultad = $7, updated_at = NOW()
		WHERE id = $8 AND estado != 'eliminada'`,
		req.AsignaturaID, req.GradoID, req.EspecialidadID, temaID,
		req.Enunciado, req.Tipo, req.Dificultad, id,
	)
	if err != nil {
		return PreguntaResponse{}, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return PreguntaResponse{}, ErrNotFound
	}

	if err := syncRespuestas(ctx, tx, id, req.Respuestas); err != nil {
		return PreguntaResponse{}, err
	}

	// Sync etiquetas (H100): an absent etiqueta_ids keeps the current tags; an
	// explicit list (also []) replaces them.
	if req.EtiquetaIDs != nil {
		if _, err = tx.ExecContext(ctx, `DELETE FROM pregunta_etiquetas WHERE pregunta_id = $1`, id); err != nil {
			return PreguntaResponse{}, err
		}
		for _, etID := range req.EtiquetaIDs {
			if _, err = tx.ExecContext(ctx, `
				INSERT INTO pregunta_etiquetas (pregunta_id, etiqueta_id)
				VALUES ($1, $2) ON CONFLICT DO NOTHING`,
				id, etID,
			); err != nil {
				return PreguntaResponse{}, err
			}
		}
	}

	// Upsert retroalimentacion
	if req.Retroalimentacion != nil {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO pregunta_retroalimentacion (pregunta_id, explicacion, referencia)
			VALUES ($1, $2, $3)
			ON CONFLICT (pregunta_id) DO UPDATE
			SET explicacion = EXCLUDED.explicacion, referencia = EXCLUDED.referencia, updated_at = NOW()`,
			id, req.Retroalimentacion.Explicacion, req.Retroalimentacion.Referencia,
		)
		if err != nil {
			return PreguntaResponse{}, err
		}
	} else {
		// Remove retro if body has no retroalimentacion
		if _, err = tx.ExecContext(ctx, `DELETE FROM pregunta_retroalimentacion WHERE pregunta_id = $1`, id); err != nil {
			return PreguntaResponse{}, err
		}
	}

	if err = tx.Commit(); err != nil {
		return PreguntaResponse{}, err
	}

	return r.GetPregunta(ctx, id)
}

// ── Etiquetas ─────────────────────────────────────────────────────────────────

// ListEtiquetas returns all tags, optionally filtered by asignatura_id and estado.
func (r *Repository) ListEtiquetas(ctx context.Context, asignaturaID int64, estado string) ([]EtiquetaResponse, error) {
	args := []any{}
	i := 1
	where := []string{"1=1"}

	if asignaturaID > 0 {
		where = append(where, fmt.Sprintf("ep.asignatura_id = $%d", i))
		args = append(args, asignaturaID)
		i++
	}
	if estado != "" {
		where = append(where, fmt.Sprintf("ep.estado = $%d", i))
		args = append(args, estado)
	}

	query := fmt.Sprintf(`
		SELECT ep.id, ep.asignatura_id, a.nombre AS asignatura, ep.nombre, ep.estado
		FROM etiquetas_pregunta ep
		JOIN asignaturas a ON a.id = ep.asignatura_id
		WHERE %s
		ORDER BY ep.nombre`, strings.Join(where, " AND "))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []EtiquetaResponse
	for rows.Next() {
		var et EtiquetaResponse
		if err := rows.Scan(&et.ID, &et.AsignaturaID, &et.Asignatura, &et.Nombre, &et.Estado); err != nil {
			return nil, err
		}
		result = append(result, et)
	}
	if result == nil {
		result = []EtiquetaResponse{}
	}
	return result, rows.Err()
}

// CreateEtiqueta inserts a new tag and returns it.
func (r *Repository) CreateEtiqueta(ctx context.Context, req CreateEtiquetaRequest) (EtiquetaResponse, error) {
	var id int64
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO etiquetas_pregunta (asignatura_id, nombre)
		VALUES ($1, $2)
		RETURNING id`,
		req.AsignaturaID, req.Nombre,
	).Scan(&id)
	if err != nil {
		return EtiquetaResponse{}, err
	}
	rows, err := r.ListEtiquetas(ctx, req.AsignaturaID, "")
	if err != nil {
		return EtiquetaResponse{}, err
	}
	for _, et := range rows {
		if et.ID == id {
			return et, nil
		}
	}
	return EtiquetaResponse{}, ErrNotFound
}

// UpdateEtiqueta updates a tag's nombre and/or estado.
func (r *Repository) UpdateEtiqueta(ctx context.Context, id int64, req UpdateEtiquetaRequest) (EtiquetaResponse, error) {
	args := []any{}
	sets := []string{}
	i := 1

	if req.Nombre != "" {
		sets = append(sets, fmt.Sprintf("nombre = $%d", i))
		args = append(args, req.Nombre)
		i++
	}
	if req.Estado != "" {
		sets = append(sets, fmt.Sprintf("estado = $%d", i))
		args = append(args, req.Estado)
		i++
	}

	if len(sets) == 0 {
		return EtiquetaResponse{}, fmt.Errorf("%w: nada que actualizar", ErrValidation)
	}

	args = append(args, id)
	res, err := r.db.ExecContext(ctx,
		fmt.Sprintf("UPDATE etiquetas_pregunta SET %s WHERE id = $%d", strings.Join(sets, ", "), i),
		args...,
	)
	if err != nil {
		return EtiquetaResponse{}, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return EtiquetaResponse{}, ErrNotFound
	}

	// Fetch updated
	var et EtiquetaResponse
	err = r.db.QueryRowContext(ctx, `
		SELECT ep.id, ep.asignatura_id, a.nombre, ep.nombre, ep.estado
		FROM etiquetas_pregunta ep
		JOIN asignaturas a ON a.id = ep.asignatura_id
		WHERE ep.id = $1`, id,
	).Scan(&et.ID, &et.AsignaturaID, &et.Asignatura, &et.Nombre, &et.Estado)
	if errors.Is(err, sql.ErrNoRows) {
		return EtiquetaResponse{}, ErrNotFound
	}
	return et, err
}

// DeleteEtiqueta permanently removes a tag.
func (r *Repository) DeleteEtiqueta(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM etiquetas_pregunta WHERE id = $1`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ── Generate Set ──────────────────────────────────────────────────────────────

// GenerateSet returns a randomized set of questions for an exam, without answer keys.
func (r *Repository) GenerateSet(ctx context.Context, req GenerateSetRequest) ([]PreguntaExamenItem, error) {
	var allItems []PreguntaExamenItem
	orden := 1

	for _, item := range req.Asignaturas {
		args := []any{req.ProcesoID, item.AsignaturaID}
		i := 3
		where := []string{"p.proceso_id = $1", "p.asignatura_id = $2", PreguntaServible("p")}

		if req.GradoID != nil {
			where = append(where, fmt.Sprintf("(p.grado_id = $%d OR p.grado_id IS NULL)", i))
			args = append(args, *req.GradoID)
			i++
		}
		if req.EspecialidadID != nil {
			where = append(where, fmt.Sprintf("(p.especialidad_id = $%d OR p.especialidad_id IS NULL)", i))
			args = append(args, *req.EspecialidadID)
			i++
		}
		if item.Dificultad != "" {
			where = append(where, fmt.Sprintf("p.dificultad = $%d", i))
			args = append(args, item.Dificultad)
			i++
		}
		if len(req.ExcluirIDs) > 0 {
			placeholders := make([]string, len(req.ExcluirIDs))
			for j, eid := range req.ExcluirIDs {
				placeholders[j] = fmt.Sprintf("$%d", i)
				args = append(args, eid)
				i++
			}
			where = append(where, fmt.Sprintf("p.id NOT IN (%s)", strings.Join(placeholders, ", ")))
		}

		args = append(args, item.NumeroPreguntas)
		query := fmt.Sprintf(`
			SELECT p.id, p.enunciado, p.tipo, a.nombre AS asignatura
			FROM preguntas p
			JOIN asignaturas a ON a.id = p.asignatura_id
			WHERE %s
			ORDER BY RANDOM()
			LIMIT $%d`, strings.Join(where, " AND "), i)

		rows, err := r.db.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, err
		}

		var pregIDs []int64
		var pregItems []PreguntaExamenItem
		for rows.Next() {
			var p PreguntaExamenItem
			if err := rows.Scan(&p.ID, &p.Enunciado, &p.Tipo, &p.Asignatura); err != nil {
				rows.Close()
				return nil, err
			}
			p.AsignaturaID = item.AsignaturaID
			pregIDs = append(pregIDs, p.ID)
			pregItems = append(pregItems, p)
		}
		rows.Close()

		if len(pregItems) < item.NumeroPreguntas {
			return nil, fmt.Errorf("%w: asignatura_id=%d solicita %d preguntas pero solo hay %d disponibles",
				ErrInsufficientQuestions, item.AsignaturaID, item.NumeroPreguntas, len(pregItems))
		}

		// Batch-fetch respuestas for all selected questions in one query
		if len(pregIDs) > 0 {
			placeholders := make([]string, len(pregIDs))
			idArgs := make([]any, len(pregIDs))
			for j, pid := range pregIDs {
				placeholders[j] = fmt.Sprintf("$%d", j+1)
				idArgs[j] = pid
			}
			respRows, err := r.db.QueryContext(ctx,
				fmt.Sprintf(`SELECT id, pregunta_id, texto, orden FROM respuestas
				             WHERE pregunta_id IN (%s)
				             ORDER BY pregunta_id, RANDOM()`, strings.Join(placeholders, ",")),
				idArgs...,
			)
			if err != nil {
				return nil, err
			}
			respByPregunta := map[int64][]RespuestaExamen{}
			for respRows.Next() {
				var re RespuestaExamen
				var pregID int64
				if err := respRows.Scan(&re.ID, &pregID, &re.Texto, &re.Orden); err != nil {
					respRows.Close()
					return nil, err
				}
				respByPregunta[pregID] = append(respByPregunta[pregID], re)
			}
			respRows.Close()
			if err := respRows.Err(); err != nil {
				return nil, err
			}

			for idx := range pregItems {
				resps := respByPregunta[pregItems[idx].ID]
				if resps == nil {
					resps = []RespuestaExamen{}
				}
				pregItems[idx].Respuestas = resps
				pregItems[idx].Orden = orden
				orden++
				allItems = append(allItems, pregItems[idx])
			}
		}
	}

	return allItems, nil
}

// ── Statistics ────────────────────────────────────────────────────────────────

// UpsertEstadisticas applies a batch of attempt outcomes (one row per answered
// question) to pregunta_estadisticas in a single statement.
func (r *Repository) UpsertEstadisticas(ctx context.Context, intentos []grading.Intento) error {
	ids := make([]int64, len(intentos))
	ok := make([]bool, len(intentos))
	for i, it := range intentos {
		ids[i] = it.PreguntaID
		ok[i] = it.Correcta
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO pregunta_estadisticas AS pe (pregunta_id, total_intentos, total_aciertos, indice_dificultad, updated_at)
		SELECT v.pregunta_id, v.n, v.ok, v.ok::decimal / v.n, NOW()
		FROM (
			SELECT t.pregunta_id, COUNT(*)::int AS n, (COUNT(*) FILTER (WHERE t.correcta))::int AS ok
			FROM unnest($1::bigint[], $2::bool[]) AS t(pregunta_id, correcta)
			GROUP BY t.pregunta_id
		) v
		-- Fixed lock order (H134): the hash aggregate's output order follows
		-- the input order on collisions, so two attempts sharing questions
		-- would lock pregunta_estadisticas rows in different orders and deadlock.
		ORDER BY v.pregunta_id
		ON CONFLICT (pregunta_id) DO UPDATE
		SET total_intentos    = pe.total_intentos + EXCLUDED.total_intentos,
		    total_aciertos    = pe.total_aciertos + EXCLUDED.total_aciertos,
		    indice_dificultad = (pe.total_aciertos + EXCLUDED.total_aciertos)::decimal /
		                        (pe.total_intentos + EXCLUDED.total_intentos),
		    updated_at        = NOW()`,
		pq.Array(ids), pq.Array(ok),
	)
	return err
}

// GetGlobalStats returns aggregated question bank statistics.
func (r *Repository) GetGlobalStats(ctx context.Context) (StatsGlobalesResponse, error) {
	var stats StatsGlobalesResponse

	// Total + breakdowns in one pass over the table
	err := r.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*)                                          AS total,
			COUNT(*) FILTER (WHERE dificultad='facil')        AS facil,
			COUNT(*) FILTER (WHERE dificultad='medio')        AS medio,
			COUNT(*) FILTER (WHERE dificultad='dificil')      AS dificil,
			COUNT(*) FILTER (WHERE tipo='unica')              AS unica,
			COUNT(*) FILTER (WHERE tipo='multiple')           AS multiple
		FROM preguntas
		WHERE estado != 'eliminada'`).Scan(
		&stats.TotalPreguntas,
		&stats.PorDificultad.Facil,
		&stats.PorDificultad.Medio,
		&stats.PorDificultad.Dificil,
		&stats.PorTipo.Unica,
		&stats.PorTipo.Multiple,
	)
	if err != nil {
		return stats, err
	}

	// Por proceso (top 10)
	procRows, err := r.db.QueryContext(ctx, `
		SELECT pr.nombre, COUNT(p.id) AS total
		FROM preguntas p
		JOIN procesos pr ON pr.id = p.proceso_id
		WHERE p.estado != 'eliminada'
		GROUP BY pr.nombre
		ORDER BY total DESC
		LIMIT 10`)
	if err != nil {
		return stats, err
	}
	defer procRows.Close()
	for procRows.Next() {
		var item PorProcesoItem
		if err := procRows.Scan(&item.Proceso, &item.Total); err != nil {
			return stats, err
		}
		stats.PorProceso = append(stats.PorProceso, item)
	}
	if stats.PorProceso == nil {
		stats.PorProceso = []PorProcesoItem{}
	}

	return stats, nil
}

// citaConTexto is a citation row with the raw passage text, which never leaves
// the repository: only its excerpt does.
type citaConTexto struct {
	CitaResponse
	Texto string `json:"texto"`
}

// decodificarCitas turns the JSON_AGG of GetPregunta into CitaResponse with
// the excerpt of each passage (contract E3-07 §5).
func decodificarCitas(raw []byte) ([]CitaResponse, error) {
	var filas []citaConTexto
	if err := json.Unmarshal(raw, &filas); err != nil {
		return nil, err
	}
	citas := make([]CitaResponse, 0, len(filas))
	for _, f := range filas {
		f.Extracto = texto.Extracto(f.Texto)
		citas = append(citas, f.CitaResponse)
	}
	return citas, nil
}
