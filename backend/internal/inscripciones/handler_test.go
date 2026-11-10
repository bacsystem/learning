package inscripciones

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

const testAnio = 2097 // year reserved for these tests' configuracion_pago rows

func setup(t *testing.T) (*sqlx.DB, *Handler, *Repository) {
	t.Helper()
	db := dbtest.Open(t)
	repo := NewRepository(db, dbtest.GCM(t), nil)
	return db, NewHandler(NewService(repo), audit.New(db)), repo
}

func setConfigPago(t *testing.T, db *sqlx.DB, anio int, monto string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO configuracion_pago (anio, monto) VALUES ($1, $2::numeric)
		ON CONFLICT (anio) DO UPDATE SET monto=EXCLUDED.monto`, anio, monto); err != nil {
		t.Fatalf("config pago: %v", err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM configuracion_pago WHERE anio=$1`, anio) })
}

func seedInscripcion(t *testing.T, db *sqlx.DB, userID, procesoID int64, estado string) int64 {
	t.Helper()
	var id int64
	if err := db.QueryRow(`INSERT INTO inscripciones (usuario_id, proceso_id, estado, monto)
		VALUES ($1, $2, $3, 10) RETURNING id`, userID, procesoID, estado).Scan(&id); err != nil {
		t.Fatalf("seed inscripcion: %v", err)
	}
	return id
}

func patchEstado(t *testing.T, h *Handler, adminID, id int64, estado string) *httptest.ResponseRecorder {
	t.Helper()
	body := fmt.Sprintf(`{"estado":%q,"observacion":"obs"}`, estado)
	req := httptest.NewRequest(http.MethodPatch, "/admin/inscripciones/x/estado", bytes.NewBufferString(body))
	req.SetPathValue("id", fmt.Sprint(id))
	return dbtest.Serve(t, h.UpdateInscripcionEstado, req, adminID, "admin")
}

func estadoOf(t *testing.T, db *sqlx.DB, id int64) string {
	t.Helper()
	var e string
	if err := db.QueryRow(`SELECT estado FROM inscripciones WHERE id=$1`, id).Scan(&e); err != nil {
		t.Fatalf("estado: %v", err)
	}
	return e
}

