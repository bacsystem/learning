package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"github.com/cbaciliod/learning/internal/crypto"
	"github.com/cbaciliod/learning/internal/db/testlock"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

const testHexKey = "0202020202020202020202020202020202020202020202020202020202020202"

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestConfigFromEnv(t *testing.T) {
	t.Run("password from stdin, email normalized, synthetic dni", func(t *testing.T) {
		cfg, err := configFromEnv(env(map[string]string{"ADMIN_EMAIL": "  Ops@Example.COM "}),
			strings.NewReader("a-long-enough-pass\nignored\n"))
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Password != "a-long-enough-pass" || cfg.Email != "ops@example.com" ||
			cfg.Username != "admin" || cfg.DNI != "admin:admin" {
			t.Fatalf("bad cfg: user=%q email=%q dni=%q pwlen=%d", cfg.Username, cfg.Email, cfg.DNI, len(cfg.Password))
		}
	})
	t.Run("email required", func(t *testing.T) {
		if _, err := configFromEnv(env(map[string]string{"ADMIN_PASSWORD": "a-long-enough-pass"}), strings.NewReader("")); err == nil {
			t.Fatal("expected error without ADMIN_EMAIL")
		}
	})
	t.Run("short password rejected", func(t *testing.T) {
		if _, err := configFromEnv(env(map[string]string{"ADMIN_EMAIL": "a@b.c"}), strings.NewReader("short\n")); err == nil {
			t.Fatal("expected error for short password")
		}
	})
	t.Run("error never contains the password", func(t *testing.T) {
		_, err := configFromEnv(env(map[string]string{"ADMIN_EMAIL": "a@b.c", "ADMIN_PASSWORD": "tiny"}), strings.NewReader(""))
		if err == nil || strings.Contains(err.Error(), "tiny") {
			t.Fatalf("unexpected err: %v", err)
		}
	})
}

// ---- integration (TEST_DATABASE_URL) ----

type fixture struct {
	db               *sqlx.DB
	raw              []byte
	inst, grado, esp int64
	suffix           string
}

func setup(t *testing.T) *fixture {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	// These tests create active admins. Hold the same session-level advisory lock
	// as internal/db/dbtest and the auth fixtures, so tests in other packages that
	// depend on the global admin count (last-admin protection) never run while
	// these admins exist. Released at the end of cleanup, after the deletes.
	lockConn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatalf("lock conn: %v", err)
	}
	if _, err := lockConn.ExecContext(context.Background(), `SELECT pg_advisory_lock($1)`, testlock.Key); err != nil {
		t.Fatalf("advisory lock: %v", err)
	}
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	f := &fixture{db: db, suffix: hex.EncodeToString(b)}
	f.raw, _ = hex.DecodeString(testHexKey)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(db.Get(&f.inst, `INSERT INTO instituciones (nombre, siglas) VALUES ($1, $2) RETURNING id`, "seedadmin-"+f.suffix, "T"+f.suffix))
	must(db.Get(&f.grado, `INSERT INTO grados (institucion_id, nombre, abreviatura, nivel, orden) VALUES ($1,'G','G','oficial',1) RETURNING id`, f.inst))
	must(db.Get(&f.esp, `INSERT INTO especialidades (institucion_id, nombre, codigo) VALUES ($1,'E','E') RETURNING id`, f.inst))
	t.Cleanup(func() {
		ids := `SELECT id FROM personas WHERE institucion_id = $1`
		if _, err := db.Exec(`DELETE FROM tokens_recuperacion WHERE usuario_id IN (SELECT id FROM usuarios WHERE persona_id IN (`+ids+`))`, f.inst); err != nil {
			t.Errorf("cleanup: %v", err)
		}
		if _, err := db.Exec(`DELETE FROM sesiones WHERE usuario_id IN (SELECT id FROM usuarios WHERE persona_id IN (`+ids+`))`, f.inst); err != nil {
			t.Errorf("cleanup: %v", err)
		}
		if _, err := db.Exec(`DELETE FROM usuarios WHERE persona_id IN (`+ids+`)`, f.inst); err != nil {
			t.Errorf("cleanup: %v", err)
		}
		if _, err := db.Exec(`DELETE FROM personas WHERE institucion_id = $1`, f.inst); err != nil {
			t.Errorf("cleanup: %v", err)
		}
		if _, err := db.Exec(`DELETE FROM especialidades WHERE institucion_id = $1`, f.inst); err != nil {
			t.Errorf("cleanup: %v", err)
		}
		if _, err := db.Exec(`DELETE FROM grados WHERE institucion_id = $1`, f.inst); err != nil {
			t.Errorf("cleanup: %v", err)
		}
		if _, err := db.Exec(`DELETE FROM instituciones WHERE id = $1`, f.inst); err != nil {
			t.Errorf("cleanup: %v", err)
		}
		_, _ = lockConn.ExecContext(context.Background(), `SELECT pg_advisory_unlock($1)`, testlock.Key)
		_ = lockConn.Close()
		db.Close()
	})
	return f
}

