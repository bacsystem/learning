package practices

import (
	"fmt"
	"net/http"
	"testing"
)

// PLT-06: a práctica needs a proceso of the user's institution.

func codigoDe(body map[string]any) string {
	s, _ := body["error"].(string)
	return s
}

func TestPLT06_PracticaDeOtraInstitucionEs403AunqueEsteInscrito(t *testing.T) {
	e := newEnv(t, defaultOpts())
	otro := e.f.ProcesoDeOtraInstitucion(t)
	body := fmt.Sprintf(`{"proceso_id":%d,"asignaturas":[{"asignatura_id":%d,"numero_preguntas":1}],"modo_calificacion":"al_final","tiempo_limite_seg":null}`,
		otro, e.f.Asignaturas[0])
	code, resp := e.do(t, "POST", "/practicas", e.tok, body)
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
			mustExec(t, e, c.sql, e.f.ProcesoID)
			code, resp := e.do(t, "POST", "/practicas", e.tok, e.createBody(1, "al_final", "null"))
			if code != http.StatusForbidden || codigoDe(resp) != c.code {
				t.Fatalf("got %d %v, want 403 %s", code, resp, c.code)
			}
		})
	}
}

// A práctica already started keeps working after its proceso stops being
// eligible; only a new one is refused (P2).
func TestPLT06_PracticaEnCursoSigueAunqueElProcesoYaNoSeaElegible(t *testing.T) {
	e := newEnv(t, defaultOpts())
	p := e.create(t, "al_final")
	otro := e.f.ProcesoDeOtraInstitucion(t)
	mustExec(t, e, `UPDATE procesos SET institucion_id=(SELECT institucion_id FROM procesos WHERE id=$2) WHERE id=$1`,
		e.f.ProcesoID, otro)
	t.Cleanup(func() { // before the other institución is deleted
		mustExec(t, e, `UPDATE procesos SET institucion_id=$2 WHERE id=$1`, e.f.ProcesoID, e.f.InstitucionID)
	})
	if code, _ := e.do(t, "GET", fmt.Sprintf("/practicas/%d", p.ID), e.tok, ""); code != http.StatusOK {
		t.Fatalf("existing práctica: want 200, got %d", code)
	}
	code, resp := e.do(t, "POST", "/practicas", e.tok, e.createBody(1, "al_final", "null"))
	if code != http.StatusForbidden || codigoDe(resp) != "institucion_no_permitida" {
		t.Fatalf("new práctica: got %d %v, want 403 institucion_no_permitida", code, resp)
	}
}