func auditCount(t *testing.T, db *sqlx.DB, accion string, entidadID int64) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE accion=$1 AND entidad_id=$2`, accion, entidadID).Scan(&n); err != nil {
		t.Fatalf("audit count: %v", err)
	}
	return n
}

// H11: only pendiente → aprobado|rechazado is allowed; decided inscriptions are final.
func TestUpdateInscripcionEstado_TransitionGuard(t *testing.T) {
	db, h, _ := setup(t)
	cat := dbtest.SeedCatalog(t, db)
	admin := dbtest.SeedUser(t, db, cat, "admin", "activo")
	user := dbtest.SeedUser(t, db, cat, "usuario", "activo")
	proc := dbtest.SeedProceso(t, db, cat, testAnio, "activo")
	id := seedInscripcion(t, db, user.ID, proc, "pendiente")

	if rr := patchEstado(t, h, admin.ID, id, "aprobado"); rr.Code != http.StatusOK {
		t.Fatalf("pendiente→aprobado: want 200, got %d %s", rr.Code, rr.Body)
	}
	if got := estadoOf(t, db, id); got != "aprobado" {
		t.Fatalf("estado = %q, want aprobado", got)
	}

	rr := patchEstado(t, h, admin.ID, id, "rechazado")
	if rr.Code != http.StatusConflict {
		t.Fatalf("aprobado→rechazado: want 409, got %d %s", rr.Code, rr.Body)
	}
	if got := estadoOf(t, db, id); got != "aprobado" {
		t.Fatalf("estado changed to %q after rejected transition", got)
	}
	if n := auditCount(t, db, "cambiar_estado_inscripcion", id); n != 1 {
		t.Fatalf("audit rows = %d, want 1 (only the successful transition)", n)
	}

	id2 := seedInscripcion(t, db, admin.ID, proc, "rechazado")
	if rr := patchEstado(t, h, admin.ID, id2, "aprobado"); rr.Code != http.StatusConflict {
		t.Fatalf("rechazado→aprobado: want 409, got %d %s", rr.Code, rr.Body)
	}
}

// The decision and its audit row commit together: if the audit cannot be
// written the inscription stays pendiente.
func TestUpdateInscripcionEstado_AuditFailureRollsBack(t *testing.T) {
	db, h, _ := setup(t)
	cat := dbtest.SeedCatalog(t, db)
	admin := dbtest.SeedUser(t, db, cat, "admin", "activo")
	user := dbtest.SeedUser(t, db, cat, "usuario", "activo")
	proc := dbtest.SeedProceso(t, db, cat, testAnio, "activo")
	id := seedInscripcion(t, db, user.ID, proc, "pendiente")
	audittest.FailInsertsBy(t, db, admin.ID)

	if rr := patchEstado(t, h, admin.ID, id, "aprobado"); rr.Code != http.StatusInternalServerError {
		t.Fatalf("want 500 when the audit fails, got %d %s", rr.Code, rr.Body)
	}
	if got := estadoOf(t, db, id); got != "pendiente" {
		t.Fatalf("estado = %q without audit, want pendiente", got)
	}
	var revisadoPor *int64
	_ = db.QueryRow(`SELECT revisado_por FROM inscripciones WHERE id=$1`, id).Scan(&revisadoPor)
	if revisadoPor != nil {
		t.Fatalf("revisado_por set without audit")
	}
	if n := auditCount(t, db, "cambiar_estado_inscripcion", id); n != 0 {
		t.Fatalf("audit rows = %d, want 0", n)
	}
}

// H11: a vanished id answers 404, not 200.
func TestUpdateInscripcionEstado_NotFound(t *testing.T) {
	db, h, _ := setup(t)
	cat := dbtest.SeedCatalog(t, db)
	admin := dbtest.SeedUser(t, db, cat, "admin", "activo")
	if rr := patchEstado(t, h, admin.ID, 9_000_000_001, "aprobado"); rr.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d %s", rr.Code, rr.Body)
	}
}

// H11: concurrent reviewers — exactly one decision wins.
func TestUpdateInscripcionEstado_ConcurrentSingleWinner(t *testing.T) {
	db, h, _ := setup(t)
	cat := dbtest.SeedCatalog(t, db)
	admin := dbtest.SeedUser(t, db, cat, "admin", "activo")
	user := dbtest.SeedUser(t, db, cat, "usuario", "activo")
	proc := dbtest.SeedProceso(t, db, cat, testAnio, "activo")
	id := seedInscripcion(t, db, user.ID, proc, "pendiente")

	codes := make([]int, 8)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range codes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			estado := "aprobado"
			if i%2 == 1 {
				estado = "rechazado"
			}
			codes[i] = patchEstado(t, h, admin.ID, id, estado).Code
		}(i)
	}
	close(start)
	wg.Wait()
	ok := 0
	for _, c := range codes {
		switch c {
		case http.StatusOK:
			ok++
		case http.StatusConflict:
		default:
			t.Fatalf("unexpected status %d", c)
		}
	}
	if ok != 1 {
		t.Fatalf("successful transitions = %d, want exactly 1 (codes %v)", ok, codes)
	}
}

func inscribir(t *testing.T, h *Handler, userID, procesoID int64) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/inscripciones",
		bytes.NewBufferString(fmt.Sprintf(`{"proceso_id":%d}`, procesoID)))
	return dbtest.Serve(t, h.Inscribir, req, userID, "usuario")
}

// H24: the monto snapshot comes from configuracion_pago of the proceso's year.
func TestInscribir_CapturesMonto(t *testing.T) {
	db, h, _ := setup(t)
	cat := dbtest.SeedCatalog(t, db)
	user := dbtest.SeedUser(t, db, cat, "usuario", "activo")
	proc := dbtest.SeedProceso(t, db, cat, testAnio, "activo")
	setConfigPago(t, db, testAnio, "123.45")

	rr := inscribir(t, h, user.ID, proc)
	if rr.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d %s", rr.Code, rr.Body)
	}
	var monto string
	if err := db.QueryRow(`SELECT monto::text FROM inscripciones WHERE usuario_id=$1 AND proceso_id=$2`,
		user.ID, proc).Scan(&monto); err != nil {
		t.Fatal(err)
	}
	if monto != "123.45" {
		t.Fatalf("monto = %s, want 123.45", monto)
	}
	// H144: a brand-new inscription is a plain INSERT (xmax = 0), not a
	// re-application, so it must not leave a reinscribir_inscripcion audit row.
	var id int64
	if err := db.QueryRow(`SELECT id FROM inscripciones WHERE usuario_id=$1 AND proceso_id=$2`,
		user.ID, proc).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if c := auditCount(t, db, "reinscribir_inscripcion", id); c != 0 {
		t.Fatalf("new inscription logged %d reinscribir_inscripcion audit rows, want 0", c)
	}
}

// H24: a year without configuracion_pago is an explicit error, not monto=0.
func TestInscribir_SinConfigPago(t *testing.T) {
	db, h, _ := setup(t)
	cat := dbtest.SeedCatalog(t, db)
	user := dbtest.SeedUser(t, db, cat, "usuario", "activo")
	proc := dbtest.SeedProceso(t, db, cat, 2098, "activo")
	_, _ = db.Exec(`DELETE FROM configuracion_pago WHERE anio=2098`)

	rr := inscribir(t, h, user.ID, proc)
	if rr.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d %s", rr.Code, rr.Body)
	}
	if !strings.Contains(rr.Body.String(), "monto") {
		t.Fatalf("error message should explain the missing monto: %s", rr.Body)
	}
	var n int
	_ = db.QueryRow(`SELECT COUNT(*) FROM inscripciones WHERE usuario_id=$1`, user.ID).Scan(&n)
	if n != 0 {
		t.Fatalf("inscripcion created without price (%d rows)", n)
	}
}

// H108: a rejected inscription whose proceso's year lost its configuracion_pago
// (the admin moved the proceso to another anio) is not "already inscribed": the
// re-application fails for the missing price and the row stays rechazado.
func TestInscribir_ReapplySinConfigPago(t *testing.T) {
	db, h, _ := setup(t)
	cat := dbtest.SeedCatalog(t, db)
	user := dbtest.SeedUser(t, db, cat, "usuario", "activo")
	proc := dbtest.SeedProceso(t, db, cat, 2098, "activo")
	_, _ = db.Exec(`DELETE FROM configuracion_pago WHERE anio=2098`)
	id := seedInscripcion(t, db, user.ID, proc, "rechazado")

	rr := inscribir(t, h, user.ID, proc)
	if rr.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d %s", rr.Code, rr.Body)
	}
	if !strings.Contains(rr.Body.String(), ErrSinConfigPago.Error()) {
		t.Fatalf("want the missing-monto error, got %s", rr.Body)
	}
	if e := estadoOf(t, db, id); e != "rechazado" {
		t.Fatalf("estado = %s, want rechazado", e)
	}
	if c := auditCount(t, db, "reinscribir_inscripcion", id); c != 0 {
		t.Fatalf("audited a re-application that did not happen")
	}
}

// H24: a failing price lookup must not be swallowed into monto=0.
func TestRepositoryCreate_PropagatesErrors(t *testing.T) {
	repo := NewRepository(dbtest.Closed(t), nil, nil)
	if _, err := repo.Create(context.Background(), 1, 1, nil); err == nil {
		t.Fatal("expected error from a broken DB, got nil")
	}
}

// H52: concurrent duplicate inscriptions → one 201, the rest 409 (never 500).
func TestInscribir_ConcurrentDuplicate(t *testing.T) {
	db, h, _ := setup(t)
	cat := dbtest.SeedCatalog(t, db)
	user := dbtest.SeedUser(t, db, cat, "usuario", "activo")
	proc := dbtest.SeedProceso(t, db, cat, testAnio, "activo")
	setConfigPago(t, db, testAnio, "50.00")

	codes := make([]int, 8)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range codes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			codes[i] = inscribir(t, h, user.ID, proc).Code
		}(i)
	}
	close(start)
	wg.Wait()
	created := 0
	for _, c := range codes {
		switch c {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
		default:
			t.Fatalf("unexpected status %d (codes %v)", c, codes)
		}
	}
	if created != 1 {
		t.Fatalf("created = %d, want 1 (codes %v)", created, codes)
	}
}

// H52: the UNIQUE(usuario, proceso) violation that a racing request hits after
// passing the service pre-check maps to ErrYaInscrito (409), not a raw error.
func TestRepositoryCreate_DuplicateIsYaInscrito(t *testing.T) {
	db, _, repo := setup(t)
	cat := dbtest.SeedCatalog(t, db)
	user := dbtest.SeedUser(t, db, cat, "usuario", "activo")
	proc := dbtest.SeedProceso(t, db, cat, testAnio, "activo")
	setConfigPago(t, db, testAnio, "50.00")
	if _, err := repo.Create(context.Background(), user.ID, proc, nil); err != nil {
		t.Fatalf("first create: %v", err)
	}
	if _, err := repo.Create(context.Background(), user.ID, proc, nil); !errors.Is(err, ErrYaInscrito) {
		t.Fatalf("duplicate create: want ErrYaInscrito, got %v", err)
	}
}

// H53: admin list is paginated and the page size is capped.
func TestListInscripciones_Paginated(t *testing.T) {
	db, h, _ := setup(t)
	cat := dbtest.SeedCatalog(t, db)
	admin := dbtest.SeedUser(t, db, cat, "admin", "activo")
	u1 := dbtest.SeedUser(t, db, cat, "usuario", "activo")
	u2 := dbtest.SeedUser(t, db, cat, "usuario", "activo")
	proc := dbtest.SeedProceso(t, db, cat, testAnio, "activo")
	seedInscripcion(t, db, u1.ID, proc, "pendiente")
	seedInscripcion(t, db, u2.ID, proc, "pendiente")

	list := func(q string) []InscripcionAdminDTO {
		req := httptest.NewRequest(http.MethodGet, "/admin/inscripciones?"+q, nil)
		rr := dbtest.Serve(t, h.ListInscripciones, req, admin.ID, "admin")
		if rr.Code != http.StatusOK {
			t.Fatalf("%s: want 200, got %d %s", q, rr.Code, rr.Body)
		}
		var out []InscripcionAdminDTO
		if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	if got := list("estado=pendiente&per_page=1"); len(got) != 1 {
		t.Fatalf("per_page=1 returned %d rows", len(got))
	}
	p1 := list("estado=pendiente&per_page=1&page=1")
	p2 := list("estado=pendiente&per_page=1&page=2")
	if len(p1) != 1 || len(p2) != 1 || p1[0].ID == p2[0].ID {
		t.Fatalf("pagination broken: %v / %v", p1, p2)
	}
	_ = list("per_page=100000&page=999999999999") // must not overflow → 200
	// H86: a page beyond int range is capped like elsewhere (an empty last
	// page), not silently turned into page 1.
	if got := list("estado=pendiente&per_page=1&page=99999999999999999999"); len(got) != 0 {
		t.Fatalf("huge page returned %d rows (treated as page 1)", len(got))
	}
	if maxListLimit > 500 {
		t.Fatalf("maxListLimit = %d, want a sane cap", maxListLimit)
	}
}

// The admin list says who reviewed a decided inscription and when; pendientes
// carry neither field.
func TestListInscripciones_RevisadoInfo(t *testing.T) {
	db, h, _ := setup(t)
	cat := dbtest.SeedCatalog(t, db)
	admin := dbtest.SeedUser(t, db, cat, "admin", "activo")
	u1 := dbtest.SeedUser(t, db, cat, "usuario", "activo")
	u2 := dbtest.SeedUser(t, db, cat, "usuario", "activo")
	proc := dbtest.SeedProceso(t, db, cat, testAnio, "activo")
	decided := seedInscripcion(t, db, u1.ID, proc, "pendiente")
	pending := seedInscripcion(t, db, u2.ID, proc, "pendiente")
	if rr := patchEstado(t, h, admin.ID, decided, "aprobado"); rr.Code != http.StatusOK {
		t.Fatalf("approve: %d %s", rr.Code, rr.Body)
	}
	var username string
	if err := db.QueryRow(`SELECT username FROM usuarios WHERE id=$1`, admin.ID).Scan(&username); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/inscripciones?per_page=500", nil)
	rr := dbtest.Serve(t, h.ListInscripciones, req, admin.ID, "admin")
	var out []map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, row := range out {
		switch int64(row["id"].(float64)) {
		case decided:
			seen++
			if row["revisado_por"] != username || row["revisado_at"] == nil || row["revisado_at"] == "" {
				t.Fatalf("decided row lacks revisado info: %v", row)
			}
		case pending:
			seen++
			if _, ok := row["revisado_por"]; ok {
				t.Fatalf("pendiente row has revisado_por: %v", row)
			}
			if _, ok := row["revisado_at"]; ok {
				t.Fatalf("pendiente row has revisado_at: %v", row)
			}
		}
	}
	if seen != 2 {
		t.Fatalf("rows not listed (%d of 2)", seen)
	}
}

// H54: undecryptable PII becomes "" (never ciphertext) in the admin list.
func TestListInscripciones_DecryptFailureIsEmpty(t *testing.T) {
	db, h, _ := setup(t)
	cat := dbtest.SeedCatalog(t, db)
	admin := dbtest.SeedUser(t, db, cat, "admin", "activo")
	user := dbtest.SeedUser(t, db, cat, "usuario", "activo")
	proc := dbtest.SeedProceso(t, db, cat, testAnio, "activo")
	seedInscripcion(t, db, user.ID, proc, "pendiente")
	if _, err := db.Exec(`UPDATE personas SET nombres='not-a-ciphertext' WHERE id=$1`, user.PersonaID); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/admin/inscripciones?estado=pendiente", nil)
	rr := dbtest.Serve(t, h.ListInscripciones, req, admin.ID, "admin")
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	if strings.Contains(rr.Body.String(), "not-a-ciphertext") {
		t.Fatal("ciphertext leaked into the response")
	}
	var list []InscripcionAdminDTO
	_ = json.Unmarshal(rr.Body.Bytes(), &list)
	found := false
	for _, it := range list {
		if it.UsuarioID == user.ID {
			found = true
			if !strings.HasPrefix(it.NombreCompleto, " Paterno") {
				t.Fatalf("nombre_completo = %q, want empty nombres + decrypted apellidos", it.NombreCompleto)
			}
		}
	}
	if !found {
		t.Fatal("seeded inscripcion not listed")
	}
}

// H29: every 500 is logged with the operation name.
func TestInternalErrorsAreLogged(t *testing.T) {
	var buf bytes.Buffer
	prev := log.Logger
	log.Logger = zerolog.New(&buf)
	t.Cleanup(func() { log.Logger = prev })

	h := NewHandler(NewService(NewRepository(dbtest.Closed(t), nil, nil)), nil)
	cases := []struct {
		name string
		fn   http.HandlerFunc
		req  *http.Request
	}{
		{"stats", h.GetStatsPublicos, httptest.NewRequest(http.MethodGet, "/stats/publico", nil)},
		{"procesos_publico", h.ListProcesosPublico, httptest.NewRequest(http.MethodGet, "/procesos/publico", nil)},
		{"procesos", h.ListProcesos, httptest.NewRequest(http.MethodGet, "/procesos", nil)},
		{"inscribir", h.Inscribir, httptest.NewRequest(http.MethodPost, "/inscripciones", bytes.NewBufferString(`{"proceso_id":1}`))},
		{"mis", h.MisInscripciones, httptest.NewRequest(http.MethodGet, "/inscripciones/me", nil)},
		{"list_admin", h.ListInscripciones, httptest.NewRequest(http.MethodGet, "/admin/inscripciones", nil)},
		{"patch", h.UpdateInscripcionEstado, func() *http.Request {
			r := httptest.NewRequest(http.MethodPatch, "/admin/inscripciones/1/estado", bytes.NewBufferString(`{"estado":"aprobado"}`))
			r.SetPathValue("id", "1")
			return r
		}()},
	}
	for _, c := range cases {
		buf.Reset()
		rr := dbtest.Serve(t, c.fn, c.req, 1, "admin")
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("%s: want 500, got %d", c.name, rr.Code)
		}
		out := buf.String()
		if !strings.Contains(out, `"level":"error"`) || !strings.Contains(out, `"op":"inscripciones.`) {
			t.Fatalf("%s: 500 not logged with op: %q", c.name, out)
		}
	}
}

