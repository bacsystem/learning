package balotario

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cbaciliod/learning/internal/fuentes"
	qt "github.com/cbaciliod/learning/internal/questions/questionstest"
)

func rutaConfirmar(id int64) string {
	return fmt.Sprintf("/admin/fuentes/%d/balotario/importaciones", id)
}

// cuerpo is the body of a confirmation of the version for the fixture.
func (x *fx) cuerpo(version int) string {
	return fmt.Sprintf(`{"version":%d,"proceso_id":%d,"especialidad_id":%d,"asignatura_id":%d}`,
		version, x.f.ProcesoID, x.f.EspecialidadID, x.f.Asignaturas[0])
}

func (x *fx) post(path, token, body string) (int, map[string]any) {
	w := qt.Do(x.mux, "POST", path, token, body)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

// otraEspecialidad creates an especialidad in the institution of the fixture.
func (x *fx) otraEspecialidad(t *testing.T, estado string) int64 {
	t.Helper()
	var id int64
	if err := x.db.Get(&id, `INSERT INTO especialidades (institucion_id, nombre, codigo, nivel, estado) VALUES ($1, $2, $3, 'suboficial', $4) RETURNING id`,
		x.f.InstitucionID, "Otra "+x.f.Suffix+estado, "O"+x.f.Suffix+estado, estado); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		// A fuente may list it (fuente_especialidades): the link goes first.
		for _, q := range []string{`DELETE FROM fuente_especialidades WHERE especialidad_id = $1`, `DELETE FROM especialidades WHERE id = $1`} {
			if _, err := x.db.Exec(q, id); err != nil {
				t.Errorf("cleanup %q: %v", q, err)
			}
		}
	})
	return id
}

func (x *fx) contar(t *testing.T, consulta string, args ...any) int {
	t.Helper()
	var n int
	if err := x.db.Get(&n, consulta, args...); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestConfirmar_Validaciones(t *testing.T) {
	x := nuevoFx(t)
	contenido := bloques(2)
	activo := x.publicar(t, "balotario", contenido, [][2]int{{1, 8}})
	guia := x.publicar(t, "guia", contenido+" ", [][2]int{{1, 8}})
	retirada := x.publicar(t, "balotario", contenido+"  ", [][2]int{{1, 8}})
	if _, err := x.db.Exec(`UPDATE fuentes SET estado = 'retirada', motivo_retiro = 'prueba' WHERE id = $1`, retirada); err != nil {
		t.Fatal(err)
	}
	inactiva := x.otraEspecialidad(t, "inactivo")
	ajena := x.otraEspecialidad(t, "activo")
	soloAjena := x.publicarPara(t, "balotario", contenido+"   ", [][2]int{{1, 8}}, []int64{ajena})
	enorme := x.publicar(t, "balotario", bloques(1005), [][2]int{{1, 4020}})
	sinValidas := x.publicar(t, "balotario", sinClave(1, 2), [][2]int{{1, 6}})
	var asignaturaSuelta int64
	if err := x.db.Get(&asignaturaSuelta, `INSERT INTO asignaturas (nombre, codigo) VALUES ($1, $2) RETURNING id`, "Suelta "+x.f.Suffix, "S"+x.f.Suffix); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = x.db.Exec(`DELETE FROM asignaturas WHERE id = $1`, asignaturaSuelta) })

	con := func(campo string, valor any) string {
		var m map[string]any
		_ = json.Unmarshal([]byte(x.cuerpo(1)), &m)
		m[campo] = valor
		b, _ := json.Marshal(m)
		return string(b)
	}
	tests := []struct {
		name    string
		path    string
		token   string
		body    string
		status  int
		code    string
		detalle string // a fragment the detail must contain
	}{
		{"sin token", rutaConfirmar(activo), "", x.cuerpo(1), 401, "", ""},
		{"usuario no admin", rutaConfirmar(activo), x.usuario, x.cuerpo(1), 403, "", ""},
		{"id invalido", "/admin/fuentes/0/balotario/importaciones", x.admin, x.cuerpo(1), 400, "parametro_invalido", ""},
		{"json roto", rutaConfirmar(activo), x.admin, `{`, 400, "cuerpo_invalido", ""},
		{"campo desconocido", rutaConfirmar(activo), x.admin, con("otro", 1), 400, "cuerpo_invalido", ""},
		{"faltan campos", rutaConfirmar(activo), x.admin, `{"version":1}`, 422, "validacion", "proceso_id"},
		{"campo en cero", rutaConfirmar(activo), x.admin, con("asignatura_id", 0), 422, "validacion", "asignatura_id"},
		{"fuente inexistente", rutaConfirmar(987654321), x.admin, x.cuerpo(1), 404, "no_encontrado", ""},
		{"no es balotario", rutaConfirmar(guia), x.admin, x.cuerpo(1), 422, "tipo_fuente_invalido", ""},
		{"retirada", rutaConfirmar(retirada), x.admin, x.cuerpo(1), 422, "fuente_no_activa", "retirada"},
		{"version desactualizada", rutaConfirmar(activo), x.admin, x.cuerpo(7), 409, "version_desactualizada", "version_vigente:1"},
		{"proceso inexistente", rutaConfirmar(activo), x.admin, con("proceso_id", 987654321), 422, "referencia_invalida", "proceso_id"},
		{"especialidad inexistente", rutaConfirmar(activo), x.admin, con("especialidad_id", 987654321), 422, "especialidad_invalida", ""},
		{"especialidad inactiva", rutaConfirmar(activo), x.admin, con("especialidad_id", inactiva), 422, "especialidad_invalida", ""},
		{"especialidad fuera de las de la fuente", rutaConfirmar(soloAjena), x.admin, x.cuerpo(1), 422, "especialidad_no_corresponde", fmt.Sprint(ajena)},
		{"area no asignada al proceso", rutaConfirmar(activo), x.admin, con("asignatura_id", asignaturaSuelta), 422, "referencia_invalida", "asignatura_id"},
		{"mas de 1000 preguntas", rutaConfirmar(enorme), x.admin, x.cuerpo(1), 422, "limite_preguntas", "detectadas:1005"},
		{"sin preguntas validas", rutaConfirmar(sinValidas), x.admin, x.cuerpo(1), 422, "sin_validas", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, body := x.post(tt.path, tt.token, tt.body)
			if status != tt.status {
				t.Fatalf("status = %d, want %d (%v)", status, tt.status, body)
			}
			if tt.code != "" && body["error"] != tt.code {
				t.Errorf("error = %v, want %s", body["error"], tt.code)
			}
			if tt.detalle != "" && !strings.Contains(fmt.Sprint(body["detalle"]), tt.detalle) {
				t.Errorf("detalle = %v, want it to contain %q", body["detalle"], tt.detalle)
			}
		})
	}
	// None of the refusals created anything.
	if n := x.contar(t, `SELECT COUNT(*) FROM balotario_importaciones WHERE fuente_id > $1`, activo-1); n != 0 {
		t.Errorf("importaciones creadas por las negativas: %d", n)
	}
}

