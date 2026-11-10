// Package questionstest provides database fixtures for integration tests of the
// exam modules (questions, simulations, practices). It is only imported from
// _test.go files.
package questionstest

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cbaciliod/learning/internal/auth"
	"github.com/cbaciliod/learning/internal/jwt"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // postgres driver
)

// JWTSecret is the secret used to sign tokens in integration tests.
const JWTSecret = "test-secret-32-bytes-exactly-ok!"

// DB returns a live DB connection or skips the test when TEST_DATABASE_URL is unset.
func DB(t *testing.T) *sqlx.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		t.Fatalf("db connect: %v", err)
	}
	// The Postgres server is shared: keep each test's pool small (concurrency
	// tests then queue on the pool instead of exhausting max_connections).
	db.SetMaxOpenConns(8)
	t.Cleanup(func() { db.Close() })
	return db
}

// AsignaturaSpec describes one asignatura of the seeded proceso.
type AsignaturaSpec struct {
	NumeroPreguntas int     // proceso_asignaturas.numero_preguntas
	Peso            float64 // proceso_asignaturas.peso_porcentual
	Coeficiente     float64 // proceso_asignaturas.coeficiente (0 → 1)
	Unicas          int     // number of 'unica' questions in the bank
	Multiples       int     // number of 'multiple' questions in the bank
}

// Options configures Seed.
type Options struct {
	Nivel          string // 'suboficial' | 'oficial'; default 'suboficial'
	Asignaturas    []AsignaturaSpec
	Inscripcion    string // inscripciones.estado for the main user; "" = none
	TiempoTotalSeg int    // configuracion_examen.tiempo_total_seg; default 3600
	NotaMinima     float64
	SinConfig      bool // do not create configuracion_examen
}

// Pregunta is one seeded question.
type Pregunta struct {
	ID           int64
	AsignaturaID int64
	Tipo         string
	Respuestas   []int64 // all alternatives, by orden
	Correctas    []int64 // the correct ones
	Incorrectas  []int64
}

// Fixture holds the IDs of everything Seed created.
type Fixture struct {
	DB             *sqlx.DB
	Suffix         string
	InstitucionID  int64
	GradoID        int64
	EspecialidadID int64
	UsuarioID      int64
	ProcesoID      int64
	ConfigID       int64
	Nivel          string
	Asignaturas    []int64
	Preguntas      []Pregunta

	usuarios []int64
	personas []int64
}

var seq atomic.Int64

func must(t *testing.T, err error, what string) {
	t.Helper()
	if err != nil {
		t.Fatalf("seed %s: %v", what, err)
	}
}

// Seed creates an isolated institución/proceso/usuario/question bank and
// registers a cleanup that deletes everything it created.
func Seed(t *testing.T, db *sqlx.DB, opts Options) *Fixture {
	t.Helper()
	ctx := context.Background()
	if opts.Nivel == "" {
		opts.Nivel = "suboficial"
	}
	if opts.TiempoTotalSeg == 0 {
		opts.TiempoTotalSeg = 3600
	}
	if opts.NotaMinima == 0 {
		opts.NotaMinima = 55
	}
	f := &Fixture{DB: db, Nivel: opts.Nivel}
	f.Suffix = fmt.Sprintf("%x%x%d", time.Now().UnixNano(), os.Getpid(), seq.Add(1))
	t.Cleanup(func() { f.cleanup(t) })

	must(t, db.QueryRowContext(ctx,
		`INSERT INTO instituciones (nombre, siglas) VALUES ($1, $2) RETURNING id`,
		"Inst "+f.Suffix, fmt.Sprintf("T%x", time.Now().UnixNano()+seq.Add(1))).Scan(&f.InstitucionID), "institucion")
	must(t, db.QueryRowContext(ctx,
		`INSERT INTO grados (institucion_id, nombre, abreviatura, nivel) VALUES ($1, $2, 'G', $3) RETURNING id`,
		f.InstitucionID, "Grado "+f.Suffix, opts.Nivel).Scan(&f.GradoID), "grado")
	must(t, db.QueryRowContext(ctx,
		`INSERT INTO especialidades (institucion_id, nombre, codigo, nivel) VALUES ($1, $2, $3, $4) RETURNING id`,
		f.InstitucionID, "Esp "+f.Suffix, "E"+f.Suffix, opts.Nivel).Scan(&f.EspecialidadID), "especialidad")
	must(t, db.QueryRowContext(ctx,
		`INSERT INTO procesos (institucion_id, nombre, tipo, anio, nivel) VALUES ($1, $2, 'ascenso', 2026, $3) RETURNING id`,
		f.InstitucionID, "Proceso "+f.Suffix, opts.Nivel).Scan(&f.ProcesoID), "proceso")

	f.UsuarioID = f.NewUser(t, opts.Inscripcion)

	if !opts.SinConfig {
		must(t, db.QueryRowContext(ctx,
			`INSERT INTO configuracion_examen (proceso_id, tiempo_total_seg, total_preguntas, nota_minima, nota_maxima)
			 VALUES ($1, $2, 10, $3, 100) RETURNING id`,
			f.ProcesoID, opts.TiempoTotalSeg, opts.NotaMinima).Scan(&f.ConfigID), "configuracion")
	}

	for i, a := range opts.Asignaturas {
		var asigID int64
		must(t, db.QueryRowContext(ctx,
			`INSERT INTO asignaturas (nombre, codigo) VALUES ($1, $2) RETURNING id`,
			fmt.Sprintf("Asig %d %s", i, f.Suffix), fmt.Sprintf("A%d_%s", i, f.Suffix)).Scan(&asigID), "asignatura")
		f.Asignaturas = append(f.Asignaturas, asigID)
		coef := a.Coeficiente
		if coef == 0 {
			coef = 1
		}
		_, err := db.ExecContext(ctx,
			`INSERT INTO proceso_asignaturas (proceso_id, asignatura_id, numero_preguntas, peso_porcentual, coeficiente)
			 VALUES ($1, $2, $3, $4, $5)`, f.ProcesoID, asigID, a.NumeroPreguntas, a.Peso, coef)
		must(t, err, "proceso_asignatura")
		for q := 0; q < a.Unicas; q++ {
			f.addPregunta(t, asigID, "unica", 4, 1)
		}
		for q := 0; q < a.Multiples; q++ {
			f.addPregunta(t, asigID, "multiple", 4, 2)
		}
	}
	return f
}

