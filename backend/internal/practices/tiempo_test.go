package practices

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	qt "github.com/cbaciliod/learning/internal/questions/questionstest"
)

// ── H82 ───────────────────────────────────────────────────────────────────────

func (e *env) createTimed(t *testing.T, seg int) PracticaEnCursoResponse {
	t.Helper()
	w := qt.Do(e.mux, "POST", "/practicas", e.tok, e.createBody(3, "al_final", fmt.Sprint(seg)))
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var p PracticaEnCursoResponse
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	return p
}

// The client derives its deadline from the server clock, so a reload does not
// restart the countdown.
func TestH82_PracticaExponeTiempoRestante(t *testing.T) {
	e := newEnv(t, defaultOpts())
	p := e.createTimed(t, 600)
	if p.TiempoRestanteSeg == nil || *p.TiempoRestanteSeg < 595 || *p.TiempoRestanteSeg > 600 {
		t.Fatalf("tiempo_restante_seg at creation: %v", p.TiempoRestanteSeg)
	}
	mustExec(t, e, `UPDATE practicas SET fecha_inicio = fecha_inicio - INTERVAL '400 seconds' WHERE id=$1`, p.ID)
	g := e.get(t, p.ID)
	if g.TiempoRestanteSeg == nil || *g.TiempoRestanteSeg < 195 || *g.TiempoRestanteSeg > 200 {
		t.Fatalf("tiempo_restante_seg after reload: %v", g.TiempoRestanteSeg)
	}
}

func TestH82_PracticaSinLimiteNoTieneTiempoRestante(t *testing.T) {
	e := newEnv(t, defaultOpts())
	p := e.create(t, "al_final")
	_, body := e.do(t, "GET", fmt.Sprintf("/practicas/%d", p.ID), e.tok, "")
	if v, ok := body["tiempo_restante_seg"]; !ok || v != nil {
		t.Fatalf("untimed práctica: want tiempo_restante_seg null, got %v (present=%v)", v, ok)
	}
}

func TestH82_GetPracticaExpiradaDevuelveTiempoAgotadoConResultado(t *testing.T) {
	e := newEnv(t, defaultOpts())
	p := e.createTimed(t, 60)
	mustExec(t, e, `UPDATE practicas SET fecha_inicio = NOW() - INTERVAL '10 minutes' WHERE id=$1`, p.ID)
	code, body := e.do(t, "GET", fmt.Sprintf("/practicas/%d", p.ID), e.tok, "")
	if code != http.StatusConflict || body["error"] != "tiempo_agotado" || body["resultado_id"] != float64(p.ID) {
		t.Fatalf("GET expired: want 409 tiempo_agotado with resultado_id, got %d %v", code, body)
	}
	if code, _ := e.do(t, "GET", fmt.Sprintf("/practicas/%d/resultado", p.ID), e.tok, ""); code != http.StatusOK {
		t.Fatalf("resultado after expiry: %d", code)
	}
}
