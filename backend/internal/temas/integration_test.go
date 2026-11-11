package temas

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/cbaciliod/learning/internal/audit"
	"github.com/cbaciliod/learning/internal/db/dbtest"
	"github.com/jmoiron/sqlx"
)

// entorno is the real mux with the real session checker over the test DB, an
// admin, a non-admin user and one empty asignatura.
type entorno struct {
	db      *sqlx.DB
	mux     *http.ServeMux
	cat     dbtest.Catalog
	adminID int64
	admin   string
	usuario string
	asig    int64
}

func nuevoEntorno(t *testing.T) *entorno {
	t.Helper()
	db := dbtest.Open(t)
	e := &entorno{db: db, mux: http.NewServeMux(), cat: dbtest.SeedCatalog(t, db)}
	Mount(e.mux, NewService(NewRepository(db)), audit.New(db), dbtest.JWTSecret, dbtest.SessionChecker(t, db))
	admin := dbtest.SeedUser(t, db, e.cat, "admin", "activo")
	usuario := dbtest.SeedUser(t, db, e.cat, "usuario", "activo")
	e.adminID = admin.ID
	e.admin = dbtest.RealBearer(t, db, admin.ID, "admin")
	e.usuario = dbtest.RealBearer(t, db, usuario.ID, "usuario")
	e.asig = e.nuevaAsignatura(t)
	return e
}

// nuevaAsignatura seeds an asignatura; its preguntas are removed before
// dbtest's cleanup deletes its temas and the asignatura itself.
func (e *entorno) nuevaAsignatura(t *testing.T) int64 {
	t.Helper()
	id := dbtest.SeedAsignatura(t, e.db)
	t.Cleanup(func() {
		if _, err := e.db.Exec(`DELETE FROM preguntas WHERE asignatura_id = $1`, id); err != nil {
			t.Errorf("cleanup preguntas: %v", err)
		}
	})
	return id
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
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("%s %s: invalid JSON body %q", method, path, rr.Body)
	}
	return rr.Code, out
}

func (e *entorno) crear(t *testing.T, asig int64, nombre string, especialidades ...int64) map[string]any {
	t.Helper()
	ids, _ := json.Marshal(especialidades)
	if especialidades == nil {
		ids = []byte("[]")
	}
	status, body := e.do(t, "POST", "/admin/temas", e.admin,
		fmt.Sprintf(`{"asignatura_id":%d,"nombre":%q,"especialidad_ids":%s}`, asig, nombre, ids))
	if status != http.StatusCreated {
		t.Fatalf("crear %q: %d %v", nombre, status, body)
	}
	return body
}

// sembrar inserts n temas in estado for asig, after the existing ones.
func (e *entorno) sembrar(t *testing.T, asig int64, n int, estado string) []int64 {
	t.Helper()
	var ids []int64
	if err := e.db.Select(&ids, `
		INSERT INTO temas (asignatura_id, nombre, nombre_clave, estado, orden)
		SELECT $1, 'Sembrado ' || $3 || ' ' || g, 'sembrado ' || $3 || ' ' || g, $3,
			(SELECT COALESCE(MAX(orden), 0) FROM temas WHERE asignatura_id = $1) + g
		FROM generate_series(1, $2) AS g
		RETURNING id`, asig, n, estado); err != nil {
		t.Fatalf("sembrar temas: %v", err)
	}
	return ids
}

// sembrarPreguntas inserts n active preguntas (plus one deleted, which never
// counts) classified under temaID.
func (e *entorno) sembrarPreguntas(t *testing.T, asig, temaID int64, n int) {
	t.Helper()
	proceso := dbtest.SeedProceso(t, e.db, e.cat, 2030, "activo")
	if _, err := e.db.Exec(`
		INSERT INTO preguntas (institucion_id, proceso_id, asignatura_id, tema_id, enunciado, estado)
		SELECT $1, $2, $3, $4, 'Pregunta de tema ' || g, CASE WHEN g > $5 THEN 'eliminada' ELSE 'activo' END
		FROM generate_series(1, $5 + 1) AS g`, e.cat.InstitucionID, proceso, asig, temaID, n); err != nil {
		t.Fatalf("sembrar preguntas: %v", err)
	}
}

