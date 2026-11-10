package admin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/cbaciliod/learning/internal/audit"
	"github.com/cbaciliod/learning/internal/audit/audittest"
	"github.com/cbaciliod/learning/internal/db/dbtest"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func setup(t *testing.T) (*sqlx.DB, *Handler) {
	t.Helper()
	db := dbtest.Open(t)
	return db, NewHandler(NewRepository(db, dbtest.GCM(t), nil), audit.New(db))
}

func count(t *testing.T, db *sqlx.DB, q string, args ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRow(q, args...).Scan(&n); err != nil {
		t.Fatalf("count %q: %v", q, err)
	}
	return n
}

func patchRol(t *testing.T, h *Handler, actorID, targetID int64, rol string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPatch, "/admin/usuarios/x/rol",
		bytes.NewBufferString(fmt.Sprintf(`{"rol":%q}`, rol)))
	req.SetPathValue("id", fmt.Sprint(targetID))
	return dbtest.Serve(t, h.handlePatchUsuarioRol, req, actorID, "admin")
}

// H23: unknown id → 404 and nothing audited.
func TestPatchRol_NotFound(t *testing.T) {
	db, h := setup(t)
	cat := dbtest.SeedCatalog(t, db)
	admin := dbtest.SeedUser(t, db, cat, "admin", "activo")
	const ghost = 9_000_000_002
	rr := patchRol(t, h, admin.ID, ghost, "admin")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d %s", rr.Code, rr.Body)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM audit_log WHERE accion='cambiar_rol' AND entidad_id=$1`, ghost); n != 0 {
		t.Fatalf("audit written for a failed change (%d rows)", n)
	}
}

// H2: a role change revokes the target's sessions in the same transaction.
func TestPatchRol_RevokesSessions(t *testing.T) {
	db, h := setup(t)
	cat := dbtest.SeedCatalog(t, db)
	admin := dbtest.SeedUser(t, db, cat, "admin", "activo")
	user := dbtest.SeedUser(t, db, cat, "usuario", "activo")
	dbtest.SeedSession(t, db, user.ID)
	dbtest.SeedSession(t, db, user.ID)
	dbtest.SeedSession(t, db, admin.ID)

	if rr := patchRol(t, h, admin.ID, user.ID, "admin"); rr.Code != http.StatusOK {
		t.Fatalf("promote: want 200, got %d %s", rr.Code, rr.Body)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM sesiones WHERE usuario_id=$1`, user.ID); n != 0 {
		t.Fatalf("target still has %d sessions after role change", n)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM sesiones WHERE usuario_id=$1`, admin.ID); n != 1 {
		t.Fatalf("actor sessions touched: %d", n)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM audit_log WHERE accion='cambiar_rol' AND entidad_id=$1`, user.ID); n != 1 {
		t.Fatalf("audit rows = %d, want 1", n)
	}
}

// The audit row is written in the role change's transaction: if it cannot be
// written, the role and the sessions stay as they were.
func TestPatchRol_AuditFailureRollsBack(t *testing.T) {
	db, h := setup(t)
	cat := dbtest.SeedCatalog(t, db)
	admin := dbtest.SeedUser(t, db, cat, "admin", "activo")
	user := dbtest.SeedUser(t, db, cat, "usuario", "activo")
	dbtest.SeedSession(t, db, user.ID)
	audittest.FailInsertsBy(t, db, admin.ID)

	if rr := patchRol(t, h, admin.ID, user.ID, "admin"); rr.Code != http.StatusInternalServerError {
		t.Fatalf("want 500 when the audit fails, got %d %s", rr.Code, rr.Body)
	}
	var rol string
	if err := db.QueryRow(`SELECT rol FROM usuarios WHERE id=$1`, user.ID).Scan(&rol); err != nil || rol != "usuario" {
		t.Fatalf("role changed without audit: %q (%v)", rol, err)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM sesiones WHERE usuario_id=$1`, user.ID); n != 1 {
		t.Fatalf("sessions revoked without audit: %d left", n)
	}
	if n := audittest.Count(t, db, "cambiar_rol", user.ID); n != 0 {
		t.Fatalf("audit rows = %d, want 0", n)
	}
}

