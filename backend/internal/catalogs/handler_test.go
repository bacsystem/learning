package catalogs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cbaciliod/learning/internal/audit"
	"github.com/cbaciliod/learning/internal/audit/audittest"
	"github.com/cbaciliod/learning/internal/db/dbtest"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type env struct {
	db    *sqlx.DB
	h     *Handler
	cat   dbtest.Catalog
	admin dbtest.User
}

func setup(t *testing.T) env {
	t.Helper()
	db := dbtest.Open(t)
	cat := dbtest.SeedCatalog(t, db)
	admin := dbtest.SeedUser(t, db, cat, "admin", "activo")
	return env{db: db, h: NewHandler(NewService(NewRepository(db)), audit.New(db)), cat: cat, admin: admin}
}

// call runs a handler as the seeded admin; pv are path values (key, value, ...).
func (e env) call(t *testing.T, fn http.HandlerFunc, method, body string, pv ...string) *httptest.ResponseRecorder {
	t.Helper()
	var rd *bytes.Buffer
	if body != "" {
		rd = bytes.NewBufferString(body)
	} else {
		rd = &bytes.Buffer{}
	}
	req := httptest.NewRequest(method, "/x", rd)
	for i := 0; i+1 < len(pv); i += 2 {
		req.SetPathValue(pv[i], pv[i+1])
	}
	return dbtest.Serve(t, fn, req, e.admin.ID, "admin")
}

func (e env) get(t *testing.T, fn http.HandlerFunc, query string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/x?"+query, nil)
	return dbtest.Serve(t, fn, req, e.admin.ID, "admin")
}

func want(t *testing.T, rr *httptest.ResponseRecorder, code int, what string) {
	t.Helper()
	if rr.Code != code {
		t.Fatalf("%s: want %d, got %d %s", what, code, rr.Code, rr.Body)
	}
}

func (e env) assign(t *testing.T, proc, asig int64, peso string) {
	t.Helper()
	if _, err := e.db.Exec(`INSERT INTO proceso_asignaturas (proceso_id, asignatura_id, numero_preguntas, peso_porcentual)
		VALUES ($1,$2,10,$3::numeric)`, proc, asig, peso); err != nil {
		t.Fatal(err)
	}
}

func (e env) sum(t *testing.T, proc int64) string {
	t.Helper()
	var s string
	_ = e.db.QueryRow(`SELECT COALESCE(SUM(peso_porcentual),0)::text FROM proceso_asignaturas WHERE proceso_id=$1`, proc).Scan(&s)
	return s
}

// H26: changes that leave an ACTIVE proceso's weights ≠ 100 are rejected (409)
// and rolled back; inactive procesos can be edited freely.
func TestPesos_ProcesoActivo(t *testing.T) {
	e := setup(t)
	a1, a2, a3 := dbtest.SeedAsignatura(t, e.db), dbtest.SeedAsignatura(t, e.db), dbtest.SeedAsignatura(t, e.db)
	activo := dbtest.SeedProceso(t, e.db, e.cat, 2097, "activo")
	e.assign(t, activo, a1, "70")
	e.assign(t, activo, a2, "30")
	pid, s1, s2 := fmt.Sprint(activo), fmt.Sprint(a1), fmt.Sprint(a2)

	rr := e.call(t, e.h.DeleteProcesoAsignatura, http.MethodDelete, "", "id", pid, "asignaturaId", s2)
	want(t, rr, http.StatusConflict, "delete leaving 70")
	rr = e.call(t, e.h.UpdateProcesoAsignatura, http.MethodPut,
		`{"numero_preguntas":10,"peso_porcentual":50,"coeficiente":1}`, "id", pid, "asignaturaId", s1)
	want(t, rr, http.StatusConflict, "update leaving 80")
	rr = e.call(t, e.h.AsignarAsignatura, http.MethodPost,
		fmt.Sprintf(`{"asignatura_id":%d,"numero_preguntas":5,"peso_porcentual":10,"coeficiente":1}`, a3), "id", pid)
	want(t, rr, http.StatusConflict, "assign leaving 110")
	if got := e.sum(t, activo); got != "100.00" {
		t.Fatalf("rejected changes were not rolled back: sum = %s", got)
	}
	// a change that keeps 100 is fine
	rr = e.call(t, e.h.UpdateProcesoAsignatura, http.MethodPut,
		`{"numero_preguntas":20,"peso_porcentual":70,"coeficiente":1.5}`, "id", pid, "asignaturaId", s1)
	want(t, rr, http.StatusOK, "update keeping 100")

	inactivo := dbtest.SeedProceso(t, e.db, e.cat, 2097, "inactivo")
	rr = e.call(t, e.h.AsignarAsignatura, http.MethodPost,
		fmt.Sprintf(`{"asignatura_id":%d,"numero_preguntas":5,"peso_porcentual":40,"coeficiente":1}`, a1), "id", fmt.Sprint(inactivo))
	want(t, rr, http.StatusCreated, "assign on inactive proceso")
	// activation still requires 100
	rr = e.call(t, e.h.UpdateProcesoEstado, http.MethodPatch, `{"estado":"activo"}`, "id", fmt.Sprint(inactivo))
	want(t, rr, http.StatusBadRequest, "activate with 40")
}

