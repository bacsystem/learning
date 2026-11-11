package catalogs

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/cbaciliod/learning/internal/questions"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// constraintPesos is the unique (proceso, especialidad or none, asignatura)
// constraint of migration 060 (ADR 0001).
const constraintPesos = "uq_proceso_asignaturas_proceso_esp_asig"

// AuditorPesos writes the pesos.guardar audit row inside the save transaction.
type AuditorPesos func(ctx context.Context, tx sqlx.ExecerContext, detalle map[string]any) error

// areaColumns selects an AreaPeso from proceso_asignaturas pa joined to asignaturas a.
const areaColumns = `pa.asignatura_id, a.nombre, a.codigo, pa.peso_porcentual, pa.numero_preguntas, pa.coeficiente`

// ResumenPesos returns the compatibility summary and the per-especialidad
// summary of every active especialidad of the proceso's institucion (PE1).
func (r *Repository) ResumenPesos(ctx context.Context, procesoID int64) (ResumenPesos, error) {
	if err := existeProceso(ctx, r.db, procesoID); err != nil {
		return ResumenPesos{}, err
	}
	res := ResumenPesos{ProcesoID: procesoID}
	if err := sqlx.GetContext(ctx, r.db, &res.Compatibilidad, `
		SELECT COUNT(*) AS areas, COALESCE(SUM(peso_porcentual), 0) AS suma_pesos
		FROM proceso_asignaturas WHERE proceso_id = $1 AND especialidad_id IS NULL`, procesoID); err != nil {
		return ResumenPesos{}, fmt.Errorf("resumen de compatibilidad: %w", err)
	}
	res.Especialidades = []ResumenEspecialidad{}
	if err := sqlx.SelectContext(ctx, r.db, &res.Especialidades, `
		SELECT e.id AS especialidad_id, e.nombre, COUNT(pa.id) AS areas,
		       COALESCE(SUM(pa.peso_porcentual), 0) AS suma_pesos,
		       COALESCE(SUM(pa.numero_preguntas), 0) AS suma_preguntas
		FROM procesos p
		JOIN especialidades e ON e.institucion_id = p.institucion_id AND e.estado = 'activo'
		LEFT JOIN proceso_asignaturas pa ON pa.proceso_id = p.id AND pa.especialidad_id = e.id
		WHERE p.id = $1
		GROUP BY e.id, e.nombre
		ORDER BY e.nombre, e.id`, procesoID); err != nil {
		return ResumenPesos{}, fmt.Errorf("resumen por especialidad: %w", err)
	}
	for i := range res.Especialidades {
		res.Especialidades[i].Estado = estadoPesos(res.Especialidades[i].Areas)
	}
	return res, nil
}

// ConfiguracionPesos returns the weights of (proceso, especialidad) (PE2).
// ErrNotFound: unknown proceso; ErrInvalidRef: especialidad unknown, inactive
// or from another institucion.
func (r *Repository) ConfiguracionPesos(ctx context.Context, procesoID, especialidadID int64) (ConfiguracionPesos, error) {
	if err := existeProceso(ctx, r.db, procesoID); err != nil {
		return ConfiguracionPesos{}, err
	}
	nombre, err := especialidadDelProceso(ctx, r.db, procesoID, especialidadID)
	if err != nil {
		return ConfiguracionPesos{}, err
	}
	c := ConfiguracionPesos{ProcesoID: procesoID, EspecialidadID: especialidadID, Especialidad: nombre, Plantilla: []AreaPeso{}}
	if c.Areas, err = areasDe(ctx, r.db, procesoID, &especialidadID); err != nil {
		return ConfiguracionPesos{}, err
	}
	c.Estado = estadoPesos(len(c.Areas))
	if c.Estado == EstadoPesosIncompleta {
		if c.Plantilla, err = areasDe(ctx, r.db, procesoID, nil); err != nil {
			return ConfiguracionPesos{}, err
		}
	}
	for _, a := range c.Areas {
		c.SumaPesos += a.PesoPorcentual
		c.SumaPreguntas += a.NumeroPreguntas
	}
	c.SumaPesos = float64(centesimas(c.SumaPesos)) / 100
	return c, nil
}

