package inscripciones

import (
	"context"
	"crypto/cipher"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/cbaciliod/learning/internal/audit"
	"github.com/cbaciliod/learning/internal/db/pgerr"
	"github.com/cbaciliod/learning/internal/db/pii"
	"github.com/jmoiron/sqlx"
)

// maxListLimit caps admin list page sizes.
const maxListLimit = 500

type Repository struct {
	db     *sqlx.DB
	gcm    cipher.AEAD
	rawKey []byte
}

func NewRepository(db *sqlx.DB, gcm cipher.AEAD, rawKey []byte) *Repository {
	return &Repository{db: db, gcm: gcm, rawKey: rawKey}
}

func (r *Repository) decrypt(s, field string) string { return pii.Decrypt(r.gcm, s, field) }

// ── Procesos ──────────────────────────────────────────────────────────────────

type ProcesoRow struct {
	ID          int64   `db:"id"`
	Nombre      string  `db:"nombre"`
	Tipo        string  `db:"tipo"`
	GradoID     *int64  `db:"grado_id"`
	GradoNombre string  `db:"grado_nombre"`
	Nivel       string  `db:"nivel"`
	Descripcion *string `db:"descripcion"`
	Anio        int     `db:"anio"`
	Estado      string  `db:"estado"`
}

func (r *Repository) ListProcesosActivos(ctx context.Context) ([]ProcesoRow, error) {
	var rows []ProcesoRow
	err := r.db.SelectContext(ctx, &rows, `
		SELECT p.id, p.nombre, p.tipo, p.grado_id, p.descripcion, p.anio, p.estado,
		       COALESCE(g.nombre, '') AS grado_nombre,
		       COALESCE(p.nivel, '') AS nivel
		FROM procesos p
		LEFT JOIN grados g ON g.id = p.grado_id
		WHERE p.estado = 'activo'
		ORDER BY p.anio DESC, p.nombre`)
	return rows, err
}

// ── Inscripciones ─────────────────────────────────────────────────────────────

type InscripcionRow struct {
	ID          int64        `db:"id"`
	UsuarioID   int64        `db:"usuario_id"`
	ProcesoID   int64        `db:"proceso_id"`
	Estado      string       `db:"estado"`
	Observacion *string      `db:"observacion"`
	RevisadoPor *int64       `db:"revisado_por"`
	RevisadoAt  sql.NullTime `db:"revisado_at"`
	CreatedAt   time.Time    `db:"created_at"`
}

// InscripcionDetail joins inscripcion with proceso, persona and usuario for display.
type InscripcionDetail struct {
	ID            int64      `db:"id"`
	UsuarioID     int64      `db:"usuario_id"`
	Username      string     `db:"username"`
	ProcesoID     int64      `db:"proceso_id"`
	ProcesoNombre string     `db:"proceso_nombre"`
	ProcesoTipo   string     `db:"proceso_tipo"`
	ProcesoAnio   int        `db:"proceso_anio"`
	Estado        string     `db:"estado"`
	Observacion   *string    `db:"observacion"`
	CreatedAt     time.Time  `db:"created_at"`
	RevisadoAt    *time.Time `db:"revisado_at"`
	RevisadoPor   *string    `db:"revisado_por"` // reviewer's username
	// encrypted — decoded after fetch
	Nombres            string `db:"nombres"`
	ApellidoPaterno    string `db:"apellido_paterno"`
	ApellidoMaterno    string `db:"apellido_materno"`
	DNI                string `db:"dni"`
	GradoNombre        string `db:"grado_nombre"`
	EspecialidadNombre string `db:"especialidad_nombre"`
}

// ReapplyAuditFunc writes the audit row of a re-application inside its
// transaction; nil audits nothing.
type ReapplyAuditFunc func(ctx context.Context, tx sqlx.ExecerContext, inscripcionID int64) error

// Create inserts a pendiente inscription capturing the monto configured for the
// proceso's year, in a single statement (no window between price lookup and
// insert, no swallowed lookup error).
//
// A previously rejected inscription is re-applied in place (H71): the UNIQUE
// (usuario_id, proceso_id) row goes back to pendiente at the current price,
// with the previous review cleared and created_at reset to the new request
// date; onReapply audits it in the same transaction. The ON CONFLICT ... WHERE
// estado='rechazado' guard makes concurrent re-applications serialize on the
// row lock: one wins, the rest see pendiente and get ErrYaInscrito.
//
// It returns ErrSinConfigPago when the year has no configuracion_pago row (also
// for a rechazado inscription being re-applied) and ErrYaInscrito when a
// pendiente/aprobado inscription already exists.
func (r *Repository) Create(ctx context.Context, usuarioID, procesoID int64, onReapply ReapplyAuditFunc) (*InscripcionRow, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var res struct {
		InscripcionRow
		Reaplicada bool `db:"reaplicada"`
	}
	err = tx.GetContext(ctx, &res, `
		INSERT INTO inscripciones (usuario_id, proceso_id, monto)
		SELECT $1, pr.id, cp.monto
		FROM procesos pr
		JOIN configuracion_pago cp ON cp.anio = pr.anio
		WHERE pr.id = $2
		ON CONFLICT (usuario_id, proceso_id) DO UPDATE
		SET estado='pendiente', monto=EXCLUDED.monto, observacion=NULL,
		    revisado_por=NULL, revisado_at=NULL, created_at=NOW(), updated_at=NOW()
		WHERE inscripciones.estado='rechazado'
		RETURNING id, usuario_id, proceso_id, estado, observacion, revisado_por, revisado_at, created_at,
		          (xmax::text <> '0') AS reaplicada`,
		usuarioID, procesoID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		// Either a pendiente/aprobado row blocked the upsert, or the SELECT
		// produced nothing because the year has no price. A rechazado row (or
		// none) can only mean the latter (H108): the upsert would have
		// re-applied it otherwise.
		var estado string
		err := tx.GetContext(ctx, &estado, `SELECT estado FROM inscripciones
			WHERE usuario_id=$1 AND proceso_id=$2`, usuarioID, procesoID)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrSinConfigPago
		case err != nil:
			return nil, err
		case estado == "rechazado":
			return nil, ErrSinConfigPago
		}
		return nil, ErrYaInscrito
	case pgerr.IsUniqueViolation(err):
		return nil, ErrYaInscrito
	case err != nil:
		return nil, err
	}
	if res.Reaplicada && onReapply != nil {
		if err := onReapply(ctx, tx, res.ID); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &res.InscripcionRow, nil
}

