package simulations

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
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
			{NumeroPreguntas: 2, Peso: 60, Unicas: 2},
			{NumeroPreguntas: 1, Peso: 40, Multiples: 1},
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

func (e *env) create(t *testing.T) SimulacroEnCursoResponse {
	t.Helper()
	w := qt.Do(e.mux, "POST", "/simulacros", e.tok, fmt.Sprintf(`{"proceso_id":%d}`, e.f.ProcesoID))
	if w.Code != http.StatusCreated {
		t.Fatalf("create simulacro: %d %s", w.Code, w.Body.String())
	}
	var s SimulacroEnCursoResponse
	if err := json.Unmarshal(w.Body.Bytes(), &s); err != nil {
		t.Fatal(err)
	}
	return s
}

func (e *env) responder(t *testing.T, simID, preguntaID, respuestaID int64) int {
	t.Helper()
	code, _ := e.do(t, "POST", fmt.Sprintf("/simulacros/%d/responder", simID), e.tok,
		fmt.Sprintf(`{"pregunta_id":%d,"respuesta_id":%d}`, preguntaID, respuestaID))
	return code
}

func (e *env) setRespuestas(t *testing.T, simID, preguntaID int64, ids []int64) (int, map[string]any) {
	t.Helper()
	b, _ := json.Marshal(map[string]any{"respuesta_ids": ids})
	return e.do(t, "PUT", fmt.Sprintf("/simulacros/%d/preguntas/%d/respuestas", simID, preguntaID), e.tok, string(b))
}