func TestConfirmar_CreaBorradoresConTrazabilidad(t *testing.T) {
	x := nuevoFx(t)
	// 3 questions of 4 lines (1–4, 5–8, 9–12) and one without key; the passages
	// overlap them: p1 = 1–6, p2 = 7–12, p3 = 13–16.
	contenido := bloques(3) + "4. Marca doble\nA) Uno\n<mark>B) Dos</mark>\n<mark>C) Tres</mark>\n5. Sin clave\nA) Uno\nB) Dos\n"
	id := x.publicar(t, "balotario", contenido, [][2]int{{1, 6}, {7, 12}, {13, 16}, {17, 20}})

	status, body := x.post(rutaConfirmar(id), x.admin, x.cuerpo(1))
	if status != http.StatusCreated {
		t.Fatalf("status = %d (%v)", status, body)
	}
	if got := fmt.Sprint(body); !strings.Contains(got, "creadas:4") || !strings.Contains(got, "con_problemas:1") ||
		!strings.Contains(got, "unica:3") || !strings.Contains(got, "multiple:1") || !strings.Contains(got, "con_alertas:0") ||
		body["fuente_id"] != float64(id) || body["version"] != float64(1) || body["importacion_id"] == nil {
		t.Errorf("body = %v", body)
	}
	importacionID := int64(body["importacion_id"].(float64))

	// The questions: drafts, origin balotario, the initial area, the especialidad and the answers of the file.
	type fila struct {
		ID           int64  `db:"id"`
		Enunciado    string `db:"enunciado"`
		Tipo         string `db:"tipo"`
		Estado       string `db:"estado"`
		Origen       string `db:"origen"`
		Dificultad   string `db:"dificultad"`
		Asignatura   int64  `db:"asignatura_id"`
		Especialidad *int64 `db:"especialidad_id"`
		Institucion  int64  `db:"institucion_id"`
		Proceso      int64  `db:"proceso_id"`
		Numero       int    `db:"numero"`
		Inicio       int    `db:"linea_inicio"`
		Fin          int    `db:"linea_fin"`
	}
	var filas []fila
	if err := x.db.Select(&filas, `SELECT p.id, p.enunciado, p.tipo, p.estado, p.origen, p.dificultad, p.asignatura_id, p.especialidad_id,
			p.institucion_id, p.proceso_id, pb.numero, pb.linea_inicio, pb.linea_fin
		FROM preguntas p JOIN pregunta_balotario pb ON pb.pregunta_id = p.id
		WHERE pb.importacion_id = $1 ORDER BY pb.numero`, importacionID); err != nil || len(filas) != 4 {
		t.Fatalf("filas = %d (%v)", len(filas), err)
	}
	for i, f := range filas {
		if f.Estado != "borrador" || f.Origen != "balotario" || f.Dificultad != "medio" || f.Asignatura != x.f.Asignaturas[0] ||
			f.Especialidad == nil || *f.Especialidad != x.f.EspecialidadID || f.Institucion != x.f.InstitucionID || f.Proceso != x.f.ProcesoID ||
			f.Numero != i+1 || f.Inicio != 4*i+1 || f.Fin != 4*i+4 {
			t.Errorf("pregunta %d = %+v", i+1, f)
		}
	}
	if filas[0].Enunciado != "Enunciado 1" || filas[3].Tipo != "multiple" || filas[0].Tipo != "unica" {
		t.Errorf("enunciado/tipo = %+v %+v", filas[0], filas[3])
	}
	var respuestas []string
	if err := x.db.Select(&respuestas, `SELECT orden || ':' || texto || ':' || es_correcta FROM respuestas WHERE pregunta_id = $1 ORDER BY orden`, filas[0].ID); err != nil ||
		fmt.Sprint(respuestas) != "[1:Uno:false 2:Dos:true 3:Tres:false]" {
		t.Errorf("respuestas = %v (%v)", respuestas, err)
	}

	// Each question cites the passages of its lines: 1 → p1, 2 → p1 and p2, 3 → p2, 4 → p3.
	var citas []string
	if err := x.db.Select(&citas, `SELECT pb.numero || ':' || COUNT(*) FROM pregunta_citas pc JOIN pregunta_balotario pb ON pb.pregunta_id = pc.pregunta_id
		WHERE pb.importacion_id = $1 GROUP BY pb.numero ORDER BY pb.numero`, importacionID); err != nil || fmt.Sprint(citas) != "[1:1 2:2 3:1 4:1]" {
		t.Errorf("citas = %v (%v)", citas, err)
	}
	// A draft is not served to the alumnos.
	if n := x.contar(t, `SELECT COUNT(*) FROM preguntas p WHERE p.proceso_id = $1 AND `+"p.estado IN ('aprobada','activo')", x.f.ProcesoID); n != 1 {
		t.Errorf("servibles = %d, want 1 (the seeded one)", n)
	}

	// The import record and the audit row, without text.
	var imp struct {
		Creadas      int    `db:"creadas"`
		ConProblemas int    `db:"con_problemas"`
		CreadoPor    *int64 `db:"creado_por"`
		Asignatura   int64  `db:"asignatura_id"`
	}
	if err := x.db.Get(&imp, `SELECT creadas, con_problemas, creado_por, asignatura_id FROM balotario_importaciones WHERE id = $1`, importacionID); err != nil ||
		imp.Creadas != 4 || imp.ConProblemas != 1 || imp.CreadoPor == nil || *imp.CreadoPor != x.f.UsuarioID || imp.Asignatura != x.f.Asignaturas[0] {
		t.Errorf("importacion = %+v (%v)", imp, err)
	}
	var detalle string
	if err := x.db.Get(&detalle, `SELECT detalle::text FROM audit_log WHERE accion = 'balotario.importar' AND entidad = 'fuente' AND entidad_id = $1`, id); err != nil {
		t.Fatal(err)
	}
	for _, clave := range []string{"importacion_id", "fuente_version_id", "proceso_id", "especialidad_id", "asignatura_id", `"creadas": 4`, `"con_problemas": 1`, `"unica": 3`, `"multiple": 1`, `"con_alertas": 0`} {
		if !strings.Contains(detalle, clave) {
			t.Errorf("auditoría sin %q: %s", clave, detalle)
		}
	}
	if strings.Contains(detalle, "Enunciado") || strings.Contains(detalle, "secreto") {
		t.Errorf("la auditoría lleva texto: %s", detalle)
	}

	t.Run("la segunda confirmacion es ya_importado", func(t *testing.T) {
		status, body := x.post(rutaConfirmar(id), x.admin, x.cuerpo(1))
		if status != 409 || body["error"] != "ya_importado" || !strings.Contains(fmt.Sprint(body["detalle"]), fmt.Sprint(importacionID)) {
			t.Errorf("status=%d body=%v", status, body)
		}
		if n := x.contar(t, `SELECT COUNT(*) FROM pregunta_balotario WHERE importacion_id = $1`, importacionID); n != 4 {
			t.Errorf("preguntas tras la segunda: %d", n)
		}
	})
	t.Run("una version nueva se puede importar y la vieja queda desactualizada", func(t *testing.T) {
		nueva := publicarVersion(t, x, id, bloques(1), [][2]int{{1, 4}})
		if status, body := x.post(rutaConfirmar(id), x.admin, x.cuerpo(1)); status != 409 || body["error"] != "version_desactualizada" {
			t.Errorf("version vieja: %d %v", status, body)
		}
		if status, body := x.post(rutaConfirmar(id), x.admin, x.cuerpo(nueva)); status != http.StatusCreated {
			t.Errorf("version nueva: %d %v", status, body)
		}
		_, previa := x.get(rutaPrevia(id, ""), x.admin)
		if fmt.Sprint(previa["otras_importaciones"]) == "[]" || previa["importacion"] == nil {
			t.Errorf("la vista previa no muestra las dos importaciones: %v", previa)
		}
	})
}

