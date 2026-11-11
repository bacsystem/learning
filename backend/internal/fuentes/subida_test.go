package fuentes_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"

	"github.com/cbaciliod/learning/internal/audit"
	"github.com/cbaciliod/learning/internal/db/dbtest"
	"github.com/cbaciliod/learning/internal/fuentes"
	"github.com/cbaciliod/learning/internal/middleware"
	"github.com/jmoiron/sqlx"
)

// subidaEnv is the real fuentes mux behind the real body limit, with an admin,
// a non-admin user, two active especialidades (A, B) and a proceso.
type subidaEnv struct {
	db        *sqlx.DB
	h         http.Handler
	adminID   int64
	admin     string
	usuario   string
	espA      int64
	espB      int64
	proceso   int64
	cat       dbtest.Catalog
	publicar  *fuentes.PgSource
	sinCupoDe int64
}

// citaTodo is a Citantes that says every version is cited.
type citaTodo struct{}

func (citaTodo) ContarPorFuente(context.Context, sqlx.QueryerContext, int64) (int, error) {
	return 3, nil
}
func (citaTodo) VersionCitada(context.Context, sqlx.QueryerContext, int64) (bool, error) {
	return true, nil
}

func nuevoSubidaEnv(t *testing.T, citantes fuentes.Citantes) *subidaEnv {
	t.Helper()
	db := dbtest.Open(t)
	e := &subidaEnv{db: db, cat: dbtest.SeedCatalog(t, db), publicar: fuentes.NewPgSource(db)}
	admin := dbtest.SeedUser(t, db, e.cat, "admin", "activo")
	usuario := dbtest.SeedUser(t, db, e.cat, "usuario", "activo")
	e.adminID = admin.ID
	e.admin = dbtest.RealBearer(t, db, admin.ID, "admin")
	e.usuario = dbtest.RealBearer(t, db, usuario.ID, "usuario")
	e.espA = e.cat.EspecialidadID
	e.espB = e.especialidad(t, "activo")
	e.proceso = dbtest.SeedProceso(t, db, e.cat, 2030, "activo")
	// Registered after users and catalog: runs first, before their cleanup.
	var desde int64
	if err := db.Get(&desde, `SELECT COALESCE(MAX(id), 0) FROM fuentes`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { limpiarFuentes(t, db, desde) })

	mux := http.NewServeMux()
	fuentes.Mount(mux, fuentes.NewService(fuentes.NewRepository(db), citantes), audit.New(db), dbtest.JWTSecret, dbtest.SessionChecker(t, db))
	e.h = middleware.MaxBodySize(middleware.DefaultJSONBodyLimit, middleware.DefaultMultipartBodyLimit, fuentes.RutasSubida...)(mux)
	return e
}