func (e *entorno) nuevaEspecialidad(t *testing.T, estado string) int64 {
	t.Helper()
	var id int64
	u := dbtest.Uniq()
	if err := e.db.QueryRow(`INSERT INTO especialidades (institucion_id, nombre, codigo, estado)
		VALUES ($1, $2, $3, $4) RETURNING id`, e.cat.InstitucionID, "Esp "+u, "X"+u, estado).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func (e *entorno) contar(t *testing.T, asig int64, estado string) int {
	t.Helper()
	var n int
	if err := e.db.Get(&n, `SELECT COUNT(*) FROM temas WHERE asignatura_id = $1 AND estado = $2`, asig, estado); err != nil {
		t.Fatal(err)
	}
	return n
}

type filaAudit struct {
	UsuarioID int64  `db:"usuario_id"`
	Entidad   string `db:"entidad"`
	Detalle   []byte `db:"detalle"`
}

// auditoria returns the audit rows of accion for entidadID, oldest first.
func (e *entorno) auditoria(t *testing.T, accion string, entidadID int64) []filaAudit {
	t.Helper()
	var filas []filaAudit
	if err := e.db.Select(&filas, `SELECT usuario_id, entidad, detalle FROM audit_log
		WHERE accion = $1 AND entidad_id = $2 ORDER BY id`, accion, entidadID); err != nil {
		t.Fatal(err)
	}
	return filas
}

func (e *entorno) detalleAudit(t *testing.T, accion, entidad string, entidadID int64) map[string]any {
	t.Helper()
	filas := e.auditoria(t, accion, entidadID)
	if len(filas) != 1 {
		t.Fatalf("audit %s/%d: %d rows, want 1", accion, entidadID, len(filas))
	}
	if filas[0].UsuarioID != e.adminID || filas[0].Entidad != entidad {
		t.Errorf("audit %s: actor %d entidad %q, want %d %q", accion, filas[0].UsuarioID, filas[0].Entidad, e.adminID, entidad)
	}
	var d map[string]any
	if err := json.Unmarshal(filas[0].Detalle, &d); err != nil {
		t.Fatal(err)
	}
	return d
}

func idDe(body map[string]any) int64 { return int64(body["id"].(float64)) }

func codigo(body map[string]any) string { s, _ := body["error"].(string); return s }

// ── T3 ────────────────────────────────────────────────────────────────────────

// Escenario «Crear tema» y regla 7 (un tema nuevo va al final).
func TestCrearTema(t *testing.T) {
	e := nuevoEntorno(t)
	esp := e.cat.EspecialidadID

	primero := e.crear(t, e.asig, "  Vigilancia   epidemiológica ", esp)
	if primero["estado"] != "activo" || primero["orden"] != 1.0 || primero["nombre"] != "Vigilancia epidemiológica" {
		t.Errorf("tema creado = %v", primero)
	}
	if primero["aplica_a_todas"] != false || primero["preguntas_count"] != 0.0 || primero["descripcion"] != nil {
		t.Errorf("tema creado = %v", primero)
	}
	especialidades := primero["especialidades"].([]any)
	if len(especialidades) != 1 || especialidades[0].(map[string]any)["id"] != float64(esp) {
		t.Errorf("especialidades = %v", especialidades)
	}
	if s, _ := primero["created_at"].(string); !strings.HasSuffix(s, "Z") {
		t.Errorf("created_at = %q, want RFC 3339 UTC", s)
	}

	segundo := e.crear(t, e.asig, "Brotes")
	if segundo["orden"] != 2.0 || segundo["aplica_a_todas"] != true || len(segundo["especialidades"].([]any)) != 0 {
		t.Errorf("segundo tema = %v", segundo)
	}

	d := e.detalleAudit(t, "tema.crear", "tema", idDe(primero))
	if d["nombre"] != "Vigilancia epidemiológica" || d["asignatura_id"] != float64(e.asig) {
		t.Errorf("audit detalle = %v", d)
	}
}

// Escenarios «Nombre duplicado», «Nombre de un tema archivado» y las
// referencias de §1: nada se guarda ni se audita.
func TestCrearTema_Rechazos(t *testing.T) {
	e := nuevoEntorno(t)
	e.crear(t, e.asig, "Vigilancia epidemiológica")
	archivado := e.crear(t, e.asig, "Salud ambiental")
	if status, body := e.do(t, "PATCH", fmt.Sprintf("/admin/temas/%d/estado", idDe(archivado)), e.admin,
		`{"estado":"archivado"}`); status != http.StatusOK {
		t.Fatalf("archivar: %d %v", status, body)
	}
	var inactiva int64
	if err := e.db.QueryRow(`INSERT INTO asignaturas (nombre, codigo, estado) VALUES ('Inactiva', $1, 'inactivo')
		RETURNING id`, "I"+dbtest.Uniq()).Scan(&inactiva); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = e.db.Exec(`DELETE FROM asignaturas WHERE id = $1`, inactiva) })
	espInactiva := e.nuevaEspecialidad(t, "inactivo")

	cases := []struct {
		name, body, code string
	}{
		{"duplicado sin tildes ni mayúsculas", fmt.Sprintf(`{"asignatura_id":%d,"nombre":"vigilancia  EPIDEMIOLOGICA"}`, e.asig), "tema_duplicado"},
		{"duplicado de un archivado", fmt.Sprintf(`{"asignatura_id":%d,"nombre":"Salud ambiental"}`, e.asig), "tema_duplicado"},
		{"asignatura inexistente", `{"asignatura_id":999999999,"nombre":"Nuevo tema"}`, "referencia_invalida"},
		{"asignatura inactiva", fmt.Sprintf(`{"asignatura_id":%d,"nombre":"Nuevo tema"}`, inactiva), "referencia_invalida"},
		{"especialidad inexistente", fmt.Sprintf(`{"asignatura_id":%d,"nombre":"Nuevo tema","especialidad_ids":[999999999]}`, e.asig), "referencia_invalida"},
		{"especialidad inactiva", fmt.Sprintf(`{"asignatura_id":%d,"nombre":"Nuevo tema","especialidad_ids":[%d]}`, e.asig, espInactiva), "referencia_invalida"},
		{"nombre corto", fmt.Sprintf(`{"asignatura_id":%d,"nombre":"ab"}`, e.asig), "validacion"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if status, body := e.do(t, "POST", "/admin/temas", e.admin, c.body); status != http.StatusUnprocessableEntity || codigo(body) != c.code {
				t.Errorf("got %d %v, want 422 %s", status, body, c.code)
			}
		})
	}
	if n := e.contar(t, e.asig, "activo") + e.contar(t, e.asig, "archivado"); n != 2 {
		t.Errorf("%d temas stored, want 2", n)
	}
	var auditados int
	if err := e.db.Get(&auditados, `SELECT COUNT(*) FROM audit_log WHERE accion = 'tema.crear' AND usuario_id = $1`, e.adminID); err != nil {
		t.Fatal(err)
	}
	if auditados != 2 {
		t.Errorf("%d tema.crear rows, want 2", auditados)
	}

	// The same name in another asignatura is free.
	e.crear(t, e.nuevaAsignatura(t), "Vigilancia epidemiológica")
}

