package questions

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	qt "github.com/cbaciliod/learning/internal/questions/questionstest"
)

// ── H19 / H20 ─────────────────────────────────────────────────────────────────

// asignaturaAjena inserts an asignatura that is not assigned to the fixture's
// proceso.
func asignaturaAjena(t *testing.T, f *qt.Fixture) int64 {
	t.Helper()
	var id int64
	if err := f.DB.QueryRow(`INSERT INTO asignaturas (nombre, codigo) VALUES ($1, $2) RETURNING id`,
		"Ajena "+f.Suffix, "AJ_"+f.Suffix).Scan(&id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := f.DB.Exec(`DELETE FROM etiquetas_pregunta WHERE asignatura_id = $1`, id); err != nil {
			t.Errorf("cleanup etiquetas: %v", err)
		}
		if _, err := f.DB.Exec(`DELETE FROM asignaturas WHERE id = $1`, id); err != nil {
			t.Errorf("cleanup asignatura: %v", err)
		}
	})
	return id
}

func crearEtiqueta(t *testing.T, mux http.Handler, tok string, asignaturaID int64, nombre string) int64 {
	t.Helper()
	w := qt.Do(mux, "POST", "/etiquetas", tok, fmt.Sprintf(`{"asignatura_id":%d,"nombre":%q}`, asignaturaID, nombre))
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("create etiqueta: %d %s", w.Code, w.Body.String())
	}
	var et EtiquetaResponse
	if err := json.Unmarshal(w.Body.Bytes(), &et); err != nil {
		t.Fatal(err)
	}
	return et.ID
}

func crearBody(f *qt.Fixture, asignaturaID int64, etiquetaIDs []int64, extra string) string {
	ids, _ := json.Marshal(etiquetaIDs)
	return fmt.Sprintf(`{"proceso_id":%d,"asignatura_id":%d,"enunciado":"Pregunta de clasificacion %s","tipo":"unica",
		"dificultad":"medio","etiqueta_ids":%s,%s"respuestas":[{"texto":"A","es_correcta":true,"orden":1},{"texto":"B","es_correcta":false,"orden":2}]}`,
		f.ProcesoID, asignaturaID, f.Suffix, ids, extra)
}

func TestH19_CrearRechazaAsignaturaYEtiquetasAjenas(t *testing.T) {
	f, mux := newMux(t, qt.Options{Asignaturas: []qt.AsignaturaSpec{{NumeroPreguntas: 1, Peso: 100, Unicas: 1}}})
	admin := f.Token(t, f.UsuarioID, "admin")
	ajena := asignaturaAjena(t, f)
	t.Cleanup(func() {
		if _, err := f.DB.Exec(`DELETE FROM etiquetas_pregunta WHERE asignatura_id = $1`, f.Asignaturas[0]); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	etAjena := crearEtiqueta(t, mux, admin, ajena, "ajena")
	etPropia := crearEtiqueta(t, mux, admin, f.Asignaturas[0], "propia")

	cases := []struct {
		name string
		body string
		msg  string
	}{
		{"asignatura fuera del proceso", crearBody(f, ajena, nil, ""), "no está asignada"},
		{"etiqueta de otra asignatura", crearBody(f, f.Asignaturas[0], []int64{etAjena}, ""), "etiquetas"},
		{"etiqueta inexistente", crearBody(f, f.Asignaturas[0], []int64{etPropia + 100000}, ""), "etiquetas"},
		{"grado inexistente", crearBody(f, f.Asignaturas[0], nil, `"grado_id":999999999,`), "grado"},
	}
	for _, c := range cases {
		w := qt.Do(mux, "POST", "/preguntas", admin, c.body)
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), c.msg) {
			t.Errorf("%s: got %d %s, want 400 mentioning %q", c.name, w.Code, w.Body.String(), c.msg)
		}
	}

	var n int
	if err := f.DB.Get(&n, `SELECT COUNT(*) FROM preguntas WHERE proceso_id = $1 AND enunciado LIKE 'Pregunta de clasificacion%'`, f.ProcesoID); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("%d rejected preguntas were persisted", n)
	}

	// The valid combination still works.
	if w := qt.Do(mux, "POST", "/preguntas", admin, crearBody(f, f.Asignaturas[0], []int64{etPropia}, "")); w.Code != http.StatusCreated {
		t.Fatalf("valid create: %d %s", w.Code, w.Body.String())
	}
}