// H26: coeficiente must be > 0 (and fit DECIMAL(5,2)).
func TestCoeficienteValidation(t *testing.T) {
	e := setup(t)
	a1 := dbtest.SeedAsignatura(t, e.db)
	proc := dbtest.SeedProceso(t, e.db, e.cat, 2097, "inactivo")
	for _, c := range []string{"0", "-1", "1000"} {
		rr := e.call(t, e.h.AsignarAsignatura, http.MethodPost,
			fmt.Sprintf(`{"asignatura_id":%d,"numero_preguntas":5,"peso_porcentual":40,"coeficiente":%s}`, a1, c), "id", fmt.Sprint(proc))
		want(t, rr, http.StatusUnprocessableEntity, "assign coeficiente="+c)
	}
	e.assign(t, proc, a1, "40")
	rr := e.call(t, e.h.UpdateProcesoAsignatura, http.MethodPut,
		`{"numero_preguntas":10,"peso_porcentual":40,"coeficiente":0}`, "id", fmt.Sprint(proc), "asignaturaId", fmt.Sprint(a1))
	want(t, rr, http.StatusUnprocessableEntity, "update coeficiente=0")
}

// H55: dangling references → 422 (not FK 500); unknown proceso → 404.
func TestInvalidReferences(t *testing.T) {
	e := setup(t)
	const ghost = 9_000_000_005
	rr := e.call(t, e.h.CreateGrado, http.MethodPost,
		fmt.Sprintf(`{"institucion_id":%d,"nombre":"Grado X","abreviatura":"GX","nivel":"oficial","orden":3}`, ghost))
	want(t, rr, http.StatusUnprocessableEntity, "grado with unknown institucion")
	rr = e.call(t, e.h.CreateEspecialidad, http.MethodPost,
		fmt.Sprintf(`{"institucion_id":%d,"nombre":"Esp X","codigo":"EX"}`, ghost))
	want(t, rr, http.StatusUnprocessableEntity, "especialidad with unknown institucion")
	rr = e.call(t, e.h.CreateProceso, http.MethodPost,
		fmt.Sprintf(`{"institucion_id":%d,"nombre":"Proc X","tipo":"ascenso","anio":2030}`, ghost))
	want(t, rr, http.StatusUnprocessableEntity, "proceso with unknown institucion")
	proc := dbtest.SeedProceso(t, e.db, e.cat, 2097, "inactivo")
	rr = e.call(t, e.h.AsignarAsignatura, http.MethodPost,
		fmt.Sprintf(`{"asignatura_id":%d,"numero_preguntas":5,"peso_porcentual":40,"coeficiente":1}`, ghost), "id", fmt.Sprint(proc))
	want(t, rr, http.StatusUnprocessableEntity, "assign unknown asignatura")
	a1 := dbtest.SeedAsignatura(t, e.db)
	rr = e.call(t, e.h.AsignarAsignatura, http.MethodPost,
		fmt.Sprintf(`{"asignatura_id":%d,"numero_preguntas":5,"peso_porcentual":40,"coeficiente":1}`, a1), "id", fmt.Sprint(ghost))
	want(t, rr, http.StatusNotFound, "assign to unknown proceso")
}

