package questions

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	qt "github.com/cbaciliod/learning/internal/questions/questionstest"
)

// ── H100 ──────────────────────────────────────────────────────────────────────

func getPregunta(t *testing.T, mux http.Handler, tok string, id int64) PreguntaResponse {
	t.Helper()
	w := qt.Do(mux, "GET", fmt.Sprintf("/preguntas/%d", id), tok, "")
	if w.Code != http.StatusOK {
		t.Fatalf("get pregunta: %d %s", w.Code, w.Body.String())
	}
	var p PreguntaResponse
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	return p
}

// putPregunta sends the edit form as the admin page does: every field of the
// GET response, plus etiqueta_ids when given (omitted when nil).
func putPregunta(t *testing.T, mux http.Handler, tok string, p PreguntaResponse, etiquetaIDs []int64) PreguntaResponse {
	t.Helper()
	alts := []map[string]any{}
	for _, r := range p.Respuestas {
		alts = append(alts, map[string]any{"id": r.ID, "texto": r.Texto, "es_correcta": r.EsCorrecta, "orden": r.Orden})
	}
	body := map[string]any{
		"asignatura_id": p.AsignaturaID,
		"enunciado":     p.Enunciado,
		"tipo":          p.Tipo,
		"dificultad":    p.Dificultad,
		"respuestas":    alts,
	}
	if etiquetaIDs != nil {
		body["etiqueta_ids"] = etiquetaIDs
	}
	b, _ := json.Marshal(body)
	w := qt.Do(mux, "PUT", fmt.Sprintf("/preguntas/%d", p.ID), tok, string(b))
	if w.Code != http.StatusOK {
		t.Fatalf("put pregunta: %d %s", w.Code, w.Body.String())
	}
	var out PreguntaResponse
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestH100_EditarSinTocarEtiquetasLasConserva(t *testing.T) {
	f, mux := newMux(t, qt.Options{Asignaturas: []qt.AsignaturaSpec{{NumeroPreguntas: 1, Peso: 100, Unicas: 1}}})
	admin := f.Token(t, f.UsuarioID, "admin")
	t.Cleanup(func() {
		if _, err := f.DB.Exec(`DELETE FROM etiquetas_pregunta WHERE asignatura_id=$1`, f.Asignaturas[0]); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})

	var etIDs []int64
	for _, nombre := range []string{"algebra", "geometria"} {
		w := qt.Do(mux, "POST", "/etiquetas", admin, fmt.Sprintf(`{"asignatura_id":%d,"nombre":%q}`, f.Asignaturas[0], nombre))
		if w.Code != http.StatusCreated && w.Code != http.StatusOK {
			t.Fatalf("create etiqueta: %d %s", w.Code, w.Body.String())
		}
		var et EtiquetaResponse
		_ = json.Unmarshal(w.Body.Bytes(), &et)
		etIDs = append(etIDs, et.ID)
	}

	p := getPregunta(t, mux, admin, f.Preguntas[0].ID)
	putPregunta(t, mux, admin, p, etIDs)

	// The GET returns the ids, so the edit page can preselect them.
	p = getPregunta(t, mux, admin, p.ID)
	if fmt.Sprint(p.EtiquetaIDs) != fmt.Sprint(etIDs) || len(p.Etiquetas) != 2 {
		t.Fatalf("GET etiqueta_ids = %v (etiquetas %v), want %v", p.EtiquetaIDs, p.Etiquetas, etIDs)
	}

	// Saving the form as loaded (ids from the GET) keeps the tags.
	if got := putPregunta(t, mux, admin, p, p.EtiquetaIDs); fmt.Sprint(got.EtiquetaIDs) != fmt.Sprint(etIDs) {
		t.Fatalf("edit with loaded ids lost tags: %v", got.EtiquetaIDs)
	}
	// A client that omits etiqueta_ids does not wipe them either.
	if got := putPregunta(t, mux, admin, p, nil); fmt.Sprint(got.EtiquetaIDs) != fmt.Sprint(etIDs) {
		t.Fatalf("edit without etiqueta_ids lost tags: %v", got.EtiquetaIDs)
	}
	// An explicit empty list still clears them.
	if got := putPregunta(t, mux, admin, p, []int64{}); len(got.EtiquetaIDs) != 0 || len(got.Etiquetas) != 0 {
		t.Fatalf("explicit [] must clear tags: %v %v", got.EtiquetaIDs, got.Etiquetas)
	}
}
