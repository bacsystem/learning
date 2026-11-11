package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/cbaciliod/learning/internal/auth"
	"github.com/cbaciliod/learning/internal/catalogs"
	"github.com/cbaciliod/learning/internal/db/dbtest"
	"github.com/cbaciliod/learning/internal/questions"
	"github.com/cbaciliod/learning/internal/simulations"
	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"
)

const (
	passAdmin      = "admin-demo-123456"
	passPostulante = "postulante-demo-123456"
	anioPrueba     = 2099 // far from any real configuracion_pago
)

func TestMain(m *testing.M) {
	bcryptCost = bcrypt.MinCost // the seed's hashing is not under test here
	os.Exit(m.Run())
}

func entorno(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func base() map[string]string {
	return map[string]string{"ENV": "preprod", "DATABASE_URL": "postgres://x", "ENCRYPTION_KEY": dbtest.TestHexKey,
		"DEMO_ADMIN_PASSWORD": passAdmin, "DEMO_POSTULANTE_PASSWORD": passPostulante}
}

// The guards run before the database is opened: anything but ENV=preprod or
// ENV=development, exactly, and any missing or short password abort.
func TestConfigFromEnv_Guardias(t *testing.T) {
	cases := []struct {
		name   string
		cambio map[string]string
		ok     bool
	}{
		{"preprod", nil, true},
		{"development", map[string]string{"ENV": "development"}, true},
		{"production", map[string]string{"ENV": "production"}, false},
		{"ENV vacío", map[string]string{"ENV": ""}, false},
		{"test", map[string]string{"ENV": "test"}, false},
		{"mayúsculas", map[string]string{"ENV": "PREPROD"}, false},
		{"con espacios", map[string]string{"ENV": " development"}, false},
		{"sin contraseña de admin", map[string]string{"DEMO_ADMIN_PASSWORD": ""}, false},
		{"contraseña de postulante corta", map[string]string{"DEMO_POSTULANTE_PASSWORD": "corta"}, false},
		{"contraseña de 73", map[string]string{"DEMO_ADMIN_PASSWORD": strings.Repeat("a", 73)}, false},
		{"sin DATABASE_URL", map[string]string{"DATABASE_URL": ""}, false},
		{"sin ENCRYPTION_KEY", map[string]string{"ENCRYPTION_KEY": ""}, false},
		{"año válido", map[string]string{"DEMO_ANIO": "2030"}, true},
		{"año fuera de rango", map[string]string{"DEMO_ANIO": "2023"}, false},
		{"año no numérico", map[string]string{"DEMO_ANIO": "dos mil"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := base()
			for k, v := range c.cambio {
				m[k] = v
			}
			cfg, err := configFromEnv(entorno(m))
			if (err == nil) != c.ok {
				t.Fatalf("err = %v, want ok=%v", err, c.ok)
			}
			if err != nil && (strings.Contains(err.Error(), passAdmin) || strings.Contains(err.Error(), passPostulante)) {
				t.Errorf("the error leaks a password: %v", err)
			}
			if c.ok && c.cambio["DEMO_ANIO"] == "" && cfg.Anio != anioPorDefecto {
				t.Errorf("anio = %d, want %d", cfg.Anio, anioPorDefecto)
			}
		})
	}
}

// ── Integration (TEST_DATABASE_URL) ───────────────────────────────────────────

func configPrueba(t *testing.T) config {
	t.Helper()
	m := base()
	m["DEMO_ANIO"] = fmt.Sprint(anioPrueba)
	cfg, err := configFromEnv(entorno(m))
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func correr(t *testing.T, db *sqlx.DB, cfg config) string {
	t.Helper()
	rawKey, _ := hex.DecodeString(cfg.EncryptionKey)
	res, err := sembrar(context.Background(), db, dbtest.GCM(t), rawKey, cfg)
	if err != nil {
		t.Fatalf("sembrar: %v", err)
	}
	return res
}

// conteos counts the demo rows of every table the seed writes.
func conteos(t *testing.T, db *sqlx.DB) map[string]int {
	t.Helper()
	consultas := map[string]string{
		"instituciones":        `SELECT COUNT(*) FROM instituciones WHERE nombre IN ($1, $2)`,
		"especialidades":       `SELECT COUNT(*) FROM especialidades WHERE codigo LIKE 'DEMO-%' AND institucion_id IN (SELECT id FROM instituciones WHERE nombre IN ($1, $2))`,
		"asignaturas":          `SELECT COUNT(*) FROM asignaturas WHERE codigo LIKE 'DEMO-%' AND $1 <> $2`,
		"procesos":             `SELECT COUNT(*) FROM procesos WHERE institucion_id IN (SELECT id FROM instituciones WHERE nombre IN ($1, $2))`,
		"proceso_asignaturas":  `SELECT COUNT(*) FROM proceso_asignaturas WHERE proceso_id IN (SELECT p.id FROM procesos p JOIN instituciones i ON i.id = p.institucion_id WHERE i.nombre IN ($1, $2))`,
		"preguntas":            `SELECT COUNT(*) FROM preguntas WHERE estado = 'activo' AND proceso_id IN (SELECT p.id FROM procesos p JOIN instituciones i ON i.id = p.institucion_id WHERE i.nombre IN ($1, $2))`,
		"configuracion_examen": `SELECT COUNT(*) FROM configuracion_examen WHERE proceso_id IN (SELECT p.id FROM procesos p JOIN instituciones i ON i.id = p.institucion_id WHERE i.nombre IN ($1, $2))`,
		"usuarios":             `SELECT COUNT(*) FROM usuarios WHERE username LIKE 'demo\_%' AND $1 <> $2`,
		"inscripciones":        `SELECT COUNT(*) FROM inscripciones WHERE usuario_id IN (SELECT id FROM usuarios WHERE username LIKE 'demo\_%') AND $1 <> $2`,
	}
	out := map[string]int{}
	for k, q := range consultas {
		var n int
		if err := db.Get(&n, q, institucionSalud, institucionCompat); err != nil {
			t.Fatalf("%s: %v", k, err)
		}
		out[k] = n
	}
	return out
}

// limpiar removes every demo row, children first.
func limpiar(t *testing.T, db *sqlx.DB, pagoPrevio bool) {
	procesos := `(SELECT p.id FROM procesos p JOIN instituciones i ON i.id = p.institucion_id WHERE i.nombre IN ('` + institucionSalud + `', '` + institucionCompat + `'))`
	usuarios := `(SELECT id FROM usuarios WHERE username LIKE 'demo\_%')`
	stmts := []string{
		`DELETE FROM simulacros WHERE usuario_id IN ` + usuarios,
		`DELETE FROM postulante_profesiones WHERE usuario_id IN ` + usuarios,
		`DELETE FROM inscripciones WHERE usuario_id IN ` + usuarios,
		`DELETE FROM sesiones WHERE usuario_id IN ` + usuarios,
		`DELETE FROM audit_log WHERE usuario_id IN ` + usuarios,
		`DELETE FROM usuarios WHERE username LIKE 'demo\_%'`,
		`DELETE FROM personas WHERE institucion_id IN (SELECT id FROM instituciones WHERE nombre IN ('` + institucionSalud + `', '` + institucionCompat + `'))`,
		`DELETE FROM respuestas WHERE pregunta_id IN (SELECT id FROM preguntas WHERE proceso_id IN ` + procesos + `)`,
		`DELETE FROM preguntas WHERE proceso_id IN ` + procesos,
		`DELETE FROM configuracion_examen WHERE proceso_id IN ` + procesos,
		`DELETE FROM proceso_asignaturas WHERE proceso_id IN ` + procesos,
		`DELETE FROM procesos WHERE id IN ` + procesos,
		`DELETE FROM grados WHERE institucion_id IN (SELECT id FROM instituciones WHERE nombre IN ('` + institucionSalud + `', '` + institucionCompat + `'))`,
		`DELETE FROM especialidades WHERE institucion_id IN (SELECT id FROM instituciones WHERE nombre IN ('` + institucionSalud + `', '` + institucionCompat + `'))`,
		`DELETE FROM asignaturas WHERE codigo LIKE 'DEMO-%'`,
		`DELETE FROM instituciones WHERE nombre IN ('` + institucionSalud + `', '` + institucionCompat + `')`,
	}
	if !pagoPrevio {
		stmts = append(stmts, fmt.Sprintf(`DELETE FROM configuracion_pago WHERE anio = %d`, anioPrueba))
	}
	for _, q := range stmts {
		if _, err := db.Exec(q); err != nil {
			t.Errorf("cleanup %q: %v", q, err)
		}
	}
}

func preparar(t *testing.T) (*sqlx.DB, config) {
	t.Helper()
	db := dbtest.Open(t) // takes the shared test lock: other packages assert global counts
	var pagoPrevio bool
	if err := db.Get(&pagoPrevio, `SELECT EXISTS (SELECT 1 FROM configuracion_pago WHERE anio = $1)`, anioPrueba); err != nil {
		t.Fatal(err)
	}
	limpiar(t, db, pagoPrevio)
	t.Cleanup(func() { limpiar(t, db, pagoPrevio) })
	return db, configPrueba(t)
}

// Idempotence: a second run creates nothing and leaves the same counts; a
// run over a partial state completes it.
func TestSembrar_Idempotente(t *testing.T) {
	db, cfg := preparar(t)
	correr(t, db, cfg)
	despues := conteos(t, db)
	want := map[string]int{"instituciones": 2, "especialidades": 4, "asignaturas": 6, "procesos": 2,
		"proceso_asignaturas": 6 + 2 + 3, "preguntas": (6 + 2) * preguntasPorArea, "configuracion_examen": 2,
		"usuarios": 5, "inscripciones": 4}
	if fmt.Sprint(despues) != fmt.Sprint(want) {
		t.Fatalf("counts = %v, want %v", despues, want)
	}
	if res := correr(t, db, cfg); res != "creados: map[]" {
		t.Errorf("second run created rows: %s", res)
	}
	if again := conteos(t, db); fmt.Sprint(again) != fmt.Sprint(want) {
		t.Errorf("counts after a second run = %v", again)
	}

	// Partial state: a lost configuration and question are recreated.
	if _, err := db.Exec(`DELETE FROM configuracion_examen WHERE proceso_id IN (SELECT id FROM procesos WHERE nombre = $1)`, procesoCompatibilidad); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE preguntas SET estado = 'eliminada' WHERE enunciado LIKE 'Pregunta demo 1 de DEMO-SP%'`); err != nil {
		t.Fatal(err)
	}
	correr(t, db, cfg)
	if after := conteos(t, db); fmt.Sprint(after) != fmt.Sprint(want) {
		t.Errorf("counts after repairing a partial state = %v, want %v", after, want)
	}
}

// The seed is usable: every user logs in with its password through the real
// auth mux and is active with the tour done; the approved Medicina user
// creates a simulacro; Medicina has saved weights and Enfermería estimated.
func TestSembrar_DatosUsables(t *testing.T) {
	db, cfg := preparar(t)
	correr(t, db, cfg)
	rawKey, _ := hex.DecodeString(cfg.EncryptionKey)
	authRepo := auth.NewRepository(db, dbtest.GCM(t), rawKey)
	mux := http.NewServeMux()
	auth.Mount(mux, auth.NewService(authRepo, dbtest.JWTSecret, "http://localhost"), dbtest.JWTSecret, authRepo.FindActiveSesion)
	qSvc := questions.NewService(questions.NewRepository(db))
	simulations.Mount(mux, simulations.NewService(simulations.NewRepository(db), simulations.NewQuestionsAdapter(qSvc)), dbtest.JWTSecret, authRepo.FindActiveSesion)

	tokens := map[string]string{}
	for _, u := range usuariosDemo {
		password := cfg.PostulantePassword
		if u.Rol == "admin" {
			password = cfg.AdminPassword
		}
		body, _ := json.Marshal(map[string]string{"username": u.Username, "password": password})
		req := httptest.NewRequest("POST", "/auth/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		var resp struct {
			Token string `json:"token"`
			User  struct {
				Rol string `json:"rol"`
			} `json:"user"`
		}
		_ = json.Unmarshal(rr.Body.Bytes(), &resp)
		if rr.Code != http.StatusOK || resp.Token == "" || resp.User.Rol != u.Rol {
			t.Fatalf("login %s: %d %s", u.Username, rr.Code, rr.Body)
		}
		tokens[u.Username] = resp.Token
		var activo, tour bool
		if err := db.QueryRow(`SELECT estado = 'activo', tour_completado FROM usuarios WHERE username = $1`, u.Username).Scan(&activo, &tour); err != nil || !activo || !tour {
			t.Errorf("%s: activo=%v tour=%v %v", u.Username, activo, tour, err)
		}
	}

	var procesoID int64
	if err := db.Get(&procesoID, `SELECT id FROM procesos WHERE nombre = $1 AND anio = $2`, procesoSERUMS, anioPrueba); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/simulacros", strings.NewReader(fmt.Sprintf(`{"proceso_id":%d}`, procesoID)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokens["demo_medicina"])
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Errorf("demo_medicina creates a simulacro: %d %s", rr.Code, rr.Body)
	}

	estados := map[string]string{}
	var filas []struct {
		Username string `db:"username"`
		Estado   string `db:"estado"`
	}
	if err := db.Select(&filas, `SELECT u.username, i.estado FROM inscripciones i JOIN usuarios u ON u.id = i.usuario_id
		WHERE u.username LIKE 'demo\_%' AND i.proceso_id = $1`, procesoID); err != nil {
		t.Fatal(err)
	}
	for _, f := range filas {
		estados[f.Username] = f.Estado
	}
	if fmt.Sprint(estados) != "map[demo_enfermeria:aprobado demo_esp_inactiva:aprobado demo_medicina:aprobado demo_pendiente:pendiente]" {
		t.Errorf("inscripciones = %v", estados)
	}

	pesos := catalogs.NewService(catalogs.NewRepository(db))
	esp := func(codigo string) int64 {
		var id int64
		if err := db.Get(&id, `SELECT e.id FROM especialidades e JOIN instituciones i ON i.id = e.institucion_id WHERE e.codigo = $1 AND i.nombre = $2`, codigo, institucionSalud); err != nil {
			t.Fatal(err)
		}
		return id
	}
	med, err := pesos.Pesos(context.Background(), procesoID, esp("DEMO-MED"), catalogs.ModoOficial)
	if err != nil || med.PesosEstimados || len(med.Areas) != 3 {
		t.Errorf("Medicina weights = %+v, %v", med, err)
	}
	enf, err := pesos.Pesos(context.Background(), procesoID, esp("DEMO-ENF"), catalogs.ModoEstimable)
	if err != nil || !enf.PesosEstimados || len(enf.Areas) != 6 {
		t.Errorf("Enfermería weights = %+v, %v", enf, err)
	}
	var inactiva string
	if err := db.Get(&inactiva, `SELECT e.estado FROM especialidades e JOIN instituciones i ON i.id = e.institucion_id WHERE e.codigo = 'DEMO-ODO' AND i.nombre = $1`, institucionSalud); err != nil || inactiva != "inactivo" {
		t.Errorf("Odontología = %q, %v", inactiva, err)
	}
	var descripcion string
	if err := db.Get(&descripcion, `SELECT descripcion FROM procesos WHERE id = $1`, procesoID); err != nil || descripcion != "" {
		t.Errorf("descripcion = %q, %v", descripcion, err)
	}
}

// A demo username already held by a persona that is not its demo persona is
// never adopted.
func TestSembrar_NoAdoptaUsuariosAjenos(t *testing.T) {
	db, cfg := preparar(t)
	cat := dbtest.SeedCatalog(t, db)
	ajeno := dbtest.SeedUser(t, db, cat, "usuario", "activo")
	if _, err := db.Exec(`UPDATE usuarios SET username = 'demo_medicina' WHERE id = $1`, ajeno.ID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`UPDATE usuarios SET username = $2 WHERE id = $1`, ajeno.ID, ajeno.Username) })
	rawKey, _ := hex.DecodeString(cfg.EncryptionKey)
	if _, err := sembrar(context.Background(), db, dbtest.GCM(t), rawKey, cfg); err == nil || !strings.Contains(err.Error(), "demo_medicina") {
		t.Errorf("err = %v, want a refusal on demo_medicina", err)
	}
}
