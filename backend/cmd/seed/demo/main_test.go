package main

import (
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
	"github.com/cbaciliod/learning/internal/perfiles"
	"github.com/cbaciliod/learning/internal/questions"
	"github.com/cbaciliod/learning/internal/simulations"
	"github.com/cbaciliod/learning/internal/temas"
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
		"preguntas":            `SELECT COUNT(*) FROM preguntas WHERE estado = 'aprobada' AND proceso_id IN (SELECT p.id FROM procesos p JOIN instituciones i ON i.id = p.institucion_id WHERE i.nombre IN ($1, $2))`,
		"configuracion_examen": `SELECT COUNT(*) FROM configuracion_examen WHERE proceso_id IN (SELECT p.id FROM procesos p JOIN instituciones i ON i.id = p.institucion_id WHERE i.nombre IN ($1, $2))`,
		"temas":                `SELECT COUNT(*) FROM temas WHERE asignatura_id IN (SELECT id FROM asignaturas WHERE codigo LIKE 'DEMO-%') AND $1 <> $2`,
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
		`DELETE FROM tema_especialidades WHERE tema_id IN (SELECT t.id FROM temas t JOIN asignaturas a ON a.id = t.asignatura_id WHERE a.codigo LIKE 'DEMO-%')`,
		`DELETE FROM temas WHERE asignatura_id IN (SELECT id FROM asignaturas WHERE codigo LIKE 'DEMO-%')`,
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
	want := map[string]int{"instituciones": 2, "especialidades": 5, "asignaturas": 6, "procesos": 3,
		"proceso_asignaturas": 6 + 3 + 2 + 6, "preguntas": (6 + 3 + 2) * 10, "configuracion_examen": 3,
		"temas": 2, "usuarios": 5, "inscripciones": 6}
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
	if _, err := db.Exec(`DELETE FROM temas WHERE nombre = $1`, temasDemo[0].Nombre); err != nil {
		t.Fatal(err)
	}
	correr(t, db, cfg)
	if after := conteos(t, db); fmt.Sprint(after) != fmt.Sprint(want) {
		t.Errorf("counts after repairing a partial state = %v, want %v", after, want)
	}
}

