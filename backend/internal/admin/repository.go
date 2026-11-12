package admin

import (
	"context"
	"crypto/cipher"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cbaciliod/learning/internal/audit"
	"github.com/cbaciliod/learning/internal/db"
	"github.com/cbaciliod/learning/internal/db/pii"
	"github.com/jmoiron/sqlx"
)

// zonaNegocio is the business timezone (H79): "today", months and years in
// the dashboard, revenue and audit-log date filters follow the Peruvian
// calendar, not the DB's UTC. internal/config has no timezone setting, so it
// is a constant here; it is spliced into SQL as a literal (zonaSQL), never
// user input.
const zonaNegocio = "America/Lima"

const zonaSQL = "'" + zonaNegocio + "'"

// localTS converts a timestamptz expression to Lima wall-clock time.
func localTS(expr string) string { return "(" + expr + " AT TIME ZONE " + zonaSQL + ")" }

// dayStartSQL is the UTC instant where the Lima day containing $n begins,
// shifted by offset days (e.g. "+ INTERVAL '1 day'"); the result is a
// timestamptz, so range predicates on indexed columns stay sargable.
func dayStartSQL(param, offset string) string {
	return "((DATE_TRUNC('day', " + localTS(param+"::timestamptz") + ") " + offset + ") AT TIME ZONE " + zonaSQL + ")"
}

type Repository struct {
	db     *sqlx.DB
	gcm    cipher.AEAD
	rawKey []byte
	// now is the reference instant for "today/this month" (tests pin it).
	now func() time.Time
}

func NewRepository(db *sqlx.DB, gcm cipher.AEAD, rawKey []byte) *Repository {
	return &Repository{db: db, gcm: gcm, rawKey: rawKey, now: time.Now}
}

func (r *Repository) decrypt(s, field string) string { return pii.Decrypt(r.gcm, s, field) }

// ── Dashboard ─────────────────────────────────────────────────────────────────