// Escenarios «Límite de temas» y «Archivados no cuentan para el límite», y el
// orden de validación (unicidad antes que límite).
func TestCrearTema_Limite(t *testing.T) {
	cases := []struct {
		name                string
		activos, archivados int
		nombre              string
		status              int
		code                string
	}{
		{"60 activos", 60, 0, "Uno más", http.StatusUnprocessableEntity, "limite_temas"},
		{"59 activos y 5 archivados", 59, 5, "Uno más", http.StatusCreated, ""},
		{"duplicado con 60 activos", 60, 0, "Sembrado activo 1", http.StatusUnprocessableEntity, "tema_duplicado"},
	}
	e := nuevoEntorno(t)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			asig := e.nuevaAsignatura(t)
			e.sembrar(t, asig, c.activos, "activo")
			e.sembrar(t, asig, c.archivados, "archivado")
			status, body := e.do(t, "POST", "/admin/temas", e.admin, fmt.Sprintf(`{"asignatura_id":%d,"nombre":%q}`, asig, c.nombre))
			if status != c.status || codigo(body) != c.code {
				t.Fatalf("got %d %v, want %d %q", status, body, c.status, c.code)
			}
			if status == http.StatusCreated && body["orden"] != float64(c.activos+1) {
				t.Errorf("orden = %v, want %d", body["orden"], c.activos+1)
			}
		})
	}
}

