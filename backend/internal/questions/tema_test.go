package questions

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	qt "github.com/cbaciliod/learning/internal/questions/questionstest"
)

// sembrarTema inserts a tema in asignaturaID; the fixture's cleanup removes it.
func sembrarTema(t *testing.T, f *qt.Fixture, asignaturaID int64, nombre, estado string) int64 {
	t.Helper()
	var id int64
	if err := f.DB.QueryRow(`INSERT INTO temas (asignatura_id, nombre, nombre_clave, estado)
		VALUES ($1, $2::text, lower($2::text), $3) RETURNING id`, asignaturaID, nombre, estado).Scan(&id); err != nil {
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

// T8: PUT /preguntas/{id}. tema_id ausente conserva el tema salvo que cambie
// la asignatura (entonces null), null lo quita y un valor se valida; el tema
// actual se puede reenviar aunque se haya archivado.
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
	otroActivo := sembrarTema(t, f, asig, "Otro activo", "activo")
	otroArchivado := sembrarTema(t, f, asig, "Otro archivado", "archivado")
	deOtraAsig := sembrarTema(t, f, otraAsig, "De la otra", "activo")
	path := fmt.Sprintf("/preguntas/%d", p.ID)

	alts := []map[string]any{}
	for i := range p.Respuestas {
		alts = append(alts, map[string]any{"texto": fmt.Sprintf("Alt %d", i+1), "es_correcta": i == 0, "orden": i + 1})
	}
	// tema is the raw JSON of tema_id, or "" to leave the field out.
	body := func(asignaturaID int64, tema string) string {
		m := map[string]any{"asignatura_id": asignaturaID, "enunciado": "Enunciado editado de prueba",
			"tipo": p.Tipo, "dificultad": "medio", "respuestas": alts}
		if tema != "" {
			m["tema_id"] = json.RawMessage(tema)
		}
		b, _ := json.Marshal(m)
		return string(b)
	}
	setTema := func(t *testing.T, asignaturaID int64, temaID *int64) {
		t.Helper()
		if _, err := f.DB.Exec(`UPDATE preguntas SET asignatura_id = $1, tema_id = $2 WHERE id = $3`, asignaturaID, temaID, p.ID); err != nil {
			t.Fatal(err)
		}
	}
	num := func(id int64) string { return fmt.Sprint(id) }

	cases := []struct {
		name       string
		inicial    *int64 // tema before the PUT (in asig)
		archivarlo bool   // archive the initial tema before the PUT
		asignatura int64
		tema       string
		status     int
		want       *int64
	}{
		{"ausente conserva el tema", &actual, false, asig, "", http.StatusOK, &actual},
		{"ausente conserva un tema archivado", &actual, true, asig, "", http.StatusOK, &actual},
		{"ausente con cambio de asignatura lo limpia", &actual, false, otraAsig, "", http.StatusOK, nil},
		{"null explícito lo quita", &actual, false, asig, "null", http.StatusOK, nil},
		{"valor activo lo reemplaza", &actual, false, asig, num(otroActivo), http.StatusOK, &otroActivo},
		{"reenviar el actual archivado", &actual, true, asig, num(actual), http.StatusOK, &actual},
		{"cambio de asignatura con tema de la nueva", &actual, false, otraAsig, num(deOtraAsig), http.StatusOK, &deOtraAsig},
		{"otro tema archivado", &actual, false, asig, num(otroArchivado), http.StatusUnprocessableEntity, &actual},
		{"tema de otra asignatura", &actual, false, asig, num(deOtraAsig), http.StatusUnprocessableEntity, &actual},
		{"cambio de asignatura reenviando el actual", &actual, false, otraAsig, num(actual), http.StatusUnprocessableEntity, &actual},
		{"tema inexistente", nil, false, asig, "999999999", http.StatusUnprocessableEntity, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			setTema(t, asig, c.inicial)
			estado := "activo"
			if c.archivarlo {
				estado = "archivado"
			}
			if _, err := f.DB.Exec(`UPDATE temas SET estado = $1 WHERE id = $2`, estado, actual); err != nil {
				t.Fatal(err)
			}
			w := qt.Do(mux, "PUT", path, admin, body(c.asignatura, c.tema))
			if w.Code != c.status {
				t.Fatalf("got %d %s, want %d", w.Code, w.Body, c.status)
			}
			if c.status == http.StatusUnprocessableEntity {
				if code, _ := codigoDe(t, w.Body.Bytes()); code != "tema_invalido" {
					t.Errorf("code %q, want tema_invalido", code)
				}
			}
			var stored *int64
			if err := f.DB.Get(&stored, `SELECT tema_id FROM preguntas WHERE id = $1`, p.ID); err != nil {
				t.Fatal(err)
			}
			if (stored == nil) != (c.want == nil) || (stored != nil && *stored != *c.want) {
				t.Errorf("stored tema_id = %v, want %v", stored, c.want)
			}
			if c.status == http.StatusOK {
				if temaID, _ := temaDe(t, w.Body.Bytes()); (temaID == nil) != (c.want == nil) {
					t.Errorf("response tema_id = %v, want %v", temaID, c.want)
				}
			}
		})
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

// #184: the search is a literal substring; `%` and `_` in the term match
// themselves, not «anything» or «one character».
func TestListarPreguntas_BusquedaEsLiteral(t *testing.T) {
	f, mux := newMux(t, qt.Options{Asignaturas: []qt.AsignaturaSpec{{NumeroPreguntas: 3, Peso: 100, Unicas: 3}}})
	admin := f.Token(t, f.UsuarioID, "admin")
	enunciados := []string{"Cobertura del 100% de vacunas", "Cobertura del 100 de vacunas", "Dosis a_b y a-b"}
	for i, e := range enunciados {
		if _, err := f.DB.Exec(`UPDATE preguntas SET enunciado = $1 WHERE id = $2`, e, f.Preguntas[i].ID); err != nil {
			t.Fatal(err)
		}
	}
	tests := []struct {
		name string
		term string
		want []int64
	}{
		{"percent is literal", "100%", []int64{f.Preguntas[0].ID}},
		{"underscore is literal", "a_b", []int64{f.Preguntas[2].ID}},
		{"a plain term is still a substring", "cobertura", []int64{f.Preguntas[0].ID, f.Preguntas[1].ID}},
		{"a lone percent matches only the question that has one", "%", []int64{f.Preguntas[0].ID}},
		{"a lone underscore matches only the question that has one", "_", []int64{f.Preguntas[2].ID}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := qt.Do(mux, "GET", fmt.Sprintf("/preguntas?proceso_id=%d&search=%s", f.ProcesoID, url.QueryEscape(tt.term)), admin, "")
			var list struct {
				Data []struct {
					ID int64 `json:"id"`
				} `json:"data"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil || w.Code != http.StatusOK {
				t.Fatalf("list: %d %s", w.Code, w.Body)
			}
			got := map[int64]bool{}
			for _, d := range list.Data {
				got[d.ID] = true
			}
			if len(got) != len(tt.want) {
				t.Fatalf("ids %v, want %v", got, tt.want)
			}
			for _, id := range tt.want {
				if !got[id] {
					t.Errorf("missing %d in %v", id, got)
				}
			}
		})
	}
}
