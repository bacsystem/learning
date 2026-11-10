package practices

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/cbaciliod/learning/internal/questions"
	qt "github.com/cbaciliod/learning/internal/questions/questionstest"
)

// ── H73 ───────────────────────────────────────────────────────────────────────

func dosAsignaturasOpts() qt.Options {
	return qt.Options{
		Inscripcion: "aprobado",
		Asignaturas: []qt.AsignaturaSpec{
			{NumeroPreguntas: 2, Peso: 100, Unicas: 2, Multiples: 1},
			{NumeroPreguntas: 1, Peso: 0, Unicas: 1},
		},
	}
}

func porAsignatura(t *testing.T, e *env, id int64) []any {
	t.Helper()
	code, body := e.do(t, "GET", fmt.Sprintf("/practicas/%d/resultado", id), e.tok, "")
	if code != http.StatusOK {
		t.Fatalf("resultado: %d %v", code, body)
	}
	return body["por_asignatura"].([]any)
}

func TestH73_MoverPreguntaDuranteLaPracticaNoPierdeLaRespuesta(t *testing.T) {
	e := newEnv(t, dosAsignaturasOpts())
	p := e.create(t, "al_final")
	for _, q := range p.Preguntas {
		for _, rid := range e.f.Pregunta(q.ID).Correctas {
			e.toggle(t, p.ID, q.ID, rid)
		}
	}
	mustExec(t, e, `UPDATE preguntas SET asignatura_id=$1 WHERE id=$2`, e.f.Asignaturas[1], p.Preguntas[0].ID)
	r := e.finalizar(t, p.ID)
	if r.Correctas != 3 {
		t.Fatalf("finalize: %+v", r)
	}
	asg := porAsignatura(t, e, p.ID)
	if len(asg) != 1 {
		t.Fatalf("por_asignatura: %v", asg)
	}
	a := asg[0].(map[string]any)
	if a["correctas"] != 3.0 || a["total_preguntas"] != 3.0 {
		t.Fatalf("moved answer lost from the original asignatura: %v", a)
	}
}

// H111: answering AFTER the question moved rewrites its rows through
// replaceSeleccion, which must keep the snapshot taken at creation instead of
// reading the question's current asignatura.
func TestH111_MoverLaPreguntaYLuegoResponderConservaElSnapshot(t *testing.T) {
	e := newEnv(t, dosAsignaturasOpts())
	p := e.create(t, "al_final")
	mustExec(t, e, `UPDATE preguntas SET asignatura_id=$1 WHERE id=$2`, e.f.Asignaturas[1], p.Preguntas[0].ID)
	for _, q := range p.Preguntas {
		for _, rid := range e.f.Pregunta(q.ID).Correctas {
			if code, body := e.toggle(t, p.ID, q.ID, rid); code != http.StatusOK {
				t.Fatalf("toggle: %d %v", code, body)
			}
		}
	}
	r := e.finalizar(t, p.ID)
	if r.Correctas != 3 {
		t.Fatalf("finalize: %+v", r)
	}
	asg := porAsignatura(t, e, p.ID)
	if len(asg) != 1 {
		t.Fatalf("por_asignatura: %v", asg)
	}
	a := asg[0].(map[string]any)
	if a["correctas"] != 3.0 || a["total_preguntas"] != 3.0 {
		t.Fatalf("answer given after the move left the original asignatura: %v", a)
	}
}

func TestH73_ResultadoDePracticaCompletadaNoCambiaRetroactivamente(t *testing.T) {
	e := newEnv(t, dosAsignaturasOpts())
	p := e.create(t, "al_final")
	u := e.f.Pregunta(preguntaDeTipo(t, p, "unica").ID)
	e.toggle(t, p.ID, u.ID, u.Correctas[0])
	e.finalizar(t, p.ID)
	before := fmt.Sprint(porAsignatura(t, e, p.ID))
	mustExec(t, e, `UPDATE preguntas SET asignatura_id=$1 WHERE id=$2`, e.f.Asignaturas[1], u.ID)
	if after := fmt.Sprint(porAsignatura(t, e, p.ID)); after != before {
		t.Fatalf("completed breakdown changed after moving a question:\n%s\n%s", before, after)
	}
}