const detailSelect = `
		SELECT i.id, i.usuario_id, u.username, i.proceso_id,
		       pr.nombre AS proceso_nombre, pr.tipo AS proceso_tipo, pr.anio AS proceso_anio,
		       i.estado, i.observacion, i.created_at,
		       i.revisado_at, rv.username AS revisado_por,
		       p.nombres, p.apellido_paterno, p.apellido_materno, p.dni,
		       g.nombre AS grado_nombre, e.nombre AS especialidad_nombre
		FROM inscripciones i
		JOIN usuarios    u  ON u.id = i.usuario_id
		LEFT JOIN usuarios rv ON rv.id = i.revisado_por
		JOIN personas    p  ON p.id = u.persona_id
		JOIN procesos    pr ON pr.id = i.proceso_id
		JOIN grados      g  ON g.id = p.grado_id
		JOIN especialidades e ON e.id = p.especialidad_id`

func (r *Repository) ListByUsuario(ctx context.Context, usuarioID int64) ([]InscripcionDetail, error) {
	var rows []InscripcionDetail
	err := r.db.SelectContext(ctx, &rows, detailSelect+`
		WHERE i.usuario_id = $1
		ORDER BY i.created_at DESC`, usuarioID)
	if err != nil {
		return nil, err
	}
	return r.decryptDetails(rows), nil
}

// ListByEstado is used by admins to list inscriptions filtered by estado, one
// page at a time (page is 1-based; the caller clamps page and perPage).
func (r *Repository) ListByEstado(ctx context.Context, estado string, page, perPage int) ([]InscripcionDetail, error) {
	query := detailSelect
	args := []any{}
	if estado != "" {
		query += ` WHERE i.estado = $1`
		args = append(args, estado)
	}
	query += fmt.Sprintf(` ORDER BY i.created_at DESC, i.id DESC LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+2)
	args = append(args, perPage, int64(page-1)*int64(perPage))

	var rows []InscripcionDetail
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, err
	}
	return r.decryptDetails(rows), nil
}

// UpdateEstado decides a pendiente inscription. The origin state is part of
// the WHERE clause, so concurrent reviewers cannot overwrite each other and a
// decided inscription is never changed again (it would retroactively change
// revenue and simulacro access). It returns ErrInscripcionNotFound when the id
// does not exist and ErrTransicionInvalida when it is no longer pendiente.
// auditFn writes the audit row in the same transaction; its error rolls the
// decision back.
func (r *Repository) UpdateEstado(ctx context.Context, id int64, estado, observacion string, adminID int64, auditFn audit.TxFunc) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `
		UPDATE inscripciones
		SET estado=$1, observacion=$2, revisado_por=$3, revisado_at=NOW(), updated_at=NOW()
		WHERE id=$4 AND estado='pendiente'`,
		estado, observacion, adminID, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 1 {
		if err := audit.RunTx(ctx, tx, auditFn); err != nil {
			return err
		}
		return tx.Commit()
	}
	var actual string
	err = tx.QueryRowContext(ctx, `SELECT estado FROM inscripciones WHERE id=$1`, id).Scan(&actual)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrInscripcionNotFound
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("%w Estado actual: %s.", ErrTransicionInvalida, actual)
}

func (r *Repository) decryptDetails(rows []InscripcionDetail) []InscripcionDetail {
	for i, row := range rows {
		rows[i].Nombres = r.decrypt(row.Nombres, "nombres")
		rows[i].ApellidoPaterno = r.decrypt(row.ApellidoPaterno, "apellido_paterno")
		rows[i].ApellidoMaterno = r.decrypt(row.ApellidoMaterno, "apellido_materno")
		rows[i].DNI = r.decrypt(row.DNI, "dni")
	}
	return rows
}

type StatsPublicosRow struct {
	TotalUsuarios   int64 `db:"total_usuarios"`
	TotalSimulacros int64 `db:"total_simulacros"`
	TotalPracticas  int64 `db:"total_practicas"`
	TotalProcesos   int64 `db:"total_procesos"`
}

func (r *Repository) GetStatsPublicos(ctx context.Context) (StatsPublicosRow, error) {
	var row StatsPublicosRow
	err := r.db.QueryRowContext(ctx, `
		SELECT
			(SELECT COUNT(*) FROM usuarios)                               AS total_usuarios,
			(SELECT COUNT(*) FROM simulacros WHERE estado='completado')   AS total_simulacros,
			(SELECT COUNT(*) FROM practicas  WHERE estado='completada')   AS total_practicas,
			(SELECT COUNT(*) FROM procesos   WHERE estado='activo')       AS total_procesos
	`).Scan(&row.TotalUsuarios, &row.TotalSimulacros, &row.TotalPracticas, &row.TotalProcesos)
	return row, err
}
