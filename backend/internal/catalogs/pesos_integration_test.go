package catalogs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/cbaciliod/learning/internal/db/dbtest"
	"github.com/cbaciliod/learning/internal/middleware"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

func pqInt64s(ids []int64) any { return pq.Array(ids) }

func sortStrings(s []string) { sort.Strings(s) }

// pesosEnv is the real catalogs mux over the test DB with one proceso of the
// seeded institucion, two active especialidades (Medicina, Enfermería), three
// asignaturas and a compatibility configuration a1 60 % + a2 40 %.
type pesosEnv struct {
	db                   *sqlx.DB
	mux                  *http.ServeMux
	svc                  *Service
	cat                  dbtest.Catalog
	adminID              int64
	admin, usuario       string
	proceso              int64
	medicina, enfermeria int64
	a1, a2, a3           int64
}

func nuevoPesosEnv(t *testing.T) *pesosEnv {
	t.Helper()
	db := dbtest.Open(t)
	e := &pesosEnv{db: db, mux: http.NewServeMux(), cat: dbtest.SeedCatalog(t, db)}
	e.svc = NewService(NewRepository(db))
	Mount(e.mux, e.svc, dbtest.JWTSecret, dbtest.SessionChecker(t, db))
	admin := dbtest.SeedUser(t, db, e.cat, "admin", "activo")
	usuario := dbtest.SeedUser(t, db, e.cat, "usuario", "activo")
	e.adminID = admin.ID
	e.admin = dbtest.RealBearer(t, db, admin.ID, "admin")
	e.usuario = dbtest.RealBearer(t, db, usuario.ID, "usuario")
	e.proceso = dbtest.SeedProceso(t, db, e.cat, 2030, "inactivo")
	e.medicina = e.cat.EspecialidadID
	e.enfermeria = e.especialidad(t, e.cat.InstitucionID, "activo")
	e.a1, e.a2, e.a3 = dbtest.SeedAsignatura(t, db), dbtest.SeedAsignatura(t, db), dbtest.SeedAsignatura(t, db)
	e.exec(t, `INSERT INTO proceso_asignaturas (proceso_id, asignatura_id, numero_preguntas, peso_porcentual, coeficiente)
		VALUES ($1, $2, 30, 60, 1), ($1, $3, 20, 40, 1)`, e.proceso, e.a1, e.a2)
	return e
}

func (e *pesosEnv) exec(t *testing.T, q string, args ...any) {
	t.Helper()
	if _, err := e.db.Exec(q, args...); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
}

func (e *pesosEnv) especialidad(t *testing.T, institucionID int64, estado string) int64 {
	t.Helper()
	var id int64
	u := dbtest.Uniq()
	if err := e.db.QueryRow(`INSERT INTO especialidades (institucion_id, nombre, codigo, estado)
		VALUES ($1, $2, $3, $4) RETURNING id`, institucionID, "Esp "+u, "P"+u, estado).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func (e *pesosEnv) do(t *testing.T, method, path, bearer, body string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", bearer)
	}
	rr := httptest.NewRecorder()
	e.mux.ServeHTTP(rr, req)
	var out map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	return rr.Code, out
}

func (e *pesosEnv) pathPesos(esp int64) string {
	return fmt.Sprintf("/admin/procesos/%d/pesos/%d", e.proceso, esp)
}

func cuerpo(areas ...AreaPesoRequest) string {
	b, _ := json.Marshal(GuardarPesosRequest{Areas: areas})
	return string(b)
}

func (e *pesosEnv) guardar(t *testing.T, esp int64, areas ...AreaPesoRequest) map[string]any {
	t.Helper()
	status, body := e.do(t, "PUT", e.pathPesos(esp), e.admin, cuerpo(areas...))
	if status != http.StatusOK {
		t.Fatalf("guardar pesos: %d %v", status, body)
	}
	return body
}