// NewUser creates another usuario (same grado/especialidad) with an optional
// inscripción in the fixture's proceso and returns its id.
func (f *Fixture) NewUser(t *testing.T, inscripcion string) int64 {
	t.Helper()
	ctx := context.Background()
	n := seq.Add(1)
	var personaID, usuarioID int64
	must(t, f.DB.QueryRowContext(ctx,
		`INSERT INTO personas (dni, nombres, apellido_paterno, apellido_materno, grado_id, especialidad_id,
		                       institucion_id, telefono, email, dni_hmac, email_hmac)
		 VALUES ('x','N','P','M',$1,$2,$3,'0','e',$4,$4) RETURNING id`,
		f.GradoID, f.EspecialidadID, f.InstitucionID, fmt.Sprintf("hmac_%s_%d", f.Suffix, n)).Scan(&personaID), "persona")
	f.personas = append(f.personas, personaID)
	username := fmt.Sprintf("u%x%x", time.Now().UnixNano(), n)
	must(t, f.DB.QueryRowContext(ctx,
		`INSERT INTO usuarios (persona_id, username, password_hash, estado) VALUES ($1, $2, 'x', 'activo') RETURNING id`,
		personaID, username).Scan(&usuarioID), "usuario")
	f.usuarios = append(f.usuarios, usuarioID)
	if inscripcion != "" {
		_, err := f.DB.ExecContext(ctx,
			`INSERT INTO inscripciones (usuario_id, proceso_id, estado) VALUES ($1, $2, $3)`,
			usuarioID, f.ProcesoID, inscripcion)
		must(t, err, "inscripcion")
	}
	return usuarioID
}

