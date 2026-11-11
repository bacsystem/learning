// Package dbtest provides helpers for integration tests that need a real
// PostgreSQL database. Tests are skipped when TEST_DATABASE_URL is unset.
//
// Every Open call takes a session-level advisory lock that is held until the
// test ends, so integration tests from different packages that use this helper
// never interleave (several assertions read global aggregates such as revenue
// totals or the number of active admins).
package dbtest

import (
	"context"
	"crypto/cipher"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cbaciliod/learning/internal/auth"
	"github.com/cbaciliod/learning/internal/crypto"
	"github.com/cbaciliod/learning/internal/db/testlock"
	jwtpkg "github.com/cbaciliod/learning/internal/jwt"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// TestHexKey is the AES-256 key used to encrypt seeded PII.
const TestHexKey = "0000000000000000000000000000000000000000000000000000000000000001"

var seq atomic.Int64

// Open returns a connection pool to TEST_DATABASE_URL or skips the test.
func Open(t *testing.T) *sqlx.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		t.Fatalf("db connect: %v", err)
	}
	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("db conn: %v", err)
	}
	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, testlock.Key); err != nil {
		t.Fatalf("advisory lock: %v", err)
	}
	t.Cleanup(func() {
		_, _ = conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock($1)`, testlock.Key)
		_ = conn.Close()
		_ = db.Close()
	})
	return db
}

// GCM returns the cipher matching TestHexKey.
func GCM(t *testing.T) cipher.AEAD {
	t.Helper()
	gcm, err := crypto.NewCipher(TestHexKey)
	if err != nil {
		t.Fatalf("gcm: %v", err)
	}
	return gcm
}

// Uniq returns a short string unique within this process run.
func Uniq() string {
	return fmt.Sprintf("%x%x", time.Now().UnixNano()%0xffffff, seq.Add(1))
}

// Catalog is a seeded institucion with one grado and one especialidad.
type Catalog struct {
	InstitucionID  int64
	GradoID        int64
	EspecialidadID int64
}

// SeedCatalog inserts an institucion, grado and especialidad and removes them
// (and procesos/asignaturas hanging from the institucion) when the test ends.
func SeedCatalog(t *testing.T, db *sqlx.DB) Catalog {
	t.Helper()
	u := Uniq()
	var c Catalog
	must(t, db.QueryRow(`INSERT INTO instituciones (nombre, siglas) VALUES ($1, $2) RETURNING id`,
		"Inst test "+u, "T"+u).Scan(&c.InstitucionID))
	must(t, db.QueryRow(`INSERT INTO grados (institucion_id, nombre, abreviatura, nivel, orden)
		VALUES ($1, $2, $3, 'suboficial', 1) RETURNING id`, c.InstitucionID, "Grado "+u, "G"+u).Scan(&c.GradoID))
	must(t, db.QueryRow(`INSERT INTO especialidades (institucion_id, nombre, codigo)
		VALUES ($1, $2, $3) RETURNING id`, c.InstitucionID, "Esp "+u, "E"+u).Scan(&c.EspecialidadID))
	t.Cleanup(func() {
		id := c.InstitucionID
		for _, q := range []string{
			`DELETE FROM simulacros WHERE institucion_id=$1`,
			`DELETE FROM configuracion_examen WHERE proceso_id IN (SELECT id FROM procesos WHERE institucion_id=$1)`,
			`DELETE FROM practicas WHERE institucion_id=$1`,
			`DELETE FROM inscripciones WHERE proceso_id IN (SELECT id FROM procesos WHERE institucion_id=$1)`,
			`DELETE FROM proceso_asignaturas WHERE proceso_id IN (SELECT id FROM procesos WHERE institucion_id=$1)`,
			`DELETE FROM procesos WHERE institucion_id=$1`,
			`DELETE FROM grados WHERE institucion_id=$1`,
			`DELETE FROM tema_especialidades WHERE especialidad_id IN (SELECT id FROM especialidades WHERE institucion_id=$1)`,
			`DELETE FROM especialidades WHERE institucion_id=$1`,
			`DELETE FROM instituciones WHERE id=$1`,
		} {
			if _, err := db.Exec(q, id); err != nil {
				t.Logf("cleanup %q: %v", q, err)
			}
		}
	})
	return c
}

// SeedProceso inserts a proceso for the catalog's institucion.
func SeedProceso(t *testing.T, db *sqlx.DB, c Catalog, anio int, estado string) int64 {
	t.Helper()
	var id int64
	must(t, db.QueryRow(`INSERT INTO procesos (institucion_id, nombre, tipo, anio, estado)
		VALUES ($1, $2, 'ascenso', $3, $4) RETURNING id`,
		c.InstitucionID, "Proceso "+Uniq(), anio, estado).Scan(&id))
	return id
}

// SeedAsignatura inserts an asignatura removed at test end.
func SeedAsignatura(t *testing.T, db *sqlx.DB) int64 {
	t.Helper()
	var id int64
	u := Uniq()
	must(t, db.QueryRow(`INSERT INTO asignaturas (nombre, codigo) VALUES ($1, $2) RETURNING id`,
		"Asig "+u, "A"+u).Scan(&id))
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM proceso_asignaturas WHERE asignatura_id=$1`, id)
		_, _ = db.Exec(`DELETE FROM temas WHERE asignatura_id=$1`, id)
		_, _ = db.Exec(`DELETE FROM asignaturas WHERE id=$1`, id)
	})
	return id
}

