package fuentes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// Admin limits (SIA-E2-03 regla 4; SIA-E2-06 regla 2).
const (
	MaxFuentesPorProfesion = 100
	MaxFuentesTotal        = 500
	MaxVersionesVivas      = 20
	AccionVersionCrear     = "fuente.version_crear"

	// claveBloqueo serialises every change that counts fuentes or checks the
	// global hash (contract S1 «Límite de fuentes»); distinct from the test
	// lock and the seed lock.
	claveBloqueo = "learning:fuentes"

	constraintUnaProcesando = "uq_fuente_versiones_una_procesando"
)

// Repository is the admin side of the fuentes store, outside the port: the
// uploads S1/S2 and the detail view. It shares the inserts with PgSource.
type Repository struct {
	db *sqlx.DB
}

// NewRepository creates the admin repository over db.
func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

// CrearFuente runs S1 steps 7–9 and the creation in one transaction under the
// fuentes lock: references, global duplicate with detail, fuentes limit,
// then the same inserts and fuente.crear event as GuardarFuente.
func (r *Repository) CrearFuente(ctx context.Context, f NuevaFuente, aud Auditor) (int64, error) {
	var id int64
	err := r.enTxBloqueada(ctx, func(tx *sqlx.Tx) error {
		if err := comprobarReferencias(ctx, tx, f.EspecialidadIDs, f.ProcesoID); err != nil {
			return err
		}
		sha, _, _ := Huella(f.Contenido)
		if err := comprobarDuplicado(ctx, tx, sha); err != nil {
			return err
		}
		if err := comprobarLimite(ctx, tx, f.EspecialidadIDs, sinExcluir); err != nil {
			return err
		}
		var err error
		id, err = insertarFuente(ctx, tx, f, aud)
		return err
	})
	return id, err
}

// NuevaVersion runs S2 in one transaction under the fuentes lock and returns
// the new version number. Order: 404, estado_invalido, duplicate (also
// against this fuente), fuentes limit (only for a 'fallida' fuente) and the
// 20-version limit, archiving the oldest version nobody cites.
func (r *Repository) NuevaVersion(ctx context.Context, fuenteID int64, nombre string, contenido []byte,
	creadoPor *int64, citantes Citantes, aud Auditor) (int, error) {
	var numero int
	err := r.enTxBloqueada(ctx, func(tx *sqlx.Tx) error {
		fuente, err := bloquearParaCambio(ctx, tx, fuenteID)
		if err != nil {
			return err
		}
		admite := fuente.Estado == EstadoFallida || (fuente.Estado == EstadoActiva && !fuente.Procesando)
		if !admite {
			return ErrEstadoInvalido
		}
		sha, tamano, lineas := Huella(contenido)
		if err := comprobarDuplicado(ctx, tx, sha); err != nil {
			return err
		}
		if fuente.Estado == EstadoFallida {
			especialidades, err := especialidadesDe(ctx, tx, fuenteID)
			if err != nil {
				return err
			}
			if err := comprobarLimite(ctx, tx, especialidades, sinExcluir); err != nil {
				return err
			}
		}
		archivada, err := liberarVersion(ctx, tx, fuenteID, fuente.VigenteID, citantes)
		if err != nil {
			return err
		}
		if err := tx.GetContext(ctx, &numero, `
			INSERT INTO fuente_versiones (fuente_id, numero, nombre_archivo, sha256, tamano_bytes, lineas, contenido, creado_por)
			SELECT $1, MAX(numero) + 1, $2, $3, $4, $5, $6, $7 FROM fuente_versiones WHERE fuente_id = $1
			RETURNING numero`, fuenteID, nombre, sha, tamano, lineas, string(contenido), creadoPor); err != nil {
			return mapearVersion(mapearDuplicada(fmt.Errorf("crear versión de la fuente %d: %w", fuenteID, err)))
		}
		estadoNuevo := fuente.Estado
		if fuente.Estado == EstadoFallida {
			estadoNuevo = EstadoProcesando
		}
		if _, err := tx.ExecContext(ctx, `UPDATE fuentes SET estado = $2, updated_at = NOW() WHERE id = $1`,
			fuenteID, estadoNuevo); err != nil {
			return fmt.Errorf("actualizar fuente %d: %w", fuenteID, err)
		}
		return auditar(ctx, tx, aud, Evento{Accion: AccionVersionCrear, Entidad: EntidadFuente, EntidadID: fuenteID, Detalle: map[string]any{
			"version": numero, "sha256": sha, "estado_anterior": fuente.Estado, "estado_nuevo": estadoNuevo,
			"version_archivada": archivada,
		}})
	})
	return numero, err
}