// H55: ValidateSO3Grado maps only "not found" to ErrNotFound; other DB errors
// propagate (→ 500), they are not disguised as 404.
func TestValidateSO3Grado_DBErrorIsNotNotFound(t *testing.T) {
	repo := NewRepository(dbtest.Closed(t))
	id := int64(1)
	err := ValidateSO3Grado(context.Background(), repo, "pase_efectividad", &id)
	if err == nil || err == ErrNotFound {
		t.Fatalf("want a propagated DB error, got %v", err)
	}
}

// H27: every catalog mutation is audited with the acting admin.
func TestMutationsAreAudited(t *testing.T) {
	e := setup(t)
	auditRows := func(accion string, id int64) int {
		var n int
		_ = e.db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE usuario_id=$1 AND accion=$2 AND entidad_id=$3`,
			e.admin.ID, accion, id).Scan(&n)
		return n
	}
	idOf := func(rr *httptest.ResponseRecorder) int64 {
		var v struct {
			ID int64 `json:"id"`
		}
		_ = json.Unmarshal(rr.Body.Bytes(), &v)
		return v.ID
	}
	u := dbtest.Uniq()

	rr := e.call(t, e.h.CreateInstitucion, http.MethodPost, fmt.Sprintf(`{"nombre":"Inst audit %s","siglas":"IA%s"}`, u, u))
	want(t, rr, http.StatusCreated, "create institucion")
	inst := idOf(rr)
	t.Cleanup(func() { _, _ = e.db.Exec(`DELETE FROM instituciones WHERE id=$1`, inst) })
	want(t, e.call(t, e.h.UpdateInstitucion, http.MethodPut, fmt.Sprintf(`{"nombre":"Inst audit2 %s","siglas":"IB%s"}`, u, u), "id", fmt.Sprint(inst)), http.StatusOK, "update institucion")
	want(t, e.call(t, e.h.UpdateInstitucionEstado, http.MethodPatch, `{"estado":"inactivo"}`, "id", fmt.Sprint(inst)), http.StatusOK, "estado institucion")

	rr = e.call(t, e.h.CreateGrado, http.MethodPost, fmt.Sprintf(`{"institucion_id":%d,"nombre":"Grado %s","abreviatura":"GA","nivel":"oficial","orden":7}`, e.cat.InstitucionID, u))
	want(t, rr, http.StatusCreated, "create grado")
	grado := idOf(rr)
	want(t, e.call(t, e.h.UpdateGrado, http.MethodPut, fmt.Sprintf(`{"institucion_id":%d,"nombre":"Grado2 %s","abreviatura":"GB","nivel":"oficial","orden":7}`, e.cat.InstitucionID, u), "id", fmt.Sprint(grado)), http.StatusOK, "update grado")
	want(t, e.call(t, e.h.UpdateGradoEstado, http.MethodPatch, `{"estado":"inactivo"}`, "id", fmt.Sprint(grado)), http.StatusOK, "estado grado")

	rr = e.call(t, e.h.CreateEspecialidad, http.MethodPost, fmt.Sprintf(`{"institucion_id":%d,"nombre":"Esp %s","codigo":"EA%s"}`, e.cat.InstitucionID, u, u))
	want(t, rr, http.StatusCreated, "create especialidad")
	esp := idOf(rr)
	want(t, e.call(t, e.h.UpdateEspecialidad, http.MethodPut, fmt.Sprintf(`{"institucion_id":%d,"nombre":"Esp2 %s","codigo":"EB%s"}`, e.cat.InstitucionID, u, u), "id", fmt.Sprint(esp)), http.StatusOK, "update especialidad")
	want(t, e.call(t, e.h.UpdateEspecialidadEstado, http.MethodPatch, `{"estado":"inactivo"}`, "id", fmt.Sprint(esp)), http.StatusOK, "estado especialidad")

	rr = e.call(t, e.h.CreateAsignatura, http.MethodPost, fmt.Sprintf(`{"nombre":"Asig %s","codigo":"AA%s"}`, u, u))
	want(t, rr, http.StatusCreated, "create asignatura")
	asig := idOf(rr)
	t.Cleanup(func() {
		_, _ = e.db.Exec(`DELETE FROM proceso_asignaturas WHERE asignatura_id=$1`, asig)
		_, _ = e.db.Exec(`DELETE FROM asignaturas WHERE id=$1`, asig)
	})
	want(t, e.call(t, e.h.UpdateAsignatura, http.MethodPut, fmt.Sprintf(`{"nombre":"Asig2 %s","codigo":"AB%s"}`, u, u), "id", fmt.Sprint(asig)), http.StatusOK, "update asignatura")
	want(t, e.call(t, e.h.UpdateAsignaturaEstado, http.MethodPatch, `{"estado":"inactivo"}`, "id", fmt.Sprint(asig)), http.StatusOK, "estado asignatura")

	rr = e.call(t, e.h.CreateProceso, http.MethodPost, fmt.Sprintf(`{"institucion_id":%d,"nombre":"Proc %s","tipo":"ascenso","anio":2030}`, e.cat.InstitucionID, u))
	want(t, rr, http.StatusCreated, "create proceso")
	proc := idOf(rr)
	pid := fmt.Sprint(proc)
	want(t, e.call(t, e.h.UpdateProceso, http.MethodPut, fmt.Sprintf(`{"institucion_id":%d,"nombre":"Proc2 %s","tipo":"ascenso","anio":2030}`, e.cat.InstitucionID, u), "id", pid), http.StatusOK, "update proceso")
	want(t, e.call(t, e.h.UpdateProcesoEstado, http.MethodPatch, `{"estado":"inactivo"}`, "id", pid), http.StatusOK, "estado proceso")
	want(t, e.call(t, e.h.AsignarAsignatura, http.MethodPost, fmt.Sprintf(`{"asignatura_id":%d,"numero_preguntas":5,"peso_porcentual":100,"coeficiente":1}`, asig), "id", pid), http.StatusCreated, "asignar")
	want(t, e.call(t, e.h.UpdateProcesoAsignatura, http.MethodPut, `{"numero_preguntas":6,"peso_porcentual":100,"coeficiente":2}`, "id", pid, "asignaturaId", fmt.Sprint(asig)), http.StatusOK, "update asignacion")
	want(t, e.call(t, e.h.DeleteProcesoAsignatura, http.MethodDelete, "", "id", pid, "asignaturaId", fmt.Sprint(asig)), http.StatusOK, "delete asignacion")

	for _, c := range []struct {
		accion string
		id     int64
	}{
		{"crear_institucion", inst}, {"actualizar_institucion", inst}, {"cambiar_estado_institucion", inst},
		{"crear_grado", grado}, {"actualizar_grado", grado}, {"cambiar_estado_grado", grado},
		{"crear_especialidad", esp}, {"actualizar_especialidad", esp}, {"cambiar_estado_especialidad", esp},
		{"crear_asignatura", asig}, {"actualizar_asignatura", asig}, {"cambiar_estado_asignatura", asig},
		{"crear_proceso", proc}, {"actualizar_proceso", proc}, {"cambiar_estado_proceso", proc},
		{"asignar_asignatura_proceso", proc}, {"actualizar_asignatura_proceso", proc}, {"quitar_asignatura_proceso", proc},
	} {
		if n := auditRows(c.accion, c.id); n != 1 {
			t.Errorf("audit %s(%d): %d rows, want 1", c.accion, c.id, n)
		}
	}

	// failed mutations are not audited
	want(t, e.call(t, e.h.UpdateGradoEstado, http.MethodPatch, `{"estado":"activo"}`, "id", "9000000006"), http.StatusNotFound, "estado unknown grado")
	if n := auditRows("cambiar_estado_grado", 9_000_000_006); n != 0 {
		t.Fatalf("failed mutation audited")
	}
}

// H53/H50: limit is validated and capped, page is honoured.
func TestListQuery(t *testing.T) {
	e := setup(t)
	for i := 0; i < 3; i++ {
		dbtest.SeedAsignatura(t, e.db)
	}
	want(t, e.get(t, e.h.ListAsignaturas, "limit=abc"), http.StatusBadRequest, "limit=abc")
	want(t, e.get(t, e.h.ListAsignaturas, "institucion_id=x"), http.StatusBadRequest, "institucion_id=x")
	list := func(q string) []AsignaturaResponse {
		rr := e.get(t, e.h.ListAsignaturas, q)
		want(t, rr, http.StatusOK, q)
		var out []AsignaturaResponse
		_ = json.Unmarshal(rr.Body.Bytes(), &out)
		return out
	}
	p1, p2 := list("limit=1&page=1"), list("limit=1&page=2")
	if len(p1) != 1 || len(p2) != 1 || p1[0].ID == p2[0].ID {
		t.Fatalf("page not honoured: %v / %v", p1, p2)
	}
	if q, err := parseListQuery(httptest.NewRequest(http.MethodGet, "/x?limit=1000000&page=99999999999", nil)); err != nil || q.Limit != maxLimit || q.Page > maxPage {
		t.Fatalf("limit/page not capped: %+v %v", q, err)
	}
	if len(list("limit=all")) < 3 {
		t.Fatal("limit=all must still list the catalog")
	}
}

// The catalog mutations that run in a transaction write their audit row in it:
// a failed audit rolls the change back, a successful change writes one row.
func TestCatalogTxMutations_AuditInSameTx(t *testing.T) {
	e := setup(t)
	a1, a2 := dbtest.SeedAsignatura(t, e.db), dbtest.SeedAsignatura(t, e.db)
	proc := dbtest.SeedProceso(t, e.db, e.cat, 2097, "inactivo")
	pid, s1, s2 := fmt.Sprint(proc), fmt.Sprint(a1), fmt.Sprint(a2)
	inst := fmt.Sprint(e.cat.InstitucionID)
	assignBody := func(asig int64, peso int) string {
		return fmt.Sprintf(`{"asignatura_id":%d,"numero_preguntas":5,"peso_porcentual":%d,"coeficiente":1}`, asig, peso)
	}
	updBody := func(peso int) string {
		return fmt.Sprintf(`{"numero_preguntas":7,"peso_porcentual":%d,"coeficiente":1}`, peso)
	}
	var n int
	countPA := func() int {
		_ = e.db.QueryRow(`SELECT COUNT(*) FROM proceso_asignaturas WHERE proceso_id=$1`, proc).Scan(&n)
		return n
	}
	estado := func(table string, id int64) string {
		var s string
		_ = e.db.QueryRow(`SELECT estado FROM `+table+` WHERE id=$1`, id).Scan(&s)
		return s
	}

	stop := audittest.FailInsertsBy(t, e.db, e.admin.ID)
	want(t, e.call(t, e.h.AsignarAsignatura, http.MethodPost, assignBody(a1, 100), "id", pid), http.StatusInternalServerError, "assign, audit fails")
	if countPA() != 0 {
		t.Fatal("assignment kept without audit")
	}
	e.assign(t, proc, a1, "100")
	want(t, e.call(t, e.h.UpdateProcesoAsignatura, http.MethodPut, updBody(100), "id", pid, "asignaturaId", s1), http.StatusInternalServerError, "update, audit fails")
	var np int
	_ = e.db.QueryRow(`SELECT numero_preguntas FROM proceso_asignaturas WHERE proceso_id=$1 AND asignatura_id=$2`, proc, a1).Scan(&np)
	if np != 10 {
		t.Fatalf("assignment updated without audit: numero_preguntas=%d", np)
	}
	want(t, e.call(t, e.h.DeleteProcesoAsignatura, http.MethodDelete, "", "id", pid, "asignaturaId", s1), http.StatusInternalServerError, "delete, audit fails")
	if countPA() != 1 {
		t.Fatal("assignment deleted without audit")
	}
	want(t, e.call(t, e.h.UpdateProcesoEstado, http.MethodPatch, `{"estado":"activo"}`, "id", pid), http.StatusInternalServerError, "activate, audit fails")
	if got := estado("procesos", proc); got != "inactivo" {
		t.Fatalf("proceso activated without audit: %s", got)
	}
	want(t, e.call(t, e.h.UpdateInstitucionEstado, http.MethodPatch, `{"estado":"inactivo"}`, "id", inst), http.StatusInternalServerError, "deactivate institucion, audit fails")
	if got := estado("instituciones", e.cat.InstitucionID); got != "activo" {
		t.Fatalf("institucion deactivated without audit: %s", got)
	}
	if got := estado("grados", e.cat.GradoID); got != "activo" {
		t.Fatalf("cascade applied without audit: grado %s", got)
	}
	for _, accion := range []string{"asignar_asignatura_proceso", "actualizar_asignatura_proceso",
		"quitar_asignatura_proceso", "cambiar_estado_proceso"} {
		if c := audittest.Count(t, e.db, accion, proc); c != 0 {
			t.Fatalf("%s: %d audit rows after rollback", accion, c)
		}
	}
	stop()

	want(t, e.call(t, e.h.UpdateProcesoAsignatura, http.MethodPut, updBody(60), "id", pid, "asignaturaId", s1), http.StatusOK, "update")
	want(t, e.call(t, e.h.AsignarAsignatura, http.MethodPost, assignBody(a2, 40), "id", pid), http.StatusCreated, "assign")
	want(t, e.call(t, e.h.DeleteProcesoAsignatura, http.MethodDelete, "", "id", pid, "asignaturaId", s2), http.StatusOK, "delete")
	want(t, e.call(t, e.h.UpdateProcesoAsignatura, http.MethodPut, updBody(100), "id", pid, "asignaturaId", s1), http.StatusOK, "update to 100")
	want(t, e.call(t, e.h.UpdateProcesoEstado, http.MethodPatch, `{"estado":"activo"}`, "id", pid), http.StatusOK, "activate")
	want(t, e.call(t, e.h.UpdateInstitucionEstado, http.MethodPatch, `{"estado":"inactivo"}`, "id", inst), http.StatusOK, "deactivate institucion")
	for accion, wantN := range map[string]int{"asignar_asignatura_proceso": 1, "actualizar_asignatura_proceso": 2,
		"quitar_asignatura_proceso": 1, "cambiar_estado_proceso": 1} {
		if c := audittest.Count(t, e.db, accion, proc); c != wantN {
			t.Fatalf("%s: %d audit rows, want %d", accion, c, wantN)
		}
	}
	if c := audittest.Count(t, e.db, "cambiar_estado_institucion", e.cat.InstitucionID); c != 1 {
		t.Fatalf("cambiar_estado_institucion: %d audit rows, want 1", c)
	}
}

// H29: 500s are logged with the operation name.
func TestInternalErrorsAreLogged(t *testing.T) {
	var buf bytes.Buffer
	prev := log.Logger
	log.Logger = zerolog.New(&buf)
	t.Cleanup(func() { log.Logger = prev })

	h := NewHandler(NewService(NewRepository(dbtest.Closed(t))), nil)
	cases := []struct {
		name string
		fn   http.HandlerFunc
	}{
		{"instituciones", h.ListInstituciones}, {"grados", h.ListGrados}, {"especialidades", h.ListEspecialidades},
		{"asignaturas", h.ListAsignaturas}, {"procesos", h.ListProcesos},
	}
	for _, c := range cases {
		buf.Reset()
		rr := dbtest.Serve(t, c.fn, httptest.NewRequest(http.MethodGet, "/x", nil), 1, "admin")
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("%s: want 500, got %d", c.name, rr.Code)
		}
		if out := buf.String(); !strings.Contains(out, `"level":"error"`) || !strings.Contains(out, `"op":"catalogs.`) {
			t.Fatalf("%s: 500 not logged with op: %q", c.name, out)
		}
	}
	// the underlying cause is logged, not replaced by a generic sentinel
	for _, fn := range []http.HandlerFunc{h.UpdateGradoEstado, h.UpdateEspecialidadEstado} {
		buf.Reset()
		req := httptest.NewRequest(http.MethodPatch, "/x", bytes.NewBufferString(`{"estado":"inactivo"}`))
		req.SetPathValue("id", "1")
		if rr := dbtest.Serve(t, fn, req, 1, "admin"); rr.Code != http.StatusInternalServerError {
			t.Fatalf("want 500, got %d", rr.Code)
		}
		if !strings.Contains(buf.String(), "database is closed") {
			t.Fatalf("root cause not logged: %q", buf.String())
		}
	}
}