// filas returns every proceso_asignaturas row of the proceso as
// "esp|asig|peso|preguntas|coef" (esp 0 = compatibility), in a stable order.
func (e *pesosEnv) filas(t *testing.T) []string {
	t.Helper()
	var out []string
	if err := e.db.Select(&out, `
		SELECT COALESCE(especialidad_id, 0) || '|' || asignatura_id || '|' || peso_porcentual || '|' || numero_preguntas || '|' || coeficiente
		FROM proceso_asignaturas WHERE proceso_id = $1
		ORDER BY COALESCE(especialidad_id, 0), asignatura_id`, e.proceso); err != nil {
		t.Fatal(err)
	}
	return out
}

func (e *pesosEnv) auditorias(t *testing.T) []map[string]any {
	t.Helper()
	var raw [][]byte
	if err := e.db.Select(&raw, `SELECT detalle FROM audit_log
		WHERE accion = 'pesos.guardar' AND entidad = 'proceso' AND entidad_id = $1 AND usuario_id = $2 ORDER BY id`,
		e.proceso, e.adminID); err != nil {
		t.Fatal(err)
	}
	out := make([]map[string]any, len(raw))
	for i, r := range raw {
		if err := json.Unmarshal(r, &out[i]); err != nil {
			t.Fatal(err)
		}
	}
	return out
}

func codigoPesos(body map[string]any) string { s, _ := body["error"].(string); return s }

func asignaturasDe(body map[string]any, campo string) []int64 {
	var ids []int64
	for _, item := range body[campo].([]any) {
		ids = append(ids, int64(item.(map[string]any)["asignatura_id"].(float64)))
	}
	return ids
}

// ── PE3 ───────────────────────────────────────────────────────────────────────

// Escenarios «Configuración completa» y «Pesos distintos por especialidad».
func TestGuardarPesos_Completa(t *testing.T) {
	e := nuevoPesosEnv(t)
	compat := e.filas(t)

	body := e.guardar(t, e.medicina, area(e.a1, 33.33, 40), area(e.a3, 33.33, 40), area(e.a2, 33.34, 40))
	if body["estado"] != "completa" || body["suma_pesos"] != 100.0 || body["suma_preguntas"] != 120.0 || len(body["plantilla"].([]any)) != 0 {
		t.Fatalf("configuración = %v", body)
	}
	if len(body["areas"].([]any)) != 3 || body["especialidad_id"] != float64(e.medicina) {
		t.Errorf("areas = %v", body["areas"])
	}
	auds := e.auditorias(t)
	if len(auds) != 1 || len(auds[0]["antes"].([]any)) != 0 || len(auds[0]["despues"].([]any)) != 3 || auds[0]["especialidad_id"] != float64(e.medicina) {
		t.Errorf("audit = %v", auds)
	}

	e.guardar(t, e.enfermeria, area(e.a2, 100, 50))
	if _, med := e.do(t, "GET", e.pathPesos(e.medicina), e.admin, ``); fmt.Sprint(asignaturasDe(med, "areas")) == fmt.Sprint([]int64{e.a2}) || len(med["areas"].([]any)) != 3 {
		t.Errorf("Enfermería changed Medicina: %v", med["areas"])
	}
	_, enf := e.do(t, "GET", e.pathPesos(e.enfermeria), e.admin, ``)
	if fmt.Sprint(asignaturasDe(enf, "areas")) != fmt.Sprint([]int64{e.a2}) {
		t.Errorf("Enfermería = %v", enf["areas"])
	}
	// Compatibility rows are never touched (regla 2).
	for _, f := range compat {
		if !strings.Contains(strings.Join(e.filas(t), " "), f) {
			t.Errorf("compatibility row %s changed: %v", f, e.filas(t))
		}
	}
}

