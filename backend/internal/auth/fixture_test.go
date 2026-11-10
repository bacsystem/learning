package auth

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"net/http"
	"testing"
	"time"

	"github.com/cbaciliod/learning/internal/db/testlock"
	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"
)

const testJWTSecret = "test-secret-32-bytes-exactly-ok!"

// fixture seeds an isolated set of catalog rows (institución, grado,
// especialidad) and lets tests create personas/usuarios that hang off them.
// Everything it (or the code under test) creates under that institución is
// deleted on cleanup, so tests can be re-run against the same database.
type fixture struct {
	t       *testing.T
	db      *sqlx.DB
	repo    *Repository
	svc     *Service
	instID  int64
	gradoID int64
	espID   int64
	domain  string // unique e-mail domain for this fixture
	suffix  string

	// lockConn holds the cross-package advisory lock (see holdAdminLock)
	// from the first admin this fixture creates until its cleanup ends.
	lockConn *sql.Conn
}

// holdAdminLock takes testlock.Key, the session-level advisory lock that
// internal/db/dbtest.Open also takes: tests of other packages assert on the
// number of active admins, so any auth test that creates an admin holds it
// and they never interleave. (dbtest itself cannot be imported here: it
// imports auth.) It is taken on a dedicated connection, once per
// fixture. It is released at the very end of cleanup, after the admins this
// fixture created have been deleted.
func (f *fixture) holdAdminLock() {
	f.t.Helper()
	if f.lockConn != nil {
		return
	}
	ctx := context.Background()
	conn, err := f.db.Conn(ctx)
	if err != nil {
		f.t.Fatalf("advisory lock conn: %v", err)
	}
	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, testlock.Key); err != nil {
		_ = conn.Close()
		f.t.Fatalf("advisory lock: %v", err)
	}
	f.lockConn = conn
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	_, svc := testSetup(t)
	f := &fixture{t: t, db: svc.repo.db, repo: svc.repo, svc: svc}
	f.suffix = fmt.Sprintf("%d%04d", time.Now().UnixNano()%1e9, rand.Intn(10000))
	f.domain = "t" + f.suffix + ".test"

	mustGet := func(dest *int64, q string, args ...any) {
		t.Helper()
		if err := f.db.QueryRow(q, args...).Scan(dest); err != nil {
			t.Fatalf("fixture seed: %v (%s)", err, q)
		}
	}
	mustGet(&f.instID, `INSERT INTO instituciones (nombre, siglas) VALUES ($1,$2) RETURNING id`,
		"Inst test "+f.suffix, "T"+f.suffix)
	mustGet(&f.gradoID, `INSERT INTO grados (institucion_id, nombre, abreviatura, nivel, orden) VALUES ($1,$2,'TG','suboficial',1) RETURNING id`,
		f.instID, "Grado test "+f.suffix)
	mustGet(&f.espID, `INSERT INTO especialidades (institucion_id, nombre, codigo) VALUES ($1,$2,$3) RETURNING id`,
		f.instID, "Esp test "+f.suffix, "E"+f.suffix)

	t.Cleanup(f.cleanup)
	return f
}

func (f *fixture) cleanup() {
	users := `(SELECT u.id FROM usuarios u JOIN personas p ON p.id=u.persona_id WHERE p.institucion_id=$1)`
	stmts := []string{
		`DELETE FROM audit_log WHERE usuario_id IN ` + users,
		`DELETE FROM audit_log WHERE entidad='usuarios' AND entidad_id IN ` + users,
		`DELETE FROM sesiones WHERE usuario_id IN ` + users,
		`DELETE FROM tokens_recuperacion WHERE usuario_id IN ` + users,
		`UPDATE usuarios SET activated_by=NULL WHERE activated_by IN ` + users,
		`DELETE FROM usuarios WHERE id IN ` + users,
		`DELETE FROM audit_log WHERE entidad='personas' AND entidad_id IN (SELECT id FROM personas WHERE institucion_id=$1)`,
		`DELETE FROM personas WHERE institucion_id=$1`,
		`DELETE FROM especialidades WHERE institucion_id=$1`,
		`DELETE FROM grados WHERE institucion_id=$1`,
		`DELETE FROM instituciones WHERE id=$1`,
	}
	for _, q := range stmts {
		if _, err := f.db.Exec(q, f.instID); err != nil {
			f.t.Errorf("fixture cleanup: %v (%s)", err, q)
		}
	}
	if _, err := f.db.Exec(`DELETE FROM email_queue WHERE to_address ILIKE $1`, "%@"+f.domain); err != nil {
		f.t.Errorf("fixture cleanup email_queue: %v", err)
	}
	// Unknown DNIs some tests register with.
	f.clearFailureCounters("00000000")
	f.clearFailureCounters("00000001")
	if f.lockConn != nil {
		_, _ = f.lockConn.ExecContext(context.Background(), `SELECT pg_advisory_unlock($1)`, testlock.Key)
		_ = f.lockConn.Close()
		f.lockConn = nil
	}
}

