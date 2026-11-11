package questions

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/cbaciliod/learning/internal/audit"
	qt "github.com/cbaciliod/learning/internal/questions/questionstest"
)

func newMux(t *testing.T, opts qt.Options) (*qt.Fixture, *http.ServeMux) {
	t.Helper()
	db := qt.DB(t)
	f := qt.Seed(t, db, opts)
	mux := http.NewServeMux()
	Mount(mux, NewService(NewRepository(db)), audit.New(db), qt.JWTSecret, qt.AllowSession)
	return f, mux
}

// seedAttempt creates an en_curso práctica whose answer to p is respuestaID.
func seedAttempt(t *testing.T, f *qt.Fixture, p qt.Pregunta, respuestaID int64) int64 {
	t.Helper()
	var practicaID int64
	if err := f.DB.QueryRow(`INSERT INTO practicas (usuario_id, institucion_id, proceso_id, modo_calificacion, total_preguntas)
		VALUES ($1,$2,$3,'al_final',1) RETURNING id`, f.UsuarioID, f.InstitucionID, f.ProcesoID).Scan(&practicaID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.DB.Exec(`INSERT INTO practica_respuestas (practica_id, pregunta_id, asignatura_id, respuesta_id, respondida_at, orden)
		VALUES ($1,$2,$3,$4,NOW(),1)`, practicaID, p.ID, p.AsignaturaID, respuestaID); err != nil {
		t.Fatal(err)
	}
	return practicaID
}

func updateBody(f *qt.Fixture, p qt.Pregunta, alts []map[string]any) string {
	b, _ := json.Marshal(map[string]any{
		"asignatura_id": p.AsignaturaID,
		"enunciado":     "Enunciado editado de prueba",
		"tipo":          p.Tipo,
		"dificultad":    "medio",
		"respuestas":    alts,
	})
	return string(b)
}

func TestH7_UpdatePreguntaConservaIDsDeAlternativas(t *testing.T) {
	f, mux := newMux(t, qt.Options{Asignaturas: []qt.AsignaturaSpec{{NumeroPreguntas: 1, Peso: 100, Unicas: 1}}})
	admin := f.Token(t, f.UsuarioID, "admin")
	p := f.Preguntas[0]
	practicaID := seedAttempt(t, f, p, p.Respuestas[0])

	// Legacy payload without ids: matched by orden, updated in place.
	alts := []map[string]any{}
	for i := range p.Respuestas {
		alts = append(alts, map[string]any{"texto": fmt.Sprintf("Texto nuevo %d", i+1), "es_correcta": i == 0, "orden": i + 1})
	}
	w := qt.Do(mux, "PUT", fmt.Sprintf("/preguntas/%d", p.ID), admin, updateBody(f, p, alts))
	if w.Code != http.StatusOK {
		t.Fatalf("update: %d %s", w.Code, w.Body.String())
	}
	var resp PreguntaResponse
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	for i, r := range resp.Respuestas {
		if r.ID != p.Respuestas[i] {
			t.Fatalf("alternative %d got a new id %d (was %d)", i, r.ID, p.Respuestas[i])
		}
	}
	var got *int64
	if err := f.DB.QueryRow(`SELECT respuesta_id FROM practica_respuestas WHERE practica_id=$1`, practicaID).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got == nil || *got != p.Respuestas[0] {
		t.Fatalf("in-progress answer lost: %v", got)
	}

	// Removing an alternative that has attempts → 409.
	alts = []map[string]any{}
	for i, rid := range p.Respuestas[1:] {
		alts = append(alts, map[string]any{"id": rid, "texto": "x", "es_correcta": i == 0, "orden": i + 1})
	}
	if w := qt.Do(mux, "PUT", fmt.Sprintf("/preguntas/%d", p.ID), admin, updateBody(f, p, alts)); w.Code != http.StatusConflict {
		t.Fatalf("remove answered alternative: want 409, got %d %s", w.Code, w.Body.String())
	}

	// Removing an alternative without attempts is fine; new ones get inserted.
	alts = []map[string]any{
		{"id": p.Respuestas[0], "texto": "a", "es_correcta": true, "orden": 1},
		{"id": p.Respuestas[1], "texto": "b", "es_correcta": false, "orden": 2},
		{"texto": "nueva", "es_correcta": false, "orden": 3},
	}
	w = qt.Do(mux, "PUT", fmt.Sprintf("/preguntas/%d", p.ID), admin, updateBody(f, p, alts))
	if w.Code != http.StatusOK {
		t.Fatalf("update with ids: %d %s", w.Code, w.Body.String())
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Respuestas) != 3 || resp.Respuestas[0].ID != p.Respuestas[0] || resp.Respuestas[1].ID != p.Respuestas[1] {
		t.Fatalf("respuestas after update: %+v", resp.Respuestas)
	}

	// An id from another question → 400.
	alts[2]["id"] = p.Respuestas[3] // deleted above, no longer belongs
	if w := qt.Do(mux, "PUT", fmt.Sprintf("/preguntas/%d", p.ID), admin, updateBody(f, p, alts)); w.Code != http.StatusBadRequest {
		t.Fatalf("foreign id: want 400, got %d", w.Code)
	}
}

// The admin editor sends ids: reordering the alternatives moves each id with
// its own text (no text is rewritten over another alternative's id), and the
// attempt that pointed at an alternative still points at the same text.
func TestH7_ReordenarConIDsConservaTextoDeCadaAlternativa(t *testing.T) {
	f, mux := newMux(t, qt.Options{Asignaturas: []qt.AsignaturaSpec{{NumeroPreguntas: 1, Peso: 100, Unicas: 1}}})
	admin := f.Token(t, f.UsuarioID, "admin")
	p := f.Preguntas[0]
	if len(p.Respuestas) < 3 {
		t.Fatalf("fixture needs ≥3 alternatives, got %d", len(p.Respuestas))
	}
	seedAttempt(t, f, p, p.Respuestas[0])

	// Name every alternative after its id, in the original order.
	texto := map[int64]string{}
	alts := []map[string]any{}
	for i, rid := range p.Respuestas {
		texto[rid] = fmt.Sprintf("Alternativa original %d", rid)
		alts = append(alts, map[string]any{"id": rid, "texto": texto[rid], "es_correcta": i == 0, "orden": i + 1})
	}
	if w := qt.Do(mux, "PUT", fmt.Sprintf("/preguntas/%d", p.ID), admin, updateBody(f, p, alts)); w.Code != http.StatusOK {
		t.Fatalf("name alternatives: %d %s", w.Code, w.Body.String())
	}

	// Reverse the order, keeping each id with its text.
	reversed := []map[string]any{}
	for i := len(p.Respuestas) - 1; i >= 0; i-- {
		rid := p.Respuestas[i]
		reversed = append(reversed, map[string]any{"id": rid, "texto": texto[rid], "es_correcta": i == 0, "orden": len(reversed) + 1})
	}
	w := qt.Do(mux, "PUT", fmt.Sprintf("/preguntas/%d", p.ID), admin, updateBody(f, p, reversed))
	if w.Code != http.StatusOK {
		t.Fatalf("reorder: %d %s", w.Code, w.Body.String())
	}
	var resp PreguntaResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Respuestas) != len(p.Respuestas) {
		t.Fatalf("alternatives = %d, want %d", len(resp.Respuestas), len(p.Respuestas))
	}
	for i, r := range resp.Respuestas {
		wantID := p.Respuestas[len(p.Respuestas)-1-i]
		if r.ID != wantID || r.Texto != texto[wantID] {
			t.Fatalf("position %d: got id %d %q, want id %d %q", i, r.ID, r.Texto, wantID, texto[wantID])
		}
		if r.EsCorrecta != (r.ID == p.Respuestas[0]) {
			t.Fatalf("es_correcta moved away from its alternative: %+v", r)
		}
	}
	var answered string
	if err := f.DB.QueryRow(`SELECT r.texto FROM practica_respuestas pr JOIN respuestas r ON r.id = pr.respuesta_id
		WHERE pr.pregunta_id=$1`, p.ID).Scan(&answered); err != nil {
		t.Fatal(err)
	}
	if answered != texto[p.Respuestas[0]] {
		t.Fatalf("attempt now points at %q, want %q", answered, texto[p.Respuestas[0]])
	}
}