// publicarVersion adds a version with the content to the fuente and publishes
// it with one passage per range of lines; it returns its number.
func publicarVersion(t *testing.T, x *fx, fuenteID int64, contenido string, rangos [][2]int) int {
	t.Helper()
	sha, tamano, lineas := fuentes.Huella([]byte(contenido))
	var numero int
	if err := x.db.Get(&numero, `INSERT INTO fuente_versiones (fuente_id, numero, nombre_archivo, sha256, tamano_bytes, lineas, contenido)
		SELECT $1, COALESCE(MAX(numero), 0) + 1, 'nueva.md', $2, $3, $4, $5 FROM fuente_versiones WHERE fuente_id = $1
		RETURNING numero`, fuenteID, sha, tamano, lineas, contenido); err != nil {
		t.Fatal(err)
	}
	ps := make([]fuentes.PasajeNuevo, len(rangos))
	for i, r := range rangos {
		ps[i] = fuentes.PasajeNuevo{Orden: i + 1, LineaInicio: r[0], LineaFin: r[1], Texto: fmt.Sprintf("v%d pasaje %d", numero, i+1), Palabras: 3}
	}
	if err := x.src.PublicarVersion(context.Background(), fuenteID, numero, ps, nil); err != nil {
		t.Fatal(err)
	}
	return numero
}