// Several simultaneous creates with 59 active temas: the asignatura lock lets
// exactly one through.
func TestCrearTema_Concurrente(t *testing.T) {
	e := nuevoEntorno(t)
	e.sembrar(t, e.asig, MaxTemasActivos-1, "activo")
	const n = 5
	statuses := make([]int, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			req := httptest.NewRequest("POST", "/admin/temas",
				strings.NewReader(fmt.Sprintf(`{"asignatura_id":%d,"nombre":"Concurrente %d"}`, e.asig, i)))
			req.Header.Set("Authorization", e.admin)
			rr := httptest.NewRecorder()
			e.mux.ServeHTTP(rr, req)
			statuses[i] = rr.Code
		}()
	}
	close(start)
	wg.Wait()
	creados := 0
	for _, s := range statuses {
		switch s {
		case http.StatusCreated:
			creados++
		case http.StatusUnprocessableEntity:
		default:
			t.Errorf("unexpected status %d", s)
		}
	}
	if creados != 1 || e.contar(t, e.asig, "activo") != MaxTemasActivos {
		t.Errorf("created %d (statuses %v), active %d; want 1 and %d", creados, statuses, e.contar(t, e.asig, "activo"), MaxTemasActivos)
	}
}

// ── T4 ────────────────────────────────────────────────────────────────────────