// H23: the last active admin cannot be demoted.
func TestPatchRol_LastAdmin(t *testing.T) {
	db, h := setup(t)
	if n := count(t, db, `SELECT COUNT(*) FROM usuarios WHERE rol='admin' AND estado='activo'`); n != 0 {
		t.Skipf("database already has %d active admins; test needs a clean usuarios table", n)
	}
	cat := dbtest.SeedCatalog(t, db)
	admin := dbtest.SeedUser(t, db, cat, "admin", "activo")
	if rr := patchRol(t, h, admin.ID, admin.ID, "usuario"); rr.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d %s", rr.Code, rr.Body)
	}
}

// H23: two admins demoting each other at the same time must leave one admin.
func TestPatchRol_ConcurrentMutualDemotion(t *testing.T) {
	db, h := setup(t)
	if n := count(t, db, `SELECT COUNT(*) FROM usuarios WHERE rol='admin' AND estado='activo'`); n != 0 {
		t.Skipf("database already has %d active admins; test needs a clean usuarios table", n)
	}
	cat := dbtest.SeedCatalog(t, db)
	for round := 0; round < 15; round++ {
		a := dbtest.SeedUser(t, db, cat, "admin", "activo")
		b := dbtest.SeedUser(t, db, cat, "admin", "activo")
		var wg sync.WaitGroup
		codes := make([]int, 2)
		start := make(chan struct{})
		for i, pair := range [][2]int64{{a.ID, b.ID}, {b.ID, a.ID}} {
			wg.Add(1)
			go func(i int, actor, target int64) {
				defer wg.Done()
				<-start
				codes[i] = patchRol(t, h, actor, target, "usuario").Code
			}(i, pair[0], pair[1])
		}
		close(start)
		wg.Wait()
		left := count(t, db, `SELECT COUNT(*) FROM usuarios WHERE rol='admin' AND estado='activo'`)
		if left != 1 {
			t.Fatalf("round %d: %d active admins left (codes %v), want 1", round, left, codes)
		}
		// demote the survivor's partner back to a clean state for the next round
		_, _ = db.Exec(`UPDATE usuarios SET rol='usuario' WHERE id IN ($1,$2)`, a.ID, b.ID)
	}
}

// H22: user detail counts completed simulacros and averages nota_centesimal.
// H56: total_practicas counts only completadas (same rule as the dashboard).
func TestUsuarioDetalle_Stats(t *testing.T) {
	db, h := setup(t)
	cat := dbtest.SeedCatalog(t, db)
	admin := dbtest.SeedUser(t, db, cat, "admin", "activo")
	user := dbtest.SeedUser(t, db, cat, "usuario", "activo")
	proc := dbtest.SeedProceso(t, db, cat, 2097, "activo")
	var cfg int64
	if err := db.QueryRow(`INSERT INTO configuracion_examen (proceso_id, tiempo_total_seg, total_preguntas)
		VALUES ($1, 600, 10) RETURNING id`, proc).Scan(&cfg); err != nil {
		t.Fatal(err)
	}
	for _, s := range []struct {
		estado string
		nota   any
	}{{"completado", 80}, {"completado", 60}, {"en_curso", nil}} {
		if _, err := db.Exec(`INSERT INTO simulacros (usuario_id, institucion_id, proceso_id, configuracion_id,
			tiempo_limite_seg, estado, nota_centesimal) VALUES ($1,$2,$3,$4,600,$5,$6)`,
			user.ID, cat.InstitucionID, proc, cfg, s.estado, s.nota); err != nil {
			t.Fatal(err)
		}
	}
	for _, e := range []string{"completada", "en_curso", "eliminada", "abandonada"} {
		if _, err := db.Exec(`INSERT INTO practicas (usuario_id, institucion_id, proceso_id, modo_calificacion, estado)
			VALUES ($1,$2,$3,'inmediato',$4)`, user.ID, cat.InstitucionID, proc, e); err != nil {
			t.Fatal(err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/usuarios/x", nil)
	req.SetPathValue("id", fmt.Sprint(user.ID))
	rr := dbtest.Serve(t, h.handleGetUsuarioDetalle, req, admin.ID, "admin")
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d %s", rr.Code, rr.Body)
	}
	var d UsuarioDetalleResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &d); err != nil {
		t.Fatal(err)
	}
	if d.TotalSimulacros != 2 || d.NotaPromedio != 70 {
		t.Fatalf("simulacros = %d avg = %v, want 2 / 70", d.TotalSimulacros, d.NotaPromedio)
	}
	if d.TotalPracticas != 1 {
		t.Fatalf("practicas = %d, want 1", d.TotalPracticas)
	}
	if d.Nombres != user.Nombres {
		t.Fatalf("nombres = %q, want decrypted %q", d.Nombres, user.Nombres)
	}
}

// H54: undecryptable PII is withheld (""), never echoed as ciphertext.
func TestUsuarioDetalle_DecryptFailure(t *testing.T) {
	db, h := setup(t)
	cat := dbtest.SeedCatalog(t, db)
	admin := dbtest.SeedUser(t, db, cat, "admin", "activo")
	user := dbtest.SeedUser(t, db, cat, "usuario", "activo")
	if _, err := db.Exec(`UPDATE personas SET telefono='opaque-cipher-text' WHERE id=$1`, user.PersonaID); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/admin/usuarios/x", nil)
	req.SetPathValue("id", fmt.Sprint(user.ID))
	rr := dbtest.Serve(t, h.handleGetUsuarioDetalle, req, admin.ID, "admin")
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d %s", rr.Code, rr.Body)
	}
	if strings.Contains(rr.Body.String(), "opaque-cipher-text") {
		t.Fatal("ciphertext leaked")
	}
}