// Escenarios «Suma inválida» (sin y con configuración previa), «Exceso de
// preguntas», «Unicidad» y el orden de validación de PE3: nada se guarda ni
// se audita.
func TestGuardarPesos_Rechazos(t *testing.T) {
	e := nuevoPesosEnv(t)
	otraInst := dbtest.SeedCatalog(t, e.db)
	ajena := otraInst.EspecialidadID
	inactiva := e.especialidad(t, e.cat.InstitucionID, "inactivo")

	type caso struct {
		name   string
		path   string
		body   string
		status int
		code   string
	}
	casos := func() []caso {
		return []caso{
			{"suma 95", e.pathPesos(e.medicina), cuerpo(area(e.a1, 50, 10), area(e.a2, 45, 10)), 422, "pesos_no_suman_100"},
			{"210 preguntas", e.pathPesos(e.medicina), cuerpo(area(e.a1, 50, 110), area(e.a2, 50, 100)), 422, "limite_preguntas"},
			{"área repetida", e.pathPesos(e.medicina), cuerpo(area(e.a1, 50, 10), area(e.a1, 50, 10)), 409, "peso_duplicado"},
			{"sin áreas", e.pathPesos(e.medicina), `{"areas":[]}`, 422, "validacion"},
			{"campo desconocido", e.pathPesos(e.medicina), `{"areas":[],"x":1}`, 400, "cuerpo_invalido"},
			{"asignatura inexistente", e.pathPesos(e.medicina), cuerpo(area(999999999, 100, 10)), 422, "referencia_invalida"},
			{"especialidad de otra institución", e.pathPesos(ajena), cuerpo(area(e.a1, 100, 10)), 422, "referencia_invalida"},
			{"especialidad inactiva", e.pathPesos(inactiva), cuerpo(area(e.a1, 100, 10)), 422, "referencia_invalida"},
			{"especialidad inexistente", e.pathPesos(999999999), cuerpo(area(e.a1, 100, 10)), 422, "referencia_invalida"},
			{"proceso inexistente", "/admin/procesos/999999999/pesos/1", cuerpo(area(e.a1, 100, 10)), 404, "no_encontrado"},
			{"id no numérico", "/admin/procesos/x/pesos/1", cuerpo(area(e.a1, 100, 10)), 400, "parametro_invalido"},
			// Two broken rules at once: the first in the contract's order wins.
			{"validación antes que referencia", e.pathPesos(999999999), cuerpo(area(e.a1, 100, 0)), 422, "validacion"},
			{"referencia antes que repetida", e.pathPesos(e.medicina), cuerpo(area(999999999, 50, 10), area(999999999, 50, 10)), 422, "referencia_invalida"},
			{"repetida antes que preguntas y suma", e.pathPesos(e.medicina), cuerpo(area(e.a1, 10, 110), area(e.a1, 10, 100)), 409, "peso_duplicado"},
			{"preguntas antes que suma", e.pathPesos(e.medicina), cuerpo(area(e.a1, 10, 110), area(e.a2, 10, 100)), 422, "limite_preguntas"},
		}
	}
	for _, previa := range []string{"sin configuración previa", "con configuración completa previa"} {
		t.Run(previa, func(t *testing.T) {
			if previa != "sin configuración previa" {
				e.guardar(t, e.medicina, area(e.a1, 70, 10), area(e.a3, 30, 10))
			}
			antes, audits := e.filas(t), len(e.auditorias(t))
			for _, c := range casos() {
				t.Run(c.name, func(t *testing.T) {
					if status, body := e.do(t, "PUT", c.path, e.admin, c.body); status != c.status || codigoPesos(body) != c.code {
						t.Errorf("got %d %v, want %d %s", status, body, c.status, c.code)
					}
				})
			}
			if despues := e.filas(t); fmt.Sprint(despues) != fmt.Sprint(antes) {
				t.Errorf("rows changed:\n before %v\n after  %v", antes, despues)
			}
			if n := len(e.auditorias(t)); n != audits {
				t.Errorf("%d new audit rows", n-audits)
			}
			_, conf := e.do(t, "GET", e.pathPesos(e.medicina), e.admin, ``)
			want := "incompleta"
			if previa != "sin configuración previa" {
				want = "completa"
			}
			if conf["estado"] != want {
				t.Errorf("estado = %v, want %s", conf["estado"], want)
			}
		})
	}
}

