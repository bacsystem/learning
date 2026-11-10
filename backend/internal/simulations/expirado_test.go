package simulations

import (
	"fmt"
	"net/http"
	"testing"
)

// ── H82 ───────────────────────────────────────────────────────────────────────

// GET on a simulacro whose time is over finalizes it and answers with a
// response the client can tell apart: 409 tiempo_agotado + resultado_id.
func TestH82_GetSimulacroExpiradoDevuelveTiempoAgotadoConResultado(t *testing.T) {
	e := newEnv(t, defaultOpts())
	s := e.create(t)
	mustExec(t, e, `UPDATE simulacros SET fecha_inicio = NOW() - INTERVAL '2 hours' WHERE id=$1`, s.ID)
	code, body := e.do(t, "GET", fmt.Sprintf("/simulacros/%d", s.ID), e.tok, "")
	if code != http.StatusConflict || body["error"] != "tiempo_agotado" || body["resultado_id"] != float64(s.ID) {
		t.Fatalf("GET expired: want 409 tiempo_agotado with resultado_id, got %d %v", code, body)
	}
	if code, _ := e.do(t, "GET", fmt.Sprintf("/simulacros/%d/resultado", s.ID), e.tok, ""); code != http.StatusOK {
		t.Fatalf("resultado after expiry: %d", code)
	}
}