func TestEditarTema(t *testing.T) {
	e := nuevoEntorno(t)
	esp2 := e.nuevaEspecialidad(t, "activo")
	tema := e.crear(t, e.asig, "Vigilancia epidemiológica", e.cat.EspecialidadID)
	otro := e.crear(t, e.asig, "Brotes")
	path := fmt.Sprintf("/admin/temas/%d", idDe(tema))

	// Changing only case and accents of its own name is not a duplicate.
	status, body := e.do(t, "PUT", path, e.admin,
		fmt.Sprintf(`{"nombre":"VIGILANCIA epidemiologica","descripcion":"  Notificación  ","especialidad_ids":[%d]}`, esp2))
	if status != http.StatusOK || body["nombre"] != "VIGILANCIA epidemiologica" || body["descripcion"] != "Notificación" {
		t.Fatalf("editar: %d %v", status, body)
	}
	if esps := body["especialidades"].([]any); len(esps) != 1 || esps[0].(map[string]any)["id"] != float64(esp2) {
		t.Errorf("especialidades = %v", esps)
	}
	d := e.detalleAudit(t, "tema.editar", "tema", idDe(tema))
	antes, despues := d["antes"].(map[string]any), d["despues"].(map[string]any)
	if antes["nombre"] != "Vigilancia epidemiológica" || despues["nombre"] != "VIGILANCIA epidemiologica" {
		t.Errorf("audit antes/despues = %v / %v", antes, despues)
	}
	if fmt.Sprint(antes["especialidad_ids"]) != fmt.Sprintf("[%d]", e.cat.EspecialidadID) || fmt.Sprint(despues["especialidad_ids"]) != fmt.Sprintf("[%d]", esp2) {
		t.Errorf("audit especialidades = %v / %v", antes["especialidad_ids"], despues["especialidad_ids"])
	}

	// Empty description and especialidades: null and «aplica a todas».
	status, body = e.do(t, "PUT", path, e.admin, `{"nombre":"Vigilancia","descripcion":""}`)
	if status != http.StatusOK || body["descripcion"] != nil || body["aplica_a_todas"] != true {
		t.Errorf("vaciar: %d %v", status, body)
	}

	// An archived tema can be edited too.
	e.do(t, "PATCH", path+"/estado", e.admin, `{"estado":"archivado"}`)
	if status, body := e.do(t, "PUT", path, e.admin, `{"nombre":"Vigilancia archivada"}`); status != http.StatusOK || body["estado"] != "archivado" {
		t.Errorf("editar archivado: %d %v", status, body)
	}

	cases := []struct {
		name, path, body string
		status           int
		code             string
	}{
		{"nombre de otro tema", path, `{"nombre":"brotes"}`, http.StatusUnprocessableEntity, "tema_duplicado"},
		{"inexistente", "/admin/temas/999999999", `{"nombre":"Lo que sea"}`, http.StatusNotFound, "no_encontrado"},
		{"especialidad inexistente", path, `{"nombre":"Vigilancia","especialidad_ids":[999999999]}`, http.StatusUnprocessableEntity, "referencia_invalida"},
		{"no se cambia de asignatura", path, fmt.Sprintf(`{"nombre":"Vigilancia","asignatura_id":%d}`, e.asig), http.StatusBadRequest, "cuerpo_invalido"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if status, body := e.do(t, "PUT", c.path, e.admin, c.body); status != c.status || codigo(body) != c.code {
				t.Errorf("got %d %v, want %d %s", status, body, c.status, c.code)
			}
		})
	}
	if _, body := e.do(t, "GET", fmt.Sprintf("/admin/temas/%d", idDe(otro)), e.admin, ``); body["nombre"] != "Brotes" {
		t.Errorf("otro tema cambió: %v", body)
	}
}

// ── T5 ────────────────────────────────────────────────────────────────────────

// Escenarios «Archivar tema con preguntas» y «Reactivar tema».
func TestArchivarYReactivar(t *testing.T) {
	e := nuevoEntorno(t)
	e.sembrar(t, e.asig, 19, "activo")
	tema := e.crear(t, e.asig, "Vigilancia epidemiológica", e.cat.EspecialidadID)
	id := idDe(tema)
	e.sembrarPreguntas(t, e.asig, id, 10)
	path := fmt.Sprintf("/admin/temas/%d/estado", id)

	if _, body := e.do(t, "GET", fmt.Sprintf("/admin/temas/%d", id), e.admin, ``); body["preguntas_count"] != 10.0 {
		t.Fatalf("preguntas_count = %v, want 10 (deleted ones excluded)", body["preguntas_count"])
	}

	status, body := e.do(t, "PATCH", path, e.admin, `{"estado":"archivado"}`)
	if status != http.StatusOK || body["estado"] != "archivado" || body["preguntas_count"] != 10.0 {
		t.Fatalf("archivar: %d %v", status, body)
	}
	var conTema int
	if err := e.db.Get(&conTema, `SELECT COUNT(*) FROM preguntas WHERE tema_id = $1`, id); err != nil {
		t.Fatal(err)
	}
	if conTema != 11 {
		t.Errorf("%d preguntas keep the tema, want 11", conTema)
	}
	_, activos := e.do(t, "GET", fmt.Sprintf("/asignaturas/%d/temas", e.asig), e.usuario, ``)
	for _, a := range activos["data"].([]any) {
		if idDe(a.(map[string]any)) == id {
			t.Errorf("archived tema offered for classification")
		}
	}
	d := e.detalleAudit(t, "tema.archivar", "tema", id)
	if d["estado_anterior"] != "activo" || d["estado_nuevo"] != "archivado" || d["preguntas_count"] != 10.0 {
		t.Errorf("audit archivar = %v", d)
	}

	// 20 active temas now (19 seeded + one created after archiving).
	e.crear(t, e.asig, "Otro tema activo")
	status, body = e.do(t, "PATCH", path, e.admin, `{"estado":"activo"}`)
	if status != http.StatusOK || body["estado"] != "activo" || body["orden"] != 21.0 || body["preguntas_count"] != 10.0 {
		t.Fatalf("reactivar: %d %v", status, body)
	}
	if len(body["especialidades"].([]any)) != 1 {
		t.Errorf("especialidades lost: %v", body["especialidades"])
	}
	d = e.detalleAudit(t, "tema.reactivar", "tema", id)
	if d["estado_nuevo"] != "activo" || d["orden"] != 21.0 || d["preguntas_count"] != 10.0 {
		t.Errorf("audit reactivar = %v", d)
	}
}

