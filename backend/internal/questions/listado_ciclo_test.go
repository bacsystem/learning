package questions

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	qt "github.com/cbaciliod/learning/internal/questions/questionstest"
)

func (c *cicloFx) listar(t *testing.T, query string) PreguntaListResponse {
	t.Helper()
	w := qt.Do(c.mux, "GET", "/preguntas?"+query, c.admin, "")
	c.esperar(t, w, http.StatusOK, "")
	var out PreguntaListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// Escenario «Alta manual»: POST /preguntas creates a manual draft, not served.
func TestPostPregunta_NaceBorradorManual(t *testing.T) {
	c := newCicloFx(t)
	body := jsonString(t, map[string]any{
		"proceso_id": c.f.ProcesoID, "asignatura_id": c.p.AsignaturaID, "enunciado": "Enunciado de alta manual para el ciclo",
		"tipo": "unica", "dificultad": "medio",
		"respuestas": []map[string]any{{"texto": "Sí", "es_correcta": true, "orden": 1}, {"texto": "No", "es_correcta": false, "orden": 2}},
	})
	w := qt.Do(c.mux, "POST", "/preguntas", c.admin, body)
	c.esperar(t, w, http.StatusCreated, "")
	var p PreguntaResponse
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if p.Estado != EstadoBorrador || p.Origen != OrigenManual || p.MotivoRechazo != nil || p.Citas == nil || len(*p.Citas) != 0 {
		t.Errorf("alta = estado %s origen %s motivo %v citas %v", p.Estado, p.Origen, p.MotivoRechazo, p.Citas)
	}
	// The body cannot choose them: estado and origen are ignored.
	otro := jsonString(t, map[string]any{
		"proceso_id": c.f.ProcesoID, "asignatura_id": c.p.AsignaturaID, "enunciado": "Otro enunciado de alta manual", "tipo": "unica", "dificultad": "medio",
		"estado": "aprobada", "origen": "ia",
		"respuestas": []map[string]any{{"texto": "Sí", "es_correcta": true, "orden": 1}, {"texto": "No", "es_correcta": false, "orden": 2}},
	})
	w = qt.Do(c.mux, "POST", "/preguntas", c.admin, otro)
	c.esperar(t, w, http.StatusCreated, "")
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil || p.Estado != EstadoBorrador || p.Origen != OrigenManual {
		t.Errorf("el cuerpo no elige estado ni origen: %s %s (%v)", p.Estado, p.Origen, err)
	}
}

// GET /preguntas: filters estado/origen (exact values), and the item carries
// estado, origen, motivo_rechazo and citas_count but not the citas.
func TestGetPreguntas_FiltrosYCamposDelCiclo(t *testing.T) {
	c := newCicloFx(t)
	c.poner(t, EstadoRechazada, OrigenIA)
	c.citar(t, pasajeDePrueba(t, c.f.DB))
	base := "proceso_id=" + fmt.Sprint(c.f.ProcesoID)

	l := c.listar(t, base+"&estado=rechazada&origen=ia")
	if l.Total != 1 || len(l.Data) != 1 {
		t.Fatalf("listado = %+v", l)
	}
	it := l.Data[0]
	if it.Estado != EstadoRechazada || it.Origen != OrigenIA || it.MotivoRechazo == nil || *it.MotivoRechazo != "Clave incorrecta" ||
		it.CitasCount == nil || *it.CitasCount != 1 || it.Citas != nil {
		t.Errorf("item = %+v", it)
	}
	raw := qt.Do(c.mux, "GET", "/preguntas?"+base+"&estado=rechazada", c.admin, "").Body.String()
	var generic struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &generic); err != nil || len(generic.Data) != 1 {
		t.Fatalf("listado: %s", raw)
	}
	if _, hay := generic.Data[0]["citas"]; hay {
		t.Error("el listado no trae el array citas")
	}

	for _, q := range []string{"estado=aprobada", "origen=manual", "estado=activo", "estado=borrador&origen=ia"} {
		if l := c.listar(t, base+"&"+q); l.Total != 0 {
			t.Errorf("%s: total %d, want 0", q, l.Total)
		}
	}
	// A draft is listed without filter; an eliminada never.
	c.poner(t, EstadoBorrador, OrigenManual)
	if l := c.listar(t, base); l.Total != 1 {
		t.Errorf("sin filtro: total %d", l.Total)
	}
	c.poner(t, EstadoEliminada, OrigenManual)
	if l := c.listar(t, base); l.Total != 0 {
		t.Errorf("eliminada no se lista: total %d", l.Total)
	}
}

func TestGetPreguntas_FiltroInvalido(t *testing.T) {
	c := newCicloFx(t)
	for _, q := range []string{"estado=eliminada", "estado=xx", "origen=otro", "estado=" + url.QueryEscape("activo' OR 1=1")} {
		w := qt.Do(c.mux, "GET", "/preguntas?"+q, c.admin, "")
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: %d, want 400", q, w.Code)
		}
	}
}