func (r *Repository) GetDashboardStats(ctx context.Context) (*DashboardResponse, error) {
	resp := &DashboardResponse{}
	err := r.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*)                                                        AS total_registrados,
			COUNT(*) FILTER (WHERE estado='activo')                         AS total_activos,
			COUNT(*) FILTER (WHERE estado='pendiente')                      AS total_pendientes
		FROM usuarios`).Scan(
		&resp.TotalUsuariosRegistrados,
		&resp.TotalUsuariosActivos,
		&resp.TotalUsuariosPendientes,
	)
	if err != nil {
		return nil, err
	}

	// Activity metrics count only finished work: practicas 'completada' and
	// simulacros 'completado'. The same rule is used by the weekly series, the
	// user detail and the public stats, so the numbers agree across screens
	// (en_curso is not finished yet; eliminada/abandonada were discarded).
	// "Today" is the Lima day containing r.now() (H79).
	today := `created_at >= ` + dayStartSQL("$1", "") + ` AND created_at < ` + dayStartSQL("$1", "+ INTERVAL '1 day'")
	err = r.db.QueryRowContext(ctx, `
		SELECT
			(SELECT COUNT(*) FROM practicas WHERE `+today+` AND estado = 'completada'),
			(SELECT COUNT(*) FROM simulacros WHERE `+today+` AND estado = 'completado'),
			(SELECT COUNT(DISTINCT usuario_id) FROM sesiones
			 WHERE last_seen_at > NOW() - INTERVAL '5 minutes' AND expires_at > NOW())`, r.now()).Scan(
		&resp.TotalPracticasHoy,
		&resp.TotalSimulacrosHoy,
		&resp.UsuariosOnline,
	)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// serieDiariaSQL is the per-Lima-day count of rows of table in estado over the
// last 7 Lima days including today ($1 = r.now()). generate_series supplies
// every day, so days without activity come back with 0 (H142) and the chart
// always has 7 points. table and estado are constants of this file.
func serieDiariaSQL(table, estado string) string {
	week := `created_at >= ` + dayStartSQL("$1", "- INTERVAL '6 days'") +
		` AND created_at < ` + dayStartSQL("$1", "+ INTERVAL '1 day'")
	today := `DATE_TRUNC('day', ` + localTS("$1::timestamptz") + `)`
	return `
		WITH dias AS (
			SELECT generate_series(` + today + ` - INTERVAL '6 days', ` + today + `, INTERVAL '1 day')::date AS fecha
		), agg AS (
			SELECT DATE_TRUNC('day', ` + localTS("created_at") + `)::date AS fecha, COUNT(*) AS total
			FROM ` + table + `
			WHERE ` + week + ` AND estado='` + estado + `'
			GROUP BY 1
		)
		SELECT dias.fecha, COALESCE(agg.total, 0) AS total
		FROM dias LEFT JOIN agg USING (fecha)
		ORDER BY dias.fecha ASC`
}

func (r *Repository) serieDiaria(ctx context.Context, table, estado string, now time.Time) ([]DiaMetricaItem, error) {
	rows, err := r.db.QueryContext(ctx, serieDiariaSQL(table, estado), now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []DiaMetricaItem{}
	for rows.Next() {
		var item DiaMetricaItem
		var fecha time.Time
		if err := rows.Scan(&fecha, &item.Total); err != nil {
			return nil, err
		}
		item.Fecha = fecha.Format("2006-01-02")
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) GetDashboardActividad(ctx context.Context) (practicas []DiaMetricaItem, simulacros []DiaMetricaItem, top []AsignaturaMetricaItem, err error) {
	// Last 7 Lima days including today, grouped by Lima day (H79), zero-filled.
	now := r.now()
	if practicas, err = r.serieDiaria(ctx, "practicas", "completada", now); err != nil {
		return
	}
	if simulacros, err = r.serieDiaria(ctx, "simulacros", "completado", now); err != nil {
		return
	}

	// Top 5 asignaturas by number of questions practiced, counting only
	// completed practices like the other activity metrics (H141).
	topRows, err := r.db.QueryContext(ctx, `
		SELECT a.nombre, SUM(pa.numero_preguntas) AS total
		FROM practica_asignaturas pa
		JOIN practicas p ON p.id = pa.practica_id AND p.estado = 'completada'
		JOIN asignaturas a ON a.id = pa.asignatura_id
		GROUP BY a.id, a.nombre
		ORDER BY total DESC, a.id
		LIMIT 5`)
	if err != nil {
		return
	}
	defer topRows.Close()
	for topRows.Next() {
		var item AsignaturaMetricaItem
		if err = topRows.Scan(&item.Nombre, &item.Total); err != nil {
			return
		}
		top = append(top, item)
	}
	if top == nil {
		top = []AsignaturaMetricaItem{}
	}

	return
}

// ── Usuario Detalle ───────────────────────────────────────────────────────────

type usuarioDetalleRow struct {
	ID              int64         `db:"id"`
	PersonaID       int64         `db:"persona_id"`
	DNI             string        `db:"dni"`
	Nombres         string        `db:"nombres"`
	ApellidoPaterno string        `db:"apellido_paterno"`
	ApellidoMaterno string        `db:"apellido_materno"`
	Grado           string        `db:"grado"`
	Especialidad    string        `db:"especialidad"`
	Institucion     string        `db:"institucion"`
	Telefono        string        `db:"telefono"`
	Email           string        `db:"email"`
	Username        string        `db:"username"`
	Rol             string        `db:"rol"`
	Estado          string        `db:"estado"`
	ActivatedAt     sql.NullTime  `db:"activated_at"`
	ActivatedBy     sql.NullInt64 `db:"activated_by"`
	CreatedAt       time.Time     `db:"created_at"`
	UpdatedAt       time.Time     `db:"updated_at"`
	TotalPracticas  int64         `db:"total_practicas"`
	TotalSimulacros int64         `db:"total_simulacros"`
	NotaPromedio    float64       `db:"nota_promedio"`
}

func (r *Repository) GetUsuarioDetalle(ctx context.Context, id int64) (*UsuarioDetalleResponse, error) {
	var row usuarioDetalleRow
	err := r.db.GetContext(ctx, &row, `
		SELECT
			u.id, u.persona_id,
			p.dni, p.nombres, p.apellido_paterno, p.apellido_materno,
			COALESCE(g.nombre,'') AS grado,
			COALESCE(e.nombre,'') AS especialidad,
			COALESCE(i.nombre,'') AS institucion,
			COALESCE(p.telefono,'') AS telefono,
			COALESCE(p.email,'') AS email,
			u.username, u.rol, u.estado,
			u.activated_at, u.activated_by,
			u.created_at, u.updated_at,
			COALESCE(pr_stats.total_practicas, 0)  AS total_practicas,
			COALESCE(sim_stats.total_simulacros, 0) AS total_simulacros,
			COALESCE(sim_stats.nota_promedio, 0)    AS nota_promedio
		FROM usuarios u
		JOIN personas p ON p.id = u.persona_id
		LEFT JOIN grados g ON g.id = p.grado_id
		LEFT JOIN especialidades e ON e.id = p.especialidad_id
		LEFT JOIN instituciones i ON i.id = p.institucion_id
		LEFT JOIN (
			SELECT usuario_id, COUNT(*) AS total_practicas
			FROM practicas WHERE estado='completada'
			GROUP BY usuario_id
		) pr_stats ON pr_stats.usuario_id = u.id
		LEFT JOIN (
			SELECT usuario_id, COUNT(*) AS total_simulacros, AVG(nota_centesimal) AS nota_promedio
			FROM simulacros WHERE estado='completado'
			GROUP BY usuario_id
		) sim_stats ON sim_stats.usuario_id = u.id
		WHERE u.id = $1`, id)
	if err != nil {
		return nil, err
	}

	resp := &UsuarioDetalleResponse{
		ID:              row.ID,
		PersonaID:       row.PersonaID,
		DNI:             r.decrypt(row.DNI, "dni"),
		Nombres:         r.decrypt(row.Nombres, "nombres"),
		ApellidoPaterno: r.decrypt(row.ApellidoPaterno, "apellido_paterno"),
		ApellidoMaterno: r.decrypt(row.ApellidoMaterno, "apellido_materno"),
		Grado:           row.Grado,
		Especialidad:    row.Especialidad,
		Institucion:     row.Institucion,
		Telefono:        r.decrypt(row.Telefono, "telefono"),
		Email:           r.decrypt(row.Email, "email"),
		Username:        row.Username,
		Rol:             row.Rol,
		Estado:          row.Estado,
		CreatedAt:       row.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       row.UpdatedAt.Format(time.RFC3339),
		TotalPracticas:  row.TotalPracticas,
		TotalSimulacros: row.TotalSimulacros,
		NotaPromedio:    row.NotaPromedio,
	}
	if row.ActivatedAt.Valid {
		s := row.ActivatedAt.Time.Format(time.RFC3339)
		resp.ActivatedAt = &s
	}
	if row.ActivatedBy.Valid {
		s := fmt.Sprintf("%d", row.ActivatedBy.Int64)
		resp.ActivatedBy = &s
	}

	return resp, nil
}

// UpdateUsuarioRol changes a user's role and revokes all of their sessions in
// the same transaction, so tokens issued with the old role stop working.
//
// Last-admin protection is race-free: the target row and every active admin
// row are locked (in id order, so concurrent calls cannot deadlock) before the
// check. A concurrent demotion that committed first is re-evaluated by
// PostgreSQL when the lock is granted, so two admins demoting each other can
// never leave zero admins. Returns errUsuarioNotFound / errLastAdmin.
// auditFn writes the audit row in the same transaction; its error rolls the
// change back.
func (r *Repository) UpdateUsuarioRol(ctx context.Context, id int64, rol string, auditFn audit.TxFunc) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	type lockedRow struct {
		ID     int64  `db:"id"`
		Rol    string `db:"rol"`
		Estado string `db:"estado"`
	}
	var rows []lockedRow
	if err := tx.SelectContext(ctx, &rows, `
		SELECT id, rol, estado FROM usuarios
		WHERE id = $1 OR (rol = 'admin' AND estado = 'activo')
		ORDER BY id
		FOR UPDATE`, id); err != nil {
		return err
	}
	var target *lockedRow
	otherAdmins := 0
	for i := range rows {
		if rows[i].ID == id {
			target = &rows[i]
		} else if rows[i].Rol == "admin" && rows[i].Estado == "activo" {
			otherAdmins++
		}
	}
	if target == nil {
		return errUsuarioNotFound
	}
	if target.Rol != rol {
		if target.Rol == "admin" && target.Estado == "activo" && otherAdmins == 0 {
			return errLastAdmin
		}
		if _, err := tx.ExecContext(ctx, `UPDATE usuarios SET rol=$1, updated_at=NOW() WHERE id=$2`, rol, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM sesiones WHERE usuario_id=$1`, id); err != nil {
			return err
		}
	} // else no change: keep sessions (the request is still audited)
	if err := audit.RunTx(ctx, tx, auditFn); err != nil {
		return err
	}
	return tx.Commit()
}