// Escenarios «Reactivar por encima del límite» y «Reactivar un tema activo».
func TestCambiarEstado_Rechazos(t *testing.T) {
	e := nuevoEntorno(t)
	activo := e.sembrar(t, e.asig, 1, "activo")[0]
	archivado := e.sembrar(t, e.asig, 1, "archivado")[0]
	lleno := e.nuevaAsignatura(t)
	e.sembrar(t, lleno, MaxTemasActivos, "activo")
	archivadoEnLleno := e.sembrar(t, lleno, 1, "archivado")[0]

	cases := []struct {
		name   string
		id     int64
		body   string
		status int
		code   string
	}{
		{"reactivar un activo", activo, `{"estado":"activo"}`, http.StatusConflict, "estado_invalido"},
		{"archivar un archivado", archivado, `{"estado":"archivado"}`, http.StatusConflict, "estado_invalido"},
		{"reactivar con 60 activos", archivadoEnLleno, `{"estado":"activo"}`, http.StatusUnprocessableEntity, "limite_temas"},
		{"estado desconocido", activo, `{"estado":"borrado"}`, http.StatusUnprocessableEntity, "validacion"},
		{"inexistente", 999999999, `{"estado":"archivado"}`, http.StatusNotFound, "no_encontrado"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, body := e.do(t, "PATCH", fmt.Sprintf("/admin/temas/%d/estado", c.id), e.admin, c.body)
			if status != c.status || codigo(body) != c.code {
				t.Errorf("got %d %v, want %d %s", status, body, c.status, c.code)
			}
		})
	}
	var estado string
	if err := e.db.Get(&estado, `SELECT estado FROM temas WHERE id = $1`, archivadoEnLleno); err != nil {
		t.Fatal(err)
	}
	if estado != "archivado" || len(e.auditoria(t, "tema.reactivar", archivadoEnLleno)) != 0 {
		t.Errorf("rejected reactivation left estado %q or an audit row", estado)
	}
}

// ── T6 ────────────────────────────────────────────────────────────────────────