// Detalle returns the FuenteDetalle of a fuente, or ErrNoEncontrado: its
// summary (listado.go), metadata, the mark of the current version, every
// version newest first and the citing contents.
func (r *Repository) Detalle(ctx context.Context, id int64, citantes Citantes) (FuenteDetalle, error) {
	var f struct {
		filaResumen
		Anio         *int           `db:"anio"`
		ProcesoID    sql.NullInt64  `db:"proceso_id"`
		Proceso      sql.NullString `db:"proceso"`
		MotivoRetiro *string        `db:"motivo_retiro"`
		VigenteID    sql.NullInt64  `db:"version_vigente_id"`
	}
	err := sqlx.GetContext(ctx, r.db, &f, `
		SELECT`+columnasResumen+`, f.anio, f.proceso_id, p.nombre AS proceso, f.motivo_retiro, f.version_vigente_id`+
		desdeResumen+` LEFT JOIN procesos p ON p.id = f.proceso_id WHERE f.id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return FuenteDetalle{}, ErrNoEncontrado
	}
	if err != nil {
		return FuenteDetalle{}, fmt.Errorf("leer fuente %d: %w", id, err)
	}
	especialidades, err := especialidadesConNombre(ctx, r.db, []int64{id})
	if err != nil {
		return FuenteDetalle{}, err
	}
	d := FuenteDetalle{ResumenFuente: f.resumen(especialidades[id]), Anio: f.Anio, MotivoRetiro: f.MotivoRetiro,
		Versiones: []VersionDetalle{}}
	if f.ProcesoID.Valid {
		d.Proceso = &ProcesoRef{ID: f.ProcesoID.Int64, Nombre: f.Proceso.String}
	}
	var versiones []struct {
		ID int64 `db:"id"`
		VersionDetalle
		OficialNota *string    `db:"oficial_nota"`
		OficialAt   *time.Time `db:"oficial_at"`
	}
	if err := sqlx.SelectContext(ctx, r.db, &versiones, `
		SELECT id, numero, estado, motivo_fallo, nombre_archivo, sha256, tamano_bytes, lineas, pasajes_count,
		       oficial_verificada, oficial_entidad, oficial_nota, oficial_at, created_at, procesada_at
		FROM fuente_versiones WHERE fuente_id = $1 ORDER BY numero DESC`, id); err != nil {
		return FuenteDetalle{}, fmt.Errorf("versiones de la fuente %d: %w", id, err)
	}
	for _, v := range versiones {
		vd := v.VersionDetalle
		vd.CreatedAt = vd.CreatedAt.UTC()
		d.Versiones = append(d.Versiones, vd)
		esVigente := f.VigenteID.Valid && v.ID == f.VigenteID.Int64
		if esVigente && v.OficialVerificada && v.OficialEntidad != nil && v.OficialNota != nil && v.OficialAt != nil {
			d.Oficial = &Oficial{EntidadEmisora: *v.OficialEntidad, Nota: *v.OficialNota, MarcadoAt: v.OficialAt.UTC()}
		}
	}
	if d.ContenidosCitantes, err = citantes.ContarPorFuente(ctx, r.db, id); err != nil {
		return FuenteDetalle{}, fmt.Errorf("citantes de la fuente %d: %w", id, err)
	}
	return d, nil
}

// DuplicadoDe returns the detail of the fuente holding the hash; S1 and S2 use
// it after a rollback when the unique index (the net under the lock) fired.
func (r *Repository) DuplicadoDe(ctx context.Context, sha string) error {
	return comprobarDuplicado(ctx, r.db, sha)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func (r *Repository) enTxBloqueada(ctx context.Context, fn func(tx *sqlx.Tx) error) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("iniciar transacción: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, claveBloqueo); err != nil {
		return fmt.Errorf("bloqueo de fuentes: %w", err)
	}
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return mapearVersion(mapearDuplicada(fmt.Errorf("confirmar: %w", err)))
	}
	return nil
}

// comprobarReferencias: every especialidad exists and is active, and the
// proceso exists (S1 step 7).
func comprobarReferencias(ctx context.Context, q sqlx.QueryerContext, especialidades []int64, procesoID *int64) error {
	if len(especialidades) > 0 {
		var activas int
		if err := sqlx.GetContext(ctx, q, &activas, `
			SELECT COUNT(*) FROM especialidades WHERE id = ANY($1::bigint[]) AND estado = 'activo'`,
			pq.Array(especialidades)); err != nil {
			return fmt.Errorf("validar especialidades: %w", err)
		}
		if activas != len(especialidades) {
			return ErrReferenciaInvalida
		}
	}
	if procesoID != nil {
		var existe bool
		if err := sqlx.GetContext(ctx, q, &existe, `SELECT EXISTS (SELECT 1 FROM procesos WHERE id = $1)`, *procesoID); err != nil {
			return fmt.Errorf("validar proceso: %w", err)
		}
		if !existe {
			return ErrReferenciaInvalida
		}
	}
	return nil
}

// comprobarDuplicado rejects a hash present in any version of any fuente, in
// any state, archived included (S1 step 8), with the fuente's detail.
func comprobarDuplicado(ctx context.Context, q sqlx.QueryerContext, sha string) error {
	var d struct {
		FuenteID int64  `db:"fuente_id"`
		Titulo   string `db:"titulo"`
		Estado   string `db:"estado"`
		Version  int    `db:"numero"`
	}
	err := sqlx.GetContext(ctx, q, &d, `
		SELECT v.fuente_id, f.titulo, f.estado, v.numero
		FROM fuente_versiones v JOIN fuentes f ON f.id = v.fuente_id WHERE v.sha256 = $1`, sha)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("buscar duplicado: %w", err)
	}
	return &DuplicadaError{FuenteID: d.FuenteID, Titulo: d.Titulo, Estado: d.Estado, Version: d.Version}
}

// sinExcluir is the excluir of comprobarLimite when the fuente does not count yet.
const sinExcluir int64 = 0

// comprobarLimite checks that one more counted fuente (activa or procesando)
// with these especialidades (none = all) fits: ≤ 100 per active profession,
// where a fuente without professions counts for every one, then ≤ 500 in
// total. The first profession that breaks it, by id, goes in the detail.
// excluir leaves out a fuente that already counts and is being changed (S7),
// so it is counted once, with its new especialidades.
func comprobarLimite(ctx context.Context, tx *sqlx.Tx, especialidades []int64, excluir int64) error {
	if especialidades == nil {
		especialidades = []int64{}
	}
	var llena sql.NullInt64
	if err := tx.GetContext(ctx, &llena, `
		WITH contadas AS (SELECT id FROM fuentes WHERE estado IN ('activa', 'procesando') AND id <> $3),
		     sin_esp  AS (SELECT COUNT(*) AS n FROM contadas c
		                  WHERE NOT EXISTS (SELECT 1 FROM fuente_especialidades fe WHERE fe.fuente_id = c.id))
		SELECT MIN(e.id) FROM especialidades e
		WHERE e.estado = 'activo' AND (cardinality($1::bigint[]) = 0 OR e.id = ANY($1::bigint[]))
		  AND (SELECT n FROM sin_esp)
		      + (SELECT COUNT(*) FROM fuente_especialidades fe JOIN contadas c ON c.id = fe.fuente_id
		         WHERE fe.especialidad_id = e.id) + 1 > $2`,
		pq.Array(especialidades), MaxFuentesPorProfesion, excluir); err != nil {
		return fmt.Errorf("límite por profesión: %w", err)
	}
	if llena.Valid {
		id := llena.Int64
		return &LimiteError{Limite: MaxFuentesPorProfesion, EspecialidadID: &id}
	}
	var total int
	if err := tx.GetContext(ctx, &total, `SELECT COUNT(*) FROM fuentes WHERE estado IN ('activa', 'procesando') AND id <> $1`, excluir); err != nil {
		return fmt.Errorf("límite total: %w", err)
	}
	if total+1 > MaxFuentesTotal {
		return &LimiteError{Limite: MaxFuentesTotal}
	}
	return nil
}

// fuenteBloqueada is the fuente as a change sees it, with its row locked.
type fuenteBloqueada struct {
	Estado     string        `db:"estado"`
	VigenteID  sql.NullInt64 `db:"version_vigente_id"`
	Procesando bool          `db:"procesando"` // some version is 'procesando'
}

// bloquearParaCambio locks the fuente for an admin change (after the fuentes
// lock, so the order is advisory → fuente → versions; no admin change locks
// a 'procesando' version, which is the worker's). ErrNoEncontrado if missing.
func bloquearParaCambio(ctx context.Context, tx *sqlx.Tx, fuenteID int64) (fuenteBloqueada, error) {
	var f fuenteBloqueada
	err := tx.GetContext(ctx, &f, `
		SELECT estado, version_vigente_id,
		       EXISTS (SELECT 1 FROM fuente_versiones WHERE fuente_id = f.id AND estado = 'procesando') AS procesando
		FROM fuentes f WHERE id = $1 FOR UPDATE`, fuenteID)
	if errors.Is(err, sql.ErrNoRows) {
		return fuenteBloqueada{}, ErrNoEncontrado
	}
	if err != nil {
		return fuenteBloqueada{}, fmt.Errorf("bloquear fuente %d: %w", fuenteID, err)
	}
	return f, nil
}

func especialidadesDe(ctx context.Context, q sqlx.QueryerContext, fuenteID int64) ([]int64, error) {
	ids := []int64{}
	if err := sqlx.SelectContext(ctx, q, &ids,
		`SELECT especialidad_id FROM fuente_especialidades WHERE fuente_id = $1 ORDER BY especialidad_id`, fuenteID); err != nil {
		return nil, fmt.Errorf("especialidades de la fuente %d: %w", fuenteID, err)
	}
	return ids, nil
}

// liberarVersion keeps at most 20 non-archived versions: with 20 already, it
// archives the oldest that is not current, not in process and not cited
// (contenido and motivo_fallo to NULL, passages deleted; number, hash, mark,
// passage count and dates kept) and returns its number; ErrLimiteVersiones if none can go.
func liberarVersion(ctx context.Context, tx *sqlx.Tx, fuenteID int64, vigente sql.NullInt64, citantes Citantes) (*int, error) {
	var vivas []struct {
		ID     int64  `db:"id"`
		Numero int    `db:"numero"`
		Estado string `db:"estado"`
	}
	if err := tx.SelectContext(ctx, &vivas, `
		SELECT id, numero, estado FROM fuente_versiones
		WHERE fuente_id = $1 AND estado <> 'archivada' ORDER BY numero`, fuenteID); err != nil {
		return nil, fmt.Errorf("versiones de la fuente %d: %w", fuenteID, err)
	}
	if len(vivas) < MaxVersionesVivas {
		return nil, nil
	}
	for _, v := range vivas {
		if (vigente.Valid && v.ID == vigente.Int64) || v.Estado == EstadoProcesando {
			continue
		}
		citada, err := citantes.VersionCitada(ctx, tx, v.ID)
		if err != nil {
			return nil, fmt.Errorf("citas de la versión %d: %w", v.ID, err)
		}
		if citada {
			continue
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM pasajes WHERE fuente_version_id = $1`, v.ID); err != nil {
			return nil, fmt.Errorf("borrar pasajes de la versión %d: %w", v.ID, err)
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE fuente_versiones SET estado = 'archivada', contenido = NULL, motivo_fallo = NULL
			WHERE id = $1`, v.ID); err != nil {
			return nil, fmt.Errorf("archivar versión %d: %w", v.ID, err)
		}
		n := v.Numero
		return &n, nil
	}
	return nil, ErrLimiteVersiones
}

// mapearVersion maps a race on the one-version-in-process index to
// ErrEstadoInvalido (the lock should make it unreachable).
func mapearVersion(err error) error {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23505" && pqErr.Constraint == constraintUnaProcesando {
		return ErrEstadoInvalido
	}
	return err
}

// BuscarPasajes is the admin search for citing (ADR 0007): the same SQL as
// PgSource.BuscarPasajes, over every especialidad, with the fuente's title.
// consulta is already validated.
func (r *Repository) BuscarPasajes(ctx context.Context, consulta string) ([]PasajeConFuente, error) {
	filas, err := buscarPasajesActivos(ctx, r.db, consulta, 0, MaxResultados)
	if err != nil {
		return nil, err
	}
	res := make([]PasajeConFuente, 0, len(filas))
	for _, f := range filas {
		res = append(res, PasajeConFuente{Pasaje: f.pasaje(), TituloFuente: f.TituloFuente})
	}
	return res, nil
}
