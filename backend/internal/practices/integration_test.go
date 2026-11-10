package practices

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/cbaciliod/learning/internal/questions"
	qt "github.com/cbaciliod/learning/internal/questions/questionstest"
)

type env struct {
	f   *qt.Fixture
	mux *http.ServeMux
	tok string
}

func newEnv(t *testing.T, opts qt.Options) *env {
	t.Helper()
	db := qt.DB(t)
	f := qt.Seed(t, db, opts)
	qsvc := questions.NewService(questions.NewRepository(db))
	svc := NewService(NewRepository(db), NewQuestionsAdapter(qsvc))
	mux := http.NewServeMux()
	Mount(mux, svc, qt.JWTSecret, qt.AllowSession)
	return &env{f: f, mux: mux, tok: f.Token(t, f.UsuarioID, "usuario")}
}

func defaultOpts() qt.Options {
	return qt.Options{
		Inscripcion: "aprobado",
		Asignaturas: []qt.AsignaturaSpec{
			{NumeroPreguntas: 2, Peso: 100, Unicas: 2, Multiples: 1},
		},
	}
}

func (e *env) do(t *testing.T, method, path, tok, body string) (int, map[string]any) {
	t.Helper()
	w := qt.Do(e.mux, method, path, tok, body)
	var m map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &m)
	return w.Code, m
}

func (e *env) createBody(n int, modo string, tiempo string) string {
	return fmt.Sprintf(`{"proceso_id":%d,"asignaturas":[{"asignatura_id":%d,"numero_preguntas":%d}],"modo_calificacion":%q,"tiempo_limite_seg":%s}`,
		e.f.ProcesoID, e.f.Asignaturas[0], n, modo, tiempo)
}

