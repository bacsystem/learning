package perfiles

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cbaciliod/learning/internal/catalogs"
	"github.com/cbaciliod/learning/internal/db/dbtest"
	"github.com/jmoiron/sqlx"
)

// entorno: the real mux and session checker over the test DB, an institucion
// with Enfermería (the persona's especialidad), Obstetricia and an inactive
// especialidad, one proceso and a user with an approved inscripción.
type entorno struct {
	db                                *sqlx.DB
	mux                               *http.ServeMux
	cat                               dbtest.Catalog
	proceso                           int64
	enfermeria, obstetricia, inactiva int64
	usuario                           dbtest.User
	token                             string
}

func nuevoEntorno(t *testing.T) *entorno {
	t.Helper()
	db := dbtest.Open(t)
	e := &entorno{db: db, mux: http.NewServeMux(), cat: dbtest.SeedCatalog(t, db)}
	Mount(e.mux, NewService(NewRepository(db), catalogs.NewService(catalogs.NewRepository(db))), dbtest.JWTSecret, dbtest.SessionChecker(t, db))
	e.enfermeria = e.cat.EspecialidadID
	e.obstetricia = e.especialidad(t, e.cat.InstitucionID, "activo")
	e.inactiva = e.especialidad(t, e.cat.InstitucionID, "inactivo")
	e.proceso = dbtest.SeedProceso(t, db, e.cat, 2030, "activo")
	e.usuario = dbtest.SeedUser(t, db, e.cat, "usuario", "activo")
	e.inscribir(t, e.usuario.ID, e.proceso, "aprobado")
	e.token = dbtest.RealBearer(t, db, e.usuario.ID, "usuario")
	return e
}