// User is a seeded usuario with its persona.
type User struct {
	ID        int64
	PersonaID int64
	Username  string
	Nombres   string
}

// SeedUser inserts a persona (PII encrypted with TestHexKey) and a usuario.
// Rows referencing the user are removed when the test ends.
func SeedUser(t *testing.T, db *sqlx.DB, c Catalog, rol, estado string) User {
	t.Helper()
	gcm := GCM(t)
	u := Uniq()
	enc := func(s string) string {
		v, err := crypto.Encrypt(gcm, s)
		if err != nil {
			t.Fatalf("encrypt: %v", err)
		}
		return v
	}
	usr := User{Username: "u" + u, Nombres: "Nombre" + u}
	must(t, db.QueryRow(`INSERT INTO personas (dni, nombres, apellido_paterno, apellido_materno,
		grado_id, especialidad_id, institucion_id, telefono, email, dni_hmac, email_hmac)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING id`,
		enc("D"+u), enc(usr.Nombres), enc("Paterno"), enc("Materno"),
		c.GradoID, c.EspecialidadID, c.InstitucionID, enc("999"), enc(u+"@test.local"),
		"hmac-dni-"+u, "hmac-email-"+u).Scan(&usr.PersonaID))
	must(t, db.QueryRow(`INSERT INTO usuarios (persona_id, username, password_hash, rol, estado)
		VALUES ($1, $2, 'x', $3, $4) RETURNING id`, usr.PersonaID, usr.Username, rol, estado).Scan(&usr.ID))
	t.Cleanup(func() {
		id := usr.ID
		for _, q := range []string{
			`UPDATE usuarios SET activated_by=NULL WHERE activated_by=$1`,
			`UPDATE inscripciones SET revisado_por=NULL WHERE revisado_por=$1`,
			`DELETE FROM sesiones WHERE usuario_id=$1`,
			`DELETE FROM audit_log WHERE usuario_id=$1`,
			`DELETE FROM inscripciones WHERE usuario_id=$1`,
			`DELETE FROM simulacros WHERE usuario_id=$1`,
			`DELETE FROM practicas WHERE usuario_id=$1`,
			`DELETE FROM usuarios WHERE id=$1`,
		} {
			if _, err := db.Exec(q, id); err != nil {
				t.Logf("cleanup %q: %v", q, err)
			}
		}
		_, _ = db.Exec(`DELETE FROM personas WHERE id=$1`, usr.PersonaID)
	})
	return usr
}

// SeedSession inserts an active session for the user.
func SeedSession(t *testing.T, db *sqlx.DB, userID int64) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO sesiones (usuario_id, token_hash, expires_at)
		VALUES ($1, $2, NOW() + INTERVAL '1 hour')`, userID, "tok-"+Uniq())
	must(t, err)
}

// Closed returns a pool whose connections are closed: every query fails.
// Useful to exercise 500 paths.
func Closed(t *testing.T) *sqlx.DB {
	t.Helper()
	db, err := sqlx.Open("postgres", "postgres://invalid:invalid@127.0.0.1:1/none?sslmode=disable&connect_timeout=1")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	_ = db.Close()
	return db
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil && err != sql.ErrNoRows {
		t.Fatalf("seed: %v", err)
	}
	if err == sql.ErrNoRows {
		t.Fatalf("seed: no rows")
	}
}

const testJWTSecret = "dbtest-secret-32-bytes-exactly-ok!"

// Serve runs h behind auth.RequireAuth with a real JWT for the given user, so
// handlers see genuine claims via auth.ClaimsFromContext. rol "" means
// anonymous (no Authorization header, RequireAuth skipped).
func Serve(t *testing.T, h http.HandlerFunc, r *http.Request, userID int64, rol string) *httptest.ResponseRecorder {
	t.Helper()
	rr := httptest.NewRecorder()
	if rol == "" {
		h(rr, r)
		return rr
	}
	tok, _, err := jwtpkg.Issue(testJWTSecret, userID, rol, "", 0, 0, 0)
	if err != nil {
		t.Fatalf("jwt: %v", err)
	}
	r.Header.Set("Authorization", "Bearer "+tok)
	// The session checker returns the same user and role as the token, so RequireAuth
	// (which now trusts the DB role, H2) sees an active session.
	always := func(context.Context, string) (*auth.Session, error) {
		return &auth.Session{UserID: userID, Rol: rol, Estado: "activo"}, nil
	}
	auth.RequireAuth(testJWTSecret, always)(h).ServeHTTP(rr, r)
	return rr
}