func (e *subidaEnv) especialidad(t *testing.T, estado string) int64 {
	t.Helper()
	var id int64
	u := dbtest.Uniq()
	if err := e.db.QueryRow(`INSERT INTO especialidades (institucion_id, nombre, codigo, estado) VALUES ($1, $2, $3, $4) RETURNING id`,
		e.cat.InstitucionID, "Esp "+u, "U"+u, estado).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func (e *subidaEnv) exec(t *testing.T, q string, args ...any) {
	t.Helper()
	if _, err := e.db.Exec(q, args...); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
}

func contenidoUnico() []byte {
	return []byte("# Guía " + dbtest.Uniq() + "\nVigilancia epidemiológica.\n")
}

func (e *subidaEnv) subir(t *testing.T, ruta, bearer string, contenido []byte, campos map[string][]string) (int, map[string]any) {
	t.Helper()
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	for k, vs := range campos {
		for _, v := range vs {
			_ = w.WriteField(k, v)
		}
	}
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", `form-data; name="file"; filename="guia.md"`)
	h.Set("Content-Type", "text/markdown")
	part, _ := w.CreatePart(h)
	_, _ = part.Write(contenido)
	_ = w.Close()
	method, path, _ := strings.Cut(ruta, " ")
	req := httptest.NewRequest(method, path, &b)
	req.Header.Set("Content-Type", w.FormDataContentType())
	if bearer != "" {
		req.Header.Set("Authorization", bearer)
	}
	rr := httptest.NewRecorder()
	e.h.ServeHTTP(rr, req)
	var out map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	return rr.Code, out
}

func (e *subidaEnv) crear(t *testing.T, contenido []byte, esp ...int64) int64 {
	t.Helper()
	campos := map[string][]string{"titulo": {"Fuente " + dbtest.Uniq()}, "tipo": {"guia"}}
	for _, id := range esp {
		campos["especialidad_ids"] = append(campos["especialidad_ids"], fmt.Sprint(id))
	}
	status, body := e.subir(t, fuentes.RutaCrear, e.admin, contenido, campos)
	if status != http.StatusAccepted {
		t.Fatalf("crear: %d %v", status, body)
	}
	return int64(body["id"].(float64))
}

// activa creates a fuente through S1 and publishes its version 1 (worker role).
func (e *subidaEnv) activa(t *testing.T, contenido []byte, esp ...int64) int64 {
	t.Helper()
	id := e.crear(t, contenido, esp...)
	if err := e.publicar.PublicarVersion(context.Background(), id, 1,
		[]fuentes.PasajeNuevo{{Orden: 1, LineaInicio: 1, LineaFin: 2, Texto: "Vigilancia", Palabras: 1}}, nil); err != nil {
		t.Fatal(err)
	}
	return id
}

func (e *subidaEnv) contarFuentes(t *testing.T) int {
	t.Helper()
	var n int
	if err := e.db.Get(&n, `SELECT COUNT(*) FROM fuentes`); err != nil {
		t.Fatal(err)
	}
	return n
}

// sembrar inserts n fuentes in estado with the especialidades (none = all).
// 'retirada' gets a current version with a unique hash.
func (e *subidaEnv) sembrar(t *testing.T, n int, estado string, esp []int64) {
	t.Helper()
	var ids []int64
	if err := e.db.Select(&ids, `INSERT INTO fuentes (titulo, tipo) SELECT 'Relleno ' || g, 'otro' FROM generate_series(1, $1) g RETURNING id`, n); err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		for _, es := range esp {
			e.exec(t, `INSERT INTO fuente_especialidades (fuente_id, especialidad_id) VALUES ($1, $2)`, id, es)
		}
		switch estado {
		case fuentes.EstadoRetirada:
			var vid int64
			if err := e.db.QueryRow(`INSERT INTO fuente_versiones (fuente_id, numero, estado, nombre_archivo, sha256, tamano_bytes, lineas, contenido)
				VALUES ($1, 1, 'lista', 'r.md', encode(sha256(convert_to($2, 'UTF8')), 'hex'), 1, 1, 'x') RETURNING id`,
				id, fmt.Sprintf("relleno %d %s", id, dbtest.Uniq())).Scan(&vid); err != nil {
				t.Fatal(err)
			}
			e.exec(t, `UPDATE fuentes SET estado = 'retirada', version_vigente_id = $2, motivo_retiro = 'relleno' WHERE id = $1`, id, vid)
		case fuentes.EstadoFallida:
			e.exec(t, `UPDATE fuentes SET estado = 'fallida' WHERE id = $1`, id)
		}
	}
}

// sinProfesion counts the counted fuentes (activa, procesando) without
// professions already in the shared DB: they count for every profession.
func (e *subidaEnv) sinProfesion(t *testing.T) int {
	t.Helper()
	var n int
	if err := e.db.Get(&n, `SELECT COUNT(*) FROM fuentes f WHERE estado IN ('activa', 'procesando')
		AND NOT EXISTS (SELECT 1 FROM fuente_especialidades fe WHERE fe.fuente_id = f.id)`); err != nil {
		t.Fatal(err)
	}
	return n
}

func codigoDe(body map[string]any) string { s, _ := body["error"].(string); return s }

// ── S1 ────────────────────────────────────────────────────────────────────────