// A failure in the middle of the transaction leaves nothing: here the second
// write that fails is the citation (a question over 21 passages breaks the
// invariant of E3-07 §4.2), after the import and the questions were inserted.
func TestConfirmar_FalloParcialNoDejaNada(t *testing.T) {
	x := nuevoFx(t)
	var md strings.Builder
	md.WriteString("1. Enunciado largo\n")
	for i := 0; i < 21; i++ {
		fmt.Fprintf(&md, "línea %d\n", i)
	}
	md.WriteString("A) Uno\n<mark>B) Dos</mark>\n") // lines 1–24; one passage per line
	rangos := make([][2]int, 24)
	for i := range rangos {
		rangos[i] = [2]int{i + 1, i + 1}
	}
	id := x.publicar(t, "balotario", md.String(), rangos)

	status, body := x.post(rutaConfirmar(id), x.admin, x.cuerpo(1))
	if status != http.StatusInternalServerError || body["error"] != "error_interno" {
		t.Fatalf("status = %d, body = %v", status, body)
	}
	if n := x.contar(t, `SELECT COUNT(*) FROM balotario_importaciones WHERE fuente_id = $1`, id); n != 0 {
		t.Errorf("importaciones = %d", n)
	}
	if n := x.contar(t, `SELECT COUNT(*) FROM preguntas WHERE proceso_id = $1 AND origen = 'balotario'`, x.f.ProcesoID); n != 0 {
		t.Errorf("preguntas = %d", n)
	}
	if n := x.contar(t, `SELECT COUNT(*) FROM audit_log WHERE accion = 'balotario.importar' AND entidad_id = $1`, id); n != 0 {
		t.Errorf("auditoría = %d", n)
	}
	// The error does not leak the cause.
	if strings.Contains(fmt.Sprint(body), "pasajes") {
		t.Errorf("el error filtra la causa: %v", body)
	}
}

