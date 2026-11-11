package questions

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/cbaciliod/learning/internal/questions/questionstest"
)

const retroDePrueba = "Explicación de la pregunta."

// cuerpoPut is a PUT body identical to what is stored; mod edits it.
func (c *cicloFx) cuerpoPut(t *testing.T, mod func(b map[string]any, alts []map[string]any)) string {
	t.Helper()
	if _, err := c.f.DB.Exec(`UPDATE pregunta_retroalimentacion SET explicacion = $2, referencia = NULL WHERE pregunta_id = $1`, c.p.ID, retroDePrueba); err != nil {
		t.Fatal(err)
	}
	var enunciado, tipo, dificultad string
	if err := c.f.DB.QueryRow(`SELECT enunciado, tipo, dificultad FROM preguntas WHERE id = $1`, c.p.ID).Scan(&enunciado, &tipo, &dificultad); err != nil {
		t.Fatal(err)
	}
	rows, err := c.f.DB.Query(`SELECT id, texto, es_correcta, orden FROM respuestas WHERE pregunta_id = $1 ORDER BY orden`, c.p.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var alts []map[string]any
	for rows.Next() {
		var id int64
		var texto string
		var ok bool
		var orden int
		if err := rows.Scan(&id, &texto, &ok, &orden); err != nil {
			t.Fatal(err)
		}
		alts = append(alts, map[string]any{"id": id, "texto": texto, "es_correcta": ok, "orden": orden})
	}
	b := map[string]any{
		"asignatura_id": c.p.AsignaturaID, "enunciado": enunciado, "tipo": tipo, "dificultad": dificultad,
		"retroalimentacion": map[string]any{"explicacion": retroDePrueba},
	}
	if mod != nil {
		mod(b, alts)
	}
	b["respuestas"] = alts
	return jsonString(t, b)
}

func (c *cicloFx) putPregunta(body string) *httptest.ResponseRecorder {
	return qt.Do(c.mux, "PUT", fmt.Sprintf("/preguntas/%d", c.p.ID), c.admin, body)
}

// P3 of E3-02: editing the content sends an approved or inherited question to
// review; metadata does not; comparing is against what is stored.
func TestPutPregunta_ContenidoPasaARevision(t *testing.T) {
	tests := []struct {
		name    string
		estado  string
		mod     func(b map[string]any, alts []map[string]any)
		quedaEn string
	}{
		{"aprobada: enunciado", EstadoAprobada, func(b map[string]any, _ []map[string]any) { b["enunciado"] = "Enunciado corregido de la pregunta" }, EstadoEnRevision},
		{"activo: texto de una alternativa", EstadoActivo, func(_ map[string]any, a []map[string]any) { a[1]["texto"] = "Texto corregido" }, EstadoEnRevision},
		{"aprobada: cuál es la correcta", EstadoAprobada, func(_ map[string]any, a []map[string]any) { a[0]["es_correcta"], a[1]["es_correcta"] = false, true }, EstadoEnRevision},
		{"aprobada: explicación", EstadoAprobada, func(b map[string]any, _ []map[string]any) {
			b["retroalimentacion"] = map[string]any{"explicacion": "Otra explicación distinta."}
		}, EstadoEnRevision},
		{"aprobada: quitar la explicación", EstadoAprobada, func(b map[string]any, _ []map[string]any) { delete(b, "retroalimentacion") }, EstadoEnRevision},
		{"aprobada: solo dificultad", EstadoAprobada, func(b map[string]any, _ []map[string]any) { b["dificultad"] = "dificil" }, EstadoAprobada},
		{"aprobada: reenviar lo mismo", EstadoAprobada, nil, EstadoAprobada},
		{"aprobada: solo el orden", EstadoAprobada, func(_ map[string]any, a []map[string]any) {
			a[0]["orden"], a[1]["orden"] = a[1]["orden"], a[0]["orden"]
		}, EstadoAprobada},
		{"borrador: enunciado", EstadoBorrador, func(b map[string]any, _ []map[string]any) { b["enunciado"] = "Enunciado corregido de la pregunta" }, EstadoBorrador},
		{"en_revision: enunciado", EstadoEnRevision, func(b map[string]any, _ []map[string]any) { b["enunciado"] = "Enunciado corregido de la pregunta" }, EstadoEnRevision},
		{"rechazada: enunciado", EstadoRechazada, func(b map[string]any, _ []map[string]any) { b["enunciado"] = "Enunciado corregido de la pregunta" }, EstadoRechazada},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCicloFx(t)
			c.poner(t, tt.estado, OrigenManual)
			c.esperar(t, c.putPregunta(c.cuerpoPut(t, tt.mod)), http.StatusOK, "")
			e, _ := c.estado(t)
			if e != tt.quedaEn {
				t.Fatalf("estado = %s, want %s", e, tt.quedaEn)
			}
			aud := c.auditoria(t, accionEstado)
			if tt.quedaEn == EstadoEnRevision && tt.estado != EstadoEnRevision {
				if len(aud) != 1 || aud[0]["estado_anterior"] != tt.estado || aud[0]["estado_nuevo"] != EstadoEnRevision || aud[0]["motivo"] != motivoEdicion {
					t.Errorf("auditoría: %+v", aud)
				}
			} else if len(aud) != 0 {
				t.Errorf("no debe auditar: %+v", aud)
			}
		})
	}
}

// Origin balotario: the official key cannot change; the rest follows the table.
func TestPutPregunta_ClaveOficialDeBalotario(t *testing.T) {
	c := newCicloFx(t)
	c.poner(t, EstadoAprobada, OrigenBalotario)

	cambiaClave := c.cuerpoPut(t, func(b map[string]any, a []map[string]any) {
		b["enunciado"] = "Enunciado corregido de la pregunta"
		a[0]["es_correcta"], a[1]["es_correcta"] = false, true
	})
	c.esperar(t, c.putPregunta(cambiaClave), http.StatusUnprocessableEntity, CodigoClaveOficial)
	var enunciado string
	var clave bool
	if err := c.f.DB.QueryRow(`SELECT p.enunciado, (SELECT es_correcta FROM respuestas WHERE pregunta_id = p.id AND orden = 1) FROM preguntas p WHERE p.id = $1`, c.p.ID).Scan(&enunciado, &clave); err != nil {
		t.Fatal(err)
	}
	if enunciado == "Enunciado corregido de la pregunta" || !clave {
		t.Errorf("clave_oficial no debe guardar nada (enunciado=%q, clave=%v)", enunciado, clave)
	}
	if e, _ := c.estado(t); e != EstadoAprobada {
		t.Errorf("estado = %s", e)
	}

	soloTexto := c.cuerpoPut(t, func(_ map[string]any, a []map[string]any) { a[1]["texto"] = "Texto corregido" })
	c.esperar(t, c.putPregunta(soloTexto), http.StatusOK, "")
	if e, _ := c.estado(t); e != EstadoEnRevision {
		t.Errorf("el resto sigue la tabla: estado = %s", e)
	}

	// Another origin may change the key.
	c.poner(t, EstadoBorrador, OrigenIA)
	c.esperar(t, c.putPregunta(cambiaClave), http.StatusOK, "")
}
