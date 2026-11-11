package temas

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// Audit actions and entities (docs/api/README.md §7).
const (
	accionCrear     = "tema.crear"
	accionEditar    = "tema.editar"
	accionArchivar  = "tema.archivar"
	accionReactivar = "tema.reactivar"
	accionReordenar = "tema.reordenar"
	entidadTema     = "tema"
	entidadArea     = "asignatura"

	// constraintNombre maps a race on the unique name to tema_duplicado.
	constraintNombre = "uq_temas_asignatura_nombre_clave"
)

// Repository is the PostgreSQL store of temas.
type Repository struct {
	db *sqlx.DB
}

// NewRepository creates a Repository over db.
func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

// temaColumns selects a Tema; the especialidades and the question count come
// from correlated subqueries so a page never runs one query per row.
const temaColumns = `
	t.id, t.asignatura_id, t.nombre, t.descripcion, t.estado, t.orden, t.created_at, t.updated_at,
	COALESCE((
		SELECT JSON_AGG(JSON_BUILD_OBJECT('id', e.id, 'nombre', e.nombre) ORDER BY e.nombre, e.id)
		FROM tema_especialidades te JOIN especialidades e ON e.id = te.especialidad_id
		WHERE te.tema_id = t.id), '[]'::json) AS especialidades,
	(SELECT COUNT(*) FROM preguntas p WHERE p.tema_id = t.id AND p.estado <> 'eliminada') AS preguntas_count`

// filtroEspecialidad keeps the temas of especialidad $n plus those that apply
// to every especialidad (no rows in tema_especialidades). $n = 0: no filter.
const filtroEspecialidad = `($%[1]d = 0
	OR NOT EXISTS (SELECT 1 FROM tema_especialidades te WHERE te.tema_id = t.id)
	OR EXISTS (SELECT 1 FROM tema_especialidades te WHERE te.tema_id = t.id AND te.especialidad_id = $%[1]d))`

type temaRow struct {
	ID             int64     `db:"id"`
	AsignaturaID   int64     `db:"asignatura_id"`
	Nombre         string    `db:"nombre"`
	Descripcion    *string   `db:"descripcion"`
	Estado         string    `db:"estado"`
	Orden          int       `db:"orden"`
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
	Especialidades []byte    `db:"especialidades"`
	PreguntasCount int       `db:"preguntas_count"`
}

func (r temaRow) tema() (Tema, error) {
	t := Tema{
		ID: r.ID, AsignaturaID: r.AsignaturaID, Nombre: r.Nombre, Descripcion: r.Descripcion,
		Estado: r.Estado, Orden: r.Orden, PreguntasCount: r.PreguntasCount,
		CreatedAt: r.CreatedAt.UTC(), UpdatedAt: r.UpdatedAt.UTC(),
	}
	if err := json.Unmarshal(r.Especialidades, &t.Especialidades); err != nil {
		return Tema{}, fmt.Errorf("especialidades del tema %d: %w", r.ID, err)
	}
	if t.Especialidades == nil {
		t.Especialidades = []EspecialidadRef{}
	}
	t.AplicaATodas = len(t.Especialidades) == 0
	return t, nil
}

// Listar returns one page of the temas of an asignatura, by orden then id.
func (r *Repository) Listar(ctx context.Context, f Filtro) ([]Tema, int, error) {
	where := `t.asignatura_id = $1 AND ($2 = '' OR t.estado = $2) AND ` + fmt.Sprintf(filtroEspecialidad, 3)
	args := []any{f.AsignaturaID, f.Estado, f.EspecialidadID}

	var total int
	if err := sqlx.GetContext(ctx, r.db, &total, `SELECT COUNT(*) FROM temas t WHERE `+where, args...); err != nil {
		return nil, 0, fmt.Errorf("contar temas: %w", err)
	}
	var rows []temaRow
	err := sqlx.SelectContext(ctx, r.db, &rows,
		`SELECT `+temaColumns+` FROM temas t WHERE `+where+` ORDER BY t.orden, t.id LIMIT $4 OFFSET $5`,
		append(args, f.PerPage, (f.Page-1)*f.PerPage)...)
	if err != nil {
		return nil, 0, fmt.Errorf("listar temas: %w", err)
	}
	temas := make([]Tema, 0, len(rows))
	for _, row := range rows {
		t, err := row.tema()
		if err != nil {
			return nil, 0, err
		}
		temas = append(temas, t)
	}
	return temas, total, nil
}