// Escenario «Reemplazo de la configuración»: the omitted area leaves the set.
func TestGuardarPesos_Reemplazo(t *testing.T) {
	e := nuevoPesosEnv(t)
	e.guardar(t, e.medicina, area(e.a1, 50, 10), area(e.a2, 25, 10), area(e.a3, 25, 10))
	body := e.guardar(t, e.medicina, area(e.a1, 40, 12), area(e.a3, 60, 8))
	if fmt.Sprint(asignaturasDe(body, "areas")) != fmt.Sprint(sortedIDs(e.db, t, e.a1, e.a3)) {
		t.Errorf("areas = %v", body["areas"])
	}
	var filasMed int
	if err := e.db.Get(&filasMed, `SELECT COUNT(*) FROM proceso_asignaturas WHERE proceso_id = $1 AND especialidad_id = $2`, e.proceso, e.medicina); err != nil {
		t.Fatal(err)
	}
	if filasMed != 2 {
		t.Errorf("%d Medicina rows, want 2", filasMed)
	}
	auds := e.auditorias(t)
	if len(auds) != 2 || len(auds[1]["antes"].([]any)) != 3 || len(auds[1]["despues"].([]any)) != 2 {
		t.Fatalf("audit = %v", auds)
	}
	primera := auds[1]["despues"].([]any)[0].(map[string]any)
	if primera["asignatura_id"] != float64(e.a1) || primera["peso_porcentual"] != 40.0 || primera["numero_preguntas"] != 12.0 {
		t.Errorf("audit despues[0] = %v", primera)
	}
}

// sortedIDs returns ids ordered like the API (asignatura nombre, then id).
func sortedIDs(db *sqlx.DB, t *testing.T, ids ...int64) []int64 {
	t.Helper()
	var out []int64
	if err := db.Select(&out, `SELECT id FROM asignaturas WHERE id = ANY($1) ORDER BY nombre, id`, pqInt64s(ids)); err != nil {
		t.Fatal(err)
	}
	return out
}

// PE3 runs under lockProceso: a save waits while another transaction holds
// the proceso row, and then applies on top of what that one committed.
func TestGuardarPesos_EsperaElBloqueoDelProceso(t *testing.T) {
	e := nuevoPesosEnv(t)
	tx, err := e.db.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	pid := dbtest.BackendPID(t, tx)
	if _, err := tx.Exec(`SELECT 1 FROM procesos WHERE id = $1 FOR UPDATE`, e.proceso); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(`INSERT INTO proceso_asignaturas (proceso_id, especialidad_id, asignatura_id, numero_preguntas, peso_porcentual)
		VALUES ($1, $2, $3, 5, 100)`, e.proceso, e.medicina, e.a3); err != nil {
		t.Fatal(err)
	}

	resultado := make(chan int, 1)
	go func() {
		req := httptest.NewRequest("PUT", e.pathPesos(e.medicina), strings.NewReader(cuerpo(area(e.a1, 100, 10))))
		req.Header.Set("Authorization", e.admin)
		rr := httptest.NewRecorder()
		e.mux.ServeHTTP(rr, req)
		resultado <- rr.Code
	}()
	espero := dbtest.BlockedBy(t, e.db, pid, resultado)
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	status := <-resultado
	if !espero {
		t.Errorf("the save did not wait for the proceso lock (answered %d)", status)
	}
	// After waiting it sees the committed a3 row and replaces it.
	if auds := e.auditorias(t); status != http.StatusOK || len(auds) != 1 || len(auds[0]["antes"].([]any)) != 1 {
		t.Errorf("status %d, audit %v; want 200 with antes = the committed row", status, auds)
	}
}

// ── PE1, PE2 ──────────────────────────────────────────────────────────────────

