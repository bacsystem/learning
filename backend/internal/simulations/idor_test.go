package simulations

import (
	"fmt"
	"net/http"
	"testing"

	qt "github.com/cbaciliod/learning/internal/questions/questionstest"
)

// estadoSimulacro snapshots what another user could tamper with: the
// attempt's estado/fecha_fin and its stored selections.
func estadoSimulacro(t *testing.T, e *env, id int64) string {
	t.Helper()
	var estado string
	var fin any
	if err := e.f.DB.QueryRow(`SELECT estado, fecha_fin FROM simulacros WHERE id=$1`, id).Scan(&estado, &fin); err != nil {
		t.Fatal(err)
	}
	rows, err := e.f.DB.Query(`SELECT pregunta_id, respuesta_id FROM simulacro_respuesta_selecciones
		WHERE simulacro_id=$1 ORDER BY pregunta_id, respuesta_id`, id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := fmt.Sprintf("%s|%v|", estado, fin)
	for rows.Next() {
		var p, r int64
		if err := rows.Scan(&p, &r); err != nil {
			t.Fatal(err)
		}
		out += fmt.Sprintf("%d:%d,", p, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// H120: user B, with a valid session of their own, hits every per-id route of
// user A's simulacro. Every call must be 404 and A's attempt must be untouched.
func TestH120_SimulacroAjenoEs404EnTodasLasRutas(t *testing.T) {
	e := newEnv(t, defaultOpts())
	s := e.create(t)
	u := preguntaDeTipo(t, s, "unica")
	m := preguntaDeTipo(t, s, "multiple")
	pu, pm := e.f.Pregunta(u.ID), e.f.Pregunta(m.ID)
	if code := e.responder(t, s.ID, u.ID, pu.Incorrectas[0]); code != http.StatusOK {
		t.Fatalf("A responder: %d", code)
	}

	tokB := e.f.Token(t, e.f.NewUser(t, "aprobado"), "usuario")
	antes := estadoSimulacro(t, e, s.ID)

	calls := []struct{ method, path, body string }{
		{"GET", fmt.Sprintf("/simulacros/%d", s.ID), ""},
		{"POST", fmt.Sprintf("/simulacros/%d/responder", s.ID), fmt.Sprintf(`{"pregunta_id":%d,"respuesta_id":%d}`, u.ID, pu.Correctas[0])},
		{"PUT", fmt.Sprintf("/simulacros/%d/preguntas/%d/respuestas", s.ID, m.ID), fmt.Sprintf(`{"respuesta_ids":[%d,%d]}`, pm.Correctas[0], pm.Correctas[1])},
		{"POST", fmt.Sprintf("/simulacros/%d/finalizar", s.ID), `{}`},
		{"GET", fmt.Sprintf("/simulacros/%d/resultado", s.ID), ""},
		{"GET", fmt.Sprintf("/simulacros/%d/revision", s.ID), ""},
	}
	for _, c := range calls {
		if w := qt.Do(e.mux, c.method, c.path, tokB, c.body); w.Code != http.StatusNotFound {
			t.Errorf("B %s %s (en curso): want 404, got %d %s", c.method, c.path, w.Code, w.Body.String())
		}
	}
	if despues := estadoSimulacro(t, e, s.ID); despues != antes {
		t.Fatalf("B changed A's simulacro:\nantes   %s\ndespués %s", antes, despues)
	}

	// Once A finishes, the result and the review (which reveal the key) stay private.
	e.finalizar(t, s.ID)
	antes = estadoSimulacro(t, e, s.ID)
	for _, c := range calls {
		if w := qt.Do(e.mux, c.method, c.path, tokB, c.body); w.Code != http.StatusNotFound {
			t.Errorf("B %s %s (completado): want 404, got %d %s", c.method, c.path, w.Code, w.Body.String())
		}
	}
	if despues := estadoSimulacro(t, e, s.ID); despues != antes {
		t.Fatalf("B changed A's finished simulacro:\nantes   %s\ndespués %s", antes, despues)
	}
	// A still reads its own attempt.
	if w := qt.Do(e.mux, "GET", fmt.Sprintf("/simulacros/%d/revision", s.ID), e.tok, ""); w.Code != http.StatusOK {
		t.Fatalf("A revision: %d", w.Code)
	}
}