func putConfig(t *testing.T, h *Handler, adminID int64, anio int, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPut, "/admin/ingresos/config/x", bytes.NewBufferString(body))
	req.SetPathValue("anio", fmt.Sprint(anio))
	return dbtest.Serve(t, h.handlePutIngresosConfig, req, adminID, "admin")
}

// H25/H66: monto is required, ≥0, ≤ 99,999,999.99, at most 2 decimals, exact.
func TestPutIngresosConfig_Validation(t *testing.T) {
	db, h := setup(t)
	cat := dbtest.SeedCatalog(t, db)
	admin := dbtest.SeedUser(t, db, cat, "admin", "activo")
	const anio = 2096
	_, _ = db.Exec(`DELETE FROM configuracion_pago WHERE anio=$1`, anio)
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM configuracion_pago WHERE anio=$1`, anio) })

	for _, body := range []string{`{}`, `{"monto":null}`, `{"monto":12.345}`, `{"monto":-1}`,
		`{"monto":100000000}`, `{"monto":1e3}`, `{"monto":"abc"}`, `{"monto":true}`} {
		rr := putConfig(t, h, admin.ID, anio, body)
		if rr.Code != http.StatusUnprocessableEntity && rr.Code != http.StatusBadRequest {
			t.Fatalf("%s: want 400/422, got %d %s", body, rr.Code, rr.Body)
		}
		if n := count(t, db, `SELECT COUNT(*) FROM configuracion_pago WHERE anio=$1`, anio); n != 0 {
			t.Fatalf("%s: row written", body)
		}
	}

	rr := putConfig(t, h, admin.ID, anio, `{"monto":99999999.99}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("max value: want 200, got %d %s", rr.Code, rr.Body)
	}
	rr = putConfig(t, h, admin.ID, anio, `{"monto":150.1}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d %s", rr.Code, rr.Body)
	}
	if !strings.Contains(rr.Body.String(), `"monto":150.10`) {
		t.Fatalf("response must carry the exact decimal: %s", rr.Body)
	}
	var stored string
	_ = db.QueryRow(`SELECT monto::text FROM configuracion_pago WHERE anio=$1`, anio).Scan(&stored)
	if stored != "150.10" {
		t.Fatalf("stored %s, want 150.10", stored)
	}
	var detalle string
	if err := db.QueryRow(`SELECT detalle::text FROM audit_log WHERE accion='actualizar_config_pago'
		AND entidad_id=$1 ORDER BY id DESC LIMIT 1`, anio).Scan(&detalle); err != nil {
		t.Fatalf("change not audited: %v", err)
	}
	if !strings.Contains(detalle, `"monto": "150.10"`) || !strings.Contains(detalle, `"monto_anterior": "99999999.99"`) {
		t.Fatalf("audit detalle = %s", detalle)
	}
	_, _ = db.Exec(`DELETE FROM audit_log WHERE accion='actualizar_config_pago' AND entidad_id=$1`, anio)
}

// The price change and its audit row commit together: a failed audit leaves
// the previous price, and a successful change writes exactly one row.
func TestPutIngresosConfig_AuditInSameTx(t *testing.T) {
	db, h := setup(t)
	cat := dbtest.SeedCatalog(t, db)
	admin := dbtest.SeedUser(t, db, cat, "admin", "activo")
	const anio = 2095
	cleanup := func() {
		_, _ = db.Exec(`DELETE FROM configuracion_pago WHERE anio=$1`, anio)
		_, _ = db.Exec(`DELETE FROM audit_log WHERE accion='actualizar_config_pago' AND entidad_id=$1`, anio)
	}
	cleanup()
	t.Cleanup(cleanup)

	if rr := putConfig(t, h, admin.ID, anio, `{"monto":10}`); rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d %s", rr.Code, rr.Body)
	}
	if n := audittest.Count(t, db, "actualizar_config_pago", anio); n != 1 {
		t.Fatalf("audit rows = %d, want 1", n)
	}

	other := dbtest.SeedUser(t, db, cat, "admin", "activo")
	audittest.FailInsertsBy(t, db, other.ID)
	if rr := putConfig(t, h, other.ID, anio, `{"monto":20}`); rr.Code != http.StatusInternalServerError {
		t.Fatalf("want 500 when the audit fails, got %d %s", rr.Code, rr.Body)
	}
	var stored string
	_ = db.QueryRow(`SELECT monto::text FROM configuracion_pago WHERE anio=$1`, anio).Scan(&stored)
	if stored != "10.00" {
		t.Fatalf("price changed without audit: %s", stored)
	}
	if n := audittest.Count(t, db, "actualizar_config_pago", anio); n != 1 {
		t.Fatalf("audit rows = %d, want 1", n)
	}
}

// H53: malformed dates → 400; a huge page never overflows into a 500.
func TestAuditLog_QueryValidation(t *testing.T) {
	db, h := setup(t)
	cat := dbtest.SeedCatalog(t, db)
	admin := dbtest.SeedUser(t, db, cat, "admin", "activo")
	for q, want := range map[string]int{
		"from=abc":                              http.StatusBadRequest,
		"to=2024-13-01":                         http.StatusBadRequest,
		"from=2024-01-01&to=2024-01-31":         http.StatusOK,
		"page=99999999999999999":                http.StatusOK,
		"page=4611686018427387904&per_page=100": http.StatusOK,
	} {
		req := httptest.NewRequest(http.MethodGet, "/admin/audit-log?"+q, nil)
		rr := dbtest.Serve(t, h.handleGetAuditLog, req, admin.ID, "admin")
		if rr.Code != want {
			t.Fatalf("%s: want %d, got %d %s", q, want, rr.Code, rr.Body)
		}
	}
}

// H56: revenue is attributed to the approval date (revisado_at).
func TestIngresos_AttributedByApprovalDate(t *testing.T) {
	db, h := setup(t)
	cat := dbtest.SeedCatalog(t, db)
	admin := dbtest.SeedUser(t, db, cat, "admin", "activo")
	user := dbtest.SeedUser(t, db, cat, "usuario", "activo")
	proc := dbtest.SeedProceso(t, db, cat, 2097, "activo")

	get := func() IngresosResponse {
		req := httptest.NewRequest(http.MethodGet, "/admin/ingresos", nil)
		rr := dbtest.Serve(t, h.handleGetIngresos, req, admin.ID, "admin")
		if rr.Code != http.StatusOK {
			t.Fatalf("want 200, got %d %s", rr.Code, rr.Body)
		}
		var out IngresosResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	before := get()
	// requested two years ago, approved now
	if _, err := db.Exec(`INSERT INTO inscripciones (usuario_id, proceso_id, estado, monto, created_at, revisado_at, revisado_por)
		VALUES ($1, $2, 'aprobado', 777.77, NOW() - INTERVAL '2 years', NOW(), $3)`, user.ID, proc, admin.ID); err != nil {
		t.Fatal(err)
	}
	after := get()
	if d := delta(t, before.TotalEsteMes, after.TotalEsteMes); d != "777.77" {
		t.Fatalf("this month delta = %v, want 777.77", d)
	}
	if d := delta(t, before.TotalEsteAnio, after.TotalEsteAnio); d != "777.77" {
		t.Fatalf("this year delta = %v, want 777.77", d)
	}
}

// H56: "prácticas hoy" counts only completed practices, like the user detail.
func TestDashboard_PracticasHoyOnlyCompleted(t *testing.T) {
	db, h := setup(t)
	cat := dbtest.SeedCatalog(t, db)
	admin := dbtest.SeedUser(t, db, cat, "admin", "activo")
	user := dbtest.SeedUser(t, db, cat, "usuario", "activo")
	proc := dbtest.SeedProceso(t, db, cat, 2097, "activo")
	get := func() DashboardResponse {
		req := httptest.NewRequest(http.MethodGet, "/admin/dashboard", nil)
		rr := dbtest.Serve(t, h.handleDashboard, req, admin.ID, "admin")
		if rr.Code != http.StatusOK {
			t.Fatalf("want 200, got %d %s", rr.Code, rr.Body)
		}
		var out DashboardResponse
		_ = json.Unmarshal(rr.Body.Bytes(), &out)
		return out
	}
	before := get()
	for _, e := range []string{"completada", "en_curso", "eliminada", "abandonada"} {
		if _, err := db.Exec(`INSERT INTO practicas (usuario_id, institucion_id, proceso_id, modo_calificacion, estado)
			VALUES ($1,$2,$3,'inmediato',$4)`, user.ID, cat.InstitucionID, proc, e); err != nil {
			t.Fatal(err)
		}
	}
	if d := get().TotalPracticasHoy - before.TotalPracticasHoy; d != 1 {
		t.Fatalf("practicas hoy delta = %d, want 1", d)
	}
}

// H29: every 500 is logged with the operation name.
func TestInternalErrorsAreLogged(t *testing.T) {
	var buf bytes.Buffer
	prev := log.Logger
	log.Logger = zerolog.New(&buf)
	t.Cleanup(func() { log.Logger = prev })

	h := NewHandler(NewRepository(dbtest.Closed(t), nil, nil), nil)
	withID := func(method, body, key, val string) *http.Request {
		r := httptest.NewRequest(method, "/x", bytes.NewBufferString(body))
		r.SetPathValue(key, val)
		return r
	}
	cases := []struct {
		name string
		fn   http.HandlerFunc
		req  *http.Request
	}{
		{"dashboard", h.handleDashboard, httptest.NewRequest(http.MethodGet, "/x", nil)},
		{"actividad", h.handleDashboardActividad, httptest.NewRequest(http.MethodGet, "/x", nil)},
		{"detalle", h.handleGetUsuarioDetalle, withID(http.MethodGet, "", "id", "1")},
		{"rol", h.handlePatchUsuarioRol, withID(http.MethodPatch, `{"rol":"admin"}`, "id", "1")},
		{"audit", h.handleGetAuditLog, httptest.NewRequest(http.MethodGet, "/x", nil)},
		{"ingresos", h.handleGetIngresos, httptest.NewRequest(http.MethodGet, "/x", nil)},
		{"config", h.handleGetIngresosConfig, httptest.NewRequest(http.MethodGet, "/x", nil)},
		{"put_config", h.handlePutIngresosConfig, withID(http.MethodPut, `{"monto":1}`, "anio", "2030")},
	}
	for _, c := range cases {
		buf.Reset()
		rr := dbtest.Serve(t, c.fn, c.req, 1, "admin")
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("%s: want 500, got %d", c.name, rr.Code)
		}
		if out := buf.String(); !strings.Contains(out, `"level":"error"`) || !strings.Contains(out, `"op":"admin.`) {
			t.Fatalf("%s: 500 not logged with op: %q", c.name, out)
		}
	}
}