func (f *fixture) cfg() config {
	return config{
		Username: "adm" + f.suffix, Email: "ops-" + f.suffix + "@example.com",
		DNI: "admin:adm" + f.suffix, Password: "a-long-enough-pass",
		InstitucionID: f.inst, GradoID: f.grado, EspecialidadID: f.esp,
	}
}

// insertRealPersona simulates a real person seeded without usuario.
func (f *fixture) insertRealPersona(t *testing.T, dni, email string) int64 {
	t.Helper()
	var id int64
	if err := f.db.Get(&id, `
		INSERT INTO personas (dni, dni_hmac, nombres, apellido_paterno, apellido_materno,
			grado_id, especialidad_id, institucion_id, telefono, email, email_hmac)
		VALUES ('x',$1,'x','x','x',$2,$3,$4,'x','x',$5) RETURNING id`,
		crypto.HMAC(f.raw, dni), f.grado, f.esp, f.inst, crypto.HMAC(f.raw, email)); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestEnsureAdminUsesDedicatedPersona(t *testing.T) {
	f := setup(t)
	gcm, _ := crypto.NewCipher(testHexKey)
	ctx := context.Background()
	real := f.insertRealPersona(t, "7"+f.suffix, "real-"+f.suffix+"@example.com")
	cfg := f.cfg()

	res, pid, err := ensureAdmin(ctx, f.db, gcm, f.raw, cfg)
	if err != nil || res != outcomeCreated {
		t.Fatalf("res=%v err=%v", res, err)
	}
	if pid == real {
		t.Fatal("admin was bound to an existing real persona")
	}
	var row struct {
		DNIHMAC   string `db:"dni_hmac"`
		EmailHMAC string `db:"email_hmac"`
		Email     string `db:"email"`
		Rol       string `db:"rol"`
	}
	if err := f.db.Get(&row, `SELECT p.dni_hmac, p.email_hmac, p.email, u.rol FROM usuarios u JOIN personas p ON p.id=u.persona_id WHERE u.username=$1`, cfg.Username); err != nil {
		t.Fatal(err)
	}
	if row.DNIHMAC != crypto.HMAC(f.raw, cfg.DNI) || row.EmailHMAC != crypto.HMAC(f.raw, cfg.Email) || row.Rol != "admin" {
		t.Fatalf("bad persona/usuario: %+v", row)
	}
	if pt, err := crypto.Decrypt(gcm, row.Email); err != nil || pt != cfg.Email {
		t.Fatalf("email not encrypted correctly: %v", err)
	}

	// idempotent
	res, pid2, err := ensureAdmin(ctx, f.db, gcm, f.raw, cfg)
	if err != nil || res != outcomeExists || pid2 != pid {
		t.Fatalf("second run: res=%v pid=%d err=%v", res, pid2, err)
	}
}

func TestEnsureAdminRebindsLegacyBinding(t *testing.T) {
	f := setup(t)
	gcm, _ := crypto.NewCipher(testHexKey)
	ctx := context.Background()
	cfg := f.cfg()
	// Legacy state (H13): admin bound to a real person's persona with a pending reset token.
	real := f.insertRealPersona(t, "8"+f.suffix, "victim-"+f.suffix+"@example.com")
	var uid int64
	// H96: el usuario previo no es admin activo; el rebind debe forzarlo.
	if err := f.db.Get(&uid, `INSERT INTO usuarios (persona_id, username, password_hash, rol, estado) VALUES ($1,$2,'x','usuario','pendiente') RETURNING id`, real, cfg.Username); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(`INSERT INTO tokens_recuperacion (usuario_id, token_hash, expires_at) VALUES ($1,$2,NOW()+interval '1 hour')`, uid, "h-"+f.suffix); err != nil {
		t.Fatal(err)
	}

	if _, err := f.db.Exec(`INSERT INTO sesiones (usuario_id, token_hash, expires_at) VALUES ($1,'s',NOW()+interval '1 hour')`, uid); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ensureAdmin(ctx, f.db, gcm, f.raw, cfg); err == nil || !strings.Contains(err.Error(), "ADMIN_REBIND") {
		t.Fatalf("expected refusal without ADMIN_REBIND, got %v", err)
	}
	cfg.Rebind = true
	res, pid, err := ensureAdmin(ctx, f.db, gcm, f.raw, cfg)
	if err != nil || res != outcomeRebound || pid == real {
		t.Fatalf("res=%v pid=%d err=%v", res, pid, err)
	}
	var pending, sessions int
	var hash string
	for _, q := range []struct {
		dest  any
		query string
	}{
		{&pending, `SELECT COUNT(*) FROM tokens_recuperacion WHERE usuario_id=$1 AND NOT usado`},
		{&sessions, `SELECT COUNT(*) FROM sesiones WHERE usuario_id=$1`},
		{&hash, `SELECT password_hash FROM usuarios WHERE id=$1`},
	} {
		if err := f.db.Get(q.dest, q.query, uid); err != nil {
			t.Fatalf("%s: %v", q.query, err)
		}
	}
	if pending != 0 || sessions != 0 {
		t.Fatalf("pending reset tokens=%d sessions=%d, want 0/0", pending, sessions)
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(cfg.Password)) != nil {
		t.Fatal("password not rotated on rebind")
	}
	f.assertAdminActivo(t, cfg.Username)
}

func (f *fixture) assertAdminActivo(t *testing.T, username string) {
	t.Helper()
	var u struct {
		Rol         string `db:"rol"`
		Estado      string `db:"estado"`
		ActivatedAt bool   `db:"activated"`
	}
	if err := f.db.Get(&u, `SELECT rol, estado, activated_at IS NOT NULL AS activated FROM usuarios WHERE username=$1`, username); err != nil {
		t.Fatal(err)
	}
	if u.Rol != "admin" || u.Estado != "activo" || !u.ActivatedAt {
		t.Fatalf("usuario %q: rol=%q estado=%q activated=%v, want admin/activo/true", username, u.Rol, u.Estado, u.ActivatedAt)
	}
}

// H96: un usuario ya ligado a la persona dedicada pero que no es admin activo
// (p.ej. 'usuario'/'pendiente') no se da por bueno: sin ADMIN_REBIND falla, y
// con ADMIN_REBIND se fuerza rol='admin', estado='activo' y se rota la clave.
func TestEnsureAdminForcesRolAndEstado(t *testing.T) {
	f := setup(t)
	gcm, _ := crypto.NewCipher(testHexKey)
	ctx := context.Background()
	cfg := f.cfg()

	if _, _, err := ensureAdmin(ctx, f.db, gcm, f.raw, cfg); err != nil {
		t.Fatal(err)
	}
	f.assertAdminActivo(t, cfg.Username)

	if _, err := f.db.Exec(`UPDATE usuarios SET rol='usuario', estado='pendiente', activated_at=NULL WHERE username=$1`, cfg.Username); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ensureAdmin(ctx, f.db, gcm, f.raw, cfg); err == nil || !strings.Contains(err.Error(), "ADMIN_REBIND") {
		t.Fatalf("expected refusal without ADMIN_REBIND for a non-admin user, got %v", err)
	}
	cfg.Rebind = true
	cfg.Password = "another-long-pass"
	res, _, err := ensureAdmin(ctx, f.db, gcm, f.raw, cfg)
	if err != nil || res != outcomeRebound {
		t.Fatalf("res=%v err=%v", res, err)
	}
	f.assertAdminActivo(t, cfg.Username)
	var hash string
	if err := f.db.Get(&hash, `SELECT password_hash FROM usuarios WHERE username=$1`, cfg.Username); err != nil {
		t.Fatalf("read password hash: %v", err)
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(cfg.Password)) != nil {
		t.Fatal("password not rotated")
	}

	// Ya correcto: idempotente aunque ADMIN_REBIND siga puesto.
	if res, _, err := ensureAdmin(ctx, f.db, gcm, f.raw, cfg); err != nil || res != outcomeExists {
		t.Fatalf("res=%v err=%v", res, err)
	}
}

func TestEnsureAdminRefusesSharedEmailOrForeignDNI(t *testing.T) {
	f := setup(t)
	gcm, _ := crypto.NewCipher(testHexKey)
	ctx := context.Background()

	cfg := f.cfg()
	f.insertRealPersona(t, "5"+f.suffix, cfg.Email) // someone else already has this email
	if _, _, err := ensureAdmin(ctx, f.db, gcm, f.raw, cfg); err == nil {
		t.Fatal("expected refusal: email belongs to another persona")
	}

	cfg2 := f.cfg()
	cfg2.Email = "other-" + f.suffix + "@example.com"
	cfg2.DNI = "6" + f.suffix
	f.insertRealPersona(t, cfg2.DNI, "owner-"+f.suffix+"@example.com") // ADMIN_DNI of a real person with another email
	if _, _, err := ensureAdmin(ctx, f.db, gcm, f.raw, cfg2); err == nil {
		t.Fatal("expected refusal: ADMIN_DNI belongs to a persona with another email")
	}
}