func (e *env) create(t *testing.T, modo string) PracticaEnCursoResponse {
	t.Helper()
	w := qt.Do(e.mux, "POST", "/practicas", e.tok, e.createBody(3, modo, "null"))
	if w.Code != http.StatusCreated {
		t.Fatalf("create practica: %d %s", w.Code, w.Body.String())
	}
	var p PracticaEnCursoResponse
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func (e *env) toggle(t *testing.T, practicaID, preguntaID, respuestaID int64) (int, map[string]any) {
	t.Helper()
	return e.do(t, "POST", fmt.Sprintf("/practicas/%d/responder", practicaID), e.tok,
		fmt.Sprintf(`{"pregunta_id":%d,"respuesta_id":%d}`, preguntaID, respuestaID))
}

func (e *env) finalizar(t *testing.T, id int64) PracticaResultadoResponse {
	t.Helper()
	w := qt.Do(e.mux, "POST", fmt.Sprintf("/practicas/%d/finalizar", id), e.tok, `{}`)
	if w.Code != http.StatusOK {
		t.Fatalf("finalizar: %d %s", w.Code, w.Body.String())
	}
	var r PracticaResultadoResponse
	if err := json.Unmarshal(w.Body.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	return r
}

func preguntaDeTipo(t *testing.T, p PracticaEnCursoResponse, tipo string) PreguntaPracticaItem {
	t.Helper()
	for _, q := range p.Preguntas {
		if q.Tipo == tipo {
			return q
		}
	}
	t.Fatalf("no hay pregunta tipo %s", tipo)
	return PreguntaPracticaItem{}
}

func mustExec(t *testing.T, e *env, q string, args ...any) {
	t.Helper()
	if _, err := e.f.DB.ExecContext(context.Background(), q, args...); err != nil {
		t.Fatalf("exec %q: %v", q, err)
	}
}

// ── H5 (schema, practica_respuestas) ──────────────────────────────────────────

func TestH5_SchemaRechazaParCruzadoEnPractica(t *testing.T) {
	e := newEnv(t, defaultOpts())
	p := e.create(t, "al_final")
	a, b := p.Preguntas[0], p.Preguntas[1]
	foreign := e.f.Pregunta(b.ID).Correctas[0]
	_, err := e.f.DB.Exec(`INSERT INTO practica_respuestas (practica_id, pregunta_id, asignatura_id, respuesta_id, orden) VALUES ($1,$2,$3,$4,1)`,
		p.ID, a.ID, e.f.Pregunta(a.ID).AsignaturaID, foreign)
	if err == nil {
		t.Fatalf("schema accepted (pregunta, respuesta) of different preguntas")
	}
}

// ── M2: foreign respuesta → 4xx ───────────────────────────────────────────────

func TestM2_ToggleRespuestaAjenaEs400(t *testing.T) {
	e := newEnv(t, defaultOpts())
	p := e.create(t, "al_final")
	a, b := p.Preguntas[0], p.Preguntas[1]
	if code, body := e.toggle(t, p.ID, a.ID, e.f.Pregunta(b.ID).Respuestas[0]); code != http.StatusBadRequest {
		t.Fatalf("foreign respuesta: want 400, got %d %v", code, body)
	}
}

// ── H6 ────────────────────────────────────────────────────────────────────────

func TestH6_ValidarPreguntaAjenaEs404(t *testing.T) {
	e := newEnv(t, qt.Options{Inscripcion: "aprobado", Asignaturas: []qt.AsignaturaSpec{{NumeroPreguntas: 1, Peso: 100, Unicas: 5}}})
	w := qt.Do(e.mux, "POST", "/practicas", e.tok, e.createBody(1, "inmediato", "null"))
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var p PracticaEnCursoResponse
	_ = json.Unmarshal(w.Body.Bytes(), &p)
	var outside int64
	for _, q := range e.f.Preguntas {
		if q.ID != p.Preguntas[0].ID {
			outside = q.ID
			break
		}
	}
	code, body := e.do(t, "POST", fmt.Sprintf("/practicas/%d/validar/%d", p.ID, outside), e.tok, "")
	if code != http.StatusNotFound || body["respuesta_correcta_id"] != nil {
		t.Fatalf("validar pregunta fuera de la práctica: want 404 without key, got %d %v", code, body)
	}
	// Inside the practice but nothing selected → 400, no key revealed.
	code, body = e.do(t, "POST", fmt.Sprintf("/practicas/%d/validar/%d", p.ID, p.Preguntas[0].ID), e.tok, "")
	if code != http.StatusBadRequest || body["respuesta_correcta_id"] != nil || body["explicacion"] != nil {
		t.Fatalf("validar sin selección: want 400 without key, got %d %v", code, body)
	}
	// With a selection it works.
	pq := e.f.Pregunta(p.Preguntas[0].ID)
	e.toggle(t, p.ID, pq.ID, pq.Correctas[0])
	code, body = e.do(t, "POST", fmt.Sprintf("/practicas/%d/validar/%d", p.ID, pq.ID), e.tok, "")
	if code != http.StatusOK || body["es_correcta"] != true {
		t.Fatalf("validar: %d %v", code, body)
	}
}

// ── H12 ───────────────────────────────────────────────────────────────────────

func TestH12_PracticaRequiereInscripcionAprobada(t *testing.T) {
	e := newEnv(t, qt.Options{Inscripcion: "pendiente", Asignaturas: defaultOpts().Asignaturas})
	if code, _ := e.do(t, "POST", "/practicas", e.tok, e.createBody(1, "al_final", "null")); code != http.StatusForbidden {
		t.Fatalf("sin inscripción aprobada: want 403, got %d", code)
	}
}

func TestH12_PracticaRequiereMismoNivel(t *testing.T) {
	e := newEnv(t, defaultOpts())
	mustExec(t, e, `UPDATE procesos SET nivel='oficial' WHERE id=$1`, e.f.ProcesoID)
	if code, _ := e.do(t, "POST", "/practicas", e.tok, e.createBody(1, "al_final", "null")); code != http.StatusForbidden {
		t.Fatalf("nivel distinto: want 403, got %d", code)
	}
}

func TestH12_TopeNumeroPreguntas(t *testing.T) {
	e := newEnv(t, defaultOpts())
	code, body := e.do(t, "POST", "/practicas", e.tok, e.createBody(MaxPreguntasPractica+1, "al_final", "null"))
	if code != http.StatusUnprocessableEntity || !strings.Contains(fmt.Sprint(body["error"]), "máximo") {
		t.Fatalf("tope: want 422 'máximo', got %d %v", code, body)
	}
}

// ── H31 / H35 ─────────────────────────────────────────────────────────────────

func TestH31_H35_MultipleSeCalificaPorConjuntoExacto(t *testing.T) {
	e := newEnv(t, defaultOpts())
	p := e.create(t, "al_final")
	m := e.f.Pregunta(preguntaDeTipo(t, p, "multiple").ID)
	// Mark only one of the two correct options.
	e.toggle(t, p.ID, m.ID, m.Correctas[0])
	r := e.finalizar(t, p.ID)
	if r.Correctas != 0 || r.Incorrectas != 1 || r.SinResponder != 2 {
		t.Fatalf("finalize: %+v", r)
	}
	code, body := e.do(t, "GET", fmt.Sprintf("/practicas/%d/resultado", p.ID), e.tok, "")
	if code != http.StatusOK {
		t.Fatalf("resultado: %d", code)
	}
	asg := body["por_asignatura"].([]any)[0].(map[string]any)
	if asg["correctas"] != 0.0 || asg["incorrectas"] != 1.0 || asg["sin_responder"] != 2.0 || asg["total_preguntas"] != 3.0 {
		t.Fatalf("resultado por asignatura counts rows not questions: %v", asg)
	}
}

func TestH35_ResultadoCuentaPreguntasNoFilas(t *testing.T) {
	e := newEnv(t, defaultOpts())
	p := e.create(t, "al_final")
	m := e.f.Pregunta(preguntaDeTipo(t, p, "multiple").ID)
	e.toggle(t, p.ID, m.ID, m.Correctas[0])
	e.toggle(t, p.ID, m.ID, m.Correctas[1])
	r := e.finalizar(t, p.ID)
	if r.Correctas != 1 || r.SinResponder != 2 || r.TotalPreguntas != 3 {
		t.Fatalf("finalize: %+v", r)
	}
	_, body := e.do(t, "GET", fmt.Sprintf("/practicas/%d/resultado", p.ID), e.tok, "")
	asg := body["por_asignatura"].([]any)[0].(map[string]any)
	if asg["correctas"] != 1.0 || asg["sin_responder"] != 2.0 {
		t.Fatalf("por_asignatura: %v", asg)
	}
	_, rev := e.do(t, "GET", fmt.Sprintf("/practicas/%d/revision", p.ID), e.tok, "")
	for _, x := range rev["preguntas"].([]any) {
		q := x.(map[string]any)
		if len(q["respuesta_usuario_ids"].([]any)) == 2 && q["es_correcta"] != true {
			t.Fatalf("revision multiple: %v", q)
		}
	}
}

// ── H37 ───────────────────────────────────────────────────────────────────────

func TestH37_ToggleUnicaConcurrenteDejaUnaSeleccion(t *testing.T) {
	e := newEnv(t, defaultOpts())
	p := e.create(t, "al_final")
	u := e.f.Pregunta(preguntaDeTipo(t, p, "unica").ID)
	qt.Concurrently(16, func(i int) {
		qt.Do(e.mux, "POST", fmt.Sprintf("/practicas/%d/responder", p.ID), e.tok,
			fmt.Sprintf(`{"pregunta_id":%d,"respuesta_id":%d}`, u.ID, u.Respuestas[i%len(u.Respuestas)]))
	})
	var n int
	if err := e.f.DB.QueryRow(`SELECT COUNT(*) FROM practica_respuestas WHERE practica_id=$1 AND pregunta_id=$2`, p.ID, u.ID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("unica has %d rows after concurrent toggles, want 1", n)
	}
}

func TestH37_AsignaturaRepetidaSeRechaza(t *testing.T) {
	e := newEnv(t, defaultOpts())
	body := fmt.Sprintf(`{"proceso_id":%d,"asignaturas":[{"asignatura_id":%d,"numero_preguntas":1},{"asignatura_id":%d,"numero_preguntas":1}],"modo_calificacion":"al_final"}`,
		e.f.ProcesoID, e.f.Asignaturas[0], e.f.Asignaturas[0])
	if code, _ := e.do(t, "POST", "/practicas", e.tok, body); code != http.StatusBadRequest {
		t.Fatalf("asignatura repetida: want 400, got %d", code)
	}
}

func TestH37_SchemaSinSeleccionDuplicada(t *testing.T) {
	e := newEnv(t, defaultOpts())
	p := e.create(t, "al_final")
	m := e.f.Pregunta(preguntaDeTipo(t, p, "multiple").ID)
	e.toggle(t, p.ID, m.ID, m.Correctas[0])
	_, err := e.f.DB.Exec(`INSERT INTO practica_respuestas (practica_id, pregunta_id, asignatura_id, respuesta_id, orden) VALUES ($1,$2,$3,$4,1)`,
		p.ID, m.ID, m.AsignaturaID, m.Correctas[0])
	if err == nil {
		t.Fatalf("schema accepted a duplicated selection")
	}
}

func TestDeseleccionarUltimaMultipleConservaLaPregunta(t *testing.T) {
	e := newEnv(t, defaultOpts())
	p := e.create(t, "al_final")
	m := e.f.Pregunta(preguntaDeTipo(t, p, "multiple").ID)
	e.toggle(t, p.ID, m.ID, m.Correctas[0])
	e.toggle(t, p.ID, m.ID, m.Correctas[0])
	_, body := e.do(t, "GET", fmt.Sprintf("/practicas/%d", p.ID), e.tok, "")
	if n := len(body["preguntas"].([]any)); n != 3 {
		t.Fatalf("question vanished after deselecting its last option: %d preguntas", n)
	}
}

// ── H8 ────────────────────────────────────────────────────────────────────────

func TestH8_PracticaToggleTrasTiempoLimite(t *testing.T) {
	e := newEnv(t, defaultOpts())
	w := qt.Do(e.mux, "POST", "/practicas", e.tok, e.createBody(3, "al_final", "60"))
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var p PracticaEnCursoResponse
	_ = json.Unmarshal(w.Body.Bytes(), &p)
	u := e.f.Pregunta(preguntaDeTipo(t, p, "unica").ID)
	e.toggle(t, p.ID, u.ID, u.Correctas[0])
	mustExec(t, e, `UPDATE practicas SET fecha_inicio = NOW() - INTERVAL '10 minutes' WHERE id=$1`, p.ID)
	if code, _ := e.toggle(t, p.ID, u.ID, u.Incorrectas[0]); code != http.StatusConflict {
		t.Fatalf("toggle fuera de tiempo: want 409, got %d", code)
	}
	mustExec(t, e, `UPDATE practica_respuestas SET respondida_at = NOW() WHERE practica_id=$1`, p.ID)
	r := e.finalizar(t, p.ID)
	if r.TiempoUsadoSeg != 60 || r.Correctas != 0 {
		t.Fatalf("finalize must clamp time and ignore late answers: %+v", r)
	}
}

// ── H9 / H10 ──────────────────────────────────────────────────────────────────

func TestH9_FinalizarPracticaConcurrente(t *testing.T) {
	e := newEnv(t, defaultOpts())
	for round := 1; round <= 3; round++ {
		p := e.create(t, "al_final")
		for _, q := range p.Preguntas {
			for _, rid := range e.f.Pregunta(q.ID).Correctas {
				e.toggle(t, p.ID, q.ID, rid)
			}
		}
		codes := make([]int, 16)
		qt.Concurrently(len(codes), func(i int) {
			codes[i] = qt.Do(e.mux, "POST", fmt.Sprintf("/practicas/%d/finalizar", p.ID), e.tok, `{}`).Code
		})
		ok := 0
		for _, c := range codes {
			if c == http.StatusOK {
				ok++
			}
		}
		if ok != 1 {
			t.Fatalf("round %d: finalize succeeded %d times (%v)", round, ok, codes)
		}
		var intentos int
		if err := e.f.DB.QueryRow(`SELECT COALESCE(SUM(total_intentos),0) FROM pregunta_estadisticas
			WHERE pregunta_id IN (SELECT id FROM preguntas WHERE proceso_id=$1)`, e.f.ProcesoID).Scan(&intentos); err != nil {
			t.Fatal(err)
		}
		if want := round * 3; intentos != want {
			t.Fatalf("round %d: total_intentos=%d, want %d (one per question, once)", round, intentos, want)
		}
	}
}

func TestH10_CrearPracticaConcurrente(t *testing.T) {
	e := newEnv(t, defaultOpts())
	codes := make([]int, 16)
	qt.Concurrently(len(codes), func(i int) {
		codes[i] = qt.Do(e.mux, "POST", "/practicas", e.tok, e.createBody(1, "al_final", "null")).Code
	})
	created := 0
	for _, c := range codes {
		if c == http.StatusCreated {
			created++
		} else if c != http.StatusConflict {
			t.Errorf("unexpected %d", c)
		}
	}
	if created != 1 {
		t.Fatalf("created %d practicas en_curso (%v)", created, codes)
	}
}

// ── H59 ───────────────────────────────────────────────────────────────────────

func TestH59_BatchDeleteDeduplicaYTopa(t *testing.T) {
	e := newEnv(t, defaultOpts())
	p := e.create(t, "al_final")
	e.finalizar(t, p.ID)
	code, body := e.do(t, "DELETE", "/practicas/batch", e.tok, fmt.Sprintf(`{"ids":[%d,%d]}`, p.ID, p.ID))
	if code != http.StatusNoContent {
		t.Fatalf("duplicated ids: want 204, got %d %v", code, body)
	}
	ids := make([]string, MaxBatchDelete+1)
	for i := range ids {
		ids[i] = fmt.Sprint(i + 1)
	}
	if code, _ := e.do(t, "DELETE", "/practicas/batch", e.tok, `{"ids":[`+strings.Join(ids, ",")+`]}`); code != http.StatusUnprocessableEntity {
		t.Fatalf("too many ids: want 422, got %d", code)
	}
}

// ── H58 ───────────────────────────────────────────────────────────────────────

func TestH58_OrdenEstablePractica(t *testing.T) {
	e := newEnv(t, defaultOpts())
	p := e.create(t, "al_final")
	sig := func(ps []PreguntaPracticaItem) string {
		out := ""
		for _, q := range ps {
			for _, r := range q.Respuestas {
				out += fmt.Sprintf("%d%s,", r.ID, r.Letra)
			}
		}
		return out
	}
	want := sig(p.Preguntas)
	for i := 0; i < 4; i++ {
		w := qt.Do(e.mux, "GET", fmt.Sprintf("/practicas/%d", p.ID), e.tok, "")
		var g PracticaEnCursoResponse
		_ = json.Unmarshal(w.Body.Bytes(), &g)
		if got := sig(g.Preguntas); got != want {
			t.Fatalf("reshuffled:\n%s\n%s", want, got)
		}
	}
}