func (e *env) finalizar(t *testing.T, simID int64) SimulacroResultadoResponse {
	t.Helper()
	w := qt.Do(e.mux, "POST", fmt.Sprintf("/simulacros/%d/finalizar", simID), e.tok, `{}`)
	if w.Code != http.StatusOK {
		t.Fatalf("finalizar: %d %s", w.Code, w.Body.String())
	}
	var r SimulacroResultadoResponse
	if err := json.Unmarshal(w.Body.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	return r
}

// answerAllCorrect answers every question of s with its full correct set.
func (e *env) answerAllCorrect(t *testing.T, s SimulacroEnCursoResponse) {
	t.Helper()
	for _, p := range s.Preguntas {
		if code, body := e.setRespuestas(t, s.ID, p.ID, e.f.Pregunta(p.ID).Correctas); code != http.StatusOK {
			t.Fatalf("set respuestas: %d %v", code, body)
		}
	}
}

func preguntaDeTipo(t *testing.T, s SimulacroEnCursoResponse, tipo string) PreguntaSimulacroItem {
	t.Helper()
	for _, p := range s.Preguntas {
		if p.Tipo == tipo {
			return p
		}
	}
	t.Fatalf("no hay pregunta tipo %s", tipo)
	return PreguntaSimulacroItem{}
}

// ── H5 ────────────────────────────────────────────────────────────────────────

func TestH5_ResponderRechazaRespuestaDeOtraPregunta(t *testing.T) {
	e := newEnv(t, defaultOpts())
	s := e.create(t)
	a, b := s.Preguntas[0], s.Preguntas[1]
	foreign := e.f.Pregunta(b.ID).Correctas[0]
	if code := e.responder(t, s.ID, a.ID, foreign); code != http.StatusUnprocessableEntity {
		t.Fatalf("respuesta de otra pregunta: want 422, got %d", code)
	}
	if code, _ := e.setRespuestas(t, s.ID, a.ID, []int64{foreign}); code != http.StatusUnprocessableEntity {
		t.Fatalf("PUT con respuesta de otra pregunta: want 422, got %d", code)
	}
	// The cheat (same key for every question) must not score.
	for _, p := range s.Preguntas {
		e.responder(t, s.ID, p.ID, foreign)
	}
	if r := e.finalizar(t, s.ID); r.NotaCentesimal == 100 {
		t.Fatalf("one correct id for every question scored 100")
	}
}

func TestH5_SchemaRechazaParPreguntaRespuestaCruzado(t *testing.T) {
	e := newEnv(t, defaultOpts())
	s := e.create(t)
	a, b := s.Preguntas[0], s.Preguntas[1]
	foreign := e.f.Pregunta(b.ID).Correctas[0]
	_, err := e.f.DB.Exec(`INSERT INTO simulacro_respuesta_selecciones (simulacro_id, pregunta_id, respuesta_id)
		VALUES ($1, $2, $3)`, s.ID, a.ID, foreign)
	if err == nil {
		t.Fatalf("schema accepted (pregunta, respuesta) of different preguntas")
	}
}

// ── H8 ────────────────────────────────────────────────────────────────────────

func TestH8_ResponderRechazaTrasTiempoLimite(t *testing.T) {
	e := newEnv(t, defaultOpts())
	s := e.create(t)
	p := preguntaDeTipo(t, s, "unica")
	mustExec(t, e, `UPDATE simulacros SET fecha_inicio = NOW() - make_interval(secs => tiempo_limite_seg + 30) WHERE id=$1`, s.ID)
	if code := e.responder(t, s.ID, p.ID, e.f.Pregunta(p.ID).Correctas[0]); code != http.StatusConflict {
		t.Fatalf("responder fuera de tiempo: want 409, got %d", code)
	}
}

func TestH8_ResponderDentroDeGraciaSeAcepta(t *testing.T) {
	e := newEnv(t, defaultOpts())
	s := e.create(t)
	p := preguntaDeTipo(t, s, "unica")
	// 2 seconds past the limit: inside the grace window.
	mustExec(t, e, `UPDATE simulacros SET fecha_inicio = NOW() - make_interval(secs => tiempo_limite_seg + 2) WHERE id=$1`, s.ID)
	if code := e.responder(t, s.ID, p.ID, e.f.Pregunta(p.ID).Correctas[0]); code != http.StatusOK {
		t.Fatalf("responder dentro de la gracia: want 200, got %d", code)
	}
}

func TestH8_FinalizarAcotaFechaFinEIgnoraRespuestasTardias(t *testing.T) {
	e := newEnv(t, defaultOpts())
	s := e.create(t)
	e.answerAllCorrect(t, s)
	// Pretend the exam started 2 hours ago (limit 1 h) and the answers came late.
	mustExec(t, e, `UPDATE simulacros SET fecha_inicio = NOW() - INTERVAL '2 hours' WHERE id=$1`, s.ID)
	r := e.finalizar(t, s.ID)
	if r.TiempoUsadoSeg != r.TiempoLimiteSeg {
		t.Fatalf("tiempo_usado %d != tiempo_limite %d", r.TiempoUsadoSeg, r.TiempoLimiteSeg)
	}
	if r.Correctas != 0 || r.SinResponder != r.TotalPreguntas {
		t.Fatalf("late answers were graded: %+v", r)
	}
}

func TestH8_ExpiradoSeFinalizaAlConsultar(t *testing.T) {
	e := newEnv(t, defaultOpts())
	s := e.create(t)
	mustExec(t, e, `UPDATE simulacros SET fecha_inicio = NOW() - INTERVAL '2 hours' WHERE id=$1`, s.ID)
	code, body := e.do(t, "GET", "/simulacros/en-curso", e.tok, "")
	if code != http.StatusOK || body["en_curso"] != false {
		t.Fatalf("en-curso after expiry: %d %v", code, body)
	}
	var estado string
	if err := e.f.DB.QueryRow(`SELECT estado FROM simulacros WHERE id=$1`, s.ID).Scan(&estado); err != nil {
		t.Fatal(err)
	}
	if estado != "completado" {
		t.Fatalf("expired simulacro still %q", estado)
	}
	// And a new one can be created right away.
	e.create(t)
}

// ── H9 ────────────────────────────────────────────────────────────────────────

func TestH9_FinalizarConcurrenteSoloUnaVez(t *testing.T) {
	e := newEnv(t, defaultOpts())
	// Several rounds make the race practically certain to show up if unguarded.
	const rounds, n = 3, 16
	for round := 1; round <= rounds; round++ {
		s := e.create(t)
		e.answerAllCorrect(t, s)
		codes := make([]int, n)
		qt.Concurrently(n, func(i int) {
			codes[i] = qt.Do(e.mux, "POST", fmt.Sprintf("/simulacros/%d/finalizar", s.ID), e.tok, `{}`).Code
		})
		ok := 0
		for _, c := range codes {
			if c == http.StatusOK {
				ok++
			} else if c != http.StatusBadRequest {
				t.Errorf("unexpected status %d", c)
			}
		}
		if ok != 1 {
			t.Fatalf("round %d: finalize succeeded %d times, want exactly 1 (%v)", round, ok, codes)
		}
		var intentos int
		if err := e.f.DB.QueryRow(`SELECT COALESCE(SUM(total_intentos),0) FROM pregunta_estadisticas
			WHERE pregunta_id IN (SELECT id FROM preguntas WHERE proceso_id=$1)`, e.f.ProcesoID).Scan(&intentos); err != nil {
			t.Fatal(err)
		}
		// H38: one attempt per question (the multiple has 2 selected rows but counts once).
		if want := round * len(s.Preguntas); intentos != want {
			t.Fatalf("round %d: total_intentos=%d, want %d (one per question, recorded once)", round, intentos, want)
		}
	}
}

// ── H10 ───────────────────────────────────────────────────────────────────────

func TestH10_CrearConcurrenteSoloUnoEnCurso(t *testing.T) {
	e := newEnv(t, defaultOpts())
	const n = 16
	codes := make([]int, n)
	qt.Concurrently(n, func(i int) {
		codes[i] = qt.Do(e.mux, "POST", "/simulacros", e.tok, fmt.Sprintf(`{"proceso_id":%d}`, e.f.ProcesoID)).Code
	})
	created := 0
	for _, c := range codes {
		switch c {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
		default:
			t.Errorf("unexpected status %d", c)
		}
	}
	var enCurso int
	if err := e.f.DB.QueryRow(`SELECT COUNT(*) FROM simulacros WHERE usuario_id=$1 AND estado='en_curso'`, e.f.UsuarioID).Scan(&enCurso); err != nil {
		t.Fatal(err)
	}
	if created != 1 || enCurso != 1 {
		t.Fatalf("created=%d en_curso=%d, want 1/1 (%v)", created, enCurso, codes)
	}
}

func TestH10_SchemaUnSoloEnCursoPorUsuario(t *testing.T) {
	e := newEnv(t, defaultOpts())
	s := e.create(t)
	_, err := e.f.DB.Exec(`INSERT INTO simulacros (usuario_id, institucion_id, proceso_id, configuracion_id, tiempo_limite_seg)
		SELECT usuario_id, institucion_id, proceso_id, configuracion_id, tiempo_limite_seg FROM simulacros WHERE id=$1`, s.ID)
	if err == nil {
		t.Fatalf("schema accepted a second en_curso simulacro")
	}
	if _, err := e.f.DB.Exec(`UPDATE simulacros SET estado='bogus' WHERE id=$1`, s.ID); err == nil {
		t.Fatalf("schema accepted an unknown estado")
	}
}

// ── H32 ───────────────────────────────────────────────────────────────────────

func TestH32_SeleccionMultiple(t *testing.T) {
	e := newEnv(t, defaultOpts())
	s := e.create(t)
	m := preguntaDeTipo(t, s, "multiple")
	u := preguntaDeTipo(t, s, "unica")
	pm := e.f.Pregunta(m.ID)

	// Full correct set; PUT is idempotent.
	for i := 0; i < 2; i++ {
		code, body := e.setRespuestas(t, s.ID, m.ID, pm.Correctas)
		if code != http.StatusOK {
			t.Fatalf("PUT set: %d %v", code, body)
		}
		if got := toIDs(body["respuesta_ids"]); !sameSet(got, pm.Correctas) {
			t.Fatalf("PUT returned %v, want %v", got, pm.Correctas)
		}
	}
	// GET reflects all selections.
	var got SimulacroEnCursoResponse
	w := qt.Do(e.mux, "GET", fmt.Sprintf("/simulacros/%d", s.ID), e.tok, "")
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	for _, p := range got.Preguntas {
		if p.ID == m.ID && (!sameSet(p.RespuestaIDs, pm.Correctas) || p.Estado != "respondida") {
			t.Fatalf("GET multiple: %+v", p)
		}
	}
	// 'unica' rejects more than one selection.
	pu := e.f.Pregunta(u.ID)
	if code, _ := e.setRespuestas(t, s.ID, u.ID, pu.Respuestas[:2]); code != http.StatusUnprocessableEntity {
		t.Fatalf("unica con 2 selecciones: want 422, got %d", code)
	}
	// Legacy single endpoint still works for 'unica'.
	if code := e.responder(t, s.ID, u.ID, pu.Correctas[0]); code != http.StatusOK {
		t.Fatalf("POST responder unica: %d", code)
	}

	r := e.finalizar(t, s.ID)
	if r.Correctas != 2 || r.SinResponder != 1 {
		t.Fatalf("result: %+v", r)
	}

	// Review returns all selections and all correct ids.
	var rev RevisionSimulacroResponse
	w = qt.Do(e.mux, "GET", fmt.Sprintf("/simulacros/%d/revision", s.ID), e.tok, "")
	if err := json.Unmarshal(w.Body.Bytes(), &rev); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, p := range rev.Preguntas {
		if p.PreguntaID == m.ID {
			found = true
			if !sameSet(p.RespuestaUsuarioIDs, pm.Correctas) || !sameSet(p.RespuestaCorrectaIDs, pm.Correctas) ||
				p.EsCorrecta == nil || !*p.EsCorrecta || p.Tipo != "multiple" {
				t.Fatalf("revision multiple: %+v", p)
			}
		}
	}
	if !found {
		t.Fatalf("multiple not in revision")
	}
}

func TestH31_SimulacroMultipleSubconjuntoEsIncorrecta(t *testing.T) {
	e := newEnv(t, defaultOpts())
	s := e.create(t)
	m := preguntaDeTipo(t, s, "multiple")
	pm := e.f.Pregunta(m.ID)
	if code, _ := e.setRespuestas(t, s.ID, m.ID, pm.Correctas[:1]); code != http.StatusOK {
		t.Fatalf("set: %d", code)
	}
	r := e.finalizar(t, s.ID)
	if r.Correctas != 0 || r.Incorrectas != 1 {
		t.Fatalf("1 of 2 correct options graded as correct: %+v", r)
	}
}

// ── H33 ───────────────────────────────────────────────────────────────────────

func TestH33_NotaUsaSnapshotDeCreacion(t *testing.T) {
	e := newEnv(t, qt.Options{
		Inscripcion: "aprobado",
		Asignaturas: []qt.AsignaturaSpec{{NumeroPreguntas: 2, Peso: 100, Unicas: 4}},
	})
	s := e.create(t)
	e.answerAllCorrect(t, s)
	// Admin changes the configuration while the attempt is running.
	mustExec(t, e, `UPDATE proceso_asignaturas SET numero_preguntas=4, peso_porcentual=10 WHERE proceso_id=$1`, e.f.ProcesoID)
	mustExec(t, e, `UPDATE configuracion_examen SET nota_minima=99, nota_maxima=20 WHERE proceso_id=$1`, e.f.ProcesoID)
	r := e.finalizar(t, s.ID)
	if r.NotaCentesimal != 100 || !r.Aprobado || r.PuntajeTotal != 100 || r.PuntajeMaximo != 100 {
		t.Fatalf("grade used live config instead of snapshot: %+v", r)
	}
	if len(r.PorAsignatura) != 1 || r.PorAsignatura[0].TotalPreguntas != 2 {
		t.Fatalf("por_asignatura: %+v", r.PorAsignatura)
	}
}

func TestH34_PuntajePorAsignaturaSumaTotal(t *testing.T) {
	e := newEnv(t, qt.Options{
		Inscripcion: "aprobado",
		Asignaturas: []qt.AsignaturaSpec{
			{NumeroPreguntas: 3, Peso: 50, Coeficiente: 2, Unicas: 3},
			{NumeroPreguntas: 3, Peso: 50, Coeficiente: 1, Unicas: 3},
		},
	})
	s := e.create(t)
	for i, p := range s.Preguntas {
		if i%2 == 0 {
			e.responder(t, s.ID, p.ID, e.f.Pregunta(p.ID).Correctas[0])
		}
	}
	r := e.finalizar(t, s.ID)
	var sum float64
	for _, a := range r.PorAsignatura {
		sum += a.Puntaje
	}
	if fmt.Sprintf("%.2f", sum) != fmt.Sprintf("%.2f", r.PuntajeTotal) {
		t.Fatalf("sum por asignatura %.2f != puntaje_total %.2f", sum, r.PuntajeTotal)
	}
	got := qt.Do(e.mux, "GET", fmt.Sprintf("/simulacros/%d/resultado", s.ID), e.tok, "")
	var rr SimulacroResultadoResponse
	_ = json.Unmarshal(got.Body.Bytes(), &rr)
	if rr.PuntajeTotal != r.PuntajeTotal || rr.Correctas != r.Correctas {
		t.Fatalf("GET resultado differs from finalize: %+v vs %+v", rr, r)
	}
}

// ── H36 ───────────────────────────────────────────────────────────────────────

func TestH36_ConfiguracionDefaultsYValidacion(t *testing.T) {
	e := newEnv(t, qt.Options{SinConfig: true, Asignaturas: []qt.AsignaturaSpec{{NumeroPreguntas: 1, Peso: 100, Unicas: 1}}})
	admin := e.f.Token(t, e.f.UsuarioID, "admin")
	code, body := e.do(t, "POST", "/simulacros/configuracion", admin,
		fmt.Sprintf(`{"proceso_id":%d,"tiempo_total_seg":600,"total_preguntas":1}`, e.f.ProcesoID))
	if code != http.StatusCreated || body["nota_minima"] != 55.0 || body["nota_maxima"] != 100.0 {
		t.Fatalf("defaults: %d %v", code, body)
	}
	id := int64(body["id"].(float64))
	for _, bad := range []string{
		fmt.Sprintf(`{"proceso_id":%d,"tiempo_total_seg":600,"total_preguntas":1,"nota_minima":80,"nota_maxima":60}`, e.f.ProcesoID),
		fmt.Sprintf(`{"proceso_id":%d,"tiempo_total_seg":600,"total_preguntas":1,"nota_maxima":150}`, e.f.ProcesoID),
		fmt.Sprintf(`{"proceso_id":%d,"tiempo_total_seg":600,"total_preguntas":1,"nota_minima":-1}`, e.f.ProcesoID),
	} {
		if code, body := e.do(t, "POST", "/simulacros/configuracion", admin, bad); code != http.StatusUnprocessableEntity {
			t.Errorf("create %s: want 422, got %d %v", bad, code, body)
		}
	}
	// PATCH: absent fields keep their value; explicit 0 for nota_minima is honoured.
	code, body = e.do(t, "PATCH", fmt.Sprintf("/simulacros/configuracion/%d", id), admin, `{"nota_minima":0}`)
	if code != http.StatusOK || body["nota_minima"] != 0.0 || body["tiempo_total_seg"] != 600.0 || body["nota_maxima"] != 100.0 {
		t.Fatalf("patch: %d %v", code, body)
	}
	if code, _ := e.do(t, "PATCH", fmt.Sprintf("/simulacros/configuracion/%d", id), admin, `{"nota_minima":101}`); code != http.StatusUnprocessableEntity {
		t.Fatalf("patch nota_minima>nota_maxima: want 422, got %d", code)
	}
}

// ── H57 ───────────────────────────────────────────────────────────────────────

func TestH57_PosicionPorNotaYUsuariosDistintos(t *testing.T) {
	e := newEnv(t, qt.Options{Inscripcion: "aprobado", Asignaturas: []qt.AsignaturaSpec{{NumeroPreguntas: 2, Peso: 100, Unicas: 2}}})
	s := e.create(t)
	p := s.Preguntas[0]
	e.responder(t, s.ID, p.ID, e.f.Pregunta(p.ID).Correctas[0]) // 1/2 → nota 50

	other := e.f.NewUser(t, "aprobado")
	ins := func(usuario int64, puntaje, maximo, nota float64) {
		mustExec(t, e, `INSERT INTO simulacros (usuario_id, institucion_id, proceso_id, configuracion_id, tiempo_limite_seg,
			fecha_fin, puntaje_total, puntaje_maximo, nota_centesimal, aprobado, estado)
			VALUES ($1,$2,$3,$4,3600,NOW(),$5,$6,$7,false,'completado')`,
			usuario, e.f.InstitucionID, e.f.ProcesoID, e.f.ConfigID, puntaje, maximo, nota)
	}
	// Other user: two attempts, both better than 50 → counts as ONE user ahead.
	ins(other, 80, 100, 80)
	ins(other, 90, 100, 90)
	// Third user: huge raw puntaje on a different scale but a worse nota → not ahead.
	third := e.f.NewUser(t, "aprobado")
	ins(third, 500, 1000, 40)

	r := e.finalizar(t, s.ID)
	if r.Posicion == nil || r.Posicion.Posicion != 2 || r.Posicion.TotalUsuarios != 3 {
		t.Fatalf("posicion: %+v", r.Posicion)
	}
}

// ── H58 ───────────────────────────────────────────────────────────────────────

func TestH58_OrdenDeAlternativasEstable(t *testing.T) {
	e := newEnv(t, defaultOpts())
	s := e.create(t)
	order := func() string {
		w := qt.Do(e.mux, "GET", fmt.Sprintf("/simulacros/%d", s.ID), e.tok, "")
		var g SimulacroEnCursoResponse
		_ = json.Unmarshal(w.Body.Bytes(), &g)
		out := ""
		for _, p := range g.Preguntas {
			for _, r := range p.Respuestas {
				out += fmt.Sprintf("%d%s,", r.ID, r.Letra)
			}
		}
		return out
	}
	first := ""
	for _, p := range s.Preguntas {
		for _, r := range p.Respuestas {
			first += fmt.Sprintf("%d%s,", r.ID, r.Letra)
		}
	}
	for i := 0; i < 4; i++ {
		if got := order(); got != first {
			t.Fatalf("alternatives reshuffled between requests:\n%s\n%s", first, got)
		}
	}
}

// ── M3 ────────────────────────────────────────────────────────────────────────

func TestM3_HistorialPorcentajeYPuntajeMaximo(t *testing.T) {
	e := newEnv(t, qt.Options{Inscripcion: "aprobado", Asignaturas: []qt.AsignaturaSpec{{NumeroPreguntas: 2, Peso: 100, Unicas: 2}}})
	s := e.create(t)
	p := s.Preguntas[0]
	e.responder(t, s.ID, p.ID, e.f.Pregunta(p.ID).Correctas[0])
	e.finalizar(t, s.ID)
	mustExec(t, e, `UPDATE simulacros SET nota_maxima = 20, nota_centesimal = 10 WHERE id=$1`, s.ID)
	code, body := e.do(t, "GET", "/simulacros/historial", e.tok, "")
	if code != http.StatusOK {
		t.Fatalf("historial: %d", code)
	}
	item := body["data"].([]any)[0].(map[string]any)
	if item["porcentaje"] != 50.0 || item["puntaje_maximo"] != 100.0 {
		t.Fatalf("historial item: %v", item)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func mustExec(t *testing.T, e *env, q string, args ...any) {
	t.Helper()
	if _, err := e.f.DB.ExecContext(context.Background(), q, args...); err != nil {
		t.Fatalf("exec %q: %v", q, err)
	}
}

func toIDs(v any) []int64 {
	arr, _ := v.([]any)
	out := make([]int64, 0, len(arr))
	for _, x := range arr {
		out = append(out, int64(x.(float64)))
	}
	return out
}

func sameSet(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	x := append([]int64(nil), a...)
	y := append([]int64(nil), b...)
	sort.Slice(x, func(i, j int) bool { return x[i] < x[j] })
	sort.Slice(y, func(i, j int) bool { return y[i] < y[j] })
	for i := range x {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}

// ── H136 ──────────────────────────────────────────────────────────────────────

func TestH136_PorcentajeHistorialIgualAlDeResultado(t *testing.T) {
	e := newEnv(t, qt.Options{Inscripcion: "aprobado", Asignaturas: []qt.AsignaturaSpec{
		{NumeroPreguntas: 2, Peso: 80, Unicas: 2},
		{NumeroPreguntas: 2, Peso: 20, Unicas: 2},
	}})
	s := e.create(t)
	for _, p := range s.Preguntas {
		q := e.f.Pregunta(p.ID)
		resp := q.Incorrectas[0]
		if q.AsignaturaID == e.f.Asignaturas[0] {
			resp = q.Correctas[0]
		}
		if code := e.responder(t, s.ID, p.ID, resp); code != http.StatusOK {
			t.Fatalf("responder: %d", code)
		}
	}
	r := e.finalizar(t, s.ID)
	code, body := e.do(t, "GET", "/simulacros/historial", e.tok, "")
	if code != http.StatusOK {
		t.Fatalf("historial: %d", code)
	}
	hist := body["data"].([]any)[0].(map[string]any)["porcentaje"]
	// Weighted: 2/2 of peso 80 + 0/2 of peso 20 = 80 % (the nota), not 2/4 = 50 %.
	if r.Porcentaje != 80 || hist != r.Porcentaje {
		t.Fatalf("porcentaje: resultado %.2f, historial %v; want both 80", r.Porcentaje, hist)
	}
	w := qt.Do(e.mux, "GET", fmt.Sprintf("/simulacros/%d/resultado", s.ID), e.tok, "")
	var rr SimulacroResultadoResponse
	if err := json.Unmarshal(w.Body.Bytes(), &rr); err != nil {
		t.Fatal(err)
	}
	if rr.Porcentaje != 80 {
		t.Fatalf("GET resultado porcentaje %.2f, want 80", rr.Porcentaje)
	}
}
