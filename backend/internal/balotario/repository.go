package balotario

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cbaciliod/learning/internal/db/pgerr"
	"github.com/cbaciliod/learning/internal/fuentes"
	"github.com/cbaciliod/learning/internal/questions"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// Repository reads from Postgres what the balotario import needs. The
// content of a fuente and its passages come from internal/fuentes (outside of
// its port, ADR 0007); the imports of balotario are its own.
type Repository struct{ db *sqlx.DB }

func NewRepository(db *sqlx.DB) *Repository { return &Repository{db: db} }

// registro is one row of balotario_importaciones with the number of its version.
type registro struct {
	ID        int64     `db:"id"`
	Version   int       `db:"version"`
	Creadas   int       `db:"creadas"`
	CreatedAt time.Time `db:"created_at"`
}

// lectura is a consistent snapshot of what the preview needs.
type lectura struct {
	Version       fuentes.VersionBalotario
	Rangos        []fuentes.RangoPasaje
	Importaciones []registro // from the most recent version to the oldest
}

// Leer reads the fuente, its current version, the ranges of its passages and
// its imports in one read-only snapshot. The ranges are only read when the
// fuente can be previewed (an active balotario): any other is refused before.
func (r *Repository) Leer(ctx context.Context, fuenteID int64) (lectura, error) {
	tx, err := r.db.BeginTxx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return lectura{}, err
	}
	defer tx.Rollback()

	v, err := fuentes.VersionVigenteBalotario(ctx, tx, fuenteID)
	if err != nil {
		return lectura{}, err
	}
	l := lectura{Version: v}
	if v.Tipo == "balotario" && v.Estado == "activa" && v.VersionID != 0 {
		if l.Rangos, err = fuentes.RangosPasajes(ctx, tx, v.VersionID); err != nil {
			return lectura{}, err
		}
	}
	if err := tx.SelectContext(ctx, &l.Importaciones, `
		SELECT bi.id, fv.numero AS version, bi.creadas, bi.created_at
		FROM balotario_importaciones bi
		JOIN fuente_versiones fv ON fv.id = bi.fuente_version_id
		WHERE bi.fuente_id = $1
		ORDER BY fv.numero DESC`, fuenteID); err != nil {
		return lectura{}, fmt.Errorf("importaciones de la fuente %d: %w", fuenteID, err)
	}
	return l, nil
}

// Auditor writes the audit row of the import in the same transaction. The
// repository adds importacion_id to the detail.
type Auditor func(ctx context.Context, tx sqlx.ExecerContext, detalle map[string]any) error

// Guardado is everything the transaction of the import writes (§4.2).
type Guardado struct {
	FuenteID        int64
	FuenteVersionID int64
	Version         int
	InstitucionID   int64
	Req             ConfirmarRequest
	ActorID         int64
	Validas         []importable
	ConProblemas    int
	Detalle         map[string]any // audit detail without importacion_id
}

// EnTransaccion runs fn in one transaction: it commits if fn returns nil.
func (r *Repository) EnTransaccion(ctx context.Context, fn func(Operaciones) error) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := fn(&opsTx{tx: tx}); err != nil {
		return err
	}
	return tx.Commit()
}

// opsTx is Operaciones over a transaction.
type opsTx struct{ tx *sqlx.Tx }

// BloquearFuente takes the fuente FOR SHARE (§4.1, step 4): it blocks the
// retirement and the publication of a new version until the import ends.
func (o *opsTx) BloquearFuente(ctx context.Context, fuenteID int64) error {
	var id int64
	err := o.tx.GetContext(ctx, &id, `SELECT id FROM fuentes WHERE id = $1 FOR SHARE`, fuenteID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNoEncontrado
	}
	if err != nil {
		return fmt.Errorf("bloquear fuente %d: %w", fuenteID, err)
	}
	return nil
}

func (o *opsTx) VersionVigente(ctx context.Context, fuenteID int64) (fuentes.VersionBalotario, error) {
	v, err := fuentes.VersionVigenteBalotario(ctx, o.tx, fuenteID)
	if errors.Is(err, fuentes.ErrNoEncontrado) {
		return v, ErrNoEncontrado
	}
	return v, err
}

func (o *opsTx) Rangos(ctx context.Context, versionID int64) ([]fuentes.RangoPasaje, error) {
	return fuentes.RangosPasajes(ctx, o.tx, versionID)
}

// ImportacionDeVersion returns the import of a version, or nil.
func (o *opsTx) ImportacionDeVersion(ctx context.Context, versionID int64) (*registro, error) {
	var reg registro
	err := o.tx.GetContext(ctx, &reg, `
		SELECT bi.id, fv.numero AS version, bi.creadas, bi.created_at
		FROM balotario_importaciones bi JOIN fuente_versiones fv ON fv.id = bi.fuente_version_id
		WHERE bi.fuente_version_id = $1`, versionID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("importación de la versión %d: %w", versionID, err)
	}
	return &reg, nil
}

