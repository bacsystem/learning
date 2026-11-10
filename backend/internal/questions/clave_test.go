package questions

import (
	"fmt"
	"net/http"
	"testing"

	qt "github.com/cbaciliod/learning/internal/questions/questionstest"
)

// altsWithKey returns every alternative of p, by id and in order, with the
// given texts and marking as correct only the ids in correct.
func altsWithKey(p qt.Pregunta, texto string, correct ...int64) []map[string]any {
	isCorrect := map[int64]bool{}
	for _, id := range correct {
		isCorrect[id] = true
	}
	alts := []map[string]any{}
	for i, rid := range p.Respuestas {
		alts = append(alts, map[string]any{"id": rid, "texto": fmt.Sprintf("%s %d", texto, i+1), "es_correcta": isCorrect[rid], "orden": i + 1})
	}
	return alts
}

// H133: once an attempt was graded with a question's key, changing that key
// would make every finished review contradict itself (the stored verdict says
// "correct" while the review marks another alternative as the right one), so
// the edit is rejected with 409. Texts can still be fixed.
func TestH133_CambiarClaveConIntentosCalificados_409(t *testing.T) {
	f, mux := newMux(t, qt.Options{Asignaturas: []qt.AsignaturaSpec{{NumeroPreguntas: 1, Peso: 100, Unicas: 1}}})
	admin := f.Token(t, f.UsuarioID, "admin")
	p := f.Preguntas[0]
	correcta := p.Correctas[0]
	var otra int64
	for _, rid := range p.Respuestas {
		if rid != correcta {
			otra = rid
			break
		}
	}
	practicaID := seedAttempt(t, f, p, correcta)
	if _, err := f.DB.Exec(`UPDATE practica_respuestas SET es_correcta = true WHERE practica_id = $1`, practicaID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.DB.Exec(`UPDATE practicas SET estado = 'completada', fecha_fin = NOW() WHERE id = $1`, practicaID); err != nil {
		t.Fatal(err)
	}
	path := fmt.Sprintf("/preguntas/%d", p.ID)

	if w := qt.Do(mux, "PUT", path, admin, updateBody(f, p, altsWithKey(p, "Corregido", otra))); w.Code != http.StatusConflict {
		t.Fatalf("change key: want 409, got %d %s", w.Code, w.Body.String())
	}
	// Moving the key to a newly inserted alternative also changes it.
	nueva := append(altsWithKey(p, "Corregido"), map[string]any{"texto": "Nueva correcta", "es_correcta": true, "orden": len(p.Respuestas) + 1})
	if w := qt.Do(mux, "PUT", path, admin, updateBody(f, p, nueva)); w.Code != http.StatusConflict {
		t.Fatalf("key moved to a new alternative: want 409, got %d %s", w.Code, w.Body.String())
	}
	var stillCorrect bool
	if err := f.DB.QueryRow(`SELECT es_correcta FROM respuestas WHERE id = $1`, correcta).Scan(&stillCorrect); err != nil {
		t.Fatal(err)
	}
	if !stillCorrect {
		t.Fatal("rejected edit changed the key anyway")
	}
	// Same key, new texts: allowed.
	if w := qt.Do(mux, "PUT", path, admin, updateBody(f, p, altsWithKey(p, "Texto corregido", correcta))); w.Code != http.StatusOK {
		t.Fatalf("same key, new texts: want 200, got %d %s", w.Code, w.Body.String())
	}
}

// Without graded attempts (only an in-progress one), the key can change: the
// attempt will be graded with the new key when it finishes.
func TestH133_CambiarClaveSinIntentosCalificados_OK(t *testing.T) {
	f, mux := newMux(t, qt.Options{Asignaturas: []qt.AsignaturaSpec{{NumeroPreguntas: 1, Peso: 100, Unicas: 1}}})
	admin := f.Token(t, f.UsuarioID, "admin")
	p := f.Preguntas[0]
	seedAttempt(t, f, p, p.Respuestas[0])
	var otra int64
	for _, rid := range p.Respuestas {
		if rid != p.Correctas[0] {
			otra = rid
			break
		}
	}
	if w := qt.Do(mux, "PUT", fmt.Sprintf("/preguntas/%d", p.ID), admin, updateBody(f, p, altsWithKey(p, "Nueva clave", otra))); w.Code != http.StatusOK {
		t.Fatalf("change key without graded attempts: want 200, got %d %s", w.Code, w.Body.String())
	}
}