// Escenarios «Subida correcta» (hasta el 202; el paso a «activa» es del worker,
// E2-05) y «Auditoría».
func TestS1_SubidaCorrecta(t *testing.T) {
	e := nuevoSubidaEnv(t, nil)
	contenido := contenidoUnico()
	status, body := e.subir(t, fuentes.RutaCrear, e.admin, contenido, map[string][]string{
		"titulo": {"  Guía de vigilancia  "}, "tipo": {"guia"}, "anio": {"2025"}, "proceso_id": {fmt.Sprint(e.proceso)},
		"especialidad_ids": {fmt.Sprint(e.espB), fmt.Sprint(e.espA)}})
	if status != http.StatusAccepted {
		t.Fatalf("got %d %v", status, body)
	}
	if body["estado"] != "procesando" || body["version_vigente"] != nil || body["titulo"] != "Guía de vigilancia" ||
		body["aplica_a_todas"] != false || len(body["especialidades"].([]any)) != 2 || body["contenidos_citantes"] != 0.0 {
		t.Errorf("detalle = %v", body)
	}
	if p := body["proceso"].(map[string]any); int64(p["id"].(float64)) != e.proceso || p["nombre"] == "" {
		t.Errorf("proceso = %v", p)
	}
	versiones := body["versiones"].([]any)
	v := versiones[0].(map[string]any)
	sha, tamano, lineas := fuentes.Huella(contenido)
	if len(versiones) != 1 || v["numero"] != 1.0 || v["estado"] != "procesando" || v["sha256"] != sha ||
		v["tamano_bytes"] != float64(tamano) || v["lineas"] != float64(lineas) || v["oficial_verificada"] != false {
		t.Errorf("versiones = %v", versiones)
	}
	if ep := body["version_en_proceso"].(map[string]any); ep["numero"] != 1.0 || ep["estado"] != "procesando" {
		t.Errorf("version_en_proceso = %v", ep)
	}

	id := int64(body["id"].(float64))
	var detalle []byte
	var actor int64
	if err := e.db.QueryRow(`SELECT usuario_id, detalle FROM audit_log WHERE accion = 'fuente.crear' AND entidad = 'fuente' AND entidad_id = $1`, id).
		Scan(&actor, &detalle); err != nil {
		t.Fatalf("audit fuente.crear: %v", err)
	}
	if actor != e.adminID || !strings.Contains(string(detalle), sha) || strings.Contains(string(detalle), "Vigilancia") {
		t.Errorf("audit actor %d detalle %s (must carry the hash, never the content)", actor, detalle)
	}
}

// Escenarios «Tipo inválido» y validación de S1 en el orden 3–7: nada se crea.
func TestS1_Rechazos(t *testing.T) {
	e := nuevoSubidaEnv(t, nil)
	inactiva := e.especialidad(t, "inactivo")
	base := map[string][]string{"titulo": {"Guía válida"}, "tipo": {"guia"}}
	con := func(k string, v ...string) map[string][]string {
		m := map[string][]string{"titulo": base["titulo"], "tipo": base["tipo"]}
		m[k] = v
		return m
	}
	antes := e.contarFuentes(t)
	cases := []struct {
		name      string
		contenido []byte
		campos    map[string][]string
		status    int
		code      string
	}{
		{"bytes no UTF-8", []byte{0xff, 0xfe, 'a'}, base, 422, "archivo_invalido"},
		{"archivo antes que metadatos", []byte{0xff}, con("titulo", "a"), 422, "archivo_invalido"},
		{"título corto", contenidoUnico(), con("titulo", "ab"), 422, "validacion"},
		{"especialidad inactiva", contenidoUnico(), con("especialidad_ids", fmt.Sprint(inactiva)), 422, "referencia_invalida"},
		{"especialidad inexistente", contenidoUnico(), con("especialidad_ids", "999999999"), 422, "referencia_invalida"},
		{"proceso inexistente", contenidoUnico(), con("proceso_id", "999999999"), 422, "referencia_invalida"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if status, body := e.subir(t, fuentes.RutaCrear, e.admin, c.contenido, c.campos); status != c.status || codigoDe(body) != c.code {
				t.Errorf("got %d %v, want %d %s", status, body, c.status, c.code)
			}
		})
	}
	if n := e.contarFuentes(t); n != antes {
		t.Errorf("%d fuentes created by rejected uploads", n-antes)
	}
	// Escenario «Permisos» and the 3 MiB file of «Archivo demasiado grande».
	if status, _ := e.subir(t, fuentes.RutaCrear, e.usuario, contenidoUnico(), base); status != http.StatusForbidden {
		t.Errorf("usuario: %d, want 403", status)
	}
	if status, body := e.subir(t, fuentes.RutaCrear, e.admin, bytes.Repeat([]byte("a"), 3*1024*1024), base); status != 413 || codigoDe(body) != "archivo_demasiado_grande" {
		t.Errorf("3 MiB: %d %v", status, body)
	}
}