func TestH7_SchemaRestringeBorrarAlternativaConIntentos(t *testing.T) {
	f, _ := newMux(t, qt.Options{Asignaturas: []qt.AsignaturaSpec{{NumeroPreguntas: 1, Peso: 100, Unicas: 1}}})
	p := f.Preguntas[0]
	seedAttempt(t, f, p, p.Respuestas[0])
	if _, err := f.DB.Exec(`DELETE FROM respuestas WHERE id=$1`, p.Respuestas[0]); err == nil {
		t.Fatalf("deleting an answered alternative succeeded (history would be lost)")
	}
}

func TestH12_GenerateSetSoloAdminYConTope(t *testing.T) {
	f, mux := newMux(t, qt.Options{Asignaturas: []qt.AsignaturaSpec{{NumeroPreguntas: 1, Peso: 100, Unicas: 2}}})
	body := func(n int) string {
		return fmt.Sprintf(`{"proceso_id":%d,"asignaturas":[{"asignatura_id":%d,"numero_preguntas":%d}]}`, f.ProcesoID, f.Asignaturas[0], n)
	}
	if w := qt.Do(mux, "POST", "/preguntas/generate-set", f.Token(t, f.UsuarioID, "usuario"), body(1)); w.Code != http.StatusForbidden {
		t.Fatalf("non-admin generate-set: want 403, got %d", w.Code)
	}
	admin := f.Token(t, f.UsuarioID, "admin")
	if w := qt.Do(mux, "POST", "/preguntas/generate-set", admin, body(1)); w.Code != http.StatusOK {
		t.Fatalf("admin generate-set: %d %s", w.Code, w.Body.String())
	}
	if w := qt.Do(mux, "POST", "/preguntas/generate-set", admin, body(MaxPreguntasSet+1)); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("over cap: want 422, got %d %s", w.Code, w.Body.String())
	}
	dup := fmt.Sprintf(`{"proceso_id":%d,"asignaturas":[{"asignatura_id":%d,"numero_preguntas":1},{"asignatura_id":%d,"numero_preguntas":1}]}`,
		f.ProcesoID, f.Asignaturas[0], f.Asignaturas[0])
	if w := qt.Do(mux, "POST", "/preguntas/generate-set", admin, dup); w.Code != http.StatusBadRequest {
		t.Fatalf("duplicated asignatura: want 400, got %d", w.Code)
	}
}