func TestH73_AsignaturaSeGuardaEnCadaFila(t *testing.T) {
	e := newEnv(t, defaultOpts())
	p := e.create(t, "al_final")
	m := e.f.Pregunta(preguntaDeTipo(t, p, "multiple").ID)
	e.toggle(t, p.ID, m.ID, m.Correctas[0])
	e.toggle(t, p.ID, m.ID, m.Correctas[1])
	var malas int
	if err := e.f.DB.QueryRow(`
		SELECT COUNT(*) FROM practica_respuestas prr JOIN preguntas pq ON pq.id = prr.pregunta_id
		WHERE prr.practica_id = $1 AND prr.asignatura_id IS DISTINCT FROM pq.asignatura_id`, p.ID).Scan(&malas); err != nil {
		t.Fatal(err)
	}
	if malas != 0 {
		t.Fatalf("%d rows without the asignatura snapshot", malas)
	}
}

// ── H110 ──────────────────────────────────────────────────────────────────────

// moverTrasGenerar moves the first generated question to another asignatura
// right after GenerateSet, i.e. between the selection and the INSERT, as an
// admin edit in that window would.
type moverTrasGenerar struct {
	QuestionsService
	f       *qt.Fixture
	destino int64
}

func (m moverTrasGenerar) GenerateSet(ctx context.Context, req generateSetRequest) ([]preguntaExamenItem, error) {
	items, err := m.QuestionsService.GenerateSet(ctx, req)
	if err == nil && len(items) > 0 {
		_, err = m.f.DB.ExecContext(ctx, `UPDATE preguntas SET asignatura_id=$1 WHERE id=$2`, m.destino, items[0].ID)
	}
	return items, err
}

func newEnvMoviendoTrasGenerar(t *testing.T, opts qt.Options, destinoIdx int) *env {
	t.Helper()
	db := qt.DB(t)
	f := qt.Seed(t, db, opts)
	qsvc := NewQuestionsAdapter(questions.NewService(questions.NewRepository(db)))
	svc := NewService(NewRepository(db), moverTrasGenerar{QuestionsService: qsvc, f: f, destino: f.Asignaturas[destinoIdx]})
	mux := http.NewServeMux()
	Mount(mux, svc, qt.JWTSecret, qt.AllowSession)
	return &env{f: f, mux: mux, tok: f.Token(t, f.UsuarioID, "usuario")}
}

// The asignatura stored for each question is the one it was selected for,
// even if the question moved before the INSERT ran.
func TestH110_MoverPreguntaEntreGenerarEInsertarConservaLaAsignaturaSolicitada(t *testing.T) {
	e := newEnvMoviendoTrasGenerar(t, dosAsignaturasOpts(), 1)
	p := e.create(t, "al_final")
	var fuera int
	if err := e.f.DB.QueryRow(`
		SELECT COUNT(*) FROM practica_respuestas WHERE practica_id=$1 AND asignatura_id <> $2`,
		p.ID, e.f.Asignaturas[0]).Scan(&fuera); err != nil {
		t.Fatal(err)
	}
	if fuera != 0 {
		t.Fatalf("%d rows stored with an asignatura that was not requested", fuera)
	}
	for _, q := range p.Preguntas {
		for _, rid := range e.f.Pregunta(q.ID).Correctas {
			if code, body := e.toggle(t, p.ID, q.ID, rid); code != http.StatusOK {
				t.Fatalf("toggle: %d %v", code, body)
			}
		}
	}
	e.finalizar(t, p.ID)
	asg := porAsignatura(t, e, p.ID)
	if len(asg) != 1 {
		t.Fatalf("por_asignatura: %v", asg)
	}
	if a := asg[0].(map[string]any); a["correctas"] != 3.0 || a["total_preguntas"] != 3.0 {
		t.Fatalf("question moved before the insert left the breakdown: %v", a)
	}
}