// GuardarPesos replaces the whole configuration of (proceso, especialidad) in
// one transaction under the proceso lock (PE3). In the contract's order it
// checks the proceso (404), the references (422) and then reglas, the
// business rules 3–5 that the Service passes in; any error saves nothing.
func (r *Repository) GuardarPesos(ctx context.Context, procesoID, especialidadID int64, areas []AreaPesoRequest,
	reglas func() error, aud AuditorPesos) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("iniciar transacción: %w", err)
	}
	defer tx.Rollback()
	if _, err := lockProceso(ctx, tx, procesoID); err != nil {
		return err
	}
	if _, err := especialidadDelProceso(ctx, tx, procesoID, especialidadID); err != nil {
		return err
	}
	if err := asignaturasExisten(ctx, tx, areas); err != nil {
		return err
	}
	if err := reglas(); err != nil {
		return err
	}
	antes, err := areasDe(ctx, tx, procesoID, &especialidadID)
	if err != nil {
		return err
	}
	if err := reemplazarConjunto(ctx, tx, procesoID, especialidadID, areas); err != nil {
		return mapearPesoDuplicado(err)
	}
	if err := aud(ctx, tx, map[string]any{
		"especialidad_id": especialidadID, "antes": resumenAreas(antes), "despues": resumenSolicitud(areas),
	}); err != nil {
		return err
	}
	return mapearPesoDuplicado(tx.Commit())
}

// ConjuntoDeEspecialidad returns only the rows of (proceso, especialidad), by
// asignatura_id: empty means «incompleta» (PE6).
func (r *Repository) ConjuntoDeEspecialidad(ctx context.Context, procesoID, especialidadID int64) ([]AreaPeso, error) {
	return areasDe(ctx, r.db, procesoID, &especialidadID)
}

