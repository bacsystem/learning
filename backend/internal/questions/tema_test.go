package questions

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	qt "github.com/cbaciliod/learning/internal/questions/questionstest"
)

// sembrarTema inserts a tema in asignaturaID; the fixture's cleanup removes it.
func sembrarTema(t *testing.T, f *qt.Fixture, asignaturaID int64, nombre, estado string) int64 {
	t.Helper()
	var id int64
	if err := f.DB.QueryRow(`INSERT INTO temas (asignatura_id, nombre, nombre_clave, estado)
		VALUES ($1, $2, lower($2), $3) RETURNING id`, asignaturaID, nombre, estado).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func temaDe(t *testing.T, body []byte) (*float64, *string) {
	t.Helper()
	var p struct {
		TemaID *float64 `json:"tema_id"`
		Tema   *string  `json:"tema"`
	}
	if err := json.Unmarshal(body, &p); err != nil {
		t.Fatal(err)
	}
	return p.TemaID, p.Tema
}

func codigoDe(t *testing.T, body []byte) (string, string) {
	t.Helper()
	var e struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &e); err != nil {
		t.Fatal(err)
	}
	return e.Error, e.Message
}

// T8: POST /preguntas acepta un tema_id opcional, validado contra la
// asignatura de la pregunta y el estado del tema.
func TestT8_CrearPreguntaConTema(t *testing.T) {
	f, mux := newMux(t, qt.Options{Asignaturas: []qt.AsignaturaSpec{
		{NumeroPreguntas: 1, Peso: 50, Unicas: 1},
		{NumeroPreguntas: 1, Peso: 50, Unicas: 1},
	}})
	admin := f.Token(t, f.UsuarioID, "admin")
	asig, otraAsig := f.Asignaturas[0], f.Asignaturas[1]
	activo := sembrarTema(t, f, asig, "Vigilancia", "activo")
	archivado := sembrarTema(t, f, asig, "Archivado", "archivado")
	ajeno := sembrarTema(t, f, otraAsig, "Ajeno", "activo")

	cases := []struct {
		name   string
		extra  string
		status int
		tema   *int64
	}{
		{"sin tema", ``, http.StatusCreated, nil},
		{"tema null", `"tema_id":null,`, http.StatusCreated, nil},
		{"tema activo de la asignatura", fmt.Sprintf(`"tema_id":%d,`, activo), http.StatusCreated, &activo},
		{"tema archivado", fmt.Sprintf(`"tema_id":%d,`, archivado), http.StatusUnprocessableEntity, nil},
		{"tema de otra asignatura", fmt.Sprintf(`"tema_id":%d,`, ajeno), http.StatusUnprocessableEntity, nil},
		{"tema inexistente", `"tema_id":999999999,`, http.StatusUnprocessableEntity, nil},
		{"tema cero", `"tema_id":0,`, http.StatusUnprocessableEntity, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := qt.Do(mux, "POST", "/preguntas", admin, crearBody(f, asig, nil, c.extra))
			if w.Code != c.status {
				t.Fatalf("got %d %s, want %d", w.Code, w.Body, c.status)
			}
			if c.status == http.StatusUnprocessableEntity {
				if code, msg := codigoDe(t, w.Body.Bytes()); code != "tema_invalido" || msg == "" {
					t.Errorf("error body = %s, want code tema_invalido with message", w.Body)
				}
				return
			}
			temaID, tema := temaDe(t, w.Body.Bytes())
			if c.tema == nil && (temaID != nil || tema != nil) {
				t.Errorf("tema = %v %v, want null", temaID, tema)
			}
			if c.tema != nil && (temaID == nil || int64(*temaID) != *c.tema || tema == nil || *tema != "Vigilancia") {
				t.Errorf("tema = %v %v, want %d Vigilancia", temaID, tema, *c.tema)
			}
		})
	}

	var rechazadas int
	if err := f.DB.Get(&rechazadas, `SELECT COUNT(*) FROM preguntas WHERE proceso_id = $1 AND tema_id IN ($2, $3)`,
		f.ProcesoID, archivado, ajeno); err != nil {
		t.Fatal(err)
	}
	if rechazadas != 0 {
		t.Errorf("%d rejected preguntas were stored", rechazadas)
	}
}