// clearFailureCounters deletes the persisted register failure counters of dni
// (auth_intentos_fallidos keys embed its HMAC), so re-runs start fresh.
func (f *fixture) clearFailureCounters(dni string) {
	if _, err := f.db.Exec(`DELETE FROM auth_intentos_fallidos WHERE clave LIKE '%'||$1||'%'`, f.repo.dniHMAC(dni)); err != nil {
		f.t.Errorf("fixture cleanup failure counters: %v", err)
	}
}

// email returns an address in this fixture's unique domain.
func (f *fixture) email(local string) string { return local + "@" + f.domain }

// username returns a unique alphanumeric username (≤30 chars).
func (f *fixture) username(prefix string) string {
	return fmt.Sprintf("%s%s", prefix, f.suffix)[:min(30, len(prefix)+len(f.suffix))]
}

// persona inserts a pre-loaded persona and returns (id, plain DNI). email may
// be "" to simulate a persona with no e-mail on file, stored as empty
// columns. The e-mail is stored exactly as given (its HMAC too), like legacy
// rows.
func (f *fixture) persona(email string) (int64, string) {
	f.t.Helper()
	encEmail, emailHMAC := "", ""
	if email != "" {
		encEmail = f.enc(email)
		emailHMAC = f.repo.rawEmailHMAC(email)
	}
	return f.insertPersona(encEmail, emailHMAC)
}

// seededPersona inserts a persona the way cmd/seed and the CSV import do:
// the e-mail column is ALWAYS encrypted and email_hmac always set, also when
// email is "" (then email = Encrypt("") — not an empty string — and
// email_hmac = HMAC(""), shared by every persona without e-mail).
func (f *fixture) seededPersona(email string) (int64, string) {
	f.t.Helper()
	return f.insertPersona(f.enc(email), f.repo.rawEmailHMAC(email))
}

func (f *fixture) enc(s string) string {
	f.t.Helper()
	v, err := f.repo.encrypt(s)
	if err != nil {
		f.t.Fatalf("encrypt: %v", err)
	}
	return v
}

func (f *fixture) insertPersona(encEmail, emailHMAC string) (int64, string) {
	f.t.Helper()
	enc := f.enc
	for i := 0; i < 20; i++ {
		dni := fmt.Sprintf("%08d", rand.Intn(1e8))
		var id int64
		err := f.db.QueryRow(`
			INSERT INTO personas (dni,dni_hmac,nombres,apellido_paterno,apellido_materno,
				grado_id,especialidad_id,institucion_id,telefono,email,email_hmac)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
			ON CONFLICT (dni_hmac) DO NOTHING RETURNING id`,
			enc(dni), f.repo.dniHMAC(dni), enc("Juan"), enc("Perez"), enc("Lopez"),
			f.gradoID, f.espID, f.instID, enc("999999999"), encEmail, emailHMAC,
		).Scan(&id)
		if err == nil {
			f.t.Cleanup(func() { f.clearFailureCounters(dni) })
			return id, dni
		}
	}
	f.t.Fatal("could not insert persona with a unique DNI")
	return 0, ""
}

// user creates a usuario for personaID with the given role/estado and password.
func (f *fixture) user(personaID int64, username, password, rol, estado string) int64 {
	f.t.Helper()
	if rol == "admin" {
		f.holdAdminLock()
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		f.t.Fatal(err)
	}
	var id int64
	if err := f.db.QueryRow(`INSERT INTO usuarios (persona_id, username, password_hash, rol, estado)
		VALUES ($1,$2,$3,$4,$5) RETURNING id`, personaID, username, string(hash), rol, estado).Scan(&id); err != nil {
		f.t.Fatalf("insert usuario: %v", err)
	}
	return id
}

// activeUser creates persona + usuario activo and returns (userID, username).
func (f *fixture) activeUser(prefix, rol string) (int64, string) {
	f.t.Helper()
	pid, _ := f.persona(f.email(prefix))
	username := f.username(prefix)
	return f.user(pid, username, "Password123!", rol, "activo"), username
}

func (f *fixture) personaEmailHMAC(personaID int64) string {
	f.t.Helper()
	var h string
	if err := f.db.QueryRow(`SELECT email_hmac FROM personas WHERE id=$1`, personaID).Scan(&h); err != nil {
		f.t.Fatal(err)
	}
	return h
}

func (f *fixture) countUsuarios(personaID int64) int {
	f.t.Helper()
	var n int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM usuarios WHERE persona_id=$1`, personaID).Scan(&n); err != nil {
		f.t.Fatal(err)
	}
	return n
}

// login performs a real login and returns the bearer token.
func (f *fixture) login(username string) string {
	f.t.Helper()
	resp, err := f.svc.Login(context.Background(), LoginRequest{Username: username, Password: "Password123!"}, "127.0.0.1", "test")
	if err != nil {
		f.t.Fatalf("login %s: %v", username, err)
	}
	return resp.Token
}

// mux returns a ServeMux with the auth routes mounted and the real DB session checker.
func (f *fixture) mux() *http.ServeMux {
	mux := http.NewServeMux()
	Mount(mux, f.svc, testJWTSecret, f.repo.FindActiveSesion)
	return mux
}