// H71: a rejected inscription can be re-applied: the same row goes back to
// pendiente at the current price with the previous review cleared, audited.
func TestInscribir_ReapplyAfterRechazo(t *testing.T) {
	db, h, _ := setup(t)
	cat := dbtest.SeedCatalog(t, db)
	admin := dbtest.SeedUser(t, db, cat, "admin", "activo")
	user := dbtest.SeedUser(t, db, cat, "usuario", "activo")
	proc := dbtest.SeedProceso(t, db, cat, testAnio, "activo")
	setConfigPago(t, db, testAnio, "77.50")
	id := seedInscripcion(t, db, user.ID, proc, "pendiente")
	if rr := patchEstado(t, h, admin.ID, id, "rechazado"); rr.Code != http.StatusOK {
		t.Fatalf("reject: %d %s", rr.Code, rr.Body)
	}

	rr := inscribir(t, h, user.ID, proc)
	if rr.Code != http.StatusCreated {
		t.Fatalf("re-apply after rechazo: want 201, got %d %s", rr.Code, rr.Body)
	}
	var got struct {
		ID     int64  `json:"id"`
		Estado string `json:"estado"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.ID != id || got.Estado != "pendiente" {
		t.Fatalf("response %+v, want id %d pendiente", got, id)
	}
	var (
		n                      int
		estado, monto          string
		obs                    *string
		revisadoPor            *int64
		revisadoAtNull, recent bool
	)
	if err := db.QueryRow(`SELECT COUNT(*) FROM inscripciones WHERE usuario_id=$1 AND proceso_id=$2`, user.ID, proc).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT estado, monto::text, observacion, revisado_por, revisado_at IS NULL,
		created_at > NOW() - INTERVAL '1 minute' FROM inscripciones WHERE id=$1`, id).
		Scan(&estado, &monto, &obs, &revisadoPor, &revisadoAtNull, &recent); err != nil {
		t.Fatal(err)
	}
	if n != 1 || estado != "pendiente" || monto != "77.50" || obs != nil || revisadoPor != nil || !revisadoAtNull || !recent {
		t.Fatalf("after re-apply: rows=%d estado=%s monto=%s obs=%v revisado_por=%v revisado_at_null=%v recent=%v",
			n, estado, monto, obs, revisadoPor, revisadoAtNull, recent)
	}
	if c := auditCount(t, db, "reinscribir_inscripcion", id); c != 1 {
		t.Fatalf("re-application audited %d times, want 1", c)
	}
	// The re-applied inscription can be decided again.
	if rr := patchEstado(t, h, admin.ID, id, "aprobado"); rr.Code != http.StatusOK {
		t.Fatalf("approve after re-apply: %d %s", rr.Code, rr.Body)
	}
}