func (e *entorno) especialidad(t *testing.T, institucionID int64, estado string) int64 {
	t.Helper()
	var id int64
	u := dbtest.Uniq()
	if err := e.db.QueryRow(`INSERT INTO especialidades (institucion_id, nombre, codigo, estado)
		VALUES ($1, $2, $3, $4) RETURNING id`, institucionID, "Esp "+u, "F"+u, estado).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func (e *entorno) inscribir(t *testing.T, usuarioID, procesoID int64, estado string) {
	t.Helper()
	if _, err := e.db.Exec(`INSERT INTO inscripciones (usuario_id, proceso_id, estado) VALUES ($1, $2, $3)`,
		usuarioID, procesoID, estado); err != nil {
		t.Fatal(err)
	}
}

func (e *entorno) do(t *testing.T, method, path, bearer, body string) (int, map[string]any) {
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

func (e *entorno) elegir(t *testing.T, bearer string, procesoID, especialidadID int64, confirmar bool) (int, map[string]any) {
	t.Helper()
	return e.do(t, "PUT", "/serums-ia/profesion", bearer,
		fmt.Sprintf(`{"proceso_id":%d,"especialidad_id":%d,"confirmar_reinicio":%t}`, procesoID, especialidadID, confirmar))
}

// perfiles returns "especialidad|estado|archivado" of the user's rows, oldest first.
func (e *entorno) perfiles(t *testing.T, usuarioID int64) []string {
	t.Helper()
	var out []string
	if err := e.db.Select(&out, `SELECT especialidad_id || '|' || estado || '|' || (archivado_at IS NOT NULL)
		FROM postulante_profesiones WHERE usuario_id = $1 ORDER BY id`, usuarioID); err != nil {
		t.Fatal(err)
	}
	return out
}

func (e *entorno) especialidadPersona(t *testing.T) int64 {
	t.Helper()
	var id int64
	if err := e.db.Get(&id, `SELECT especialidad_id FROM personas WHERE id = $1`, e.usuario.PersonaID); err != nil {
		t.Fatal(err)
	}
	return id
}

func codigo(body map[string]any) string { s, _ := body["error"].(string); return s }

func nid(v any) int64 { f, _ := v.(float64); return int64(f) }

// Escenarios «Preselección desde los datos de persona» y «Sin preselección
// válida», más la marca de pesos de cada especialidad (regla 2).
func TestOpciones(t *testing.T) {
	e := nuevoEntorno(t)
	asig := dbtest.SeedAsignatura(t, e.db)
	if _, err := e.db.Exec(`INSERT INTO proceso_asignaturas (proceso_id, especialidad_id, asignatura_id, numero_preguntas, peso_porcentual)
		VALUES ($1, $2, $3, 10, 100)`, e.proceso, e.obstetricia, asig); err != nil {
		t.Fatal(err)
	}

	status, body := e.do(t, "GET", "/serums-ia/profesion/opciones", e.token, ``)
	if status != http.StatusOK {
		t.Fatalf("F1: %d %v", status, body)
	}
	procesos := body["procesos"].([]any)
	if len(procesos) != 1 || nid(procesos[0].(map[string]any)["id"]) != e.proceso {
		t.Fatalf("procesos = %v", procesos)
	}
	pesos := map[int64]any{}
	for _, item := range procesos[0].(map[string]any)["especialidades"].([]any) {
		m := item.(map[string]any)
		pesos[nid(m["id"])] = m["pesos"]
	}
	if len(pesos) != 2 || pesos[e.obstetricia] != "completa" || pesos[e.enfermeria] != "estimada" {
		t.Errorf("especialidades = %v (the inactive one must not appear)", pesos)
	}
	pre := body["preseleccion"].(map[string]any)
	if nid(pre["especialidad_id"]) != e.enfermeria || nid(pre["proceso_id"]) != e.proceso {
		t.Errorf("preseleccion = %v, want Enfermería in the only proceso", pre)
	}
	if n := len(e.perfiles(t, e.usuario.ID)); n != 0 {
		t.Errorf("F1 stored %d rows", n)
	}

	// The persona's especialidad is read from the DB on every request: once
	// it is inactive there is no preselection, although the JWT is unchanged.
	if _, err := e.db.Exec(`UPDATE personas SET especialidad_id = $1 WHERE id = $2`, e.inactiva, e.usuario.PersonaID); err != nil {
		t.Fatal(err)
	}
	if _, body := e.do(t, "GET", "/serums-ia/profesion/opciones", e.token, ``); body["preseleccion"] != nil {
		t.Errorf("preseleccion with an inactive especialidad = %v, want null", body["preseleccion"])
	}
}

// Escenarios «Elegir profesión» y «Elegir otra profesión no cambia los datos
// de persona», y F2 con los pesos de PE6.
func TestElegirProfesion(t *testing.T) {
	e := nuevoEntorno(t)
	asig := dbtest.SeedAsignatura(t, e.db)
	if _, err := e.db.Exec(`INSERT INTO proceso_asignaturas (proceso_id, especialidad_id, asignatura_id, numero_preguntas, peso_porcentual)
		VALUES ($1, $2, $3, 10, 100)`, e.proceso, e.obstetricia, asig); err != nil {
		t.Fatal(err)
	}

	status, body := e.elegir(t, e.token, e.proceso, e.obstetricia, false)
	if status != http.StatusOK || body["estado"] != "activo" {
		t.Fatalf("elegir: %d %v", status, body)
	}
	if nid(body["especialidad"].(map[string]any)["id"]) != e.obstetricia || nid(body["proceso"].(map[string]any)["id"]) != e.proceso {
		t.Errorf("perfil = %v", body)
	}
	p := body["pesos"].(map[string]any)
	areas := p["areas"].([]any)
	if p["pesos_estimados"] != false || len(areas) != 1 || nid(areas[0].(map[string]any)["asignatura_id"]) != asig ||
		areas[0].(map[string]any)["peso_porcentual"] != 100.0 || areas[0].(map[string]any)["nombre"] == "" {
		t.Errorf("pesos = %v", p)
	}
	if got := e.especialidadPersona(t); got != e.enfermeria {
		t.Errorf("personas.especialidad_id = %d, want unchanged %d", got, e.enfermeria)
	}
	if status, f2 := e.do(t, "GET", "/serums-ia/profesion", e.token, ``); status != http.StatusOK || nid(f2["id"]) != nid(body["id"]) {
		t.Errorf("F2: %d %v", status, f2)
	}

	// Without a saved set the weights are estimated (PE6 ModoEstimable).
	_, cambio := e.elegir(t, e.token, e.proceso, e.enfermeria, true)
	if cambio["pesos"].(map[string]any)["pesos_estimados"] != true {
		t.Errorf("Enfermería pesos = %v, want estimated", cambio["pesos"])
	}
}

// Escenario «Cambio de profesión» (reglas 3 y 4), idempotencia y confirmación.
func TestCambioDeProfesion(t *testing.T) {
	e := nuevoEntorno(t)
	if status, b := e.elegir(t, e.token, e.proceso, e.enfermeria, false); status != http.StatusOK {
		t.Fatalf("primera elección: %d %v", status, b)
	}
	// The same pair again changes nothing.
	if status, _ := e.elegir(t, e.token, e.proceso, e.enfermeria, false); status != http.StatusOK || len(e.perfiles(t, e.usuario.ID)) != 1 {
		t.Errorf("same pair: %d, rows %v", status, e.perfiles(t, e.usuario.ID))
	}
	// Another pair without confirmation: 409 with the current pair, nothing changes.
	status, body := e.elegir(t, e.token, e.proceso, e.obstetricia, false)
	if status != http.StatusConflict || codigo(body) != "confirmacion_requerida" {
		t.Fatalf("sin confirmar: %d %v", status, body)
	}
	actual := body["detalle"].(map[string]any)["actual"].(map[string]any)
	if nid(actual["proceso_id"]) != e.proceso || nid(actual["especialidad_id"]) != e.enfermeria {
		t.Errorf("detalle.actual = %v", actual)
	}
	if got := e.perfiles(t, e.usuario.ID); fmt.Sprint(got) != fmt.Sprintf("[%d|activo|false]", e.enfermeria) {
		t.Errorf("rows after 409 = %v", got)
	}
	// Confirmed: the previous profile is archived, the new one is the only active.
	if status, b := e.elegir(t, e.token, e.proceso, e.obstetricia, true); status != http.StatusOK {
		t.Fatalf("confirmado: %d %v", status, b)
	}
	want := fmt.Sprintf("[%d|archivado|true %d|activo|false]", e.enfermeria, e.obstetricia)
	if got := e.perfiles(t, e.usuario.ID); fmt.Sprint(got) != want {
		t.Errorf("rows = %v, want %s", got, want)
	}
	// Regla 4 in the schema: a second active row is rejected by the index.
	_, err := e.db.Exec(`INSERT INTO postulante_profesiones (usuario_id, proceso_id, especialidad_id) VALUES ($1, $2, $3)`,
		e.usuario.ID, e.proceso, e.enfermeria)
	if mapearActivo(err) != ErrEstadoInvalido {
		t.Errorf("second active row: err = %v, want a violation of %s", err, constraintActivo)
	}
}

// Escenarios «Sin inscripción aprobada» y «Profesión inválida», en el orden de F3.
func TestElegir_Rechazos(t *testing.T) {
	e := nuevoEntorno(t)
	pendiente := dbtest.SeedProceso(t, e.db, e.cat, 2031, "activo")
	e.inscribir(t, e.usuario.ID, pendiente, "pendiente")
	ajena := dbtest.SeedCatalog(t, e.db).EspecialidadID

	cases := []struct {
		name   string
		body   string
		status int
		code   string
	}{
		{"inscripción pendiente", fmt.Sprintf(`{"proceso_id":%d,"especialidad_id":%d}`, pendiente, e.enfermeria), 403, "inscripcion_requerida"},
		{"proceso inexistente cuenta como sin inscripción", fmt.Sprintf(`{"proceso_id":999999999,"especialidad_id":%d}`, e.enfermeria), 403, "inscripcion_requerida"},
		{"sin inscripción antes que profesión inválida", fmt.Sprintf(`{"proceso_id":%d,"especialidad_id":999999999}`, pendiente), 403, "inscripcion_requerida"},
		{"especialidad inexistente", fmt.Sprintf(`{"proceso_id":%d,"especialidad_id":999999999}`, e.proceso), 422, "especialidad_invalida"},
		{"especialidad inactiva", fmt.Sprintf(`{"proceso_id":%d,"especialidad_id":%d}`, e.proceso, e.inactiva), 422, "especialidad_invalida"},
		{"especialidad de otra institución", fmt.Sprintf(`{"proceso_id":%d,"especialidad_id":%d}`, e.proceso, ajena), 422, "especialidad_invalida"},
		{"ids en cero", `{"proceso_id":0,"especialidad_id":0}`, 422, "validacion"},
		{"usuario_id no es un campo", fmt.Sprintf(`{"proceso_id":%d,"especialidad_id":%d,"usuario_id":1}`, e.proceso, e.enfermeria), 400, "cuerpo_invalido"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if status, body := e.do(t, "PUT", "/serums-ia/profesion", e.token, c.body); status != c.status || codigo(body) != c.code {
				t.Errorf("got %d %v, want %d %s", status, body, c.status, c.code)
			}
		})
	}
	if n := len(e.perfiles(t, e.usuario.ID)); n != 0 {
		t.Errorf("%d rows stored by rejected requests", n)
	}

	// F1 explains the missing inscripción with an empty list.
	sinAprobada := dbtest.SeedUser(t, e.db, e.cat, "usuario", "activo")
	e.inscribir(t, sinAprobada.ID, e.proceso, "pendiente")
	if _, body := e.do(t, "GET", "/serums-ia/profesion/opciones", dbtest.RealBearer(t, e.db, sinAprobada.ID, "usuario"), ``); len(body["procesos"].([]any)) != 0 {
		t.Errorf("procesos without an approved inscripción = %v", body["procesos"])
	}
}

// Escenario «Aislamiento» y F2 sin perfil o con la inscripción revocada.
func TestAislamientoYPerfil(t *testing.T) {
	e := nuevoEntorno(t)
	if status, b := e.elegir(t, e.token, e.proceso, e.enfermeria, false); status != http.StatusOK {
		t.Fatalf("A elige: %d %v", status, b)
	}
	b := dbtest.SeedUser(t, e.db, e.cat, "usuario", "activo")
	tokenB := dbtest.RealBearer(t, e.db, b.ID, "usuario")
	if status, body := e.do(t, "GET", "/serums-ia/profesion", tokenB, ``); status != http.StatusNotFound || codigo(body) != "profesion_no_definida" {
		t.Errorf("B reads a profile: %d %v, want 404 profesion_no_definida", status, body)
	}
	// The profile stays, but the module is not usable once the inscripción is revoked.
	if _, err := e.db.Exec(`UPDATE inscripciones SET estado = 'rechazado' WHERE usuario_id = $1`, e.usuario.ID); err != nil {
		t.Fatal(err)
	}
	if status, body := e.do(t, "GET", "/serums-ia/profesion", e.token, ``); status != http.StatusForbidden || codigo(body) != "inscripcion_requerida" {
		t.Errorf("revoked inscripción: %d %v, want 403", status, body)
	}
	for _, route := range []string{"GET /serums-ia/profesion/opciones", "GET /serums-ia/profesion", "PUT /serums-ia/profesion"} {
		method, path, _ := strings.Cut(route, " ")
		if status, _ := e.do(t, method, path, "", `{}`); status != http.StatusUnauthorized {
			t.Errorf("%s anonymous: %d, want 401", route, status)
		}
	}
}

// Regla 4 under a race: another transaction creates the user's active profile
// and has not committed. The PUT waits on the unique index and, once the other
// one commits, answers 409 estado_invalido (not 500), with one active profile.
func TestElegir_CarreraEsEstadoInvalido(t *testing.T) {
	e := nuevoEntorno(t)
	tx, err := e.db.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	pid := dbtest.BackendPID(t, tx)
	if _, err := tx.Exec(`INSERT INTO postulante_profesiones (usuario_id, proceso_id, especialidad_id) VALUES ($1, $2, $3)`,
		e.usuario.ID, e.proceso, e.enfermeria); err != nil {
		t.Fatal(err)
	}

	type respuesta struct {
		status int
		code   string
	}
	resultado := make(chan int, 1)
	var r respuesta
	go func() {
		status, body := e.elegir(t, e.token, e.proceso, e.obstetricia, false)
		r = respuesta{status, codigo(body)}
		resultado <- status
	}()
	espero := dbtest.BlockedBy(t, e.db, pid, resultado)
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	<-resultado
	if !espero {
		t.Errorf("the PUT did not wait for the other transaction (answered %d)", r.status)
	}
	if r.status != http.StatusConflict || r.code != "estado_invalido" {
		t.Errorf("got %d %s, want 409 estado_invalido", r.status, r.code)
	}
	if got := e.perfiles(t, e.usuario.ID); fmt.Sprint(got) != fmt.Sprintf("[%d|activo|false]", e.enfermeria) {
		t.Errorf("rows = %v, want only the committed one", got)
	}
}

// Contract F1 invariant (PR #85): a profile whose inscripción was revoked is
// not preselected; the persona rule applies over the procesos offered.
func TestOpciones_PerfilConInscripcionRevocada(t *testing.T) {
	e := nuevoEntorno(t)
	if status, b := e.elegir(t, e.token, e.proceso, e.obstetricia, false); status != http.StatusOK {
		t.Fatalf("elegir: %d %v", status, b)
	}
	otro := dbtest.SeedProceso(t, e.db, e.cat, 2031, "activo")
	e.inscribir(t, e.usuario.ID, otro, "aprobado")
	if _, err := e.db.Exec(`UPDATE inscripciones SET estado = 'rechazado' WHERE usuario_id = $1 AND proceso_id = $2`, e.usuario.ID, e.proceso); err != nil {
		t.Fatal(err)
	}
	_, body := e.do(t, "GET", "/serums-ia/profesion/opciones", e.token, ``)
	pre, _ := body["preseleccion"].(map[string]any)
	if pre == nil || nid(pre["proceso_id"]) != otro || nid(pre["especialidad_id"]) != e.enfermeria {
		t.Errorf("preseleccion = %v, want the persona's Enfermería in the only approved proceso %d", body["preseleccion"], otro)
	}
	// With the persona's especialidad not valid either: null, never the stale profile.
	if _, err := e.db.Exec(`UPDATE personas SET especialidad_id = $1 WHERE id = $2`, e.inactiva, e.usuario.PersonaID); err != nil {
		t.Fatal(err)
	}
	if _, body := e.do(t, "GET", "/serums-ia/profesion/opciones", e.token, ``); body["preseleccion"] != nil {
		t.Errorf("preseleccion = %v, want null", body["preseleccion"])
	}
}
