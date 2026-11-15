package simulations

import (
	"fmt"
	"net/http"
	"testing"
)

// PLT-06: a simulacro needs a proceso of the user's institution.

func codigoDe(body map[string]any) string {
	s, _ := body["error"].(string)
	return s
}

func (e *env) postSimulacro(t *testing.T, procesoID int64) (int, map[string]any) {
	t.Helper()
	return e.do(t, "POST", "/simulacros", e.tok, fmt.Sprintf(`{"proceso_id":%d}`, procesoID))
}

func (e *env) exec(t *testing.T, q string, args ...any) {
	t.Helper()
	if _, err := e.f.DB.Exec(q, args...); err != nil {
		t.Fatalf("exec %q: %v", q, err)
	}
}

func TestPLT06_SimulacroDeOtraInstitucionEs403AunqueEsteInscrito(t *testing.T) {
	e := newEnv(t, defaultOpts())
	otro := e.f.ProcesoDeOtraInstitucion(t)
	code, resp := e.postSimulacro(t, otro)
	if code != http.StatusForbidden || codigoDe(resp) != "institucion_no_permitida" {
		t.Fatalf("got %d %v, want 403 institucion_no_permitida", code, resp)
	}
}

func TestPLT06_CodigosDeAcceso(t *testing.T) {
	cases := []struct {
		name string
		sql  string
		code string
	}{
		{"nivel", `UPDATE procesos SET nivel='oficial' WHERE id=$1`, "nivel_no_permitido"},
		{"sin inscripcion", `DELETE FROM inscripciones WHERE proceso_id=$1`, "inscripcion_requerida"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t, defaultOpts())
			e.exec(t, c.sql, e.f.ProcesoID)
			code, resp := e.postSimulacro(t, e.f.ProcesoID)
			if code != http.StatusForbidden || codigoDe(resp) != c.code {
				t.Fatalf("got %d %v, want 403 %s", code, resp, c.code)
			}
		})
	}
}

// A simulacro already started keeps working after its proceso stops being
// eligible; only a new one is refused (P2).
func TestPLT06_SimulacroEnCursoSigueAunqueElProcesoYaNoSeaElegible(t *testing.T) {
	e := newEnv(t, defaultOpts())
	s := e.create(t)
	otro := e.f.ProcesoDeOtraInstitucion(t)
	e.exec(t, `UPDATE procesos SET institucion_id=(SELECT institucion_id FROM procesos WHERE id=$2) WHERE id=$1`,
		e.f.ProcesoID, otro)
	t.Cleanup(func() { // before the other institución is deleted
		e.exec(t, `UPDATE procesos SET institucion_id=$2 WHERE id=$1`, e.f.ProcesoID, e.f.InstitucionID)
	})
	if code, _ := e.do(t, "GET", fmt.Sprintf("/simulacros/%d", s.ID), e.tok, ""); code != http.StatusOK {
		t.Fatalf("existing simulacro: want 200, got %d", code)
	}
	code, resp := e.postSimulacro(t, e.f.ProcesoID)
	if code != http.StatusForbidden || codigoDe(resp) != "institucion_no_permitida" {
		t.Fatalf("new simulacro: got %d %v, want 403 institucion_no_permitida", code, resp)
	}
}