func TestReordenar(t *testing.T) {
	e := nuevoEntorno(t)
	ids := e.sembrar(t, e.asig, 3, "activo")
	archivado := e.sembrar(t, e.asig, 1, "archivado")[0]
	ajeno := e.sembrar(t, e.nuevaAsignatura(t), 1, "activo")[0]
	path := fmt.Sprintf("/admin/asignaturas/%d/temas/orden", e.asig)
	lista := func(ids ...int64) string { b, _ := json.Marshal(ids); return fmt.Sprintf(`{"tema_ids":%s}`, b) }

	cases := []struct {
		name, path, body string
		status           int
		code             string
	}{
		{"falta uno", path, lista(ids[0], ids[1]), http.StatusConflict, "orden_desactualizado"},
		{"incluye un archivado", path, lista(ids[0], ids[1], ids[2], archivado), http.StatusConflict, "orden_desactualizado"},
		{"cambia un activo por un archivado", path, lista(ids[0], ids[1], archivado), http.StatusConflict, "orden_desactualizado"},
		{"tema de otra asignatura", path, lista(ids[0], ids[1], ajeno), http.StatusConflict, "orden_desactualizado"},
		{"repetidos", path, lista(ids[0], ids[0], ids[1]), http.StatusUnprocessableEntity, "orden_invalido"},
		{"asignatura inexistente", "/admin/asignaturas/999999999/temas/orden", `{"tema_ids":[]}`, http.StatusNotFound, "no_encontrado"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if status, body := e.do(t, "PUT", c.path, e.admin, c.body); status != c.status || codigo(body) != c.code {
				t.Errorf("got %d %v, want %d %s", status, body, c.status, c.code)
			}
		})
	}
	if len(e.auditoria(t, "tema.reordenar", e.asig)) != 0 {
		t.Fatal("a rejected reorder was audited")
	}

	nuevo := []int64{ids[2], ids[0], ids[1]}
	status, body := e.do(t, "PUT", path, e.admin, lista(nuevo...))
	if status != http.StatusOK {
		t.Fatalf("reordenar: %d %v", status, body)
	}
	data := body["data"].([]any)
	for i, item := range data {
		m := item.(map[string]any)
		if idDe(m) != nuevo[i] || m["orden"] != float64(i+1) {
			t.Errorf("data[%d] = %v, want id %d orden %d", i, m, nuevo[i], i+1)
		}
	}
	_, activos := e.do(t, "GET", fmt.Sprintf("/asignaturas/%d/temas", e.asig), e.usuario, ``)
	for i, item := range activos["data"].([]any) {
		if idDe(item.(map[string]any)) != nuevo[i] {
			t.Errorf("T7 position %d = %v, want %d", i, item, nuevo[i])
		}
	}
	d := e.detalleAudit(t, "tema.reordenar", "asignatura", e.asig)
	if fmt.Sprint(d["antes"]) != fmt.Sprint(toFloats(ids)) || fmt.Sprint(d["despues"]) != fmt.Sprint(toFloats(nuevo)) {
		t.Errorf("audit reordenar = %v", d)
	}

	// A tema created afterwards goes to the end.
	if creado := e.crear(t, e.asig, "Al final"); creado["orden"] != 4.0 {
		t.Errorf("orden del nuevo = %v, want 4", creado["orden"])
	}
}

func toFloats(ids []int64) []any {
	out := make([]any, len(ids))
	for i, id := range ids {
		out[i] = float64(id)
	}
	return out
}

// ── T1, T2, T7 ────────────────────────────────────────────────────────────────

func TestListarAdmin(t *testing.T) {
	e := nuevoEntorno(t)
	esp2 := e.nuevaEspecialidad(t, "activo")
	todas := e.crear(t, e.asig, "Para todas")
	deEsp := e.crear(t, e.asig, "Solo especialidad", e.cat.EspecialidadID)
	deOtra := e.crear(t, e.asig, "Solo la otra", esp2)
	archivado := e.sembrar(t, e.asig, 1, "archivado")[0]

	ids := func(body map[string]any) []int64 {
		var out []int64
		for _, item := range body["data"].([]any) {
			out = append(out, idDe(item.(map[string]any)))
		}
		return out
	}
	base := fmt.Sprintf("/admin/temas?asignatura_id=%d", e.asig)
	cases := []struct {
		name, query string
		want        []int64
		total       float64
	}{
		{"todos por orden", "", []int64{idDe(todas), idDe(deEsp), idDe(deOtra), archivado}, 4},
		{"solo activos", "&estado=activo", []int64{idDe(todas), idDe(deEsp), idDe(deOtra)}, 3},
		{"solo archivados", "&estado=archivado", []int64{archivado}, 1},
		{"por especialidad incluye los de todas", fmt.Sprintf("&especialidad_id=%d", e.cat.EspecialidadID), []int64{idDe(todas), idDe(deEsp), archivado}, 3},
		{"página 2 de 2 en 2", "&per_page=2&page=2", []int64{idDe(deOtra), archivado}, 4},
		{"página más allá del final", "&per_page=2&page=9", nil, 4},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, body := e.do(t, "GET", base+c.query, e.admin, ``)
			if status != http.StatusOK || fmt.Sprint(ids(body)) != fmt.Sprint(c.want) || body["total"] != c.total {
				t.Errorf("got %d ids %v total %v, want %v total %v", status, ids(body), body["total"], c.want, c.total)
			}
		})
	}

	// Escenario «Estado vacío»: an asignatura without temas (or unknown) is 200 [].
	for _, q := range []string{fmt.Sprintf("?asignatura_id=%d", e.nuevaAsignatura(t)), "?asignatura_id=999999999"} {
		status, body := e.do(t, "GET", "/admin/temas"+q, e.admin, ``)
		if status != http.StatusOK || len(body["data"].([]any)) != 0 || body["total"] != 0.0 {
			t.Errorf("%s: %d %v", q, status, body)
		}
	}

	if status, body := e.do(t, "GET", fmt.Sprintf("/admin/temas/%d", idDe(deEsp)), e.admin, ``); status != http.StatusOK || body["nombre"] != "Solo especialidad" {
		t.Errorf("T2: %d %v", status, body)
	}
	if status, body := e.do(t, "GET", "/admin/temas/999999999", e.admin, ``); status != http.StatusNotFound || codigo(body) != "no_encontrado" {
		t.Errorf("T2 404: %d %v", status, body)
	}
}