// The seed is usable: every user logs in with its password through the real
// auth mux and is active with the tour done; the approved Medicina user
// creates a simulacro; the profession options and weights are the ones qa
// tests against (complete Medicina, estimated Enfermería and Obstetricia, a
// foreign especialidad refused, two procesos for demo_enfermeria).
func TestSembrar_DatosUsables(t *testing.T) {
	db, cfg := preparar(t)
	correr(t, db, cfg)
	rawKey, _ := hex.DecodeString(cfg.EncryptionKey)
	authRepo := auth.NewRepository(db, dbtest.GCM(t), rawKey)
	mux := http.NewServeMux()
	auth.Mount(mux, auth.NewService(authRepo, dbtest.JWTSecret, "http://localhost"), dbtest.JWTSecret, authRepo.FindActiveSesion)
	qSvc := questions.NewService(questions.NewRepository(db))
	simulations.Mount(mux, simulations.NewService(simulations.NewRepository(db), simulations.NewQuestionsAdapter(qSvc)), dbtest.JWTSecret, authRepo.FindActiveSesion)
	cat := catalogs.NewService(catalogs.NewRepository(db))
	perfiles.Mount(mux, perfiles.NewService(perfiles.NewRepository(db), cat), dbtest.JWTSecret, authRepo.FindActiveSesion)
	pedir := func(metodo, ruta, token, cuerpo string) (int, []byte) {
		req := httptest.NewRequest(metodo, ruta, strings.NewReader(cuerpo))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		return rr.Code, rr.Body.Bytes()
	}

	tokens := map[string]string{}
	for _, u := range usuariosDemo {
		password := cfg.PostulantePassword
		if u.Rol == "admin" {
			password = cfg.AdminPassword
		}
		body, _ := json.Marshal(map[string]string{"username": u.Username, "password": password})
		status, resp := pedir("POST", "/auth/login", "", string(body))
		var login struct {
			Token string `json:"token"`
			User  struct {
				Rol string `json:"rol"`
			} `json:"user"`
		}
		_ = json.Unmarshal(resp, &login)
		if status != http.StatusOK || login.Token == "" || login.User.Rol != u.Rol {
			t.Fatalf("login %s: %d %s", u.Username, status, resp)
		}
		tokens[u.Username] = login.Token
		var activo, tour bool
		if err := db.QueryRow(`SELECT estado = 'activo', tour_completado FROM usuarios WHERE username = $1`, u.Username).Scan(&activo, &tour); err != nil || !activo || !tour {
			t.Errorf("%s: activo=%v tour=%v %v", u.Username, activo, tour, err)
		}
	}

	proceso := func(nombre string) int64 {
		var id int64
		if err := db.Get(&id, `SELECT id FROM procesos WHERE nombre = $1 AND anio = $2`, nombre, anioPrueba); err != nil {
			t.Fatal(err)
		}
		return id
	}
	serums := proceso(procesoSERUMS)
	esp := func(institucion, codigo string) int64 {
		var id int64
		if err := db.Get(&id, `SELECT e.id FROM especialidades e JOIN instituciones i ON i.id = e.institucion_id WHERE e.codigo = $1 AND i.nombre = $2`, codigo, institucion); err != nil {
			t.Fatal(err)
		}
		return id
	}

	if status, body := pedir("POST", "/simulacros", tokens["demo_medicina"], fmt.Sprintf(`{"proceso_id":%d}`, serums)); status != http.StatusCreated {
		t.Errorf("demo_medicina creates a simulacro: %d %s", status, body)
	}

	var inscripciones []string
	if err := db.Select(&inscripciones, `SELECT u.username || '/' || p.nombre || '=' || i.estado FROM inscripciones i
		JOIN usuarios u ON u.id = i.usuario_id JOIN procesos p ON p.id = i.proceso_id
		WHERE u.username LIKE 'demo\_%' ORDER BY 1`); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(inscripciones, ", "); got != "demo_enfermeria/Proceso de compatibilidad demo=aprobado, demo_enfermeria/SERUMS 2026-I=aprobado, demo_enfermeria/SERUMS 2026-II=aprobado, "+
		"demo_esp_inactiva/SERUMS 2026-I=aprobado, demo_medicina/SERUMS 2026-I=aprobado, demo_pendiente/SERUMS 2026-I=pendiente" {
		t.Errorf("inscripciones = %s", got)
	}

	pesosCases := []struct {
		codigo    string
		modo      catalogs.ModoPesos
		estimados bool
	}{
		{"DEMO-MED", catalogs.ModoOficial, false},
		{"DEMO-ENF", catalogs.ModoEstimable, true},
		{"DEMO-OBS", catalogs.ModoEstimable, true},
	}
	for _, c := range pesosCases {
		p, err := cat.Pesos(context.Background(), serums, esp(institucionSalud, c.codigo), c.modo)
		if err != nil || p.PesosEstimados != c.estimados || len(p.Areas) != 6 {
			t.Errorf("%s weights = %+v, %v", c.codigo, p, err)
		}
	}

	// Profession options: several procesos leave proceso_id open (#103: two
	// of them in the same institution offer the persona's Enfermería); one
	// fills it; an inactive persona especialidad leaves no preselection (esp 0).
	opcionesCases := []struct {
		usuario  string
		procesos int
		conEsp   int // procesos that offer esp
		proceso  *int64
		esp      int64
	}{
		{"demo_enfermeria", 3, 2, nil, esp(institucionSalud, "DEMO-ENF")},
		{"demo_medicina", 1, 1, &serums, esp(institucionSalud, "DEMO-MED")},
		{"demo_esp_inactiva", 1, 0, nil, 0},
	}
	for _, c := range opcionesCases {
		status, body := pedir("GET", "/serums-ia/profesion/opciones", tokens[c.usuario], "")
		var op perfiles.Opciones
		_ = json.Unmarshal(body, &op)
		sinPreseleccion := c.esp == 0
		if status != http.StatusOK || len(op.Procesos) != c.procesos || (op.Preseleccion == nil) != sinPreseleccion ||
			!sinPreseleccion && (!mismoID(op.Preseleccion.ProcesoID, c.proceso) || op.Preseleccion.EspecialidadID != c.esp) {
			t.Errorf("%s opciones: %d %s", c.usuario, status, body)
		}
		conEsp := 0
		for _, p := range op.Procesos {
			for _, e := range p.Especialidades {
				if e.ID == c.esp {
					conEsp++
				}
			}
		}
		if conEsp != c.conEsp {
			t.Errorf("%s: %d procesos offer especialidad %d, want %d", c.usuario, conEsp, c.esp, c.conEsp)
		}
	}
	ajena := fmt.Sprintf(`{"proceso_id":%d,"especialidad_id":%d}`, serums, esp(institucionCompat, "DEMO-ENF"))
	if status, body := pedir("PUT", "/serums-ia/profesion", tokens["demo_medicina"], ajena); status != http.StatusUnprocessableEntity || !strings.Contains(string(body), "especialidad_invalida") {
		t.Errorf("foreign especialidad: %d %s", status, body)
	}

	var sp int64
	if err := db.Get(&sp, `SELECT id FROM asignaturas WHERE codigo = 'DEMO-SP'`); err != nil {
		t.Fatal(err)
	}
	activos, err := temas.NewService(temas.NewRepository(db)).ListarActivos(context.Background(), sp, esp(institucionSalud, "DEMO-MED"))
	if err != nil || len(activos) != len(temasDemo) {
		t.Errorf("temas = %+v, %v", activos, err)
	}
	var inactiva, descripcion string
	if err := db.Get(&inactiva, `SELECT e.estado FROM especialidades e JOIN instituciones i ON i.id = e.institucion_id WHERE e.codigo = 'DEMO-ODO' AND i.nombre = $1`, institucionSalud); err != nil || inactiva != "inactivo" {
		t.Errorf("Odontología = %q, %v", inactiva, err)
	}
	if err := db.Get(&descripcion, `SELECT descripcion FROM procesos WHERE id = $1`, serums); err != nil || descripcion != "" {
		t.Errorf("descripcion = %q, %v", descripcion, err)
	}
}