// Escenarios «Duplicado global» y «Duplicado de una fuente retirada»: the
// hash of any version of any fuente, in any state, archived included.
func TestS1_DuplicadoGlobal(t *testing.T) {
	e := nuevoSubidaEnv(t, nil)
	ctx := context.Background()
	procesando := contenidoUnico()
	idProcesando := e.crear(t, procesando)

	fallida := contenidoUnico()
	idFallida := e.crear(t, fallida)
	if err := e.publicar.MarcarVersionFallida(ctx, idFallida, 1, fuentes.MotivoSinContenido, nil); err != nil {
		t.Fatal(err)
	}
	retirada := contenidoUnico()
	idRetirada := e.activa(t, retirada)
	e.exec(t, `UPDATE fuentes SET estado = 'retirada', motivo_retiro = 'prueba' WHERE id = $1`, idRetirada)

	archivada := contenidoUnico()
	idArchivada := e.activa(t, contenidoUnico())
	sha, _, _ := fuentes.Huella(archivada)
	e.exec(t, `INSERT INTO fuente_versiones (fuente_id, numero, estado, nombre_archivo, sha256, tamano_bytes, lineas, contenido)
		VALUES ($1, 2, 'archivada', 'v.md', $2, 1, 1, NULL)`, idArchivada, sha)

	antes := e.contarFuentes(t)
	for _, c := range []struct {
		name      string
		contenido []byte
		fuente    int64
		estado    string
		version   float64
	}{
		{"procesando", procesando, idProcesando, "procesando", 1},
		{"fallida", fallida, idFallida, "fallida", 1},
		{"retirada", retirada, idRetirada, "retirada", 1},
		{"versión archivada", archivada, idArchivada, "activa", 2},
	} {
		t.Run(c.name, func(t *testing.T) {
			status, body := e.subir(t, fuentes.RutaCrear, e.admin, c.contenido, map[string][]string{"titulo": {"Otra"}, "tipo": {"norma"}})
			d, _ := body["detalle"].(map[string]any)
			if status != http.StatusConflict || codigoDe(body) != "fuente_duplicada" || d == nil ||
				int64(d["fuente_id"].(float64)) != c.fuente || d["estado"] != c.estado || d["version"] != c.version || d["titulo"] == "" {
				t.Errorf("got %d %v", status, body)
			}
		})
	}
	if n := e.contarFuentes(t); n != antes {
		t.Errorf("%d fuentes created by duplicates", n-antes)
	}
}

