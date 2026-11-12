package questions

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	qt "github.com/cbaciliod/learning/internal/questions/questionstest"
)

// #198: a body that breaks a validation rule answers 400 in the legacy format
// with a message of our own that names the JSON fields, never the validator's
// text («Key: 'CreatePreguntaRequest.Respuestas[0].Orden' Error:Field
// validation for 'Orden' failed on the 'required' tag»).
func TestValidacion_MensajePropioSinTextoDelValidador(t *testing.T) {
	f, mux := newMux(t, qt.Options{Asignaturas: []qt.AsignaturaSpec{{NumeroPreguntas: 1, Peso: 100, Unicas: 1}}})
	admin := f.Token(t, f.UsuarioID, "admin")
	asig := f.Asignaturas[0]
	tests := []struct {
		name       string
		metodo     string
		ruta       string
		cuerpo     string
		wantCampos []string
	}{
		{"alternative without orden", "POST", "/preguntas", fmt.Sprintf(`{"proceso_id":%d,"asignatura_id":%d,"enunciado":"Pregunta de validación %s",
			"tipo":"unica","dificultad":"medio","respuestas":[{"texto":"A","es_correcta":true},{"texto":"B","es_correcta":false,"orden":2}]}`, f.ProcesoID, asig, f.Suffix),
			[]string{"respuestas[0].orden"}},
		{"several fields at once", "POST", "/preguntas", fmt.Sprintf(`{"proceso_id":%d,"asignatura_id":%d,"enunciado":"corto","tipo":"otro",
			"dificultad":"medio","respuestas":[{"texto":"A","es_correcta":true,"orden":1},{"texto":"B","es_correcta":false,"orden":2}]}`, f.ProcesoID, asig),
			[]string{"enunciado", "tipo"}},
		{"etiqueta without name", "POST", "/etiquetas", fmt.Sprintf(`{"asignatura_id":%d}`, asig), []string{"nombre"}},
		{"pregunta update with a bad dificultad", "PUT", fmt.Sprintf("/preguntas/%d", f.Preguntas[0].ID), fmt.Sprintf(`{"asignatura_id":%d,"enunciado":"Pregunta de validación %s",
			"tipo":"unica","dificultad":"imposible","respuestas":[{"texto":"A","es_correcta":true,"orden":1},{"texto":"B","es_correcta":false,"orden":2}]}`, asig, f.Suffix),
			[]string{"dificultad"}},
		{"generate-set with zero questions", "POST", "/preguntas/generate-set", fmt.Sprintf(`{"proceso_id":%d,"asignaturas":[{"asignatura_id":%d,"numero_preguntas":0}]}`, f.ProcesoID, asig),
			[]string{"asignaturas[0].numero_preguntas"}},
		{"etiqueta update with a bad estado", "PUT", "/etiquetas/1", `{"nombre":"x","estado":"borrado"}`, []string{"estado"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := qt.Do(mux, tt.metodo, tt.ruta, admin, tt.cuerpo)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status %d, want 400 (%s)", w.Code, w.Body)
			}
			var body map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || len(body) != 1 {
				t.Fatalf("body %q is not the legacy {error: text} (%v)", w.Body, err)
			}
			msg := body["error"]
			for _, interno := range []string{"Key:", "Field validation", "failed on the", "Request.", "tag"} {
				if strings.Contains(msg, interno) {
					t.Errorf("the message leaks the validator's text (%q): %s", interno, msg)
				}
			}
			for _, campo := range tt.wantCampos {
				if !strings.Contains(msg, campo) {
					t.Errorf("the message %q does not name the field %s", msg, campo)
				}
			}
		})
	}
}