// H71: pendiente and aprobado inscriptions still answer 409 and are untouched.
func TestInscribir_ReapplyNotAllowedUnlessRechazado(t *testing.T) {
	db, h, _ := setup(t)
	cat := dbtest.SeedCatalog(t, db)
	setConfigPago(t, db, testAnio, "77.50")
	for _, estado := range []string{"pendiente", "aprobado"} {
		user := dbtest.SeedUser(t, db, cat, "usuario", "activo")
		proc := dbtest.SeedProceso(t, db, cat, testAnio, "activo")
		id := seedInscripcion(t, db, user.ID, proc, estado)
		rr := inscribir(t, h, user.ID, proc)
		if rr.Code != http.StatusConflict {
			t.Fatalf("%s: want 409, got %d %s", estado, rr.Code, rr.Body)
		}
		var monto string
		if err := db.QueryRow(`SELECT monto::text FROM inscripciones WHERE id=$1`, id).Scan(&monto); err != nil {
			t.Fatal(err)
		}
		if estadoOf(t, db, id) != estado || monto != "10.00" {
			t.Fatalf("%s inscription modified: estado=%s monto=%s", estado, estadoOf(t, db, id), monto)
		}
		if c := auditCount(t, db, "reinscribir_inscripcion", id); c != 0 {
			t.Fatalf("%s: audited a re-application that did not happen", estado)
		}
	}
}

