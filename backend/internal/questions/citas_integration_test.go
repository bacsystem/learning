package questions

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	qt "github.com/cbaciliod/learning/internal/questions/questionstest"
)

func (c *cicloFx) put(body string) *httptest.ResponseRecorder {
	return qt.Do(c.mux, "PUT", fmt.Sprintf("/preguntas/%d/citas", c.p.ID), c.admin, body)
}

func (c *cicloFx) citas(t *testing.T) []int64 {
	t.Helper()
	var out []int64
	rows, err := c.f.DB.Query(`SELECT pasaje_id FROM pregunta_citas WHERE pregunta_id = $1 ORDER BY pasaje_id`, c.p.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		out = append(out, id)
	}
	return out
}

func (c *cicloFx) retirar(t *testing.T, fuenteID int64) {
	t.Helper()
	if _, err := c.f.DB.Exec(`UPDATE fuentes SET estado = 'retirada', motivo_retiro = 'Prueba' WHERE id = $1`, fuenteID); err != nil {
		t.Fatal(err)
	}
}

func lista(ids ...int64) string {
	b, _ := json.Marshal(map[string]any{"pasaje_ids": ids})
	return string(b)
}

func TestPutCitas_ReemplazaYAudita(t *testing.T) {
	c := newCicloFx(t)
	c.poner(t, EstadoBorrador, OrigenManual)
	_, a := fuenteDePrueba(t, c.f.DB)
	_, b := fuenteDePrueba(t, c.f.DB)

	w := c.put(lista(b, a))
	c.esperar(t, w, http.StatusOK, "")
	var p PreguntaResponse
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil || p.Citas == nil || len(*p.Citas) != 2 {
		t.Fatalf("respuesta: %+v (%v)", p, err)
	}
	if e, _ := c.estado(t); e != EstadoBorrador {
		t.Errorf("citar no cambia el estado: %s", e)
	}
	c.esperar(t, c.put(lista(a)), http.StatusOK, "")
	if got := c.citas(t); len(got) != 1 || got[0] != a {
		t.Errorf("citas = %v, want [%d]", got, a)
	}
	aud := c.auditoria(t, accionCitas)
	if len(aud) != 2 {
		t.Fatalf("auditoría: %+v", aud)
	}
	antes, _ := aud[1]["antes"].([]any)
	despues, _ := aud[1]["despues"].([]any)
	if len(antes) != 2 || len(despues) != 1 || antes[0].(float64) > antes[1].(float64) {
		t.Errorf("auditoría de la segunda: %+v", aud[1])
	}

	// Lista idéntica: 200, sin escribir ni auditar.
	c.esperar(t, c.put(lista(a)), http.StatusOK, "")
	if n := len(c.auditoria(t, accionCitas)); n != 2 {
		t.Errorf("la lista idéntica no se audita (%d filas)", n)
	}
	// [] quita todas.
	c.esperar(t, c.put(`{"pasaje_ids":[]}`), http.StatusOK, "")
	if got := c.citas(t); len(got) != 0 {
		t.Errorf("citas = %v", got)
	}
}

func TestPutCitas_Errores(t *testing.T) {
	c := newCicloFx(t)
	c.poner(t, EstadoBorrador, OrigenManual)
	fuente, ok := fuenteDePrueba(t, c.f.DB)
	retiradaID, retirada := fuenteDePrueba(t, c.f.DB)
	c.retirar(t, retiradaID)
	_ = fuente

	muchos := make([]int64, MaxCitas+1)
	for i := range muchos {
		muchos[i] = int64(i + 1)
	}
	tests := []struct {
		name, body string
		status     int
		code       string
	}{
		{"sin pasaje_ids", `{}`, 422, CodigoValidacion},
		{"pasaje_ids null", `{"pasaje_ids":null}`, 422, CodigoValidacion},
		{"repetidos", lista(ok, ok), 422, CodigoValidacion},
		{"límite de citas", lista(muchos...), 422, CodigoLimiteCitas},
		{"pasaje inexistente", lista(ok, 2147483000), 422, CodigoReferenciaInvalida},
		{"cita a fuente retirada", lista(retirada), 422, CodigoFuenteNoActiva},
		{"campo desconocido", `{"pasaje_ids":[],"x":1}`, 400, "cuerpo_invalido"},
		{"tipo incorrecto", `{"pasaje_ids":"1"}`, 400, "cuerpo_invalido"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c.esperar(t, c.put(tt.body), tt.status, tt.code)
			if got := c.citas(t); len(got) != 0 {
				t.Errorf("no debe escribir: %v", got)
			}
		})
	}
	if w := qt.Do(c.mux, "PUT", fmt.Sprintf("/preguntas/%d/citas", 999999999), c.admin, lista(ok)); w.Code != http.StatusNotFound {
		t.Errorf("pregunta inexistente: %d", w.Code)
	}
	c.poner(t, EstadoEliminada, OrigenManual)
	c.esperar(t, c.put(lista(ok)), http.StatusNotFound, "no_encontrado")
}