// InstitucionDelProceso returns the institution of the proceso; ok is false if it does not exist.
func (o *opsTx) InstitucionDelProceso(ctx context.Context, procesoID int64) (int64, bool, error) {
	var id int64
	err := o.tx.GetContext(ctx, &id, `SELECT institucion_id FROM procesos WHERE id = $1`, procesoID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	return id, err == nil, err
}

// EspecialidadActiva: it exists, is active and belongs to the institution (D13).
func (o *opsTx) EspecialidadActiva(ctx context.Context, especialidadID, institucionID int64) (bool, error) {
	var ok bool
	err := o.tx.GetContext(ctx, &ok, `
		SELECT EXISTS (SELECT 1 FROM especialidades WHERE id = $1 AND institucion_id = $2 AND estado = 'activo')`,
		especialidadID, institucionID)
	return ok, err
}

// AreaAsignada: the asignatura is assigned to the proceso, like questions.validarClasificacion.
func (o *opsTx) AreaAsignada(ctx context.Context, procesoID, asignaturaID int64) (bool, error) {
	var ok bool
	err := o.tx.GetContext(ctx, &ok, `
		SELECT EXISTS (SELECT 1 FROM proceso_asignaturas WHERE proceso_id = $1 AND asignatura_id = $2)`,
		procesoID, asignaturaID)
	return ok, err
}

// Guardar writes §4.2 and the audit row: the import, the questions with their
// answers (one statement per table), the citations and the traceability.
func (o *opsTx) Guardar(ctx context.Context, g Guardado, aud Auditor) (int64, error) {
	importacionID, err := o.insertarImportacion(ctx, g)
	if err != nil {
		return 0, err
	}
	ids, err := questions.InsertarLote(ctx, o.tx, nuevasPreguntas(g))
	if err != nil {
		return 0, err
	}
	if err := o.insertarCitas(ctx, ids, g.Validas); err != nil {
		return 0, err
	}
	if err := o.insertarTrazabilidad(ctx, importacionID, ids, g.Validas); err != nil {
		return 0, err
	}
	if aud != nil {
		detalle := map[string]any{"importacion_id": importacionID}
		for k, v := range g.Detalle {
			detalle[k] = v
		}
		if err := aud(ctx, o.tx, detalle); err != nil {
			return 0, err
		}
	}
	return importacionID, nil
}

// ErrYaImportadoConcurrente: another confirmation of the same version won the race.
var ErrYaImportadoConcurrente = errors.New("balotario: la versión ya se importó")

func (o *opsTx) insertarImportacion(ctx context.Context, g Guardado) (int64, error) {
	var id int64
	err := o.tx.GetContext(ctx, &id, `
		INSERT INTO balotario_importaciones (fuente_id, fuente_version_id, proceso_id, especialidad_id, asignatura_id,
			creadas, con_problemas, creado_por)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`,
		g.FuenteID, g.FuenteVersionID, g.Req.ProcesoID, g.Req.EspecialidadID, g.Req.AsignaturaID,
		len(g.Validas), g.ConProblemas, g.ActorID)
	if pgerr.IsUniqueViolation(err) {
		return 0, ErrYaImportadoConcurrente
	}
	if err != nil {
		return 0, fmt.Errorf("insertar importación: %w", err)
	}
	return id, nil
}

func nuevasPreguntas(g Guardado) []questions.NuevaPregunta {
	lote := make([]questions.NuevaPregunta, len(g.Validas))
	for i, v := range g.Validas {
		respuestas := make([]questions.CreateRespuestaItem, len(v.Alternativas))
		for j, a := range v.Alternativas {
			respuestas[j] = questions.CreateRespuestaItem{Texto: a.Texto, EsCorrecta: a.Correcta, Orden: j + 1}
		}
		lote[i] = questions.NuevaPregunta{InstitucionID: g.InstitucionID, ProcesoID: g.Req.ProcesoID, AsignaturaID: g.Req.AsignaturaID,
			EspecialidadID: &g.Req.EspecialidadID, Enunciado: v.Enunciado, Tipo: v.Tipo, Dificultad: dificultadImportada,
			Origen: questions.OrigenBalotario, Respuestas: respuestas}
	}
	return lote
}

// dificultadImportada is the default difficulty of an imported question, as in the Excel import.
const dificultadImportada = "medio"

// insertarCitas cites to each question the passages of its range (§4.2).
func (o *opsTx) insertarCitas(ctx context.Context, ids []int64, validas []importable) error {
	var preguntas, pasajes []int64
	for i, v := range validas {
		if len(v.PasajeIDs) > questions.MaxCitas {
			return fmt.Errorf("la pregunta %d cita %d pasajes (máximo %d)", v.Numero, len(v.PasajeIDs), questions.MaxCitas)
		}
		for _, pasaje := range v.PasajeIDs {
			preguntas, pasajes = append(preguntas, ids[i]), append(pasajes, pasaje)
		}
	}
	if _, err := o.tx.ExecContext(ctx, `
		INSERT INTO pregunta_citas (pregunta_id, pasaje_id)
		SELECT pregunta_id, pasaje_id FROM unnest($1::bigint[], $2::bigint[]) AS t(pregunta_id, pasaje_id)`,
		pq.Array(preguntas), pq.Array(pasajes)); err != nil {
		return fmt.Errorf("insertar citas: %w", err)
	}
	return nil
}

func (o *opsTx) insertarTrazabilidad(ctx context.Context, importacionID int64, ids []int64, validas []importable) error {
	var numeros, inicios, fines []int64
	var alertas []string
	for _, v := range validas {
		numeros, inicios, fines = append(numeros, int64(v.Numero)), append(inicios, int64(v.LineaInicio)), append(fines, int64(v.LineaFin))
		alertas = append(alertas, "{"+strings.Join(v.Alertas, ",")+"}")
	}
	if _, err := o.tx.ExecContext(ctx, `
		INSERT INTO pregunta_balotario (pregunta_id, importacion_id, numero, linea_inicio, linea_fin, alertas_estructura)
		SELECT pregunta_id, $1, numero, linea_inicio, linea_fin, alertas::text[]
		FROM unnest($2::bigint[], $3::int[], $4::int[], $5::int[], $6::text[])
			AS t(pregunta_id, numero, linea_inicio, linea_fin, alertas)`,
		importacionID, pq.Array(ids), pq.Array(numeros), pq.Array(inicios), pq.Array(fines), pq.Array(alertas)); err != nil {
		return fmt.Errorf("insertar trazabilidad: %w", err)
	}
	return nil
}
