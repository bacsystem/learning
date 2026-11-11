package catalogs

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/cbaciliod/learning/internal/audit"
	"github.com/jmoiron/sqlx"
)

// Repository wraps the database connection for the catalogs package.
type Repository struct {
	db *sqlx.DB
}

// NewRepository creates a new catalogs Repository.
func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

const (
	// maxLimit caps an explicit ?limit=N.
	maxLimit = 100
	// maxAll is the safety ceiling for ?limit=all (or no limit): catalogs are
	// small reference tables the UI loads whole into dropdowns.
	maxAll = 1000
	// maxPage bounds ?page so the OFFSET can never overflow.
	maxPage = 100_000
)

// pageClause renders "LIMIT $idx OFFSET $idx+1" for q (page is 1-based).
func pageClause(q ListQuery, idx int) (string, []any) {
	limit := q.Limit
	if limit <= 0 {
		limit = maxAll
	}
	page := max(q.Page, 1)
	return fmt.Sprintf(" LIMIT $%d OFFSET $%d", idx, idx+1), []any{limit, int64(page-1) * int64(limit)}
}

// ── Instituciones ─────────────────────────────────────────────────────────────

func (r *Repository) ListInstituciones(ctx context.Context, q ListQuery) ([]InstitucionResponse, error) {
	query := `SELECT id, nombre, siglas, estado, created_at::text FROM instituciones WHERE 1=1`
	args := []any{}
	i := 1
	if q.Estado != "" {
		query += fmt.Sprintf(" AND estado = $%d", i)
		args = append(args, q.Estado)
		i++
	}
	query += " ORDER BY nombre ASC, id ASC"
	clause, pargs := pageClause(q, i)
	query += clause
	args = append(args, pargs...)

	rows, err := r.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []InstitucionResponse
	for rows.Next() {
		var row InstitucionResponse
		if err := rows.Scan(&row.ID, &row.Nombre, &row.Siglas, &row.Estado, &row.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (r *Repository) GetInstitucion(ctx context.Context, id int64) (*InstitucionResponse, error) {
	var row InstitucionResponse
	err := r.db.QueryRowContext(ctx,
		`SELECT id, nombre, siglas, estado, created_at::text FROM instituciones WHERE id = $1`, id,
	).Scan(&row.ID, &row.Nombre, &row.Siglas, &row.Estado, &row.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &row, err
}

func (r *Repository) CreateInstitucion(ctx context.Context, req CreateInstitucionRequest) (*InstitucionResponse, error) {
	var row InstitucionResponse
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO instituciones (nombre, siglas) VALUES ($1, $2)
		 RETURNING id, nombre, siglas, estado, created_at::text`,
		req.Nombre, req.Siglas,
	).Scan(&row.ID, &row.Nombre, &row.Siglas, &row.Estado, &row.CreatedAt)
	return &row, err
}

func (r *Repository) UpdateInstitucion(ctx context.Context, id int64, req UpdateInstitucionRequest) (*InstitucionResponse, error) {
	var row InstitucionResponse
	err := r.db.QueryRowContext(ctx,
		`UPDATE instituciones SET nombre=$1, siglas=$2, updated_at=NOW()
		 WHERE id=$3
		 RETURNING id, nombre, siglas, estado, created_at::text`,
		req.Nombre, req.Siglas, id,
	).Scan(&row.ID, &row.Nombre, &row.Siglas, &row.Estado, &row.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &row, err
}

// UpdateInstitucionEstado updates estado and, when deactivating, cascades to
// grados, especialidades, and procesos in a single DB transaction. auditFn
// writes the audit row in that transaction; its error rolls everything back.
func (r *Repository) UpdateInstitucionEstado(ctx context.Context, id int64, estado string, auditFn audit.TxFunc) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	res, err := tx.ExecContext(ctx,
		`UPDATE instituciones SET estado=$1, updated_at=NOW() WHERE id=$2`, estado, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		err = ErrNotFound
		return err
	}

	// Cascade deactivation only — do NOT cascade on reactivate
	if estado == "inactivo" {
		if _, err = tx.ExecContext(ctx,
			`UPDATE grados SET estado='inactivo', updated_at=NOW() WHERE institucion_id=$1`, id); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx,
			`UPDATE especialidades SET estado='inactivo', updated_at=NOW() WHERE institucion_id=$1`, id); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx,
			`UPDATE procesos SET estado='inactivo', updated_at=NOW() WHERE institucion_id=$1`, id); err != nil {
			return err
		}
	}
	if err = audit.RunTx(ctx, tx, auditFn); err != nil {
		return err
	}

	return tx.Commit()
}

// ── Grados ────────────────────────────────────────────────────────────────────

func (r *Repository) ListGrados(ctx context.Context, q ListQuery) ([]GradoResponse, error) {
	query := `
		SELECT g.id, g.institucion_id, i.nombre AS institucion,
		       g.nombre, g.abreviatura, g.nivel, g.orden, g.estado, g.created_at::text
		FROM grados g
		JOIN instituciones i ON i.id = g.institucion_id
		WHERE 1=1`
	args := []any{}
	i := 1
	if q.InstitucionID > 0 {
		query += fmt.Sprintf(" AND g.institucion_id = $%d", i)
		args = append(args, q.InstitucionID)
		i++
	}
	if q.Estado != "" {
		query += fmt.Sprintf(" AND g.estado = $%d", i)
		args = append(args, q.Estado)
		i++
	}
	query += " ORDER BY g.orden ASC, g.nombre ASC, g.id ASC"
	clause, pargs := pageClause(q, i)
	query += clause
	args = append(args, pargs...)

	rows, err := r.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []GradoResponse
	for rows.Next() {
		var row GradoResponse
		if err := rows.Scan(&row.ID, &row.InstitucionID, &row.Institucion,
			&row.Nombre, &row.Abreviatura, &row.Nivel, &row.Orden, &row.Estado, &row.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (r *Repository) GetGrado(ctx context.Context, id int64) (*GradoResponse, error) {
	var row GradoResponse
	err := r.db.QueryRowContext(ctx, `
		SELECT g.id, g.institucion_id, i.nombre AS institucion,
		       g.nombre, g.abreviatura, g.nivel, g.orden, g.estado, g.created_at::text
		FROM grados g
		JOIN instituciones i ON i.id = g.institucion_id
		WHERE g.id = $1`, id,
	).Scan(&row.ID, &row.InstitucionID, &row.Institucion,
		&row.Nombre, &row.Abreviatura, &row.Nivel, &row.Orden, &row.Estado, &row.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &row, err
}

func (r *Repository) CreateGrado(ctx context.Context, req CreateGradoRequest) (*GradoResponse, error) {
	var row GradoResponse
	var instNombre string
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO grados (institucion_id, nombre, abreviatura, nivel, orden)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, institucion_id, nombre, abreviatura, nivel, orden, estado, created_at::text`,
		req.InstitucionID, req.Nombre, req.Abreviatura, req.Nivel, req.Orden,
	).Scan(&row.ID, &row.InstitucionID, &row.Nombre, &row.Abreviatura, &row.Nivel, &row.Orden, &row.Estado, &row.CreatedAt)
	if err != nil {
		return nil, err
	}
	if err := r.db.QueryRowContext(ctx, `SELECT nombre FROM instituciones WHERE id=$1`, row.InstitucionID).Scan(&instNombre); err != nil {
		return nil, err
	}
	row.Institucion = instNombre
	return &row, nil
}

func (r *Repository) UpdateGrado(ctx context.Context, id int64, req CreateGradoRequest) (*GradoResponse, error) {
	var row GradoResponse
	var instNombre string
	err := r.db.QueryRowContext(ctx,
		`UPDATE grados SET institucion_id=$1, nombre=$2, abreviatura=$3, nivel=$4, orden=$5, updated_at=NOW()
		 WHERE id=$6
		 RETURNING id, institucion_id, nombre, abreviatura, nivel, orden, estado, created_at::text`,
		req.InstitucionID, req.Nombre, req.Abreviatura, req.Nivel, req.Orden, id,
	).Scan(&row.ID, &row.InstitucionID, &row.Nombre, &row.Abreviatura, &row.Nivel, &row.Orden, &row.Estado, &row.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := r.db.QueryRowContext(ctx, `SELECT nombre FROM instituciones WHERE id=$1`, row.InstitucionID).Scan(&instNombre); err != nil {
		return nil, err
	}
	row.Institucion = instNombre
	return &row, nil
}

func (r *Repository) UpdateGradoEstado(ctx context.Context, id int64, estado string) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE grados SET estado=$1, updated_at=NOW() WHERE id=$2`, estado, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) CountActiveUsersByGrado(ctx context.Context, gradoID int64) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM personas p
		 JOIN usuarios u ON u.persona_id = p.id
		 WHERE p.grado_id = $1 AND u.estado = 'activo'`, gradoID,
	).Scan(&n)
	return n, err
}

// FindGradoAbreviatura returns the abreviatura for a grado by ID.
func (r *Repository) FindGradoAbreviatura(ctx context.Context, gradoID int64) (string, error) {
	var abrev string
	err := r.db.QueryRowContext(ctx,
		`SELECT abreviatura FROM grados WHERE id=$1`, gradoID,
	).Scan(&abrev)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return abrev, err
}

// ── Especialidades ────────────────────────────────────────────────────────────

func (r *Repository) ListEspecialidades(ctx context.Context, q ListQuery) ([]EspecialidadResponse, error) {
	query := `
		SELECT e.id, e.institucion_id, i.nombre AS institucion,
		       e.nombre, e.codigo, COALESCE(e.nivel,''), e.estado, e.created_at::text
		FROM especialidades e
		JOIN instituciones i ON i.id = e.institucion_id
		WHERE 1=1`
	args := []any{}
	idx := 1
	if q.InstitucionID > 0 {
		query += fmt.Sprintf(" AND e.institucion_id = $%d", idx)
		args = append(args, q.InstitucionID)
		idx++
	}
	if q.Estado != "" {
		query += fmt.Sprintf(" AND e.estado = $%d", idx)
		args = append(args, q.Estado)
		idx++
	}
	if q.Nivel != "" {
		query += fmt.Sprintf(" AND e.nivel = $%d", idx)
		args = append(args, q.Nivel)
		idx++
	}
	query += " ORDER BY e.nombre ASC, e.id ASC"
	clause, pargs := pageClause(q, idx)
	query += clause
	args = append(args, pargs...)

	rows, err := r.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []EspecialidadResponse
	for rows.Next() {
		var row EspecialidadResponse
		if err := rows.Scan(&row.ID, &row.InstitucionID, &row.Institucion,
			&row.Nombre, &row.Codigo, &row.Nivel, &row.Estado, &row.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (r *Repository) GetEspecialidad(ctx context.Context, id int64) (*EspecialidadResponse, error) {
	var row EspecialidadResponse
	err := r.db.QueryRowContext(ctx, `
		SELECT e.id, e.institucion_id, i.nombre AS institucion,
		       e.nombre, e.codigo, COALESCE(e.nivel,''), e.estado, e.created_at::text
		FROM especialidades e
		JOIN instituciones i ON i.id = e.institucion_id
		WHERE e.id = $1`, id,
	).Scan(&row.ID, &row.InstitucionID, &row.Institucion, &row.Nombre, &row.Codigo, &row.Nivel, &row.Estado, &row.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &row, err
}

func (r *Repository) CreateEspecialidad(ctx context.Context, req CreateEspecialidadRequest) (*EspecialidadResponse, error) {
	var row EspecialidadResponse
	var instNombre string
	var nivel *string
	if req.Nivel != "" {
		nivel = &req.Nivel
	}
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO especialidades (institucion_id, nombre, codigo, nivel)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, institucion_id, nombre, codigo, COALESCE(nivel,''), estado, created_at::text`,
		req.InstitucionID, req.Nombre, req.Codigo, nivel,
	).Scan(&row.ID, &row.InstitucionID, &row.Nombre, &row.Codigo, &row.Nivel, &row.Estado, &row.CreatedAt)
	if err != nil {
		return nil, err
	}
	if err := r.db.QueryRowContext(ctx, `SELECT nombre FROM instituciones WHERE id=$1`, row.InstitucionID).Scan(&instNombre); err != nil {
		return nil, err
	}
	row.Institucion = instNombre
	return &row, nil
}

func (r *Repository) UpdateEspecialidad(ctx context.Context, id int64, req CreateEspecialidadRequest) (*EspecialidadResponse, error) {
	var row EspecialidadResponse
	var instNombre string
	var nivel *string
	if req.Nivel != "" {
		nivel = &req.Nivel
	}
	err := r.db.QueryRowContext(ctx,
		`UPDATE especialidades SET institucion_id=$1, nombre=$2, codigo=$3, nivel=$4, updated_at=NOW()
		 WHERE id=$5
		 RETURNING id, institucion_id, nombre, codigo, COALESCE(nivel,''), estado, created_at::text`,
		req.InstitucionID, req.Nombre, req.Codigo, nivel, id,
	).Scan(&row.ID, &row.InstitucionID, &row.Nombre, &row.Codigo, &row.Nivel, &row.Estado, &row.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := r.db.QueryRowContext(ctx, `SELECT nombre FROM instituciones WHERE id=$1`, row.InstitucionID).Scan(&instNombre); err != nil {
		return nil, err
	}
	row.Institucion = instNombre
	return &row, nil
}

func (r *Repository) UpdateEspecialidadEstado(ctx context.Context, id int64, estado string) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE especialidades SET estado=$1, updated_at=NOW() WHERE id=$2`, estado, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) CountActiveUsersByEspecialidad(ctx context.Context, espID int64) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM personas p
		 JOIN usuarios u ON u.persona_id = p.id
		 WHERE p.especialidad_id = $1 AND u.estado = 'activo'`, espID,
	).Scan(&n)
	return n, err
}

// ── Asignaturas ───────────────────────────────────────────────────────────────

func (r *Repository) ListAsignaturas(ctx context.Context, q ListQuery) ([]AsignaturaResponse, error) {
	query := `SELECT id, nombre, codigo, estado, created_at::text FROM asignaturas WHERE 1=1`
	args := []any{}
	i := 1
	if q.Estado != "" {
		query += fmt.Sprintf(" AND estado = $%d", i)
		args = append(args, q.Estado)
		i++
	}
	query += " ORDER BY nombre ASC, id ASC"
	clause, pargs := pageClause(q, i)
	query += clause
	args = append(args, pargs...)

	rows, err := r.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []AsignaturaResponse
	for rows.Next() {
		var row AsignaturaResponse
		if err := rows.Scan(&row.ID, &row.Nombre, &row.Codigo, &row.Estado, &row.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (r *Repository) GetAsignatura(ctx context.Context, id int64) (*AsignaturaResponse, error) {
	var row AsignaturaResponse
	err := r.db.QueryRowContext(ctx,
		`SELECT id, nombre, codigo, estado, created_at::text FROM asignaturas WHERE id=$1`, id,
	).Scan(&row.ID, &row.Nombre, &row.Codigo, &row.Estado, &row.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &row, err
}

func (r *Repository) CreateAsignatura(ctx context.Context, req CreateAsignaturaRequest) (*AsignaturaResponse, error) {
	var row AsignaturaResponse
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO asignaturas (nombre, codigo) VALUES ($1, $2)
		 RETURNING id, nombre, codigo, estado, created_at::text`,
		req.Nombre, req.Codigo,
	).Scan(&row.ID, &row.Nombre, &row.Codigo, &row.Estado, &row.CreatedAt)
	return &row, err
}

func (r *Repository) UpdateAsignatura(ctx context.Context, id int64, req CreateAsignaturaRequest) (*AsignaturaResponse, error) {
	var row AsignaturaResponse
	err := r.db.QueryRowContext(ctx,
		`UPDATE asignaturas SET nombre=$1, codigo=$2, updated_at=NOW()
		 WHERE id=$3
		 RETURNING id, nombre, codigo, estado, created_at::text`,
		req.Nombre, req.Codigo, id,
	).Scan(&row.ID, &row.Nombre, &row.Codigo, &row.Estado, &row.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &row, err
}

func (r *Repository) UpdateAsignaturaEstado(ctx context.Context, id int64, estado string) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE asignaturas SET estado=$1, updated_at=NOW() WHERE id=$2`, estado, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ── Procesos ──────────────────────────────────────────────────────────────────

func (r *Repository) ListProcesos(ctx context.Context, q ListQuery) ([]ProcesoResponse, error) {
	query := `
		SELECT p.id, p.institucion_id, i.nombre AS institucion,
		       p.nombre, p.tipo, p.grado_id, g.nombre AS grado,
		       COALESCE(p.descripcion, '') AS descripcion, p.anio, p.estado, p.created_at::text
		FROM procesos p
		JOIN instituciones i ON i.id = p.institucion_id
		LEFT JOIN grados g ON g.id = p.grado_id
		WHERE 1=1`
	args := []any{}
	idx := 1
	if q.InstitucionID > 0 {
		query += fmt.Sprintf(" AND p.institucion_id = $%d", idx)
		args = append(args, q.InstitucionID)
		idx++
	}
	if q.Tipo != "" {
		query += fmt.Sprintf(" AND p.tipo = $%d", idx)
		args = append(args, q.Tipo)
		idx++
	}
	if q.Anio > 0 {
		query += fmt.Sprintf(" AND p.anio = $%d", idx)
		args = append(args, q.Anio)
		idx++
	}
	if q.Estado != "" {
		query += fmt.Sprintf(" AND p.estado = $%d", idx)
		args = append(args, q.Estado)
		idx++
	}
	query += " ORDER BY p.anio DESC, p.nombre ASC, p.id ASC"
	clause, pargs := pageClause(q, idx)
	query += clause
	args = append(args, pargs...)

	rows, err := r.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ProcesoResponse
	for rows.Next() {
		var row ProcesoResponse
		var gradoNombre sql.NullString
		if err := rows.Scan(&row.ID, &row.InstitucionID, &row.Institucion,
			&row.Nombre, &row.Tipo, &row.GradoID, &gradoNombre,
			&row.Descripcion, &row.Anio, &row.Estado, &row.CreatedAt); err != nil {
			return nil, err
		}
		if gradoNombre.Valid {
			row.Grado = &gradoNombre.String
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (r *Repository) GetProceso(ctx context.Context, id int64) (*ProcesoDetailResponse, error) {
	var row ProcesoDetailResponse
	var gradoNombre sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT p.id, p.institucion_id, i.nombre AS institucion,
		       p.nombre, p.tipo, p.grado_id, g.nombre AS grado,
		       COALESCE(p.descripcion, '') AS descripcion, p.anio, p.estado, p.created_at::text
		FROM procesos p
		JOIN instituciones i ON i.id = p.institucion_id
		LEFT JOIN grados g ON g.id = p.grado_id
		WHERE p.id = $1`, id,
	).Scan(&row.ID, &row.InstitucionID, &row.Institucion,
		&row.Nombre, &row.Tipo, &row.GradoID, &gradoNombre,
		&row.Descripcion, &row.Anio, &row.Estado, &row.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if gradoNombre.Valid {
		row.Grado = &gradoNombre.String
	}

	// Load asignaturas
	asigs, err := r.ListProcesoAsignaturasCompatibilidad(ctx, id, 0, 0)
	if err != nil {
		return nil, err
	}
	row.Asignaturas = asigs
	return &row, nil
}

func (r *Repository) CreateProceso(ctx context.Context, req CreateProcesoRequest) (*ProcesoResponse, error) {
	var row ProcesoResponse
	var gradoNombre sql.NullString
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO procesos (institucion_id, nombre, tipo, grado_id, descripcion, anio)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, institucion_id, nombre, tipo, grado_id, COALESCE(descripcion, ''), anio, estado, created_at::text`,
		req.InstitucionID, req.Nombre, req.Tipo, req.GradoID, req.Descripcion, req.Anio,
	).Scan(&row.ID, &row.InstitucionID, &row.Nombre, &row.Tipo, &row.GradoID,
		&row.Descripcion, &row.Anio, &row.Estado, &row.CreatedAt)
	if err != nil {
		return nil, err
	}
	if err := r.fillProcesoNames(ctx, &row, &gradoNombre); err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *Repository) UpdateProceso(ctx context.Context, id int64, req CreateProcesoRequest) (*ProcesoResponse, error) {
	var row ProcesoResponse
	var gradoNombre sql.NullString
	err := r.db.QueryRowContext(ctx,
		`UPDATE procesos SET institucion_id=$1, nombre=$2, tipo=$3, grado_id=$4,
		 descripcion=$5, anio=$6, updated_at=NOW()
		 WHERE id=$7
		 RETURNING id, institucion_id, nombre, tipo, grado_id, COALESCE(descripcion, ''), anio, estado, created_at::text`,
		req.InstitucionID, req.Nombre, req.Tipo, req.GradoID,
		req.Descripcion, req.Anio, id,
	).Scan(&row.ID, &row.InstitucionID, &row.Nombre, &row.Tipo, &row.GradoID,
		&row.Descripcion, &row.Anio, &row.Estado, &row.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := r.fillProcesoNames(ctx, &row, &gradoNombre); err != nil {
		return nil, err
	}
	return &row, nil
}

// fillProcesoNames resolves the institucion and grado display names.
func (r *Repository) fillProcesoNames(ctx context.Context, row *ProcesoResponse, gradoNombre *sql.NullString) error {
	if err := r.db.QueryRowContext(ctx, `SELECT nombre FROM instituciones WHERE id=$1`, row.InstitucionID).Scan(&row.Institucion); err != nil {
		return err
	}
	if row.GradoID != nil {
		if err := r.db.QueryRowContext(ctx, `SELECT nombre FROM grados WHERE id=$1`, *row.GradoID).Scan(gradoNombre); err != nil {
			return err
		}
		if gradoNombre.Valid {
			row.Grado = &gradoNombre.String
		}
	}
	return nil
}

// UpdateProcesoEstado changes a proceso's estado. Activation requires the
// asignatura weights to add up to 100 (ErrValidation otherwise); the proceso
// row is locked so a concurrent weight edit cannot slip in between the check
// and the update. auditFn writes the audit row in the same transaction.
func (r *Repository) UpdateProcesoEstado(ctx context.Context, id int64, estado string, auditFn audit.TxFunc) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := lockProceso(ctx, tx, id); err != nil {
		return err
	}
	if estado == "activo" {
		sum, err := sumPesoPorcentual(ctx, tx, id)
		if err != nil {
			return err
		}
		if !pesoValido(sum) {
			return fmt.Errorf("%w: la suma de pesos porcentuales debe ser 100%% (actual: %.2f%%)", ErrValidation, sum)
		}
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE procesos SET estado=$1, updated_at=NOW() WHERE id=$2`, estado, id); err != nil {
		return err
	}
	if err := audit.RunTx(ctx, tx, auditFn); err != nil {
		return err
	}
	return tx.Commit()
}

// SumPesoPorcentual returns the sum of peso_porcentual of the compatibility
// rows (no especialidad) of a proceso: they are the general configuration that
// activation requires at 100% (ADR 0001, L1).
func (r *Repository) SumPesoPorcentual(ctx context.Context, procesoID int64) (float64, error) {
	return sumPesoPorcentual(ctx, r.db, procesoID)
}

func sumPesoPorcentual(ctx context.Context, q sqlx.QueryerContext, procesoID int64) (float64, error) {
	var sum float64
	err := q.QueryRowxContext(ctx,
		`SELECT COALESCE(SUM(peso_porcentual), 0) FROM proceso_asignaturas WHERE proceso_id=$1 AND especialidad_id IS NULL`, procesoID,
	).Scan(&sum)
	return sum, err
}

// pesoValido reports whether a weight sum is 100 (±0.01, so that a three-way
// 33.33 split is accepted). It compares whole hundredths, the same rule as
// PE3 and the frontend (sumaCentesimasValida), so no float edge differs.
func pesoValido(sum float64) bool { return sumaCentesimasValida(centesimas(sum)) }

// lockProceso locks the proceso row for the rest of tx and returns its estado.
func lockProceso(ctx context.Context, tx *sqlx.Tx, id int64) (string, error) {
	var estado string
	err := tx.QueryRowContext(ctx, `SELECT estado FROM procesos WHERE id=$1 FOR UPDATE`, id).Scan(&estado)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return estado, err
}

// checkPesoActivo rejects (ErrConflict) a change that leaves an active
// proceso with weights that do not add up to 100: grading would silently use
// the wrong total. Inactive procesos can be edited freely and are validated on
// activation.
func checkPesoActivo(ctx context.Context, tx *sqlx.Tx, procesoID int64, estado string) error {
	if estado != "activo" {
		return nil
	}
	sum, err := sumPesoPorcentual(ctx, tx, procesoID)
	if err != nil {
		return err
	}
	if !pesoValido(sum) {
		return fmt.Errorf("%w: el proceso está activo y la suma de pesos quedaría en %.2f%% (debe ser 100%%). Desactívalo para reorganizar los pesos.", ErrConflict, sum)
	}
	return nil
}

// ── Proceso-Asignaturas ───────────────────────────────────────────────────────

// ListProcesoAsignaturas lists the set of asignaturas that applies to the
// user's especialidad (ADR 0001, L2): its own set if the proceso has one,
// otherwise the compatibility rows. Counts are filtered by grado/especialidad.
func (r *Repository) ListProcesoAsignaturas(ctx context.Context, procesoID, gradoID, especialidadID int64) ([]ProcesoAsignaturaItem, error) {
	return r.listProcesoAsignaturas(ctx, ConjuntoDePesos("pa", 1, 3), procesoID, gradoID, especialidadID)
}

// ListProcesoAsignaturasCompatibilidad lists only the compatibility rows: the
// admin catalog view. Especialidad sets have their own endpoints (PE1–PE3).
func (r *Repository) ListProcesoAsignaturasCompatibilidad(ctx context.Context, procesoID, gradoID, especialidadID int64) ([]ProcesoAsignaturaItem, error) {
	return r.listProcesoAsignaturas(ctx, filtroCompatibilidad("pa", 1), procesoID, gradoID, especialidadID)
}

// listProcesoAsignaturas runs the listing with filtro selecting the rows; $1 is
// the proceso, $2 the grado and $3 the especialidad used to count questions.
func (r *Repository) listProcesoAsignaturas(ctx context.Context, filtro string, procesoID, gradoID, especialidadID int64) ([]ProcesoAsignaturaItem, error) {
	rows, err := r.db.QueryxContext(ctx, `
		SELECT pa.asignatura_id, a.nombre, a.codigo, pa.numero_preguntas, pa.peso_porcentual, pa.coeficiente,
		       (SELECT COUNT(*) FROM preguntas p
		        WHERE p.proceso_id = pa.proceso_id
		          AND p.asignatura_id = pa.asignatura_id
		          AND p.estado = 'activo'
		          AND ($2 = 0 OR p.grado_id = $2 OR p.grado_id IS NULL)
		          AND ($3::bigint = 0 OR p.especialidad_id = $3 OR p.especialidad_id IS NULL)) AS preguntas_disponibles
		FROM proceso_asignaturas pa
		JOIN asignaturas a ON a.id = pa.asignatura_id
		WHERE `+filtro+`
		ORDER BY a.nombre ASC`, procesoID, gradoID, especialidadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ProcesoAsignaturaItem
	for rows.Next() {
		var item ProcesoAsignaturaItem
		if err := rows.Scan(&item.AsignaturaID, &item.Nombre, &item.Codigo,
			&item.NumeroPreguntas, &item.PesoPorcentual, &item.Coeficiente, &item.PreguntasDisponibles); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

// AsignarAsignatura adds a compatibility row (no especialidad) to a proceso.
// ErrNotFound: unknown proceso; ErrPesoDuplicado: already assigned;
// ErrConflict: would break an active proceso's 100% weights; FK error (→ ErrInvalidRef in the service): unknown asignatura.
// auditFn writes the audit row in the same transaction.
func (r *Repository) AsignarAsignatura(ctx context.Context, procesoID int64, req AsignarAsignaturaRequest, auditFn audit.TxFunc) (*ProcesoAsignaturaItem, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	estado, err := lockProceso(ctx, tx, procesoID)
	if err != nil {
		return nil, err
	}
	var item ProcesoAsignaturaItem
	err = tx.QueryRowContext(ctx,
		`INSERT INTO proceso_asignaturas (proceso_id, asignatura_id, numero_preguntas, peso_porcentual, coeficiente)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (proceso_id, especialidad_id, asignatura_id) DO NOTHING
		 RETURNING asignatura_id, numero_preguntas, peso_porcentual, coeficiente`,
		procesoID, req.AsignaturaID, req.NumeroPreguntas, req.PesoPorcentual, req.Coeficiente,
	).Scan(&item.AsignaturaID, &item.NumeroPreguntas, &item.PesoPorcentual, &item.Coeficiente)
	// No especialidad_id: a compatibility row. The NULLS NOT DISTINCT
	// constraint is the arbiter, so a second one is skipped (ADR 0001, L3).
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrPesoDuplicado
	}
	if err != nil {
		return nil, err
	}
	if err := tx.QueryRowContext(ctx, `SELECT nombre, codigo FROM asignaturas WHERE id=$1`, item.AsignaturaID).
		Scan(&item.Nombre, &item.Codigo); err != nil {
		return nil, err
	}
	if err := checkPesoActivo(ctx, tx, procesoID, estado); err != nil {
		return nil, err
	}
	if err := audit.RunTx(ctx, tx, auditFn); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &item, nil
}

// UpdateProcesoAsignatura changes a compatibility row (ADR 0001, L4); see
// checkPesoActivo.
func (r *Repository) UpdateProcesoAsignatura(ctx context.Context, procesoID, asignaturaID int64, req UpdateProcesoAsignaturaRequest, auditFn audit.TxFunc) error {
	return r.mutateAsignacion(ctx, procesoID, auditFn, func(tx *sqlx.Tx) (sql.Result, error) {
		return tx.ExecContext(ctx,
			`UPDATE proceso_asignaturas SET numero_preguntas=$1, peso_porcentual=$2, coeficiente=$3
			 WHERE proceso_id=$4 AND asignatura_id=$5 AND especialidad_id IS NULL`,
			req.NumeroPreguntas, req.PesoPorcentual, req.Coeficiente, procesoID, asignaturaID)
	})
}

// DeleteProcesoAsignatura removes a compatibility row (ADR 0001, L5); see
// checkPesoActivo.
func (r *Repository) DeleteProcesoAsignatura(ctx context.Context, procesoID, asignaturaID int64, auditFn audit.TxFunc) error {
	return r.mutateAsignacion(ctx, procesoID, auditFn, func(tx *sqlx.Tx) (sql.Result, error) {
		return tx.ExecContext(ctx,
			`DELETE FROM proceso_asignaturas WHERE proceso_id=$1 AND asignatura_id=$2 AND especialidad_id IS NULL`,
			procesoID, asignaturaID)
	})
}

// mutateAsignacion runs one assignment change under the proceso row lock and
// commits only if it touched a row, kept an active proceso at 100% and its
// audit row (auditFn) was written in the same transaction.
func (r *Repository) mutateAsignacion(ctx context.Context, procesoID int64, auditFn audit.TxFunc, change func(*sqlx.Tx) (sql.Result, error)) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	estado, err := lockProceso(ctx, tx, procesoID)
	if err != nil {
		return err
	}
	res, err := change(tx)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	if err := checkPesoActivo(ctx, tx, procesoID, estado); err != nil {
		return err
	}
	if err := audit.RunTx(ctx, tx, auditFn); err != nil {
		return err
	}
	return tx.Commit()
}