// Escenarios «Quitar la última cita de una aprobada» y «Revalidar tras fuente retirada».
func TestPutCitas_AprobadaYFuenteRetirada(t *testing.T) {
	c := newCicloFx(t)
	c.poner(t, EstadoAprobada, OrigenManual)
	_, a := fuenteDePrueba(t, c.f.DB)
	_, b := fuenteDePrueba(t, c.f.DB)
	c.citar(t, a)

	c.esperar(t, c.put(`{"pasaje_ids":[]}`), http.StatusConflict, CodigoUltimaCitaActiva)
	c.esperar(t, c.put(lista(a, b)), http.StatusOK, "")
	c.esperar(t, c.put(lista(b)), http.StatusOK, "")
	c.esperar(t, c.put(lista(a)), http.StatusOK, "") // sustitución en un solo PUT
	c.esperar(t, c.put(`{"pasaje_ids":[]}`), http.StatusConflict, CodigoUltimaCitaActiva)
	if e, _ := c.estado(t); e != EstadoAprobada {
		t.Errorf("citas no cambia el estado: %s", e)
	}

	// Revalidar tras fuente retirada: se conserva la cita retirada y se suma una activa.
	fr, retirada := fuenteDePrueba(t, c.f.DB)
	c.poner(t, EstadoEnRevision, OrigenManual)
	c.esperar(t, c.put(lista(a, retirada)), http.StatusOK, "") // estaba activa al citarla
	c.retirar(t, fr)
	c.esperar(t, c.put(lista(a, retirada, b)), http.StatusOK, "")
	c.esperar(t, c.put(lista(retirada, b)), http.StatusOK, "")
	c.esperar(t, c.patch(`{"estado":"aprobada","verificado":true}`), http.StatusOK, "")
	// Ahora aprobada con una activa (b) y una retirada: quitar b no se puede.
	c.esperar(t, c.put(lista(retirada)), http.StatusConflict, CodigoUltimaCitaActiva)
}

func TestPutCitas_ActivoHeredadoSinGuarda(t *testing.T) {
	c := newCicloFx(t)
	_, a := fuenteDePrueba(t, c.f.DB)
	c.citar(t, a)
	c.esperar(t, c.put(`{"pasaje_ids":[]}`), http.StatusOK, "")
}

// Aprobar y quitar la última cita a la vez no pueden dejar una aprobada sin cita.
func TestPutCitas_ConcurrenciaConAprobar(t *testing.T) {
	for i := 0; i < 5; i++ {
		c := newCicloFx(t)
		c.poner(t, EstadoBorrador, OrigenManual)
		_, a := fuenteDePrueba(t, c.f.DB)
		c.citar(t, a)

		var wAprobar, wQuitar *httptest.ResponseRecorder
		qt.Concurrently(2, func(n int) {
			if n == 0 {
				wAprobar = c.patch(`{"estado":"aprobada","verificado":true}`)
			} else {
				wQuitar = c.put(`{"pasaje_ids":[]}`)
			}
		})
		e, _ := c.estado(t)
		if e == EstadoAprobada && len(c.citas(t)) == 0 {
			t.Fatalf("aprobada sin cita: aprobar=%d quitar=%d", wAprobar.Code, wQuitar.Code)
		}
		if wAprobar.Code != 200 && wAprobar.Code != 422 {
			t.Errorf("aprobar: %d %s", wAprobar.Code, wAprobar.Body.String())
		}
		if wQuitar.Code != 200 && wQuitar.Code != 409 {
			t.Errorf("quitar: %d %s", wQuitar.Code, strings.TrimSpace(wQuitar.Body.String()))
		}
	}
}

// R4 of E3-07: citas[] carries seccion, lineas and the excerpt, never the full text.
func TestGetPregunta_CitasConExtracto(t *testing.T) {
	c := newCicloFx(t)
	_, a := fuenteDePrueba(t, c.f.DB)
	largo := "Caso probable\n de dengue  " + strings.Repeat("x", 400)
	if _, err := c.f.DB.Exec(`UPDATE pasajes SET texto = $2, seccion = '3.2 Definición de caso', linea_inicio = 120, linea_fin = 148 WHERE id = $1`, a, largo); err != nil {
		t.Fatal(err)
	}
	c.citar(t, a)

	w := qt.Do(c.mux, "GET", fmt.Sprintf("/preguntas/%d", c.p.ID), c.admin, "")
	c.esperar(t, w, http.StatusOK, "")
	var got struct {
		Citas []map[string]any `json:"citas"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil || len(got.Citas) != 1 {
		t.Fatalf("citas: %s (%v)", w.Body.String(), err)
	}
	cita := got.Citas[0]
	extracto, _ := cita["extracto"].(string)
	if !strings.HasPrefix(extracto, "Caso probable de dengue xxx") || !strings.HasSuffix(extracto, "…") || len([]rune(extracto)) != 301 {
		t.Errorf("extracto = %q", extracto)
	}
	if cita["seccion"] != "3.2 Definición de caso" || cita["linea_inicio"] != float64(120) || cita["linea_fin"] != float64(148) || cita["fuente_estado"] != "activa" {
		t.Errorf("cita = %+v", cita)
	}
	if _, hay := cita["texto"]; hay {
		t.Error("el texto completo no debe salir")
	}
}