func (f *Fixture) addPregunta(t *testing.T, asigID int64, tipo string, nResp, nCorrectas int) {
	t.Helper()
	ctx := context.Background()
	p := Pregunta{AsignaturaID: asigID, Tipo: tipo}
	must(t, f.DB.QueryRowContext(ctx,
		`INSERT INTO preguntas (institucion_id, proceso_id, asignatura_id, enunciado, tipo)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		f.InstitucionID, f.ProcesoID, asigID, "Enunciado "+f.Suffix, tipo).Scan(&p.ID), "pregunta")
	for i := 1; i <= nResp; i++ {
		var rid int64
		correcta := i <= nCorrectas
		must(t, f.DB.QueryRowContext(ctx,
			`INSERT INTO respuestas (pregunta_id, texto, es_correcta, orden) VALUES ($1, $2, $3, $4) RETURNING id`,
			p.ID, fmt.Sprintf("Alt %d", i), correcta, i).Scan(&rid), "respuesta")
		p.Respuestas = append(p.Respuestas, rid)
		if correcta {
			p.Correctas = append(p.Correctas, rid)
		} else {
			p.Incorrectas = append(p.Incorrectas, rid)
		}
	}
	_, err := f.DB.ExecContext(ctx,
		`INSERT INTO pregunta_retroalimentacion (pregunta_id, explicacion) VALUES ($1, 'porque sí')`, p.ID)
	must(t, err, "retro")
	f.Preguntas = append(f.Preguntas, p)
}

// Pregunta returns the seeded question with the given id.
func (f *Fixture) Pregunta(id int64) Pregunta {
	for _, p := range f.Preguntas {
		if p.ID == id {
			return p
		}
	}
	return Pregunta{}
}

// Token issues a JWT for usuarioID with the fixture's grado/especialidad/nivel.
func (f *Fixture) Token(t *testing.T, usuarioID int64, rol string) string {
	t.Helper()
	tok, _, err := jwt.Issue(JWTSecret, usuarioID, rol, f.Nivel, f.GradoID, f.EspecialidadID, f.InstitucionID)
	must(t, err, "jwt")
	sessions.Store(fmt.Sprintf("%x", sha256.Sum256([]byte(tok))), &auth.Session{UserID: usuarioID, Rol: rol, Estado: "activo"})
	return tok
}

// sessions maps the hash of every token issued by Token to its session, so that
// AllowSession behaves like the real DB-backed checker (RequireAuth trusts the
// session's role, not the JWT's).
var sessions sync.Map

// Concurrently runs fn(0..n-1) in n goroutines released at the same instant
// (start barrier) and waits for all of them.
func Concurrently(n int, fn func(i int)) {
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			fn(i)
		}(i)
	}
	close(start)
	wg.Wait()
}

// AllowSession is a SessionChecker that accepts every token issued by Token.
func AllowSession(_ context.Context, tokenHash string) (*auth.Session, error) {
	if s, ok := sessions.Load(tokenHash); ok {
		return s.(*auth.Session), nil
	}
	return nil, nil
}

// Do performs an HTTP request against h with a bearer token and returns the recorder.
func Do(h http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func (f *Fixture) cleanup(t *testing.T) {
	ctx := context.Background()
	stmts := []string{
		`DELETE FROM simulacros WHERE proceso_id = $1`,
		`DELETE FROM practicas WHERE proceso_id = $1`,
		`DELETE FROM pregunta_estadisticas WHERE pregunta_id IN (SELECT id FROM preguntas WHERE proceso_id = $1)`,
		`DELETE FROM respuestas WHERE pregunta_id IN (SELECT id FROM preguntas WHERE proceso_id = $1)`,
		`DELETE FROM preguntas WHERE proceso_id = $1`,
		`DELETE FROM configuracion_examen WHERE proceso_id = $1`,
		`DELETE FROM proceso_asignaturas WHERE proceso_id = $1`,
		`DELETE FROM inscripciones WHERE proceso_id = $1`,
		`DELETE FROM procesos WHERE id = $1`,
	}
	for _, s := range stmts {
		if _, err := f.DB.ExecContext(ctx, s, f.ProcesoID); err != nil {
			t.Errorf("cleanup %q: %v", s, err)
		}
	}
	for _, a := range f.Asignaturas {
		if _, err := f.DB.ExecContext(ctx, `DELETE FROM asignaturas WHERE id = $1`, a); err != nil {
			t.Errorf("cleanup asignatura: %v", err)
		}
	}
	for _, u := range f.usuarios {
		if _, err := f.DB.ExecContext(ctx, `DELETE FROM usuarios WHERE id = $1`, u); err != nil {
			t.Errorf("cleanup usuario: %v", err)
		}
	}
	for _, p := range f.personas {
		if _, err := f.DB.ExecContext(ctx, `DELETE FROM personas WHERE id = $1`, p); err != nil {
			t.Errorf("cleanup persona: %v", err)
		}
	}
	for _, s := range []string{
		`DELETE FROM especialidades WHERE id = $1`,
		`DELETE FROM grados WHERE id = $1`,
	} {
		id := f.EspecialidadID
		if strings.Contains(s, "grados") {
			id = f.GradoID
		}
		if _, err := f.DB.ExecContext(ctx, s, id); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	}
	if _, err := f.DB.ExecContext(ctx, `DELETE FROM instituciones WHERE id = $1`, f.InstitucionID); err != nil {
		t.Errorf("cleanup institucion: %v", err)
	}
}
