package perfiles

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// constraintActivo is the one-active-profile index of migration 061.
const constraintActivo = "uq_postulante_profesiones_usuario_activo"

// Repository is the PostgreSQL store of profiles.
type Repository struct {
	db *sqlx.DB
}

// NewRepository creates a Repository over db.
func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

// perfilRow is the active profile with its names.
type perfilRow struct {
	ID                 int64     `db:"id"`
	ProcesoID          int64     `db:"proceso_id"`
	Proceso            string    `db:"proceso"`
	EspecialidadID     int64     `db:"especialidad_id"`
	Especialidad       string    `db:"especialidad"`
	Estado             string    `db:"estado"`
	CreatedAt          time.Time `db:"created_at"`
	InscripcionVigente bool      `db:"inscripcion_vigente"`
}

// PerfilActivo returns the user's active profile, or ErrProfesionNoDefinida.
// InscripcionVigente tells whether the inscripción is still approved.
func (r *Repository) PerfilActivo(ctx context.Context, usuarioID int64) (perfilRow, error) {
	var p perfilRow
	err := sqlx.GetContext(ctx, r.db, &p, `
		SELECT pp.id, pp.proceso_id, pr.nombre AS proceso, pp.especialidad_id, e.nombre AS especialidad,
		       pp.estado, pp.created_at,
		       EXISTS (SELECT 1 FROM inscripciones i
		               WHERE i.usuario_id = pp.usuario_id AND i.proceso_id = pp.proceso_id AND i.estado = 'aprobado')
		         AS inscripcion_vigente
		FROM postulante_profesiones pp
		JOIN procesos pr ON pr.id = pp.proceso_id
		JOIN especialidades e ON e.id = pp.especialidad_id
		WHERE pp.usuario_id = $1 AND pp.estado = 'activo'`, usuarioID)
	if errors.Is(err, sql.ErrNoRows) {
		return perfilRow{}, ErrProfesionNoDefinida
	}
	if err != nil {
		return perfilRow{}, fmt.Errorf("leer perfil activo: %w", err)
	}
	return p, nil
}

// ProcesosAprobados returns the procesos where the user has an approved
// inscripción, each with the active especialidades of its institucion and
// whether their weights are saved (completa) or estimated. Two queries, no N+1.
func (r *Repository) ProcesosAprobados(ctx context.Context, usuarioID int64) ([]ProcesoOpcion, error) {
	procesos := []ProcesoOpcion{}
	if err := sqlx.SelectContext(ctx, r.db, &procesos, `
		SELECT p.id, p.nombre, p.anio FROM inscripciones i JOIN procesos p ON p.id = i.proceso_id
		WHERE i.usuario_id = $1 AND i.estado = 'aprobado'
		ORDER BY p.anio DESC, p.nombre, p.id`, usuarioID); err != nil {
		return nil, fmt.Errorf("procesos aprobados: %w", err)
	}
	if len(procesos) == 0 {
		return procesos, nil
	}
	ids := make([]int64, len(procesos))
	for i, p := range procesos {
		ids[i] = p.ID
	}
	var filas []struct {
		ProcesoID int64  `db:"proceso_id"`
		ID        int64  `db:"id"`
		Nombre    string `db:"nombre"`
		Codigo    string `db:"codigo"`
		Completa  bool   `db:"completa"`
	}
	if err := sqlx.SelectContext(ctx, r.db, &filas, `
		SELECT p.id AS proceso_id, e.id, e.nombre, e.codigo,
		       EXISTS (SELECT 1 FROM proceso_asignaturas pa
		               WHERE pa.proceso_id = p.id AND pa.especialidad_id = e.id) AS completa
		FROM procesos p
		JOIN especialidades e ON e.institucion_id = p.institucion_id AND e.estado = 'activo'
		WHERE p.id = ANY($1::bigint[])
		ORDER BY p.id, e.nombre, e.id`, pq.Array(ids)); err != nil {
		return nil, fmt.Errorf("especialidades de los procesos: %w", err)
	}
	porProceso := make(map[int64][]EspecialidadOpcion, len(procesos))
	for _, f := range filas {
		pesos := PesosEstimada
		if f.Completa {
			pesos = PesosCompleta
		}
		porProceso[f.ProcesoID] = append(porProceso[f.ProcesoID], EspecialidadOpcion{ID: f.ID, Nombre: f.Nombre, Codigo: f.Codigo, Pesos: pesos})
	}
	for i := range procesos {
		procesos[i].Especialidades = porProceso[procesos[i].ID]
		if procesos[i].Especialidades == nil {
			procesos[i].Especialidades = []EspecialidadOpcion{}
		}
	}
	return procesos, nil
}