func TestResumenYConfiguracionPesos(t *testing.T) {
	e := nuevoPesosEnv(t)
	e.especialidad(t, e.cat.InstitucionID, "inactivo") // never listed
	e.guardar(t, e.medicina, area(e.a1, 70, 10), area(e.a3, 30, 15))

	status, res := e.do(t, "GET", fmt.Sprintf("/admin/procesos/%d/pesos", e.proceso), e.admin, ``)
	if status != http.StatusOK {
		t.Fatalf("PE1: %d %v", status, res)
	}
	compat := res["compatibilidad"].(map[string]any)
	if compat["areas"] != 2.0 || compat["suma_pesos"] != 100.0 {
		t.Errorf("compatibilidad = %v", compat)
	}
	esps := res["especialidades"].([]any)
	if len(esps) != 2 {
		t.Fatalf("especialidades = %v (only the 2 active ones)", esps)
	}
	for _, item := range esps {
		m := item.(map[string]any)
		switch int64(m["especialidad_id"].(float64)) {
		case e.medicina:
			if m["estado"] != "completa" || m["areas"] != 2.0 || m["suma_pesos"] != 100.0 || m["suma_preguntas"] != 25.0 {
				t.Errorf("Medicina = %v", m)
			}
		case e.enfermeria:
			if m["estado"] != "incompleta" || m["areas"] != 0.0 || m["suma_pesos"] != 0.0 {
				t.Errorf("Enfermería = %v", m)
			}
		default:
			t.Errorf("unexpected especialidad %v", m)
		}
	}

	// Escenario «Pantalla sin configuración»: incompleta, no areas, the
	// compatibility rows as template.
	_, enf := e.do(t, "GET", e.pathPesos(e.enfermeria), e.admin, ``)
	if enf["estado"] != "incompleta" || len(enf["areas"].([]any)) != 0 || enf["suma_pesos"] != 0.0 {
		t.Errorf("Enfermería = %v", enf)
	}
	if fmt.Sprint(asignaturasDe(enf, "plantilla")) != fmt.Sprint(sortedIDs(e.db, t, e.a1, e.a2)) {
		t.Errorf("plantilla = %v", enf["plantilla"])
	}

	if _, body := e.do(t, "GET", e.pathPesos(999999999), e.admin, ``); body["message"] != "La especialidad no existe, está inactiva o es de otra institución." {
		t.Errorf("referencia_invalida message = %v", body["message"])
	}
	for _, c := range []struct {
		path, code string
		status     int
	}{
		{"/admin/procesos/999999999/pesos", "no_encontrado", 404},
		{"/admin/procesos/999999999/pesos/1", "no_encontrado", 404},
		{e.pathPesos(dbtest.SeedCatalog(t, e.db).EspecialidadID), "referencia_invalida", 422},
		{"/admin/procesos/0/pesos", "parametro_invalido", 400},
	} {
		if status, body := e.do(t, "GET", c.path, e.admin, ``); status != c.status || codigoPesos(body) != c.code {
			t.Errorf("%s: %d %v, want %d %s", c.path, status, body, c.status, c.code)
		}
	}
}

// ── Unicidad (regla 7, ADR 0001) ──────────────────────────────────────────────

// The constraint rejects a second row of either kind, while a compatibility
// row and an especialidad row of the same asignatura coexist.
func TestUnicidadProcesoAsignaturas(t *testing.T) {
	e := nuevoPesosEnv(t)
	insertar := func(esp *int64) error {
		_, err := e.db.Exec(`INSERT INTO proceso_asignaturas (proceso_id, especialidad_id, asignatura_id, numero_preguntas, peso_porcentual)
			VALUES ($1, $2, $3, 1, 1)`, e.proceso, esp, e.a1)
		return err
	}
	if err := insertar(&e.medicina); err != nil {
		t.Fatalf("an especialidad row next to the compatibility one: %v", err)
	}
	for name, esp := range map[string]*int64{"compatibilidad (NULL)": nil, "especialidad": &e.medicina} {
		if err := insertar(esp); !errors.Is(mapearPesoDuplicado(err), ErrPesoDuplicado) {
			t.Errorf("second %s row: err = %v, want a violation of %s", name, err, constraintPesos)
		}
	}
}