func TestH19_EditarRechazaAsignaturaAjenaYEtiquetasQueQuedanAjenas(t *testing.T) {
	f, mux := newMux(t, qt.Options{Asignaturas: []qt.AsignaturaSpec{
		{NumeroPreguntas: 1, Peso: 50, Unicas: 1},
		{NumeroPreguntas: 1, Peso: 50, Unicas: 1},
	}})
	admin := f.Token(t, f.UsuarioID, "admin")
	ajena := asignaturaAjena(t, f)
	t.Cleanup(func() {
		if _, err := f.DB.Exec(`DELETE FROM etiquetas_pregunta WHERE asignatura_id = ANY($1)`, f.Asignaturas); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	p := getPregunta(t, mux, admin, f.Preguntas[0].ID)
	et := crearEtiqueta(t, mux, admin, p.AsignaturaID, "de-la-original")
	p = putPregunta(t, mux, admin, p, []int64{et})

	put := func(asignaturaID int64, etiquetaIDs []int64) *httpResult {
		alts := []map[string]any{}
		for _, r := range p.Respuestas {
			alts = append(alts, map[string]any{"id": r.ID, "texto": r.Texto, "es_correcta": r.EsCorrecta, "orden": r.Orden})
		}
		body := map[string]any{"asignatura_id": asignaturaID, "enunciado": p.Enunciado, "tipo": p.Tipo,
			"dificultad": p.Dificultad, "respuestas": alts}
		if etiquetaIDs != nil {
			body["etiqueta_ids"] = etiquetaIDs
		}
		b, _ := json.Marshal(body)
		w := qt.Do(mux, "PUT", fmt.Sprintf("/preguntas/%d", p.ID), admin, string(b))
		return &httpResult{w.Code, w.Body.String()}
	}

	otra := f.Asignaturas[0]
	if otra == p.AsignaturaID {
		otra = f.Asignaturas[1]
	}
	if r := put(ajena, []int64{}); r.code != http.StatusBadRequest || !strings.Contains(r.body, "no está asignada") {
		t.Errorf("asignatura fuera del proceso: %d %s", r.code, r.body)
	}
	// Moving to another asignatura while omitting etiqueta_ids would keep a tag
	// of the old one.
	if r := put(otra, nil); r.code != http.StatusBadRequest || !strings.Contains(r.body, "etiquetas") {
		t.Errorf("etiquetas conservadas de otra asignatura: %d %s", r.code, r.body)
	}
	if got := getPregunta(t, mux, admin, p.ID); got.AsignaturaID != p.AsignaturaID || fmt.Sprint(got.EtiquetaIDs) != fmt.Sprint([]int64{et}) {
		t.Fatalf("rejected edits changed the pregunta: asignatura %d etiquetas %v", got.AsignaturaID, got.EtiquetaIDs)
	}
	// Moving and clearing the tags is valid.
	if r := put(otra, []int64{}); r.code != http.StatusOK {
		t.Fatalf("valid move: %d %s", r.code, r.body)
	}
}

func TestH19_ImportRechazaAsignaturaAjena(t *testing.T) {
	f, mux := newMux(t, qt.Options{Asignaturas: []qt.AsignaturaSpec{{NumeroPreguntas: 1, Peso: 100, Unicas: 1}}})
	admin := f.Token(t, f.UsuarioID, "admin")
	ajena := asignaturaAjena(t, f)

	csv := "enunciado,tipo,dificultad,respuesta_1,correcta_1,respuesta_2,correcta_2\n" +
		"Pregunta importada ajena,unica,medio,A,si,B,no\n"
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("proceso_id", fmt.Sprint(f.ProcesoID))
	_ = mw.WriteField("asignatura_id", fmt.Sprint(ajena))
	fw, err := mw.CreateFormFile("file", "p.csv")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fw.Write([]byte(csv))
	_ = mw.Close()
	req := httptest.NewRequest("POST", "/preguntas/import", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+admin)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "no está asignada") {
		t.Fatalf("import con asignatura ajena: %d %s", w.Code, w.Body.String())
	}
}

func TestH20_GetPreguntaDevuelveIDsDeGradoYEspecialidad(t *testing.T) {
	f, mux := newMux(t, qt.Options{Asignaturas: []qt.AsignaturaSpec{{NumeroPreguntas: 1, Peso: 100, Unicas: 1}}})
	admin := f.Token(t, f.UsuarioID, "admin")
	body := crearBody(f, f.Asignaturas[0], nil, fmt.Sprintf(`"grado_id":%d,"especialidad_id":%d,`, f.GradoID, f.EspecialidadID))
	w := qt.Do(mux, "POST", "/preguntas", admin, body)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var created PreguntaResponse
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	got := getPregunta(t, mux, admin, created.ID)
	if got.GradoID == nil || *got.GradoID != f.GradoID || got.EspecialidadID == nil || *got.EspecialidadID != f.EspecialidadID {
		t.Fatalf("grado_id/especialidad_id = %v/%v, want %d/%d", got.GradoID, got.EspecialidadID, f.GradoID, f.EspecialidadID)
	}
}

type httpResult struct {
	code int
	body string
}