// T8: PUT /preguntas/{id} reemplaza tema_id; se puede reenviar el tema actual
// aunque se haya archivado, y cambiar de asignatura sin tema lo deja en null.
func TestT8_EditarPreguntaConTema(t *testing.T) {
	f, mux := newMux(t, qt.Options{Asignaturas: []qt.AsignaturaSpec{
		{NumeroPreguntas: 1, Peso: 50, Unicas: 1},
		{NumeroPreguntas: 1, Peso: 50, Unicas: 1},
	}})
	admin := f.Token(t, f.UsuarioID, "admin")
	p := f.Preguntas[0]
	asig := p.AsignaturaID
	otraAsig := f.Asignaturas[1]
	if otraAsig == asig {
		otraAsig = f.Asignaturas[0]
	}
	actual := sembrarTema(t, f, asig, "Actual", "activo")
	otroArchivado := sembrarTema(t, f, asig, "Otro archivado", "archivado")
	deOtraAsig := sembrarTema(t, f, otraAsig, "De la otra", "activo")
	path := fmt.Sprintf("/preguntas/%d", p.ID)

	alts := []map[string]any{}
	for i := range p.Respuestas {
		alts = append(alts, map[string]any{"texto": fmt.Sprintf("Alt %d", i+1), "es_correcta": i == 0, "orden": i + 1})
	}
	body := func(asignaturaID int64, temaID any) string {
		m := map[string]any{"asignatura_id": asignaturaID, "enunciado": "Enunciado editado de prueba",
			"tipo": p.Tipo, "dificultad": "medio", "respuestas": alts}
		if temaID != nil {
			m["tema_id"] = temaID
		}
		b, _ := json.Marshal(m)
		return string(b)
	}
	put := func(t *testing.T, b string, status int) []byte {
		t.Helper()
		w := qt.Do(mux, "PUT", path, admin, b)
		if w.Code != status {
			t.Fatalf("PUT %s: got %d %s, want %d", b, w.Code, w.Body, status)
		}
		return w.Body.Bytes()
	}

	if temaID, _ := temaDe(t, put(t, body(asig, actual), http.StatusOK)); temaID == nil || int64(*temaID) != actual {
		t.Fatalf("tema_id = %v, want %d", temaID, actual)
	}

	// The current tema is archived meanwhile: sending it back is allowed.
	if _, err := f.DB.Exec(`UPDATE temas SET estado = 'archivado' WHERE id = $1`, actual); err != nil {
		t.Fatal(err)
	}
	if temaID, _ := temaDe(t, put(t, body(asig, actual), http.StatusOK)); temaID == nil || int64(*temaID) != actual {
		t.Errorf("re-sending the archived current tema: tema_id = %v", temaID)
	}

	// Another archived tema, or one from another asignatura, is rejected.
	for _, b := range []string{body(asig, otroArchivado), body(asig, deOtraAsig), body(otraAsig, actual)} {
		if code, _ := codigoDe(t, put(t, b, http.StatusUnprocessableEntity)); code != "tema_invalido" {
			t.Errorf("PUT %s: code %q, want tema_invalido", b, code)
		}
	}

	// Changing the asignatura without tema_id leaves the question without tema.
	if temaID, tema := temaDe(t, put(t, body(otraAsig, nil), http.StatusOK)); temaID != nil || tema != nil {
		t.Errorf("tema = %v %v, want null", temaID, tema)
	}
	if temaID, _ := temaDe(t, put(t, body(otraAsig, deOtraAsig), http.StatusOK)); temaID == nil || int64(*temaID) != deOtraAsig {
		t.Errorf("tema_id = %v, want %d", temaID, deOtraAsig)
	}
}

// T8: GET /preguntas filtra por tema_id y devuelve tema_id y tema.
func TestT8_ListarPorTema(t *testing.T) {
	f, mux := newMux(t, qt.Options{Asignaturas: []qt.AsignaturaSpec{{NumeroPreguntas: 2, Peso: 100, Unicas: 3}}})
	admin := f.Token(t, f.UsuarioID, "admin")
	tema := sembrarTema(t, f, f.Asignaturas[0], "Filtrado", "activo")
	if _, err := f.DB.Exec(`UPDATE preguntas SET tema_id = $1 WHERE id = $2`, tema, f.Preguntas[0].ID); err != nil {
		t.Fatal(err)
	}

	w := qt.Do(mux, "GET", fmt.Sprintf("/preguntas?proceso_id=%d&tema_id=%d", f.ProcesoID, tema), admin, "")
	var list struct {
		Data []struct {
			ID     int64   `json:"id"`
			TemaID *int64  `json:"tema_id"`
			Tema   *string `json:"tema"`
		} `json:"data"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil || w.Code != http.StatusOK {
		t.Fatalf("list: %d %s", w.Code, w.Body)
	}
	if list.Total != 1 || len(list.Data) != 1 || list.Data[0].ID != f.Preguntas[0].ID ||
		list.Data[0].TemaID == nil || *list.Data[0].TemaID != tema || list.Data[0].Tema == nil || *list.Data[0].Tema != "Filtrado" {
		t.Errorf("list = %+v", list)
	}
	if w := qt.Do(mux, "GET", "/preguntas?tema_id=abc", admin, ""); w.Code != http.StatusBadRequest {
		t.Errorf("tema_id=abc: %d, want 400", w.Code)
	}
}