// Escenario «Unicidad de la fila de compatibilidad» (PE5): the ON CONFLICT of
// the legacy endpoint uses the NULLS NOT DISTINCT constraint as arbiter, so a
// second compatibility row answers 409 peso_duplicado and is not inserted.
func TestAsignarAsignatura_CompatibilidadDuplicada(t *testing.T) {
	e := nuevoPesosEnv(t)
	e.exec(t, `INSERT INTO proceso_asignaturas (proceso_id, especialidad_id, asignatura_id, numero_preguntas, peso_porcentual)
		VALUES ($1, $2, $3, 5, 100)`, e.proceso, e.medicina, e.a3)
	path := fmt.Sprintf("/admin/procesos/%d/asignaturas", e.proceso)
	body := fmt.Sprintf(`{"asignatura_id":%d,"numero_preguntas":5,"peso_porcentual":10,"coeficiente":1}`, e.a3)

	// An especialidad row of a3 does not block its compatibility row.
	if status, b := e.do(t, "POST", path, e.admin, body); status != http.StatusCreated {
		t.Fatalf("first compatibility row: %d %v", status, b)
	}
	status, b := e.do(t, "POST", path, e.admin, body)
	if status != http.StatusConflict || codigoPesos(b) != "peso_duplicado" || b["message"] != ErrPesoDuplicado.Error() {
		t.Errorf("second compatibility row: %d %v, want 409 peso_duplicado", status, b)
	}
	var compat int
	if err := e.db.Get(&compat, `SELECT COUNT(*) FROM proceso_asignaturas
		WHERE proceso_id = $1 AND asignatura_id = $2 AND especialidad_id IS NULL`, e.proceso, e.a3); err != nil {
		t.Fatal(err)
	}
	if compat != 1 {
		t.Errorf("%d compatibility rows of a3, want 1", compat)
	}
}

// ── L1–L5: the legacy endpoints only see and touch compatibility rows ─────────

func TestCompatibilidad_EndpointsExistentes(t *testing.T) {
	e := nuevoPesosEnv(t)
	// Medicina's set differs from compatibility in areas (a3, no a2) and values.
	e.guardar(t, e.medicina, area(e.a1, 25, 7), area(e.a3, 75, 9))
	medicina := e.filasDeEspecialidad(t)

	// L1: activation sums only the compatibility rows (60 + 40), not 200.
	if status, b := e.do(t, "PATCH", fmt.Sprintf("/admin/procesos/%d/estado", e.proceso), e.admin, `{"estado":"activo"}`); status != http.StatusOK {
		t.Fatalf("activar: %d %v", status, b)
	}
	e.exec(t, `UPDATE procesos SET estado = 'inactivo' WHERE id = $1`, e.proceso)

	// L2 admin: only compatibility, even for an admin whose token carries Medicina.
	adminMedicina := dbtest.RealBearerEspecialidad(t, e.db, e.adminID, "admin", e.medicina)
	for _, path := range []string{fmt.Sprintf("/admin/procesos/%d/asignaturas", e.proceso), fmt.Sprintf("/admin/procesos/%d", e.proceso)} {
		got := e.listado(t, path, adminMedicina)
		if fmt.Sprint(got) != fmt.Sprint(e.compatibilidad(t)) {
			t.Errorf("%s = %v, want the compatibility rows %v", path, got, e.compatibilidad(t))
		}
	}

	// L4: editing a1 changes its compatibility row only.
	if status, b := e.do(t, "PUT", fmt.Sprintf("/admin/procesos/%d/asignaturas/%d", e.proceso, e.a1), e.admin,
		`{"numero_preguntas":31,"peso_porcentual":60,"coeficiente":2}`); status != http.StatusOK {
		t.Fatalf("editar a1: %d %v", status, b)
	}
	// L5: deleting a1 removes its compatibility row only.
	if status, b := e.do(t, "DELETE", fmt.Sprintf("/admin/procesos/%d/asignaturas/%d", e.proceso, e.a1), e.admin, ``); status != http.StatusOK {
		t.Fatalf("borrar a1: %d %v", status, b)
	}
	if got := e.filasDeEspecialidad(t); fmt.Sprint(got) != fmt.Sprint(medicina) {
		t.Errorf("Medicina rows changed by the legacy endpoints:\n before %v\n after  %v", medicina, got)
	}
	// A compatibility row that does not exist (a3 only has a Medicina row) is 404.
	if status, _ := e.do(t, "DELETE", fmt.Sprintf("/admin/procesos/%d/asignaturas/%d", e.proceso, e.a3), e.admin, ``); status != http.StatusNotFound {
		t.Errorf("borrar a3 de compatibilidad: %d, want 404", status)
	}
}