// Escenarios del límite (regla 4) y el orden 8 antes que 9.
func TestS1_LimiteDeFuentes(t *testing.T) {
	cases := []struct {
		name     string
		preparar func(t *testing.T, e *subidaEnv) (esp []int64)
		status   int
		detalle  func(e *subidaEnv) string
	}{
		{"Medicina con 100", func(t *testing.T, e *subidaEnv) []int64 {
			e.sembrar(t, 100-e.sinProfesion(t), fuentes.EstadoProcesando, []int64{e.espA})
			return []int64{e.espA}
		}, 422, func(e *subidaEnv) string { return fmt.Sprintf("map[especialidad_id:%d limite:100]", e.espA) }},
		{"sin profesiones con Medicina en 100", func(t *testing.T, e *subidaEnv) []int64 {
			e.sembrar(t, 100-e.sinProfesion(t), fuentes.EstadoProcesando, []int64{e.espA})
			e.sembrar(t, 10, fuentes.EstadoProcesando, []int64{e.espB})
			return nil
		}, 422, func(e *subidaEnv) string { return fmt.Sprintf("map[especialidad_id:%d limite:100]", e.espA) }},
		{"99 + 5 fallidas + 5 retiradas", func(t *testing.T, e *subidaEnv) []int64 {
			e.sembrar(t, 99-e.sinProfesion(t), fuentes.EstadoProcesando, []int64{e.espA})
			e.sembrar(t, 5, fuentes.EstadoFallida, []int64{e.espA})
			e.sembrar(t, 5, fuentes.EstadoRetirada, []int64{e.espA})
			return []int64{e.espA}
		}, 202, nil},
		{"500 en total", func(t *testing.T, e *subidaEnv) []int64 {
			var contadas int
			if err := e.db.Get(&contadas, `SELECT COUNT(*) FROM fuentes WHERE estado IN ('activa', 'procesando')`); err != nil {
				t.Fatal(err)
			}
			for falta := 500 - contadas; falta > 0; falta -= 90 {
				e.sembrar(t, min(90, falta), fuentes.EstadoProcesando, []int64{e.especialidad(t, "activo")})
			}
			return []int64{e.espB}
		}, 422, func(*subidaEnv) string { return "map[limite:500]" }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := nuevoSubidaEnv(t, nil)
			esp := c.preparar(t, e)
			campos := map[string][]string{"titulo": {"Una más"}, "tipo": {"guia"}}
			for _, id := range esp {
				campos["especialidad_ids"] = append(campos["especialidad_ids"], fmt.Sprint(id))
			}
			status, body := e.subir(t, fuentes.RutaCrear, e.admin, contenidoUnico(), campos)
			if status != c.status || (c.detalle != nil && (codigoDe(body) != "limite_fuentes" || fmt.Sprint(body["detalle"]) != c.detalle(e))) {
				t.Errorf("got %d %v", status, body)
			}
		})
	}

	// Step 8 before 9: a duplicate at the limit is fuente_duplicada.
	e := nuevoSubidaEnv(t, nil)
	dup := contenidoUnico()
	e.crear(t, dup, e.espA)
	e.sembrar(t, 99-e.sinProfesion(t), fuentes.EstadoProcesando, []int64{e.espA})
	status, body := e.subir(t, fuentes.RutaCrear, e.admin, dup, map[string][]string{"titulo": {"Dup"}, "tipo": {"guia"}, "especialidad_ids": {fmt.Sprint(e.espA)}})
	if status != http.StatusConflict || codigoDe(body) != "fuente_duplicada" {
		t.Errorf("duplicate at the limit: %d %v", status, body)
	}
}

// The check and the write are serialised by the fuentes advisory lock: an
// upload waits while another transaction holds it.
func TestS1_EsperaElBloqueoDeFuentes(t *testing.T) {
	e := nuevoSubidaEnv(t, nil)
	tx, err := e.db.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	pid := dbtest.BackendPID(t, tx)
	if _, err := tx.Exec(`SELECT pg_advisory_xact_lock(hashtext('learning:fuentes'))`); err != nil {
		t.Fatal(err)
	}
	resultado := make(chan int, 1)
	go func() {
		status, _ := e.subir(t, fuentes.RutaCrear, e.admin, contenidoUnico(), map[string][]string{"titulo": {"Con bloqueo"}, "tipo": {"guia"}})
		resultado <- status
	}()
	espero := dbtest.BlockedBy(t, e.db, pid, resultado)
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if status := <-resultado; !espero || status != http.StatusAccepted {
		t.Errorf("waited=%v status=%d; want true 202", espero, status)
	}
}

// ── S2 ────────────────────────────────────────────────────────────────────────

func (e *subidaEnv) version(t *testing.T, fuenteID int64, contenido []byte) (int, map[string]any) {
	t.Helper()
	return e.subir(t, fuentes.RutaNuevaVersion, e.admin, contenido, map[string][]string{"fuente_id": {fmt.Sprint(fuenteID)}})
}