// usuarioGuardado is what a run must not change on an account it skips.
type usuarioGuardado struct {
	Hash      string `db:"password_hash"`
	Rol       string `db:"rol"`
	Estado    string `db:"estado"`
	PersonaID int64  `db:"persona_id"`
	UpdatedAt string `db:"updated_at"`
}

func leerUsuario(t *testing.T, db *sqlx.DB, username string) usuarioGuardado {
	t.Helper()
	var u usuarioGuardado
	if err := db.Get(&u, `SELECT password_hash, rol, estado, persona_id, updated_at::text FROM usuarios WHERE username = $1`, username); err != nil {
		t.Fatalf("%s: %v", username, err)
	}
	return u
}

func claveEs(t *testing.T, db *sqlx.DB, username, password string) bool {
	t.Helper()
	return bcrypt.CompareHashAndPassword([]byte(leerUsuario(t, db, username).Hash), []byte(password)) == nil
}

// Password rotation only reaches demo accounts: a demo_ username whose
// persona is in a demo institution. A run with a new password rotates them
// and leaves every other account as it was, even when the demo one fails.
func TestSembrar_SoloTocaUsuariosDemo(t *testing.T) {
	const passNueva = "postulante-rotada-654321"
	cases := []struct {
		name string
		// sembrado runs the seed once, with the old password, before alterar.
		sembrado bool
		// alterar prepares the case and returns its check after the second run.
		alterar func(t *testing.T, db *sqlx.DB) func(t *testing.T)
		errDe   string // username the second run must refuse; "" = it succeeds
	}{
		{"rota la contraseña de los usuarios demo", true, func(t *testing.T, db *sqlx.DB) func(t *testing.T) {
			return func(t *testing.T) {
				if !claveEs(t, db, "demo_medicina", passNueva) || !claveEs(t, db, "demo_enfermeria", passNueva) {
					t.Error("the demo password was not rotated")
				}
			}
		}, ""},
		{"no toca un usuario que no es demo", true, func(t *testing.T, db *sqlx.DB) func(t *testing.T) {
			otro := dbtest.SeedUser(t, db, dbtest.SeedCatalog(t, db), "usuario", "activo")
			antes := leerUsuario(t, db, otro.Username)
			return func(t *testing.T) {
				if despues := leerUsuario(t, db, otro.Username); despues != antes {
					t.Errorf("non-demo user changed: %+v → %+v", antes, despues)
				}
			}
		}, ""},
		{"no rota un demo cuya persona salió de las instituciones demo", true, func(t *testing.T, db *sqlx.DB) func(t *testing.T) {
			cat := dbtest.SeedCatalog(t, db)
			var persona, institucion int64
			if err := db.QueryRow(`SELECT p.id, p.institucion_id FROM personas p JOIN usuarios u ON u.persona_id = p.id
				WHERE u.username = 'demo_medicina'`).Scan(&persona, &institucion); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`UPDATE personas SET institucion_id = $2 WHERE id = $1`, persona, cat.InstitucionID); err != nil {
				t.Fatal(err)
			}
			// Registered after SeedCatalog, so it runs before that cleanup.
			t.Cleanup(func() { _, _ = db.Exec(`UPDATE personas SET institucion_id = $2 WHERE id = $1`, persona, institucion) })
			antes := leerUsuario(t, db, "demo_medicina")
			return func(t *testing.T) {
				var ahora int64
				if err := db.Get(&ahora, `SELECT institucion_id FROM personas WHERE id = $1`, persona); err != nil || ahora != cat.InstitucionID {
					t.Errorf("persona institution = %d, %v; want it untouched", ahora, err)
				}
				if despues := leerUsuario(t, db, "demo_medicina"); despues != antes {
					t.Errorf("user changed: %+v → %+v", antes, despues)
				}
			}
		}, "demo_medicina"},
		{"no adopta un username demo de otra persona", false, func(t *testing.T, db *sqlx.DB) func(t *testing.T) {
			ajeno := dbtest.SeedUser(t, db, dbtest.SeedCatalog(t, db), "usuario", "activo")
			if _, err := db.Exec(`UPDATE usuarios SET username = 'demo_medicina' WHERE id = $1`, ajeno.ID); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _, _ = db.Exec(`UPDATE usuarios SET username = $2 WHERE id = $1`, ajeno.ID, ajeno.Username) })
			antes := leerUsuario(t, db, "demo_medicina")
			return func(t *testing.T) {
				if despues := leerUsuario(t, db, "demo_medicina"); despues != antes {
					t.Errorf("foreign user changed: %+v → %+v", antes, despues)
				}
			}
		}, "demo_medicina"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			db, cfg := preparar(t)
			if c.sembrado {
				correr(t, db, cfg)
			}
			verificar := c.alterar(t, db)
			rotada := cfg
			rotada.PostulantePassword = passNueva
			rawKey, _ := hex.DecodeString(cfg.EncryptionKey)
			_, err := sembrar(context.Background(), db, dbtest.GCM(t), rawKey, rotada)
			switch {
			case c.errDe == "" && err != nil:
				t.Fatalf("sembrar: %v", err)
			case c.errDe != "" && (err == nil || !strings.Contains(err.Error(), c.errDe)):
				t.Fatalf("err = %v, want a refusal on %s", err, c.errDe)
			}
			verificar(t)
		})
	}
}

// Every demo account is recognisable as such: the prefix that production
// usernames never carry, and a reserved email domain.
func TestUsuariosDemo_SonSinteticos(t *testing.T) {
	for _, u := range usuariosDemo {
		if !strings.HasPrefix(u.Username, prefijoUsuario) {
			t.Errorf("%s lacks the %q prefix", u.Username, prefijoUsuario)
		}
	}
	if !strings.HasSuffix(dominioEmail, ".invalid") {
		t.Errorf("dominioEmail = %q, want a reserved .invalid domain", dominioEmail)
	}
	// A dataset edit without the prefix is refused before any query: the
	// zero sembrador has no database to touch.
	if _, err := (&sembrador{}).usuario(context.Background(), usuarioDemo{Username: "admin", Rol: "admin"}, 1, 1, 1); err == nil {
		t.Error("a username without the demo prefix was accepted")
	}
}

// mismoID reports whether two optional ids are both absent or equal.
func mismoID(a, b *int64) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