// EspecialidadDePersona reads the especialidad of the user's persona from the
// database (not from the JWT, which may be stale; contract F1). nil if none.
func (r *Repository) EspecialidadDePersona(ctx context.Context, usuarioID int64) (*int64, error) {
	var id *int64
	err := sqlx.GetContext(ctx, r.db, &id, `
		SELECT p.especialidad_id FROM usuarios u JOIN personas p ON p.id = u.persona_id WHERE u.id = $1`, usuarioID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("especialidad de persona: %w", err)
	}
	return id, nil
}

// NombresAsignaturas returns the names of the given asignaturas by id.
func (r *Repository) NombresAsignaturas(ctx context.Context, ids []int64) (map[int64]string, error) {
	nombres := make(map[int64]string, len(ids))
	if len(ids) == 0 {
		return nombres, nil
	}
	var filas []struct {
		ID     int64  `db:"id"`
		Nombre string `db:"nombre"`
	}
	if err := sqlx.SelectContext(ctx, r.db, &filas, `SELECT id, nombre FROM asignaturas WHERE id = ANY($1::bigint[])`, pq.Array(ids)); err != nil {
		return nil, fmt.Errorf("nombres de asignaturas: %w", err)
	}
	for _, f := range filas {
		nombres[f.ID] = f.Nombre
	}
	return nombres, nil
}

// Elegir applies F3 rules 2–6 in one transaction: approved inscripción
// (ErrInscripcionRequerida, also for an unknown proceso), especialidad of the
// proceso's institucion (ErrEspecialidadInvalida), idempotent same pair,
// confirmation before replacing another pair (*ConfirmacionError) and then
// archive + insert. A race on the one-active index is ErrEstadoInvalido.
func (r *Repository) Elegir(ctx context.Context, usuarioID int64, par Par, confirmar bool) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("iniciar transacción: %w", err)
	}
	defer tx.Rollback()

	var aprobada bool
	if err := tx.GetContext(ctx, &aprobada, `
		SELECT EXISTS (SELECT 1 FROM inscripciones WHERE usuario_id = $1 AND proceso_id = $2 AND estado = 'aprobado')`,
		usuarioID, par.ProcesoID); err != nil {
		return fmt.Errorf("verificar inscripción: %w", err)
	}
	if !aprobada {
		return ErrInscripcionRequerida
	}
	var valida bool
	if err := tx.GetContext(ctx, &valida, `
		SELECT EXISTS (SELECT 1 FROM especialidades e JOIN procesos p ON p.institucion_id = e.institucion_id
		               WHERE p.id = $1 AND e.id = $2 AND e.estado = 'activo')`,
		par.ProcesoID, par.EspecialidadID); err != nil {
		return fmt.Errorf("verificar especialidad: %w", err)
	}
	if !valida {
		return ErrEspecialidadInvalida
	}

	// FOR UPDATE keeps the active row stable until the archive below.
	var actual struct {
		ID int64 `db:"id"`
		Par
	}
	err = tx.GetContext(ctx, &actual, `
		SELECT id, proceso_id, especialidad_id FROM postulante_profesiones
		WHERE usuario_id = $1 AND estado = 'activo' FOR UPDATE`, usuarioID)
	hayActual := err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("leer perfil activo: %w", err)
	}
	if hayActual && actual.Par == par {
		return nil
	}
	if hayActual && !confirmar {
		return &ConfirmacionError{Actual: actual.Par}
	}
	if hayActual {
		if _, err := tx.ExecContext(ctx, `
			UPDATE postulante_profesiones SET estado = 'archivado', archivado_at = NOW() WHERE id = $1`, actual.ID); err != nil {
			return fmt.Errorf("archivar perfil %d: %w", actual.ID, err)
		}
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO postulante_profesiones (usuario_id, proceso_id, especialidad_id) VALUES ($1, $2, $3)`,
		usuarioID, par.ProcesoID, par.EspecialidadID); err != nil {
		return mapearActivo(fmt.Errorf("crear perfil: %w", err))
	}
	return mapearActivo(tx.Commit())
}

func mapearActivo(err error) error {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23505" && pqErr.Constraint == constraintActivo {
		return ErrEstadoInvalido
	}
	return err
}