// H71: concurrent re-applications of a rejected inscription → one 201, the
// rest 409, still one row.
func TestInscribir_ConcurrentReapply(t *testing.T) {
	db, h, _ := setup(t)
	cat := dbtest.SeedCatalog(t, db)
	user := dbtest.SeedUser(t, db, cat, "usuario", "activo")
	proc := dbtest.SeedProceso(t, db, cat, testAnio, "activo")
	setConfigPago(t, db, testAnio, "50.00")
	id := seedInscripcion(t, db, user.ID, proc, "rechazado")

	codes := make([]int, 8)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range codes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			codes[i] = inscribir(t, h, user.ID, proc).Code
		}(i)
	}
	close(start)
	wg.Wait()
	created := 0
	for _, c := range codes {
		switch c {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
		default:
			t.Fatalf("unexpected status %d (codes %v)", c, codes)
		}
	}
	var n int
	_ = db.QueryRow(`SELECT COUNT(*) FROM inscripciones WHERE usuario_id=$1 AND proceso_id=$2`, user.ID, proc).Scan(&n)
	if created != 1 || n != 1 || estadoOf(t, db, id) != "pendiente" {
		t.Fatalf("created=%d rows=%d estado=%s (codes %v)", created, n, estadoOf(t, db, id), codes)
	}
	if c := auditCount(t, db, "reinscribir_inscripcion", id); c != 1 {
		t.Fatalf("re-application audited %d times, want 1", c)
	}
}