// AreasServibles returns, sorted, the asignaturas with servable questions of
// the proceso for the especialidad (or for every especialidad) (PE6).
func (r *Repository) AreasServibles(ctx context.Context, procesoID, especialidadID int64) ([]int64, error) {
	ids := []int64{}
	if err := sqlx.SelectContext(ctx, r.db, &ids, `
		SELECT DISTINCT p.asignatura_id FROM preguntas p
		WHERE p.proceso_id = $1 AND (p.especialidad_id = $2 OR p.especialidad_id IS NULL)
		  AND `+questions.PreguntaServible("p")+`
		ORDER BY p.asignatura_id`, procesoID, especialidadID); err != nil {
		return nil, fmt.Errorf("áreas con preguntas servibles: %w", err)
	}
	return ids, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func estadoPesos(areas int) string {
	if areas > 0 {
		return EstadoPesosCompleta
	}
	return EstadoPesosIncompleta
}

func existeProceso(ctx context.Context, q sqlx.QueryerContext, procesoID int64) error {
	var existe bool
	if err := sqlx.GetContext(ctx, q, &existe, `SELECT EXISTS (SELECT 1 FROM procesos WHERE id = $1)`, procesoID); err != nil {
		return fmt.Errorf("buscar proceso %d: %w", procesoID, err)
	}
	if !existe {
		return ErrNotFound
	}
	return nil
}

// especialidadDelProceso returns the name of an active especialidad of the
// proceso's institucion, or ErrInvalidRef.
func especialidadDelProceso(ctx context.Context, q sqlx.QueryerContext, procesoID, especialidadID int64) (string, error) {
	var nombre string
	err := sqlx.GetContext(ctx, q, &nombre, `
		SELECT e.nombre FROM especialidades e
		JOIN procesos p ON p.institucion_id = e.institucion_id
		WHERE p.id = $1 AND e.id = $2 AND e.estado = 'activo'`, procesoID, especialidadID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", &errorPesos{base: ErrInvalidRef, msg: "La especialidad no existe, está inactiva o es de otra institución."}
	}
	if err != nil {
		return "", fmt.Errorf("validar especialidad %d: %w", especialidadID, err)
	}
	return nombre, nil
}

func asignaturasExisten(ctx context.Context, tx *sqlx.Tx, areas []AreaPesoRequest) error {
	ids := make([]int64, len(areas))
	for i, a := range areas {
		ids[i] = a.AsignaturaID
	}
	var faltan int
	if err := tx.GetContext(ctx, &faltan, `
		SELECT COUNT(*) FROM unnest($1::bigint[]) AS t(id)
		WHERE NOT EXISTS (SELECT 1 FROM asignaturas a WHERE a.id = t.id)`, pq.Array(ids)); err != nil {
		return fmt.Errorf("validar asignaturas: %w", err)
	}
	if faltan > 0 {
		return &errorPesos{base: ErrInvalidRef, msg: "Alguna asignatura no existe."}
	}
	return nil
}

// areasDe returns the rows of (proceso, especialidad), or the compatibility
// rows when especialidadID is nil, by asignatura nombre then id.
func areasDe(ctx context.Context, q sqlx.QueryerContext, procesoID int64, especialidadID *int64) ([]AreaPeso, error) {
	areas := []AreaPeso{}
	if err := sqlx.SelectContext(ctx, q, &areas, `
		SELECT `+areaColumns+` FROM proceso_asignaturas pa
		JOIN asignaturas a ON a.id = pa.asignatura_id
		WHERE pa.proceso_id = $1 AND pa.especialidad_id IS NOT DISTINCT FROM $2::bigint
		ORDER BY a.nombre, pa.asignatura_id`, procesoID, especialidadID); err != nil {
		return nil, fmt.Errorf("leer áreas del proceso %d: %w", procesoID, err)
	}
	return areas, nil
}

// reemplazarConjunto deletes the areas of (proceso, especialidad) that are no
// longer sent and inserts or updates the sent ones. Compatibility rows are
// never touched (E1-03 regla 2).
func reemplazarConjunto(ctx context.Context, tx *sqlx.Tx, procesoID, especialidadID int64, areas []AreaPesoRequest) error {
	ids := make([]int64, len(areas))
	for i, a := range areas {
		ids[i] = a.AsignaturaID
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM proceso_asignaturas
		WHERE proceso_id = $1 AND especialidad_id = $2 AND asignatura_id <> ALL($3::bigint[])`,
		procesoID, especialidadID, pq.Array(ids)); err != nil {
		return fmt.Errorf("quitar áreas omitidas: %w", err)
	}
	for _, a := range areas {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO proceso_asignaturas
				(proceso_id, especialidad_id, asignatura_id, numero_preguntas, peso_porcentual, coeficiente)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (proceso_id, especialidad_id, asignatura_id) DO UPDATE
			SET numero_preguntas = EXCLUDED.numero_preguntas,
			    peso_porcentual  = EXCLUDED.peso_porcentual,
			    coeficiente      = EXCLUDED.coeficiente`,
			procesoID, especialidadID, a.AsignaturaID, a.NumeroPreguntas, a.PesoPorcentual, a.Coeficiente); err != nil {
			return fmt.Errorf("guardar área %d: %w", a.AsignaturaID, err)
		}
	}
	return nil
}

// mapearPesoDuplicado turns a violation of the unique constraint (e.g. a race
// that the lock did not cover) into ErrPesoDuplicado (E1-03 regla 7c).
func mapearPesoDuplicado(err error) error {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23505" && pqErr.Constraint == constraintPesos {
		return ErrPesoDuplicado
	}
	return err
}

func resumenAreas(areas []AreaPeso) []map[string]any {
	out := make([]map[string]any, len(areas))
	for i, a := range areas {
		out[i] = map[string]any{"asignatura_id": a.AsignaturaID, "peso_porcentual": a.PesoPorcentual,
			"numero_preguntas": a.NumeroPreguntas, "coeficiente": a.Coeficiente}
	}
	return out
}

func resumenSolicitud(areas []AreaPesoRequest) []map[string]any {
	filas := make([]AreaPeso, len(areas))
	for i, a := range areas {
		filas[i] = AreaPeso{AsignaturaID: a.AsignaturaID, PesoPorcentual: a.PesoPorcentual,
			NumeroPreguntas: a.NumeroPreguntas, Coeficiente: a.Coeficiente}
	}
	return resumenAreas(filas)
}
