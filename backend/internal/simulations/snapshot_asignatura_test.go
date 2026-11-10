package simulations

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/cbaciliod/learning/internal/questions"
	qt "github.com/cbaciliod/learning/internal/questions/questionstest"
)

// ── H73 ───────────────────────────────────────────────────────────────────────

// Moving a question to another asignatura while the attempt is en_curso must
// not change its grade: the answer stays in the asignatura it had at creation.
func TestH73_MoverPreguntaDeAsignaturaDuranteElIntentoNoPierdeLaRespuesta(t *testing.T) {
	e := newEnv(t, qt.Options{
		Inscripcion: "aprobado",
		Asignaturas: []qt.AsignaturaSpec{
			{NumeroPreguntas: 2, Peso: 100, Unicas: 2},
			{NumeroPreguntas: 0, Peso: 0, Unicas: 1}, // destination, not in the exam
		},
	})
	s := e.create(t)
	e.answerAllCorrect(t, s)
	mustExec(t, e, `UPDATE preguntas SET asignatura_id=$1 WHERE id=$2`, e.f.Asignaturas[1], s.Preguntas[0].ID)

	r := e.finalizar(t, s.ID)
	if r.NotaCentesimal != 100 || r.Correctas != 2 {
		t.Fatalf("moved answer lost from the grade: nota=%v correctas=%d", r.NotaCentesimal, r.Correctas)
	}
	if len(r.PorAsignatura) != 1 || r.PorAsignatura[0].Correctas != 2 || r.PorAsignatura[0].TotalPreguntas != 2 {
		t.Fatalf("por_asignatura must keep the original asignatura: %+v", r.PorAsignatura)
	}
	_, rev := e.do(t, "GET", fmt.Sprintf("/simulacros/%d/revision", s.ID), e.tok, "")
	for _, x := range rev["preguntas"].([]any) {
		q := x.(map[string]any)
		if q["asignatura"] != r.PorAsignatura[0].Asignatura {
			t.Fatalf("revision shows the current asignatura instead of the attempt's: %v", q["asignatura"])
		}
	}
}

// The snapshot column is filled at creation for every question row.
func TestH73_AsignaturaSeGuardaAlCrear(t *testing.T) {
	e := newEnv(t, defaultOpts())
	s := e.create(t)
	var malas int
	if err := e.f.DB.QueryRow(`
		SELECT COUNT(*) FROM simulacro_respuestas srr JOIN preguntas pq ON pq.id = srr.pregunta_id
		WHERE srr.simulacro_id = $1 AND srr.asignatura_id IS DISTINCT FROM pq.asignatura_id`, s.ID).Scan(&malas); err != nil {
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
// even if the question moved before the INSERT ran: otherwise it lands in an
// asignatura missing from simulacro_asignaturas and drops out of the grade.
func TestH110_MoverPreguntaEntreGenerarEInsertarConservaLaAsignaturaSolicitada(t *testing.T) {
	e := newEnvMoviendoTrasGenerar(t, qt.Options{
		Inscripcion: "aprobado",
		Asignaturas: []qt.AsignaturaSpec{
			{NumeroPreguntas: 2, Peso: 100, Unicas: 2},
			{NumeroPreguntas: 0, Peso: 0, Unicas: 1}, // destination, not in the exam
		},
	}, 1)
	s := e.create(t)
	e.answerAllCorrect(t, s)
	r := e.finalizar(t, s.ID)
	if r.NotaCentesimal != 100 || r.Correctas != 2 {
		t.Fatalf("question moved before the insert lost from the grade: nota=%v correctas=%d", r.NotaCentesimal, r.Correctas)
	}
	if len(r.PorAsignatura) != 1 || r.PorAsignatura[0].Correctas != 2 || r.PorAsignatura[0].TotalPreguntas != 2 {
		t.Fatalf("por_asignatura must keep the requested asignatura: %+v", r.PorAsignatura)
	}
}