// S2 on an active fuente (escenario «Nueva versión» de E2-06 hasta el 202):
// v2 in process, the fuente stays active with v1, the mark is not inherited.
func TestS2_NuevaVersion(t *testing.T) {
	e := nuevoSubidaEnv(t, nil)
	id := e.activa(t, contenidoUnico(), e.espA)
	e.exec(t, `UPDATE fuente_versiones SET oficial_verificada = true, oficial_entidad = 'MINSA', oficial_nota = 'nota', oficial_at = NOW()
		WHERE fuente_id = $1 AND numero = 1`, id)

	status, body := e.version(t, id, contenidoUnico())
	if status != http.StatusAccepted || body["estado"] != "activa" {
		t.Fatalf("got %d %v", status, body)
	}
	vv := body["version_vigente"].(map[string]any)
	ep := body["version_en_proceso"].(map[string]any)
	v2 := body["versiones"].([]any)[0].(map[string]any)
	if vv["numero"] != 1.0 || ep["numero"] != 2.0 || ep["estado"] != "procesando" || v2["numero"] != 2.0 || v2["oficial_verificada"] != false ||
		body["oficial"].(map[string]any)["entidad_emisora"] != "MINSA" {
		t.Errorf("detalle = %v", body)
	}
	var detalle []byte
	if err := e.db.QueryRow(`SELECT detalle FROM audit_log WHERE accion = 'fuente.version_crear' AND entidad_id = $1 AND usuario_id = $2`, id, e.adminID).Scan(&detalle); err != nil {
		t.Fatal(err)
	}
	var d map[string]any
	_ = json.Unmarshal(detalle, &d)
	if d["version"] != 2.0 || d["estado_anterior"] != "activa" || d["estado_nuevo"] != "activa" || d["version_archivada"] != nil || d["sha256"] == "" {
		t.Errorf("audit detalle = %v", d)
	}
	// A second upload while v2 is in process.
	if status, body := e.version(t, id, contenidoUnico()); status != http.StatusConflict || codigoDe(body) != "estado_invalido" {
		t.Errorf("second version in process: %d %v", status, body)
	}
}

// S2 per state of the fuente, and its errors in order.
func TestS2_EstadosYErrores(t *testing.T) {
	e := nuevoSubidaEnv(t, nil)
	ctx := context.Background()
	procesando := e.crear(t, contenidoUnico())
	retirada := e.activa(t, contenidoUnico())
	e.exec(t, `UPDATE fuentes SET estado = 'retirada', motivo_retiro = 'prueba' WHERE id = $1`, retirada)
	fallida := e.crear(t, contenidoUnico(), e.espA)
	if err := e.publicar.MarcarVersionFallida(ctx, fallida, 1, fuentes.MotivoSinContenido, nil); err != nil {
		t.Fatal(err)
	}
	propio := contenidoUnico()
	activa := e.activa(t, propio)

	cases := []struct {
		name   string
		fuente string
		cont   []byte
		status int
		code   string
	}{
		{"fuente en procesando", fmt.Sprint(procesando), contenidoUnico(), 409, "estado_invalido"},
		{"fuente retirada", fmt.Sprint(retirada), contenidoUnico(), 409, "estado_invalido"},
		{"fuente inexistente", "999999999", contenidoUnico(), 404, "no_encontrado"},
		{"sin fuente_id", "", contenidoUnico(), 422, "validacion"},
		{"archivo antes que fuente_id", "", []byte{0xff}, 422, "archivo_invalido"},
		{"mismo hash que su propia vigente", fmt.Sprint(activa), propio, 409, "fuente_duplicada"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, body := e.subir(t, fuentes.RutaNuevaVersion, e.admin, c.cont, map[string][]string{"fuente_id": {c.fuente}})
			if status != c.status || codigoDe(body) != c.code {
				t.Errorf("got %d %v, want %d %s", status, body, c.status, c.code)
			}
		})
	}

	// A failed fuente gets v2 and goes back to 'procesando'; at the limit it is rejected.
	e.sembrar(t, 100-e.sinProfesion(t), fuentes.EstadoProcesando, []int64{e.espA})
	if status, body := e.version(t, fallida, contenidoUnico()); status != 422 || codigoDe(body) != "limite_fuentes" {
		t.Errorf("fallida at the limit: %d %v", status, body)
	}
	e.exec(t, `UPDATE fuentes SET estado = 'fallida' WHERE titulo LIKE 'Relleno %' AND id IN (SELECT fuente_id FROM fuente_especialidades WHERE especialidad_id = $1)`, e.espA)
	status, body := e.version(t, fallida, contenidoUnico())
	if status != http.StatusAccepted || body["estado"] != "procesando" || body["version_en_proceso"].(map[string]any)["numero"] != 2.0 {
		t.Errorf("fallida: %d %v", status, body)
	}
}

