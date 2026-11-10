package practices

import (
	"fmt"
	"net/http"
	"testing"

	qt "github.com/cbaciliod/learning/internal/questions/questionstest"
)

// estadoPractica snapshots what another user could tamper with: the
// práctica's estado/fecha_fin and its answer rows (selection and validation).
func estadoPractica(t *testing.T, e *env, id int64) string {
	t.Helper()
	var estado string
	var fin any
	if err := e.f.DB.QueryRow(`SELECT estado, fecha_fin FROM practicas WHERE id=$1`, id).Scan(&estado, &fin); err != nil {
		t.Fatal(err)
	}
	rows, err := e.f.DB.Query(`SELECT pregunta_id, respuesta_id, es_correcta FROM practica_respuestas
		WHERE practica_id=$1 ORDER BY pregunta_id, respuesta_id NULLS FIRST`, id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := fmt.Sprintf("%s|%v|", estado, fin)
	for rows.Next() {
		var p int64
		var r, c any
		if err := rows.Scan(&p, &r, &c); err != nil {
			t.Fatal(err)
		}
		out += fmt.Sprintf("%d:%v:%v,", p, r, c)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// H120 + H137: user B, with a valid session of their own, hits every per-id
// route of user A's práctica. Every call must be 404 (same as a missing
// práctica, like simulacros) and A's práctica must be untouched.
func TestH120_PracticaAjenaEs404EnTodasLasRutas(t *testing.T) {
	e := newEnv(t, defaultOpts())
	p := e.create(t, "inmediato")
	u := e.f.Pregunta(preguntaDeTipo(t, p, "unica").ID)
	m := e.f.Pregunta(preguntaDeTipo(t, p, "multiple").ID)
	if code, _ := e.toggle(t, p.ID, u.ID, u.Incorrectas[0]); code != http.StatusOK {
		t.Fatalf("A toggle: %d", code)
	}

	tokB := e.f.Token(t, e.f.NewUser(t, "aprobado"), "usuario")
	antes := estadoPractica(t, e, p.ID)

	calls := []struct{ method, path, body string }{
		{"GET", fmt.Sprintf("/practicas/%d", p.ID), ""},
		{"POST", fmt.Sprintf("/practicas/%d/responder", p.ID), fmt.Sprintf(`{"pregunta_id":%d,"respuesta_id":%d}`, u.ID, u.Correctas[0])},
		{"POST", fmt.Sprintf("/practicas/%d/responder", p.ID), fmt.Sprintf(`{"pregunta_id":%d,"respuesta_id":%d}`, m.ID, m.Correctas[0])},
		{"POST", fmt.Sprintf("/practicas/%d/validar/%d", p.ID, u.ID), ""},
		{"POST", fmt.Sprintf("/practicas/%d/finalizar", p.ID), `{}`},
		{"GET", fmt.Sprintf("/practicas/%d/resultado", p.ID), ""},
		{"GET", fmt.Sprintf("/practicas/%d/revision", p.ID), ""},
		{"DELETE", fmt.Sprintf("/practicas/%d", p.ID), ""},
	}
	run := func(fase string) {
		t.Helper()
		for _, c := range calls {
			if w := qt.Do(e.mux, c.method, c.path, tokB, c.body); w.Code != http.StatusNotFound {
				t.Errorf("B %s %s (%s): want 404, got %d %s", c.method, c.path, fase, w.Code, w.Body.String())
			}
		}
	}
	run("en curso")
	if despues := estadoPractica(t, e, p.ID); despues != antes {
		t.Fatalf("B changed A's práctica:\nantes   %s\ndespués %s", antes, despues)
	}

	// Once A finishes, the result and the review (which reveal the key) stay private.
	e.finalizar(t, p.ID)
	antes = estadoPractica(t, e, p.ID)
	run("completada")
	if despues := estadoPractica(t, e, p.ID); despues != antes {
		t.Fatalf("B changed A's finished práctica:\nantes   %s\ndespués %s", antes, despues)
	}
	if w := qt.Do(e.mux, "GET", fmt.Sprintf("/practicas/%d/revision", p.ID), e.tok, ""); w.Code != http.StatusOK {
		t.Fatalf("A revision: %d", w.Code)
	}
}

// H137: a missing práctica and someone else's must be indistinguishable, but
// creating one without an approved inscripción is still 403.
func TestH137_InscripcionNoAprobadaSigueSiendo403(t *testing.T) {
	e := newEnv(t, defaultOpts())
	tokC := e.f.Token(t, e.f.NewUser(t, "pendiente"), "usuario")
	if w := qt.Do(e.mux, "POST", "/practicas", tokC, e.createBody(1, "al_final", "null")); w.Code != http.StatusForbidden {
		t.Fatalf("crear sin inscripción aprobada: want 403, got %d %s", w.Code, w.Body.String())
	}
	if code, _ := e.do(t, "GET", "/practicas/999999999", e.tok, ""); code != http.StatusNotFound {
		t.Fatalf("práctica inexistente: want 404, got %d", code)
	}
}