func TestListarActivos(t *testing.T) {
	e := nuevoEntorno(t)
	esp2 := e.nuevaEspecialidad(t, "activo")
	todas := e.crear(t, e.asig, "Para todas")
	deOtra := e.crear(t, e.asig, "Solo la otra", esp2)
	e.sembrar(t, e.asig, 1, "archivado")
	path := fmt.Sprintf("/asignaturas/%d/temas", e.asig)

	status, body := e.do(t, "GET", path, e.usuario, ``)
	data, _ := body["data"].([]any)
	if status != http.StatusOK || len(data) != 2 {
		t.Fatalf("usuario: %d %v", status, body)
	}
	primero := data[0].(map[string]any)
	if idDe(primero) != idDe(todas) || primero["aplica_a_todas"] != true || primero["orden"] != 1.0 {
		t.Errorf("data[0] = %v", primero)
	}
	if _, ok := primero["preguntas_count"]; ok {
		t.Errorf("T7 must not expose preguntas_count: %v", primero)
	}
	if segundo := data[1].(map[string]any); fmt.Sprint(segundo["especialidad_ids"]) != fmt.Sprintf("[%d]", esp2) {
		t.Errorf("data[1] = %v", segundo)
	}

	_, body = e.do(t, "GET", fmt.Sprintf("%s?especialidad_id=%d", path, e.cat.EspecialidadID), e.usuario, ``)
	if data := body["data"].([]any); len(data) != 1 || idDe(data[0].(map[string]any)) != idDe(todas) {
		t.Errorf("filtro especialidad: %v (the other one is %d)", body, idDe(deOtra))
	}
	if status, body := e.do(t, "GET", "/asignaturas/999999999/temas", e.usuario, ``); status != http.StatusNotFound || codigo(body) != "no_encontrado" {
		t.Errorf("404: %d %v", status, body)
	}
	if status, _ := e.do(t, "GET", path, "", ``); status != http.StatusUnauthorized {
		t.Errorf("anonymous: %d, want 401", status)
	}
}

// ── Permisos ──────────────────────────────────────────────────────────────────

// Escenario «Permisos»: through the real mux and session checker, T1–T6 reject
// anonymous (401) and non-admin users (403), even with a forged admin token.
func TestRoutes_AdminOnly(t *testing.T) {
	db := dbtest.Open(t)
	mux := http.NewServeMux()
	Mount(mux, NewService(NewRepository(db)), audit.New(db), dbtest.JWTSecret, dbtest.SessionChecker(t, db))
	dbtest.AssertAdminOnly(t, db, mux, []dbtest.Route{
		"GET /admin/temas",
		"GET /admin/temas/999999999",
		"POST /admin/temas",
		"PUT /admin/temas/999999999",
		"PATCH /admin/temas/999999999/estado",
		"PUT /admin/asignaturas/999999999/temas/orden",
	})
}