// sembrar20 gives an active fuente 20 live versions: v1 current, v2..v20
// failed; v2 holds the returned content's hash.
func sembrar20(t *testing.T, e *subidaEnv) (int64, []byte) {
	t.Helper()
	{
		v2 := contenidoUnico()
		id := e.activa(t, contenidoUnico())
		sha2, _, _ := fuentes.Huella(v2)
		e.exec(t, `INSERT INTO fuente_versiones (fuente_id, numero, estado, motivo_fallo, nombre_archivo, sha256, tamano_bytes, lineas, contenido)
			SELECT $1, g, 'fallida', 'sin_contenido', 'v.md',
			       CASE WHEN g = 2 THEN $2 ELSE encode(sha256(convert_to('v' || g || $3, 'UTF8')), 'hex') END, 1, 1, 'x'
			FROM generate_series(2, 20) g`, id, sha2, dbtest.Uniq())
		return id, v2
	}
}

// SIA-E2-06 regla 2: with 20 live versions the oldest one that is not
// current, not in process and not cited is archived and keeps its hash.
func TestS2_LimiteDeVersiones(t *testing.T) {
	e := nuevoSubidaEnv(t, nil)
	id, v2 := sembrar20(t, e)
	status, body := e.version(t, id, contenidoUnico())
	if status != http.StatusAccepted || body["version_en_proceso"].(map[string]any)["numero"] != 21.0 {
		t.Fatalf("got %d %v", status, body)
	}
	var estado string
	var contenidoNulo bool
	if err := e.db.QueryRow(`SELECT estado, contenido IS NULL FROM fuente_versiones WHERE fuente_id = $1 AND numero = 2`, id).Scan(&estado, &contenidoNulo); err != nil {
		t.Fatal(err)
	}
	if estado != "archivada" || !contenidoNulo {
		t.Errorf("v2 = %s (contenido nulo %v), want archivada", estado, contenidoNulo)
	}
	var detalle []byte
	_ = e.db.QueryRow(`SELECT detalle FROM audit_log WHERE accion = 'fuente.version_crear' AND entidad_id = $1`, id).Scan(&detalle)
	if !strings.Contains(string(detalle), `"version_archivada": 2`) && !strings.Contains(string(detalle), `"version_archivada":2`) {
		t.Errorf("audit detalle = %s", detalle)
	}
	// The archived version keeps its hash: uploading it again is a duplicate.
	if status, body := e.subir(t, fuentes.RutaCrear, e.admin, v2, map[string][]string{"titulo": {"Otra"}, "tipo": {"guia"}}); status != 409 || codigoDe(body) != "fuente_duplicada" {
		t.Errorf("hash of an archived version: %d %v", status, body)
	}
}

// SIA-E2-06 regla 2: when every candidate version is cited, 422
// limite_versiones and nothing changes.
func TestS2_LimiteDeVersiones_TodasCitadas(t *testing.T) {
	citada := nuevoSubidaEnv(t, citaTodo{})
	idCitada, _ := sembrar20(t, citada)
	if status, body := citada.version(t, idCitada, contenidoUnico()); status != 422 || codigoDe(body) != "limite_versiones" {
		t.Errorf("all cited: %d %v", status, body)
	}
	var vivas int
	if err := citada.db.Get(&vivas, `SELECT COUNT(*) FROM fuente_versiones WHERE fuente_id = $1 AND estado <> 'archivada'`, idCitada); err != nil || vivas != 20 {
		t.Errorf("live versions = %d, want 20 unchanged", vivas)
	}
}