// Two simultaneous confirmations of the same version: one wins, the other gets
// ya_importado and nothing is imported twice (idempotence by version, D10).
func TestConfirmar_ConcurrenteEsIdempotente(t *testing.T) {
	x := nuevoFx(t)
	id := x.publicar(t, "balotario", bloques(40), [][2]int{{1, 160}})

	const intentos = 4
	var wg sync.WaitGroup
	inicio := make(chan struct{})
	estados := make([]int, intentos)
	errores := make([]any, intentos)
	for i := range intentos {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-inicio
			w := qt.Do(x.mux, "POST", rutaConfirmar(id), x.admin, x.cuerpo(1))
			var body map[string]any
			_ = json.Unmarshal(w.Body.Bytes(), &body)
			estados[i], errores[i] = w.Code, body["error"]
		}()
	}
	close(inicio)
	wg.Wait()

	creadas, repetidas := 0, 0
	for i := range intentos {
		switch {
		case estados[i] == http.StatusCreated:
			creadas++
		case estados[i] == http.StatusConflict && errores[i] == "ya_importado":
			repetidas++
		default:
			t.Errorf("intento %d: %d %v", i, estados[i], errores[i])
		}
	}
	if creadas != 1 || repetidas != intentos-1 {
		t.Errorf("creadas = %d, repetidas = %d", creadas, repetidas)
	}
	if n := x.contar(t, `SELECT COUNT(*) FROM balotario_importaciones WHERE fuente_id = $1`, id); n != 1 {
		t.Errorf("importaciones = %d", n)
	}
	if n := x.contar(t, `SELECT COUNT(*) FROM preguntas WHERE proceso_id = $1 AND origen = 'balotario'`, x.f.ProcesoID); n != 40 {
		t.Errorf("preguntas = %d, want 40", n)
	}
}

// The confirmation holds the fuente FOR SHARE: the retirement of the fuente
// waits until the import ends (§4.1, step 4).
func TestBloquearFuente_EsperaAlRetiro(t *testing.T) {
	x := nuevoFx(t)
	id := x.publicar(t, "balotario", bloques(1), [][2]int{{1, 4}})
	repo := NewRepository(x.db)

	retiro := make(chan error, 1)
	err := repo.EnTransaccion(context.Background(), func(ops Operaciones) error {
		if err := ops.BloquearFuente(context.Background(), id); err != nil {
			return err
		}
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, err := x.db.ExecContext(ctx, `UPDATE fuentes SET estado = 'retirada', motivo_retiro = 'prueba' WHERE id = $1`, id)
			retiro <- err
		}()
		select {
		case err := <-retiro:
			return fmt.Errorf("el retiro no esperó a la importación (err=%v)", err)
		case <-time.After(300 * time.Millisecond):
			return nil
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := <-retiro; err != nil {
		t.Fatalf("el retiro debe completarse al terminar la importación: %v", err)
	}
}

// Contract §4.3: 1 000 questions with 6 alternatives in less than 5 s through
// the endpoint (the WriteTimeout is 15 s).
func TestConfirmar_MilPreguntasEnMenosDeCincoSegundos(t *testing.T) {
	x := nuevoFx(t)
	var md strings.Builder
	for i := 1; i <= MaxPreguntas; i++ {
		fmt.Fprintf(&md, "%d. Enunciado %d\nA) Uno\nB) Dos\nC) Tres\nD) Cuatro\nE) Cinco\n<mark>F) Seis</mark>\n", i, i)
	}
	id := x.publicar(t, "balotario", md.String(), [][2]int{{1, 7000}})

	inicio := time.Now()
	status, body := x.post(rutaConfirmar(id), x.admin, x.cuerpo(1))
	if d := time.Since(inicio); status != http.StatusCreated || d > 5*time.Second {
		t.Errorf("status = %d en %v (%v)", status, d, body)
	}
	if n := x.contar(t, `SELECT COUNT(*) FROM respuestas r JOIN pregunta_balotario pb ON pb.pregunta_id = r.pregunta_id`); n < 6000 {
		t.Errorf("respuestas = %d", n)
	}
}
