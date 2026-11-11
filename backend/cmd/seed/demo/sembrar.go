package main

import (
	"context"
	"crypto/cipher"
	"database/sql"
	"errors"
	"fmt"

	"github.com/cbaciliod/learning/cmd/seed/internal/persona"
	"github.com/cbaciliod/learning/internal/catalogs"
	"github.com/cbaciliod/learning/internal/inscripciones"
	"github.com/cbaciliod/learning/internal/questions"
	"github.com/cbaciliod/learning/internal/simulations"
	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"
)

// bcryptCost matches seed/admin; the tests lower it, because bcrypt is
// deliberately slow and the race detector makes it slower.
var bcryptCost = 12

const (
	claveBloqueo   = "learning:seed-demo"
	estadoActivo   = "activo"
	estadoInactivo = "inactivo"
	coeficiente    = 1.0
)

// sembrador holds the services the seed reuses: every rule they enforce
// (weights summing 100, a question's classification, a payment config for
// the year…) also applies to the demo data.
type sembrador struct {
	db     *sqlx.DB
	gcm    cipher.AEAD
	rawKey []byte
	cfg    config
	cat    *catalogs.Service
	preg   *questions.Service
	sim    *simulations.Service
	ins    *inscripciones.Service
	creado map[string]int    // created rows per kind, for the summary
	hashes map[string][]byte // bcrypt hash per rol, computed once per run
}

