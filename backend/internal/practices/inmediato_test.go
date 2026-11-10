package practices

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	qt "github.com/cbaciliod/learning/internal/questions/questionstest"
)

// ── H74 ───────────────────────────────────────────────────────────────────────

func (e *env) validar(t *testing.T, practicaID, preguntaID int64) (int, map[string]any) {
	t.Helper()
	return e.do(t, "POST", fmt.Sprintf("/practicas/%d/validar/%d", practicaID, preguntaID), e.tok, "")
}

func (e *env) get(t *testing.T, id int64) PracticaEnCursoResponse {
	t.Helper()
	w := qt.Do(e.mux, "GET", fmt.Sprintf("/practicas/%d", id), e.tok, "")
	if w.Code != http.StatusOK {
		t.Fatalf("get practica: %d %s", w.Code, w.Body.String())
	}
	var p PracticaEnCursoResponse
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	return p
}

// After validating a question in inmediato mode its key is known, so the
// selection is locked: switching to the correct answer is rejected and the
// grade (and the question statistics) use what was selected at validation.
func TestH74_InmediatoNoPermiteCambiarTrasValidar(t *testing.T) {
	e := newEnv(t, defaultOpts())
	p := e.create(t, "inmediato")
	u := e.f.Pregunta(preguntaDeTipo(t, p, "unica").ID)
	m := e.f.Pregunta(preguntaDeTipo(t, p, "multiple").ID)

	e.toggle(t, p.ID, u.ID, u.Incorrectas[0])
	if code, body := e.validar(t, p.ID, u.ID); code != http.StatusOK || body["es_correcta"] != false {
		t.Fatalf("validar: %d %v", code, body)
	}
	code, body := e.toggle(t, p.ID, u.ID, u.Correctas[0])
	if code != http.StatusConflict || body["error"] != "pregunta_validada" {
		t.Fatalf("toggle after validar: want 409 pregunta_validada, got %d %v", code, body)
	}

	// Same for multiple: no adding the missing correct option after the key is shown.
	e.toggle(t, p.ID, m.ID, m.Correctas[0])
	e.validar(t, p.ID, m.ID)
	if code, _ := e.toggle(t, p.ID, m.ID, m.Correctas[1]); code != http.StatusConflict {
		t.Fatalf("toggle multiple after validar: want 409, got %d", code)
	}

	// The locked state survives a reload.
	g := e.get(t, p.ID)
	for _, q := range g.Preguntas {
		switch q.ID {
		case u.ID:
			if !q.Validada || q.Estado != "incorrecta" || fmt.Sprint(q.RespuestaIDs) != fmt.Sprint([]int64{u.Incorrectas[0]}) ||
				fmt.Sprint(q.RespuestaCorrectaIDs) != fmt.Sprint(u.Correctas) {
				t.Fatalf("validated unica after reload: %+v", q)
			}
		case m.ID:
			if !q.Validada || fmt.Sprint(q.RespuestaCorrectaIDs) != fmt.Sprint(m.Correctas) {
				t.Fatalf("validated multiple after reload: %+v", q)
			}
		default:
			if q.Validada || q.RespuestaCorrectaIDs != nil {
				t.Fatalf("non validated question reveals its key: %+v", q)
			}
		}
	}

	r := e.finalizar(t, p.ID)
	if r.Correctas != 0 || r.Incorrectas != 2 {
		t.Fatalf("grade must use the selection at validation: %+v", r)
	}
	var aciertos int
	if err := e.f.DB.QueryRow(`SELECT COALESCE(SUM(total_aciertos),0) FROM pregunta_estadisticas WHERE pregunta_id = ANY(ARRAY[$1,$2]::bigint[])`,
		u.ID, m.ID).Scan(&aciertos); err != nil {
		t.Fatal(err)
	}
	if aciertos != 0 {
		t.Fatalf("pregunta_estadisticas inflated: %d aciertos", aciertos)
	}
}

// al_final mode keeps toggling freely (nothing was revealed).
func TestH74_AlFinalSigueSinBloqueo(t *testing.T) {
	e := newEnv(t, defaultOpts())
	p := e.create(t, "al_final")
	u := e.f.Pregunta(preguntaDeTipo(t, p, "unica").ID)
	e.toggle(t, p.ID, u.ID, u.Incorrectas[0])
	if code, _ := e.toggle(t, p.ID, u.ID, u.Correctas[0]); code != http.StatusOK {
		t.Fatalf("al_final toggle: want 200, got %d", code)
	}
}
