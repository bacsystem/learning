package practices

import (
	"encoding/json"
	"net/http"
	"sort"
	"testing"

	qt "github.com/cbaciliod/learning/internal/questions/questionstest"
)

// Escenarios «Borrador no se sirve» and «Compatibilidad con el banco
// existente» for a practice (SIA-E3-02 regla 1): of a bank with one question
// per state, a new practice takes only the aprobada and the inherited activo.
func TestServible_SeleccionDeLaPractica(t *testing.T) {
	e := newEnv(t, qt.Options{Inscripcion: "aprobado",
		Asignaturas: []qt.AsignaturaSpec{{NumeroPreguntas: 2, Peso: 100, Unicas: 5}}})
	estados := []string{"borrador", "aprobada", "activo", "en_revision", "rechazada"}
	for i, estado := range estados {
		var motivo *string
		if estado == "rechazada" {
			m := "Clave incorrecta"
			motivo = &m
		}
		if _, err := e.f.DB.Exec(`UPDATE preguntas SET estado = $2, motivo_rechazo = $3 WHERE id = $1`, e.f.Preguntas[i].ID, estado, motivo); err != nil {
			t.Fatal(err)
		}
	}
	w := qt.Do(e.mux, "POST", "/practicas", e.tok, e.createBody(2, "al_final", "null"))
	if w.Code != http.StatusCreated {
		t.Fatalf("create practica: %d %s", w.Code, w.Body)
	}
	var p PracticaEnCursoResponse
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	var ids []int64
	for _, q := range p.Preguntas {
		ids = append(ids, q.ID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	if want := []int64{e.f.Preguntas[1].ID, e.f.Preguntas[2].ID}; len(ids) != 2 || ids[0] != want[0] || ids[1] != want[1] {
		t.Errorf("practice questions = %v, want %v (aprobada, activo)", ids, want)
	}
}