// Obtener returns one tema or ErrNoEncontrado.
func (r *Repository) Obtener(ctx context.Context, id int64) (Tema, error) {
	var row temaRow
	err := sqlx.GetContext(ctx, r.db, &row, `SELECT `+temaColumns+` FROM temas t WHERE t.id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Tema{}, ErrNoEncontrado
	}
	if err != nil {
		return Tema{}, fmt.Errorf("obtener tema %d: %w", id, err)
	}
	return row.tema()
}

// ListarActivos returns the active temas of an existing asignatura (T7), or
// ErrNoEncontrado when the asignatura does not exist.
func (r *Repository) ListarActivos(ctx context.Context, asignaturaID, especialidadID int64) ([]TemaActivo, error) {
	var existe bool
	if err := sqlx.GetContext(ctx, r.db, &existe,
		`SELECT EXISTS (SELECT 1 FROM asignaturas WHERE id = $1)`, asignaturaID); err != nil {
		return nil, fmt.Errorf("buscar asignatura %d: %w", asignaturaID, err)
	}
	if !existe {
		return nil, ErrAsignaturaNoEncontrada
	}
	var rows []struct {
		ID              int64   `db:"id"`
		Nombre          string  `db:"nombre"`
		Descripcion     *string `db:"descripcion"`
		Orden           int     `db:"orden"`
		EspecialidadIDs []byte  `db:"especialidad_ids"`
	}
	err := sqlx.SelectContext(ctx, r.db, &rows, `
		SELECT t.id, t.nombre, t.descripcion, t.orden,
			COALESCE((SELECT JSON_AGG(te.especialidad_id ORDER BY te.especialidad_id)
				FROM tema_especialidades te WHERE te.tema_id = t.id), '[]'::json) AS especialidad_ids
		FROM temas t
		WHERE t.asignatura_id = $1 AND t.estado = 'activo' AND `+fmt.Sprintf(filtroEspecialidad, 2)+`
		ORDER BY t.orden, t.id`, asignaturaID, especialidadID)
	if err != nil {
		return nil, fmt.Errorf("listar temas activos: %w", err)
	}
	temas := make([]TemaActivo, 0, len(rows))
	for _, row := range rows {
		t := TemaActivo{ID: row.ID, Nombre: row.Nombre, Descripcion: row.Descripcion, Orden: row.Orden}
		if err := json.Unmarshal(row.EspecialidadIDs, &t.EspecialidadIDs); err != nil {
			return nil, fmt.Errorf("especialidades del tema %d: %w", row.ID, err)
		}
		t.AplicaATodas = len(t.EspecialidadIDs) == 0
		temas = append(temas, t)
	}
	return temas, nil
}

// Crear inserts an active tema at the end of the order of its asignatura.
// Under the asignatura lock it checks, in the contract's order, the
// references, the unique name (archived temas included) and the active limit.
func (r *Repository) Crear(ctx context.Context, asignaturaID int64, d DatosTema, aud Auditor) (int64, error) {
	var id int64
	err := r.enTx(ctx, func(tx *sqlx.Tx) error {
		estado, err := bloquearAsignatura(ctx, tx, asignaturaID)
		if errors.Is(err, ErrNoEncontrado) || (err == nil && estado != EstadoActivo) {
			return ErrReferenciaInvalida
		}
		if err != nil {
			return err
		}
		if err := validarEspecialidades(ctx, tx, d.EspecialidadIDs); err != nil {
			return err
		}
		if err := comprobarNombreLibre(ctx, tx, asignaturaID, d.NombreClave, 0); err != nil {
			return err
		}
		if err := comprobarCupo(ctx, tx, asignaturaID); err != nil {
			return err
		}
		if err := tx.GetContext(ctx, &id, `
			INSERT INTO temas (asignatura_id, nombre, nombre_clave, descripcion, orden)
			VALUES ($1, $2, $3, $4, (`+siguienteOrden+`))
			RETURNING id`,
			asignaturaID, d.Nombre, d.NombreClave, d.Descripcion); err != nil {
			return fmt.Errorf("insertar tema: %w", err)
		}
		if err := guardarEspecialidades(ctx, tx, id, d.EspecialidadIDs); err != nil {
			return err
		}
		return aud(ctx, tx, Evento{Accion: accionCrear, Entidad: entidadTema, EntidadID: id, Detalle: map[string]any{
			"asignatura_id": asignaturaID, "nombre": d.Nombre, "especialidad_ids": d.EspecialidadIDs,
		}})
	})
	return id, err
}

// Editar replaces the editable fields of a tema, archived or not.
func (r *Repository) Editar(ctx context.Context, id int64, d DatosTema, aud Auditor) error {
	return r.enTx(ctx, func(tx *sqlx.Tx) error {
		var antes struct {
			AsignaturaID int64   `db:"asignatura_id"`
			Nombre       string  `db:"nombre"`
			Descripcion  *string `db:"descripcion"`
		}
		err := tx.GetContext(ctx, &antes,
			`SELECT asignatura_id, nombre, descripcion FROM temas WHERE id = $1 FOR UPDATE`, id)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNoEncontrado
		}
		if err != nil {
			return fmt.Errorf("bloquear tema %d: %w", id, err)
		}
		especialidadesAntes, err := especialidadesDe(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := validarEspecialidades(ctx, tx, d.EspecialidadIDs); err != nil {
			return err
		}
		if err := comprobarNombreLibre(ctx, tx, antes.AsignaturaID, d.NombreClave, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE temas SET nombre = $1, nombre_clave = $2, descripcion = $3, updated_at = NOW()
			WHERE id = $4`, d.Nombre, d.NombreClave, d.Descripcion, id); err != nil {
			return fmt.Errorf("editar tema %d: %w", id, err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM tema_especialidades WHERE tema_id = $1`, id); err != nil {
			return fmt.Errorf("borrar especialidades del tema %d: %w", id, err)
		}
		if err := guardarEspecialidades(ctx, tx, id, d.EspecialidadIDs); err != nil {
			return err
		}
		return aud(ctx, tx, Evento{Accion: accionEditar, Entidad: entidadTema, EntidadID: id, Detalle: map[string]any{
			"antes":   resumen(antes.Nombre, antes.Descripcion, especialidadesAntes),
			"despues": resumen(d.Nombre, d.Descripcion, d.EspecialidadIDs),
		}})
	})
}

// CambiarEstado archives or reactivates a tema. Both lock the asignatura and
// then the tema, the same order as Crear and Reordenar. Archiving leaves
// preguntas.tema_id alone; reactivating revalidates the limit and moves the
// tema to the end of the order.
func (r *Repository) CambiarEstado(ctx context.Context, id int64, nuevo string, aud Auditor) error {
	return r.enTx(ctx, func(tx *sqlx.Tx) error {
		// asignatura_id never changes, so reading it before the lock is safe.
		var asignaturaID int64
		err := tx.GetContext(ctx, &asignaturaID, `SELECT asignatura_id FROM temas WHERE id = $1`, id)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNoEncontrado
		}
		if err != nil {
			return fmt.Errorf("buscar tema %d: %w", id, err)
		}
		if _, err := bloquearAsignatura(ctx, tx, asignaturaID); err != nil {
			return err
		}
		var actual string
		if err := tx.GetContext(ctx, &actual, `SELECT estado FROM temas WHERE id = $1 FOR UPDATE`, id); err != nil {
			return fmt.Errorf("bloquear tema %d: %w", id, err)
		}
		if actual == nuevo {
			return ErrEstadoInvalido
		}
		var preguntas int
		if err := tx.GetContext(ctx, &preguntas,
			`SELECT COUNT(*) FROM preguntas WHERE tema_id = $1 AND estado <> 'eliminada'`, id); err != nil {
			return fmt.Errorf("contar preguntas del tema %d: %w", id, err)
		}
		detalle := map[string]any{"estado_anterior": actual, "estado_nuevo": nuevo, "preguntas_count": preguntas}
		accion := accionArchivar
		if nuevo == EstadoActivo {
			accion = accionReactivar
			orden, err := reactivar(ctx, tx, id, asignaturaID)
			if err != nil {
				return err
			}
			detalle["orden"] = orden
		} else if _, err := tx.ExecContext(ctx,
			`UPDATE temas SET estado = $1, updated_at = NOW() WHERE id = $2`, nuevo, id); err != nil {
			return fmt.Errorf("archivar tema %d: %w", id, err)
		}
		return aud(ctx, tx, Evento{Accion: accion, Entidad: entidadTema, EntidadID: id, Detalle: detalle})
	})
}

func reactivar(ctx context.Context, tx *sqlx.Tx, id, asignaturaID int64) (int, error) {
	if err := comprobarCupo(ctx, tx, asignaturaID); err != nil {
		return 0, err
	}
	var orden int
	if err := tx.GetContext(ctx, &orden, `
		UPDATE temas SET estado = 'activo', orden = (`+siguienteOrden+`), updated_at = NOW()
		WHERE id = $2 RETURNING orden`, asignaturaID, id); err != nil {
		return 0, fmt.Errorf("reactivar tema %d: %w", id, err)
	}
	return orden, nil
}

// Reordenar sets orden = 1..n following ids, which must be exactly the active
// temas of the asignatura (ErrOrdenDesactualizado otherwise).
func (r *Repository) Reordenar(ctx context.Context, asignaturaID int64, ids []int64, aud Auditor) ([]PosicionTema, error) {
	posiciones := []PosicionTema{}
	err := r.enTx(ctx, func(tx *sqlx.Tx) error {
		if _, err := bloquearAsignatura(ctx, tx, asignaturaID); err != nil {
			return asignaturaNoEncontrada(err)
		}
		antes := []int64{}
		if err := tx.SelectContext(ctx, &antes, `
			SELECT id FROM temas WHERE asignatura_id = $1 AND estado = 'activo'
			ORDER BY orden, id`, asignaturaID); err != nil {
			return fmt.Errorf("listar temas activos: %w", err)
		}
		if !mismoConjunto(antes, ids) {
			return ErrOrdenDesactualizado
		}
		if err := tx.SelectContext(ctx, &posiciones, `
			UPDATE temas t SET orden = x.pos, updated_at = NOW()
			FROM unnest($1::bigint[]) WITH ORDINALITY AS x(id, pos)
			WHERE t.id = x.id
			RETURNING t.id, t.orden`, pq.Array(ids)); err != nil {
			return fmt.Errorf("reordenar temas: %w", err)
		}
		slices.SortFunc(posiciones, func(a, b PosicionTema) int { return a.Orden - b.Orden })
		return aud(ctx, tx, Evento{Accion: accionReordenar, Entidad: entidadArea, EntidadID: asignaturaID,
			Detalle: map[string]any{"antes": antes, "despues": ids}})
	})
	return posiciones, err
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// siguienteOrden is the position after the last active tema of asignatura $1.
const siguienteOrden = `SELECT COALESCE(MAX(orden), 0) + 1 FROM temas WHERE asignatura_id = $1 AND estado = 'activo'`

// enTx runs fn in a transaction, commits when it returns nil and maps a race
// on the unique name (23505) to ErrTemaDuplicado.
func (r *Repository) enTx(ctx context.Context, fn func(tx *sqlx.Tx) error) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("iniciar transacción: %w", err)
	}
	defer tx.Rollback()
	if err := fn(tx); err != nil {
		return mapearUnico(err)
	}
	if err := tx.Commit(); err != nil {
		return mapearUnico(fmt.Errorf("confirmar transacción: %w", err))
	}
	return nil
}

func mapearUnico(err error) error {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23505" && pqErr.Constraint == constraintNombre {
		return ErrTemaDuplicado
	}
	return err
}

// asignaturaNoEncontrada names the missing row when the asignatura of the
// path is the one that does not exist.
func asignaturaNoEncontrada(err error) error {
	if errors.Is(err, ErrNoEncontrado) {
		return ErrAsignaturaNoEncontrada
	}
	return err
}

// bloquearAsignatura locks the asignatura row for the rest of tx and returns
// its estado. It serialises every change that counts or orders the active
// temas (create, archive, reactivate, reorder), so no race can leave 61.
// NO KEY UPDATE serialises those changes like FOR UPDATE but, unlike it, does
// not block the FOR KEY SHARE taken by inserts of preguntas that reference the
// asignatura.
func bloquearAsignatura(ctx context.Context, tx *sqlx.Tx, id int64) (string, error) {
	var estado string
	err := tx.GetContext(ctx, &estado, `SELECT estado FROM asignaturas WHERE id = $1 FOR NO KEY UPDATE`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNoEncontrado
	}
	if err != nil {
		return "", fmt.Errorf("bloquear asignatura %d: %w", id, err)
	}
	return estado, nil
}

// validarEspecialidades requires every id to be an existing, active especialidad.
func validarEspecialidades(ctx context.Context, tx *sqlx.Tx, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	var validas int
	if err := tx.GetContext(ctx, &validas, `
		SELECT COUNT(*) FROM especialidades WHERE id = ANY($1::bigint[]) AND estado = 'activo'`,
		pq.Array(ids)); err != nil {
		return fmt.Errorf("validar especialidades: %w", err)
	}
	if validas != len(ids) {
		return ErrReferenciaInvalida
	}
	return nil
}

// comprobarNombreLibre rejects a folded name already used in the asignatura
// by any tema other than excluir, archived ones included (regla 6).
func comprobarNombreLibre(ctx context.Context, tx *sqlx.Tx, asignaturaID int64, clave string, excluir int64) error {
	var usado bool
	if err := tx.GetContext(ctx, &usado, `
		SELECT EXISTS (SELECT 1 FROM temas WHERE asignatura_id = $1 AND nombre_clave = $2 AND id <> $3)`,
		asignaturaID, clave, excluir); err != nil {
		return fmt.Errorf("buscar nombre de tema: %w", err)
	}
	if usado {
		return ErrTemaDuplicado
	}
	return nil
}

// comprobarCupo rejects one more active tema when the asignatura is full.
func comprobarCupo(ctx context.Context, tx *sqlx.Tx, asignaturaID int64) error {
	var activos int
	if err := tx.GetContext(ctx, &activos,
		`SELECT COUNT(*) FROM temas WHERE asignatura_id = $1 AND estado = 'activo'`, asignaturaID); err != nil {
		return fmt.Errorf("contar temas activos: %w", err)
	}
	if activos >= MaxTemasActivos {
		return ErrLimiteTemas
	}
	return nil
}

func guardarEspecialidades(ctx context.Context, tx *sqlx.Tx, temaID int64, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO tema_especialidades (tema_id, especialidad_id)
		SELECT $1, unnest($2::bigint[])`, temaID, pq.Array(ids)); err != nil {
		return fmt.Errorf("guardar especialidades del tema %d: %w", temaID, err)
	}
	return nil
}

func especialidadesDe(ctx context.Context, tx *sqlx.Tx, temaID int64) ([]int64, error) {
	ids := []int64{}
	if err := tx.SelectContext(ctx, &ids,
		`SELECT especialidad_id FROM tema_especialidades WHERE tema_id = $1 ORDER BY especialidad_id`, temaID); err != nil {
		return nil, fmt.Errorf("leer especialidades del tema %d: %w", temaID, err)
	}
	return ids, nil
}

func resumen(nombre string, descripcion *string, especialidadIDs []int64) map[string]any {
	return map[string]any{"nombre": nombre, "descripcion": descripcion, "especialidad_ids": especialidadIDs}
}

// mismoConjunto reports whether a and b hold the same ids (b has no repeats,
// checked by validarOrden).
func mismoConjunto(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	en := make(map[int64]struct{}, len(a))
	for _, id := range a {
		en[id] = struct{}{}
	}
	for _, id := range b {
		if _, ok := en[id]; !ok {
			return false
		}
	}
	return true
}