// sembrar runs every step under a session advisory lock, so two runs never
// interleave. Each step looks its rows up by natural key and creates only
// what is missing: a run after a partial failure completes the data.
func sembrar(ctx context.Context, conn *sqlx.DB, gcm cipher.AEAD, rawKey []byte, cfg config) (string, error) {
	lock, err := conn.Connx(ctx)
	if err != nil {
		return "", fmt.Errorf("conexión del bloqueo: %w", err)
	}
	defer lock.Close()
	if _, err := lock.ExecContext(ctx, `SELECT pg_advisory_lock(hashtext($1))`, claveBloqueo); err != nil {
		return "", fmt.Errorf("bloqueo del seed: %w", err)
	}
	defer func() {
		_, _ = lock.ExecContext(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock(hashtext($1))`, claveBloqueo)
	}()

	qSvc := questions.NewService(questions.NewRepository(conn))
	s := &sembrador{db: conn, gcm: gcm, rawKey: rawKey, cfg: cfg,
		cat: catalogs.NewService(catalogs.NewRepository(conn)), preg: qSvc,
		sim:    simulations.NewService(simulations.NewRepository(conn), simulations.NewQuestionsAdapter(qSvc)),
		ins:    inscripciones.NewService(inscripciones.NewRepository(conn, gcm, rawKey)),
		creado: map[string]int{}, hashes: map[string][]byte{}}
	if err := s.todo(ctx); err != nil {
		return "", err
	}
	return fmt.Sprintf("creados: %v", s.creado), nil
}

func (s *sembrador) todo(ctx context.Context) error {
	inst := map[string]int64{}
	for _, nombre := range []string{institucionSalud, institucionCompat} {
		id, err := s.institucion(ctx, nombre)
		if err != nil {
			return err
		}
		inst[nombre] = id
	}
	grado, err := s.grado(ctx, inst[institucionSalud])
	if err != nil {
		return err
	}
	esp, err := s.especialidades(ctx, inst[institucionSalud])
	if err != nil {
		return err
	}
	asig, err := s.asignaturas(ctx)
	if err != nil {
		return err
	}
	if err := s.pago(ctx); err != nil {
		return err
	}
	procesos := map[string]int64{}
	for _, p := range procesosDemo {
		id, err := s.proceso(ctx, inst[p.Institucion], p, asig, esp)
		if err != nil {
			return fmt.Errorf("proceso %q: %w", p.Nombre, err)
		}
		procesos[p.Nombre] = id
	}
	usuarios := map[string]int64{}
	for _, u := range usuariosDemo {
		id, err := s.usuario(ctx, u, inst[institucionSalud], grado, esp[u.Especialidad])
		if err != nil {
			return fmt.Errorf("usuario %s: %w", u.Username, err)
		}
		usuarios[u.Username] = id
	}
	for _, u := range usuariosDemo {
		if u.Inscripcion == "" {
			continue
		}
		if err := s.inscripcion(ctx, usuarios[u.Username], procesos[procesoSERUMS], u.Inscripcion, usuarios["demo_admin"]); err != nil {
			return fmt.Errorf("inscripción de %s: %w", u.Username, err)
		}
	}
	return nil
}

// ── Catálogos ─────────────────────────────────────────────────────────────────

// buscar returns the id of the row the query finds, or 0.
func (s *sembrador) buscar(ctx context.Context, q string, args ...any) (int64, error) {
	var id int64
	err := s.db.GetContext(ctx, &id, q, args...)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return id, err
}

func (s *sembrador) institucion(ctx context.Context, nombre string) (int64, error) {
	id, err := s.buscar(ctx, `SELECT id FROM instituciones WHERE nombre = $1`, nombre)
	if err != nil || id != 0 {
		return id, err
	}
	siglas := "DEMOS"
	if nombre == institucionCompat {
		siglas = "DEMOC"
	}
	r, err := s.cat.CreateInstitucion(ctx, catalogs.CreateInstitucionRequest{Nombre: nombre, Siglas: siglas})
	if err != nil {
		return 0, fmt.Errorf("institución: %w", err)
	}
	s.creado["instituciones"]++
	return r.ID, nil
}

func (s *sembrador) grado(ctx context.Context, institucionID int64) (int64, error) {
	id, err := s.buscar(ctx, `SELECT id FROM grados WHERE institucion_id = $1 AND nombre = $2`, institucionID, gradoDemo)
	if err != nil || id != 0 {
		return id, err
	}
	r, err := s.cat.CreateGrado(ctx, catalogs.CreateGradoRequest{InstitucionID: institucionID, Nombre: gradoDemo,
		Abreviatura: "GD", Nivel: nivelDemo, Orden: 1})
	if err != nil {
		return 0, fmt.Errorf("grado: %w", err)
	}
	s.creado["grados"]++
	return r.ID, nil
}

// especialidades creates them active and then sets the state each one must
// have, only when it differs: deactivating is refused once active users have
// that especialidad, so the inactive one is deactivated before the users are
// created and left alone afterwards.
func (s *sembrador) especialidades(ctx context.Context, institucionID int64) (map[string]int64, error) {
	ids := map[string]int64{}
	for _, e := range especialidadesDemo {
		var fila struct {
			ID     int64  `db:"id"`
			Estado string `db:"estado"`
		}
		err := s.db.GetContext(ctx, &fila, `SELECT id, estado FROM especialidades WHERE institucion_id = $1 AND codigo = $2`, institucionID, e.Codigo)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			r, err := s.cat.CreateEspecialidad(ctx, catalogs.CreateEspecialidadRequest{InstitucionID: institucionID,
				Nombre: e.Nombre, Codigo: e.Codigo, Nivel: nivelDemo})
			if err != nil {
				return nil, fmt.Errorf("especialidad %s: %w", e.Codigo, err)
			}
			fila.ID, fila.Estado = r.ID, r.Estado
			s.creado["especialidades"]++
		case err != nil:
			return nil, err
		}
		estado := estadoActivo
		if !e.Activa {
			estado = estadoInactivo
		}
		if fila.Estado != estado {
			if err := s.cat.UpdateEspecialidadEstado(ctx, fila.ID, estado); err != nil {
				return nil, fmt.Errorf("estado de la especialidad %s: %w", e.Codigo, err)
			}
		}
		ids[e.Codigo] = fila.ID
	}
	return ids, nil
}

func (s *sembrador) asignaturas(ctx context.Context) (map[string]int64, error) {
	ids := map[string]int64{}
	for _, a := range asignaturasDemo {
		id, err := s.buscar(ctx, `SELECT id FROM asignaturas WHERE codigo = $1`, a.Codigo)
		if err != nil {
			return nil, err
		}
		if id == 0 {
			r, err := s.cat.CreateAsignatura(ctx, catalogs.CreateAsignaturaRequest{Nombre: a.Nombre, Codigo: a.Codigo})
			if err != nil {
				return nil, fmt.Errorf("asignatura %s: %w", a.Codigo, err)
			}
			id = r.ID
			s.creado["asignaturas"]++
		}
		ids[a.Codigo] = id
	}
	return ids, nil
}

// pago gives the demo year a price (0.00) unless it already has one: an
// existing configuration is never overwritten.
func (s *sembrador) pago(ctx context.Context) error {
	res, err := s.db.ExecContext(ctx, `INSERT INTO configuracion_pago (anio, monto) VALUES ($1, $2) ON CONFLICT (anio) DO NOTHING`,
		s.cfg.Anio, montoInscripcion)
	if err != nil {
		return fmt.Errorf("configuración de pago: %w", err)
	}
	if n, _ := res.RowsAffected(); n > 0 {
		s.creado["configuracion_pago"]++
	}
	return nil
}

// ── Procesos ──────────────────────────────────────────────────────────────────

// proceso creates the proceso with descripcion ”, its compatibility rows,
// its servable questions, activates it (rows must sum 100), saves the
// per-especialidad weights and its simulacro configuration.
func (s *sembrador) proceso(ctx context.Context, institucionID int64, p procesoDemo, asig, esp map[string]int64) (int64, error) {
	id, err := s.buscar(ctx, `SELECT id FROM procesos WHERE institucion_id = $1 AND nombre = $2`, institucionID, p.Nombre)
	if err != nil {
		return 0, err
	}
	if id == 0 {
		r, err := s.cat.CreateProceso(ctx, catalogs.CreateProcesoRequest{InstitucionID: institucionID, Nombre: p.Nombre,
			Tipo: p.Tipo, Anio: s.cfg.Anio, Descripcion: ""})
		if err != nil {
			return 0, fmt.Errorf("crear: %w", err)
		}
		id = r.ID
		s.creado["procesos"]++
	}
	if err := s.areasCompatibilidad(ctx, id, p, asig); err != nil {
		return 0, err
	}
	for _, a := range p.Compatibilidad {
		if err := s.preguntas(ctx, id, asig[a.Codigo], a.Codigo); err != nil {
			return 0, err
		}
	}
	if err := s.cat.UpdateProcesoEstado(ctx, id, estadoActivo, nil); err != nil {
		return 0, fmt.Errorf("activar: %w", err)
	}
	for codigo, areas := range p.Pesos {
		req := catalogs.GuardarPesosRequest{}
		for _, a := range areas {
			req.Areas = append(req.Areas, catalogs.AreaPesoRequest{AsignaturaID: asig[a.Codigo], PesoPorcentual: a.Peso,
				NumeroPreguntas: preguntasExamen, Coeficiente: coeficiente})
		}
		if _, err := s.cat.GuardarPesos(ctx, id, esp[codigo], req, sinAuditoria); err != nil {
			return 0, fmt.Errorf("pesos de %s: %w", codigo, err)
		}
	}
	return id, s.configuracion(ctx, id, len(p.Compatibilidad)*preguntasExamen)
}

// areasCompatibilidad adds the missing compatibility rows. A new proceso is
// born active, and an active proceso rejects rows that leave it off 100 %
// (checkPesoActivo), so it is deactivated while rows are missing, as an
// admin would do; the caller reactivates it once they sum 100.
func (s *sembrador) areasCompatibilidad(ctx context.Context, procesoID int64, p procesoDemo, asig map[string]int64) error {
	var faltan []areaDemo
	for _, a := range p.Compatibilidad {
		existe, err := s.buscar(ctx, `SELECT id FROM proceso_asignaturas
			WHERE proceso_id = $1 AND asignatura_id = $2 AND especialidad_id IS NULL`, procesoID, asig[a.Codigo])
		if err != nil {
			return err
		}
		if existe == 0 {
			faltan = append(faltan, a)
		}
	}
	if len(faltan) == 0 {
		return nil
	}
	if err := s.cat.UpdateProcesoEstado(ctx, procesoID, estadoInactivo, nil); err != nil {
		return fmt.Errorf("desactivar para completar las áreas: %w", err)
	}
	for _, a := range faltan {
		if _, err := s.cat.AsignarAsignatura(ctx, procesoID, catalogs.AsignarAsignaturaRequest{AsignaturaID: asig[a.Codigo],
			NumeroPreguntas: preguntasExamen, PesoPorcentual: a.Peso, Coeficiente: coeficiente}, nil); err != nil {
			return fmt.Errorf("área %s: %w", a.Codigo, err)
		}
		s.creado["proceso_asignaturas"]++
	}
	return nil
}

// sinAuditoria: the seed is not an admin action; nothing goes to audit_log.
func sinAuditoria(context.Context, sqlx.ExecerContext, map[string]any) error { return nil }

// preguntas keeps preguntasPorArea active questions of the area in the proceso.
func (s *sembrador) preguntas(ctx context.Context, procesoID, asignaturaID int64, codigo string) error {
	for i := 1; i <= preguntasPorArea; i++ {
		enunciado := fmt.Sprintf("Pregunta demo %d de %s: ¿cuál es la opción correcta?", i, codigo)
		existe, err := s.buscar(ctx, `SELECT id FROM preguntas
			WHERE proceso_id = $1 AND asignatura_id = $2 AND enunciado = $3 AND estado <> 'eliminada'`, procesoID, asignaturaID, enunciado)
		if err != nil {
			return err
		}
		if existe != 0 {
			continue
		}
		if _, err := s.preg.CreatePregunta(ctx, questions.CreatePreguntaRequest{ProcesoID: procesoID, AsignaturaID: asignaturaID,
			Enunciado: enunciado, Tipo: "unica", Dificultad: "medio",
			Respuestas: []questions.CreateRespuestaItem{
				{Texto: "Opción correcta", EsCorrecta: true, Orden: 1},
				{Texto: "Opción incorrecta", EsCorrecta: false, Orden: 2},
			}}); err != nil {
			return fmt.Errorf("pregunta %d de %s: %w", i, codigo, err)
		}
		s.creado["preguntas"]++
	}
	return nil
}

func (s *sembrador) configuracion(ctx context.Context, procesoID int64, totalPreguntas int) error {
	existe, err := s.buscar(ctx, `SELECT id FROM configuracion_examen WHERE proceso_id = $1 AND estado = 'activo'`, procesoID)
	if err != nil || existe != 0 {
		return err
	}
	if _, err := s.sim.CreateConfiguracion(ctx, simulations.CreateConfiguracionRequest{ProcesoID: procesoID,
		TiempoTotalSeg: tiempoSimulacroSeg, TotalPreguntas: totalPreguntas}); err != nil {
		return fmt.Errorf("configuración de simulacro: %w", err)
	}
	s.creado["configuracion_examen"]++
	return nil
}

// ── Usuarios e inscripciones ──────────────────────────────────────────────────

// usuario keeps the demo account verified, active, with the tour done and the
// password of the variables, tied to its synthetic persona. A username held
// by another persona is never adopted (same rule as seed/admin, H96).
func (s *sembrador) usuario(ctx context.Context, u usuarioDemo, institucionID, gradoID, especialidadID int64) (int64, error) {
	hash, err := s.hash(u.Rol)
	if err != nil {
		return 0, err
	}
	e, err := persona.Encrypt(s.gcm, s.rawKey, persona.Plain{DNI: "demo:" + u.Username, Nombres: u.Nombres,
		ApellidoP: "Demo", ApellidoM: "Sintético", Email: u.Username + dominioEmail})
	if err != nil {
		return 0, err
	}
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var personaID int64
	err = tx.GetContext(ctx, &personaID, `SELECT id FROM personas WHERE dni_hmac = $1`, e.DNIHMAC)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		if err := tx.GetContext(ctx, &personaID, `
			INSERT INTO personas (dni, dni_hmac, nombres, apellido_paterno, apellido_materno,
				grado_id, especialidad_id, institucion_id, telefono, email, email_hmac, estado)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, 'activo') RETURNING id`,
			e.DNI, e.DNIHMAC, e.Nombres, e.ApellidoP, e.ApellidoM, gradoID, especialidadID, institucionID,
			e.Telefono, e.Email, e.EmailHMAC); err != nil {
			return 0, fmt.Errorf("persona: %w", err)
		}
		s.creado["personas"]++
	case err != nil:
		return 0, err
	default:
		if _, err := tx.ExecContext(ctx, `UPDATE personas SET grado_id = $2, especialidad_id = $3, institucion_id = $4 WHERE id = $1`,
			personaID, gradoID, especialidadID, institucionID); err != nil {
			return 0, fmt.Errorf("persona: %w", err)
		}
	}

	var existente struct {
		ID        int64 `db:"id"`
		PersonaID int64 `db:"persona_id"`
	}
	var id int64
	err = tx.GetContext(ctx, &existente, `SELECT id, persona_id FROM usuarios WHERE username = $1 FOR UPDATE`, u.Username)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		if err := tx.GetContext(ctx, &id, `
			INSERT INTO usuarios (persona_id, username, password_hash, rol, estado, activated_at, tour_completado)
			VALUES ($1, $2, $3, $4, 'activo', NOW(), true) RETURNING id`, personaID, u.Username, string(hash), u.Rol); err != nil {
			return 0, fmt.Errorf("crear usuario: %w", err)
		}
		s.creado["usuarios"]++
	case err != nil:
		return 0, err
	case existente.PersonaID != personaID:
		return 0, fmt.Errorf("el username existe ligado a otra persona (id=%d); no se adopta", existente.PersonaID)
	default:
		id = existente.ID
		if _, err := tx.ExecContext(ctx, `
			UPDATE usuarios SET password_hash = $2, rol = $3, estado = 'activo', tour_completado = true,
			       activated_at = COALESCE(activated_at, NOW()), updated_at = NOW()
			WHERE id = $1`, id, string(hash), u.Rol); err != nil {
			return 0, fmt.Errorf("actualizar usuario: %w", err)
		}
	}
	return id, tx.Commit()
}

// hash returns the bcrypt hash of the password of rol, computed once per run:
// there are only two passwords, and bcrypt is deliberately slow.
func (s *sembrador) hash(rol string) ([]byte, error) {
	if h, ok := s.hashes[rol]; ok {
		return h, nil
	}
	password := s.cfg.PostulantePassword
	if rol == "admin" {
		password = s.cfg.AdminPassword
	}
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("bcrypt: %w", err)
	}
	s.hashes[rol] = h
	return h, nil
}

// inscripcion signs the user up through the inscripciones service (proceso
// active, nivel, payment config) and approves it when the dataset says so.
func (s *sembrador) inscripcion(ctx context.Context, usuarioID, procesoID int64, estado string, adminID int64) error {
	var actual struct {
		ID     int64  `db:"id"`
		Estado string `db:"estado"`
	}
	err := s.db.GetContext(ctx, &actual, `SELECT id, estado FROM inscripciones WHERE usuario_id = $1 AND proceso_id = $2`, usuarioID, procesoID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		r, err := s.ins.Inscribir(ctx, usuarioID, procesoID, nivelDemo, nil)
		if err != nil {
			return err
		}
		actual.ID, actual.Estado = r.ID, r.Estado
		s.creado["inscripciones"]++
	case err != nil:
		return err
	}
	if estado == actual.Estado || estado != "aprobado" {
		return nil
	}
	return s.ins.UpdateEstado(ctx, actual.ID, estado, "", adminID, nil)
}