// ── Audit Log ─────────────────────────────────────────────────────────────────

type auditLogRow struct {
	ID        int64          `db:"id"`
	UsuarioID sql.NullInt64  `db:"usuario_id"`
	Username  sql.NullString `db:"username"`
	Accion    string         `db:"accion"`
	Entidad   string         `db:"entidad"`
	EntidadID sql.NullInt64  `db:"entidad_id"`
	Detalle   []byte         `db:"detalle"`
	IP        sql.NullString `db:"ip"`
	UserAgent sql.NullString `db:"user_agent"`
	CreatedAt time.Time      `db:"created_at"`
}

func (r *Repository) GetAuditLog(ctx context.Context, page, perPage int, from, to, q string) (*AuditLogResponse, error) {
	offset := int64(page-1) * int64(perPage)

	// Build dynamic params
	fromParam := sql.NullString{String: from, Valid: from != ""}
	toParam := sql.NullString{String: to, Valid: to != ""}
	q = db.EscapeLike(q)

	rows, err := r.db.QueryContext(ctx, `
		SELECT al.id, al.usuario_id, COALESCE(u.username,'') AS username,
		       al.accion, al.entidad, al.entidad_id,
		       al.detalle, CAST(al.ip AS TEXT), al.user_agent, al.created_at
		FROM audit_log al
		LEFT JOIN usuarios u ON u.id = al.usuario_id
		WHERE ($1::date IS NULL OR al.created_at >= ($1::date)::timestamp AT TIME ZONE `+zonaSQL+`)
		  AND ($2::date IS NULL OR al.created_at < ($2::date + 1)::timestamp AT TIME ZONE `+zonaSQL+`)
		  AND ($3 = '' OR u.username ILIKE '%' || $3 || '%' ESCAPE '\')
		ORDER BY al.created_at DESC, al.id DESC
		LIMIT $4 OFFSET $5`,
		fromParam, toParam, q, perPage, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []AuditLogItem
	for rows.Next() {
		var row auditLogRow
		if err := rows.Scan(&row.ID, &row.UsuarioID, &row.Username, &row.Accion, &row.Entidad,
			&row.EntidadID, &row.Detalle, &row.IP, &row.UserAgent, &row.CreatedAt); err != nil {
			return nil, err
		}
		item := AuditLogItem{
			ID:        row.ID,
			Accion:    row.Accion,
			Entidad:   row.Entidad,
			CreatedAt: row.CreatedAt.Format(time.RFC3339),
		}
		if row.UsuarioID.Valid {
			item.UsuarioID = &row.UsuarioID.Int64
		}
		if row.Username.Valid && row.Username.String != "" {
			item.Username = row.Username.String
		}
		if row.EntidadID.Valid {
			item.EntidadID = &row.EntidadID.Int64
		}
		if row.IP.Valid {
			item.IP = row.IP.String
		}
		if row.UserAgent.Valid {
			item.UserAgent = row.UserAgent.String
		}
		if len(row.Detalle) > 0 {
			var m map[string]interface{}
			if json.Unmarshal(row.Detalle, &m) == nil {
				item.Detalle = m
			}
		}
		items = append(items, item)
	}
	if items == nil {
		items = []AuditLogItem{}
	}

	// Count total
	var total int64
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM audit_log al
		LEFT JOIN usuarios u ON u.id = al.usuario_id
		WHERE ($1::date IS NULL OR al.created_at >= ($1::date)::timestamp AT TIME ZONE `+zonaSQL+`)
		  AND ($2::date IS NULL OR al.created_at < ($2::date + 1)::timestamp AT TIME ZONE `+zonaSQL+`)
		  AND ($3 = '' OR u.username ILIKE '%' || $3 || '%' ESCAPE '\')`,
		fromParam, toParam, q).Scan(&total); err != nil {
		return nil, err
	}

	return &AuditLogResponse{
		Data:    items,
		Total:   total,
		Page:    page,
		PerPage: perPage,
	}, nil
}

// ── Ingresos ──────────────────────────────────────────────────────────────────

// approvedAt is the date revenue is attributed to: the moment the inscription
// was approved (revisado_at), not when it was requested. created_at is only a
// fallback for legacy rows approved without a review timestamp.
const approvedAt = `COALESCE(revisado_at, created_at)`

// sumMonto is an exact DECIMAL sum as text with 2 places (H66): money never
// goes through float64.
const sumMonto = `ROUND(COALESCE(SUM(monto), 0), 2)::text`

// GetIngresosSummary aggregates approved revenue. Months and years are Lima
// calendar months/years of the approval instant and of r.now() (H79).
func (r *Repository) GetIngresosSummary(ctx context.Context) (*IngresosResponse, error) {
	resp := &IngresosResponse{}
	now := r.now()
	approvedLocal := localTS(approvedAt)
	nowLocal := localTS("$1::timestamptz")

	if err := r.db.QueryRowContext(ctx, `
		SELECT `+sumMonto+`,
		       (SELECT `+sumMonto+` FROM inscripciones WHERE estado='aprobado'
		          AND DATE_TRUNC('year', `+approvedLocal+`) = DATE_TRUNC('year', `+nowLocal+`)),
		       (SELECT `+sumMonto+` FROM inscripciones WHERE estado='aprobado'
		          AND DATE_TRUNC('month', `+approvedLocal+`) = DATE_TRUNC('month', `+nowLocal+`)),
		       COUNT(*)
		FROM inscripciones WHERE estado='aprobado'`, now).
		Scan(&resp.TotalRecaudado, &resp.TotalEsteAnio, &resp.TotalEsteMes, &resp.TotalInscripciones); err != nil {
		return nil, err
	}

	// Serie mensual: last 12 Lima months including the current one, with
	// "0.00" for months without approvals (H142).
	mesActual := `DATE_TRUNC('month', ` + nowLocal + `)`
	rows, err := r.db.QueryContext(ctx, `
		WITH meses AS (
			SELECT generate_series(`+mesActual+` - INTERVAL '11 months', `+mesActual+`, INTERVAL '1 month') AS mes
		), agg AS (
			SELECT DATE_TRUNC('month', `+approvedLocal+`) AS mes, `+sumMonto+` AS total
			FROM inscripciones
			WHERE estado='aprobado'
			  AND `+approvedLocal+` >= `+mesActual+` - INTERVAL '11 months'
			  AND `+approvedLocal+` < `+mesActual+` + INTERVAL '1 month'
			GROUP BY 1
		)
		SELECT TO_CHAR(meses.mes, 'YYYY-MM') AS mes, COALESCE(agg.total, '0.00') AS total
		FROM meses LEFT JOIN agg USING (mes)
		ORDER BY meses.mes ASC`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var item MesIngresoItem
		if err := rows.Scan(&item.Mes, &item.Total); err != nil {
			return nil, err
		}
		resp.SerieMensual = append(resp.SerieMensual, item)
	}
	if resp.SerieMensual == nil {
		resp.SerieMensual = []MesIngresoItem{}
	}

	return resp, nil
}

type configPagoRow struct {
	Anio      int       `db:"anio"`
	Monto     string    `db:"monto"`
	UpdatedAt time.Time `db:"updated_at"`
}

func (r *Repository) GetIngresosConfig(ctx context.Context) ([]ConfigPago, error) {
	var rows []configPagoRow
	if err := r.db.SelectContext(ctx, &rows, `SELECT anio, monto::text AS monto, updated_at FROM configuracion_pago ORDER BY anio DESC`); err != nil {
		return nil, err
	}
	result := make([]ConfigPago, len(rows))
	for i, row := range rows {
		result[i] = ConfigPago{
			Anio:      row.Anio,
			Monto:     json.Number(row.Monto),
			UpdatedAt: row.UpdatedAt.Format(time.RFC3339),
		}
	}
	return result, nil
}

// UpsertConfigPago stores the price for a year. monto is an exact decimal
// string (validated by parseMonto) so no float rounding happens on the way to
// DECIMAL(10,2). It also returns the previous value (nil if the year was new)
// for the audit trail. auditFn receives both values and writes the audit row
// in the same transaction; its error rolls the change back.
func (r *Repository) UpsertConfigPago(ctx context.Context, anio int, monto string,
	auditFn func(ctx context.Context, tx sqlx.ExecerContext, c *ConfigPago, anterior *string) error,
) (*ConfigPago, *string, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback()
	var row struct {
		configPagoRow
		Anterior *string `db:"anterior"`
	}
	err = tx.GetContext(ctx, &row, `
		WITH prev AS (SELECT monto FROM configuracion_pago WHERE anio = $1)
		INSERT INTO configuracion_pago (anio, monto)
		VALUES ($1, $2::numeric)
		ON CONFLICT (anio) DO UPDATE SET monto = EXCLUDED.monto, updated_at = NOW()
		RETURNING anio, monto::text AS monto, updated_at, (SELECT monto::text FROM prev) AS anterior`, anio, monto)
	if err != nil {
		return nil, nil, err
	}
	c := &ConfigPago{
		Anio:      row.Anio,
		Monto:     json.Number(row.Monto),
		UpdatedAt: row.UpdatedAt.Format(time.RFC3339),
	}
	if auditFn != nil {
		if err := auditFn(ctx, tx, c, row.Anterior); err != nil {
			return nil, nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, nil, err
	}
	return c, row.Anterior, nil
}