// L2 user route: the user's especialidad set if the proceso has one, the
// compatibility rows otherwise; sets are never mixed.
func TestListProcesoAsignaturas_ResuelvePorConjunto(t *testing.T) {
	e := nuevoPesosEnv(t)
	e.guardar(t, e.medicina, area(e.a1, 25, 7), area(e.a3, 75, 9))
	path := fmt.Sprintf("/procesos/%d/asignaturas", e.proceso)
	usuarioID := dbtest.SeedUser(t, e.db, e.cat, "usuario", "activo").ID

	cases := []struct {
		name string
		esp  int64
		want []string
	}{
		{"Medicina tiene conjunto propio", e.medicina, e.especialidadFilas(t)},
		{"Enfermería sin conjunto usa compatibilidad", e.enfermeria, e.compatibilidad(t)},
		{"sin especialidad usa compatibilidad", 0, e.compatibilidad(t)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := e.listado(t, path, dbtest.RealBearerEspecialidad(t, e.db, usuarioID, "usuario", c.esp))
			if fmt.Sprint(got) != fmt.Sprint(c.want) {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

// listado returns "asig|peso|preguntas" of a listing endpoint, by asignatura id.
func (e *pesosEnv) listado(t *testing.T, path, bearer string) []string {
	t.Helper()
	req := httptest.NewRequest("GET", path, nil)
	req.Header.Set("Authorization", bearer)
	rr := httptest.NewRecorder()
	e.mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("%s: %d %s", path, rr.Code, rr.Body)
	}
	var items []ProcesoAsignaturaItem
	if strings.HasPrefix(strings.TrimSpace(rr.Body.String()), "{") {
		var p ProcesoDetailResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		items = p.Asignaturas
	} else if err := json.Unmarshal(rr.Body.Bytes(), &items); err != nil {
		t.Fatal(err)
	}
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, fmt.Sprintf("%d|%.2f|%d", it.AsignaturaID, it.PesoPorcentual, it.NumeroPreguntas))
	}
	sortStrings(out)
	return out
}

func (e *pesosEnv) compatibilidad(t *testing.T) []string { return e.conjunto(t, nil) }

func (e *pesosEnv) especialidadFilas(t *testing.T) []string { return e.conjunto(t, &e.medicina) }

func (e *pesosEnv) conjunto(t *testing.T, esp *int64) []string {
	t.Helper()
	var out []string
	if err := e.db.Select(&out, `SELECT asignatura_id || '|' || to_char(peso_porcentual, 'FM990.00') || '|' || numero_preguntas
		FROM proceso_asignaturas WHERE proceso_id = $1 AND especialidad_id IS NOT DISTINCT FROM $2::bigint`, e.proceso, esp); err != nil {
		t.Fatal(err)
	}
	sortStrings(out)
	return out
}

func (e *pesosEnv) filasDeEspecialidad(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, f := range e.filas(t) {
		if !strings.HasPrefix(f, "0|") {
			out = append(out, f)
		}
	}
	return out
}

// ── PE6 ───────────────────────────────────────────────────────────────────────

// Escenarios «Sin configuración» y «Simulacro oficial sin configuración».
func TestServicioPesos(t *testing.T) {
	e := nuevoPesosEnv(t)
	ctx := context.Background()
	t.Cleanup(func() { _, _ = e.db.Exec(`DELETE FROM preguntas WHERE proceso_id = $1`, e.proceso) })
	pregunta := func(asig int64, esp *int64, estado string) {
		e.exec(t, `INSERT INTO preguntas (institucion_id, proceso_id, asignatura_id, especialidad_id, enunciado, estado)
			VALUES ($1, $2, $3, $4, 'Pregunta de pesos', $5)`, e.cat.InstitucionID, e.proceso, asig, esp, estado)
	}
	pregunta(e.a1, nil, "activo")            // for every especialidad
	pregunta(e.a2, &e.enfermeria, "activo")  // Enfermería only
	pregunta(e.a3, &e.medicina, "eliminada") // not servable

	// Without a set: equal weights among the areas with servable questions.
	p, err := e.svc.Pesos(ctx, e.proceso, e.enfermeria, ModoEstimable)
	if err != nil || !p.PesosEstimados || len(p.Areas) != 2 || p.Areas[0].PesoPorcentual != 50 || p.Areas[1].PesoPorcentual != 50 {
		t.Errorf("Enfermería estimable = %+v, %v", p, err)
	}
	p, err = e.svc.Pesos(ctx, e.proceso, e.medicina, ModoEstimable)
	if err != nil || !p.PesosEstimados || len(p.Areas) != 1 || p.Areas[0].AsignaturaID != e.a1 || p.Areas[0].PesoPorcentual != 100 {
		t.Errorf("Medicina estimable = %+v, %v", p, err)
	}
	if _, err := e.svc.Pesos(ctx, e.proceso, e.medicina, ModoOficial); !errors.Is(err, ErrFormatoNoConfigurado) {
		t.Errorf("Medicina oficial: err = %v, want ErrFormatoNoConfigurado", err)
	}
	sinPreguntas := e.especialidad(t, e.cat.InstitucionID, "activo")
	e.exec(t, `DELETE FROM preguntas WHERE proceso_id = $1 AND especialidad_id IS NULL`, e.proceso)
	if p, err := e.svc.Pesos(ctx, e.proceso, sinPreguntas, ModoEstimable); err != nil || !p.PesosEstimados || len(p.Areas) != 0 {
		t.Errorf("sin preguntas = %+v, %v", p, err)
	}

	// With a set: its rows, not estimated, in both modes.
	e.guardar(t, e.medicina, area(e.a1, 25, 7), area(e.a3, 75, 9))
	for _, modo := range []ModoPesos{ModoEstimable, ModoOficial} {
		p, err := e.svc.Pesos(ctx, e.proceso, e.medicina, modo)
		if err != nil || p.PesosEstimados || len(p.Areas) != 2 {
			t.Fatalf("modo %d = %+v, %v", modo, p, err)
		}
		for _, a := range p.Areas {
			if a.NumeroPreguntas == nil || (a.AsignaturaID == e.a1 && (a.PesoPorcentual != 25 || *a.NumeroPreguntas != 7)) {
				t.Errorf("modo %d area = %+v", modo, a)
			}
		}
	}
}

// README §3 (R3): a PE3 body without Content-Length passes the middleware,
// which wraps it in a MaxBytesReader; going past the limit while decoding
// answers 413 cuerpo_demasiado_grande and saves nothing. With Content-Length
// the middleware itself answers 413 with its legacy text.
func TestGuardarPesos_CuerpoDemasiadoGrande(t *testing.T) {
	e := nuevoPesosEnv(t)
	const limite = 256
	h := middleware.MaxBodySize(limite, limite)(e.mux)
	areas := make([]AreaPesoRequest, 0, 50)
	for i := range 50 {
		areas = append(areas, area(e.a1+int64(i), 2, 1))
	}
	grande := cuerpo(areas...)
	antes := e.filas(t)

	cases := []struct {
		name          string
		contentLength bool
		code          string
	}{
		{"chunked: lo detecta el handler", false, "cuerpo_demasiado_grande"},
		{"con Content-Length: lo corta el middleware", true, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest("PUT", e.pathPesos(e.medicina), strings.NewReader(grande))
			if !c.contentLength {
				req.ContentLength = -1
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", e.admin)
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			var body map[string]any
			_ = json.Unmarshal(rr.Body.Bytes(), &body)
			if rr.Code != http.StatusRequestEntityTooLarge {
				t.Fatalf("got %d %v, want 413", rr.Code, body)
			}
			if c.code != "" && (body["error"] != c.code || body["message"] == "") {
				t.Errorf("body = %v, want error %s with message", body, c.code)
			}
			if c.code == "" && body["message"] != nil {
				t.Errorf("middleware 413 = %v, want the legacy {error: text}", body)
			}
		})
	}
	if fmt.Sprint(e.filas(t)) != fmt.Sprint(antes) || len(e.auditorias(t)) != 0 {
		t.Error("an oversized body saved or audited something")
	}
}
