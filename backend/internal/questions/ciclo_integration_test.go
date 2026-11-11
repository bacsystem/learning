package questions

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/cbaciliod/learning/internal/questions/questionstest"
)

// cicloFx is a mux with one admin and one question per test.
type cicloFx struct {
	f     *qt.Fixture
	mux   *http.ServeMux
	admin string
	p     qt.Pregunta
}

func newCicloFx(t *testing.T) *cicloFx {
	t.Helper()
	f, mux := newMux(t, qt.Options{Asignaturas: []qt.AsignaturaSpec{{NumeroPreguntas: 1, Peso: 100, Unicas: 1}}})
	return &cicloFx{f: f, mux: mux, admin: f.Token(t, f.UsuarioID, "admin"), p: f.Preguntas[0]}
}

// poner leaves the question in the given state and origin.
func (c *cicloFx) poner(t *testing.T, estado, origen string) {
	t.Helper()
	motivo := (*string)(nil)
	if estado == EstadoRechazada {
		m := "Clave incorrecta"
		motivo = &m
	}
	if _, err := c.f.DB.Exec(`UPDATE preguntas SET estado = $2, origen = $3, motivo_rechazo = $4 WHERE id = $1`, c.p.ID, estado, origen, motivo); err != nil {
		t.Fatal(err)
	}
}

func (c *cicloFx) citar(t *testing.T, pasajeID int64) {
	t.Helper()
	if _, err := c.f.DB.Exec(`INSERT INTO pregunta_citas (pregunta_id, pasaje_id) VALUES ($1, $2)`, c.p.ID, pasajeID); err != nil {
		t.Fatal(err)
	}
}

func (c *cicloFx) patch(body string) *httptest.ResponseRecorder {
	return qt.Do(c.mux, "PATCH", fmt.Sprintf("/preguntas/%d/estado", c.p.ID), c.admin, body)
}

func (c *cicloFx) estado(t *testing.T) (estado string, motivo *string) {
	t.Helper()
	if err := c.f.DB.QueryRow(`SELECT estado, motivo_rechazo FROM preguntas WHERE id = $1`, c.p.ID).Scan(&estado, &motivo); err != nil {
		t.Fatal(err)
	}
	return estado, motivo
}

func (c *cicloFx) auditoria(t *testing.T, accion string) []map[string]any {
	t.Helper()
	rows, err := c.f.DB.Query(`SELECT detalle FROM audit_log WHERE accion = $1 AND entidad = 'pregunta' AND entidad_id = $2 ORDER BY id`, accion, c.p.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			t.Fatal(err)
		}
		var d map[string]any
		if err := json.Unmarshal(raw, &d); err != nil {
			t.Fatal(err)
		}
		out = append(out, d)
	}
	return out
}

func codigoDeRespuesta(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var e struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
		t.Fatalf("cuerpo no JSON: %s", w.Body.String())
	}
	return e.Error
}

// esperar checks the status and, when code is not empty, the error code.
func (c *cicloFx) esperar(t *testing.T, w *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if w.Code != status {
		t.Fatalf("status = %d, want %d (%s)", w.Code, status, w.Body.String())
	}
	if code != "" {
		if got := codigoDeRespuesta(t, w); got != code {
			t.Fatalf("código = %s, want %s (%s)", got, code, w.Body.String())
		}
	}
}

// Escenarios «Aprobar una pregunta manual con cita» y «Rechazar y reabrir».
func TestPatchEstado_AprobarManualConCita(t *testing.T) {
	c := newCicloFx(t)
	c.poner(t, EstadoBorrador, OrigenManual)
	c.citar(t, pasajeDePrueba(t, c.f.DB))

	w := c.patch(`{"estado":"aprobada","verificado":true,"nota":"Revisada contra la NTS."}`)
	c.esperar(t, w, http.StatusOK, "")
	var p PreguntaResponse
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil || p.Estado != EstadoAprobada || p.Citas == nil || len(*p.Citas) != 1 {
		t.Fatalf("respuesta: %+v (%v)", p, err)
	}
	if e, _ := c.estado(t); e != EstadoAprobada {
		t.Errorf("estado = %s", e)
	}
	var servible bool
	if err := c.f.DB.QueryRow(`SELECT ` + PreguntaServible("p") + ` FROM preguntas p WHERE id = ` + fmt.Sprint(c.p.ID)).Scan(&servible); err != nil || !servible {
		t.Errorf("la aprobada debe servirse (%v, %v)", servible, err)
	}
	aud := c.auditoria(t, accionEstado)
	if len(aud) != 1 || aud[0]["estado_anterior"] != EstadoBorrador || aud[0]["estado_nuevo"] != EstadoAprobada ||
		aud[0]["verificado"] != true || aud[0]["nota"] != "Revisada contra la NTS." {
		t.Errorf("auditoría: %+v", aud)
	}
}

func TestPatchEstado_RechazarYReabrir(t *testing.T) {
	c := newCicloFx(t)
	c.poner(t, EstadoBorrador, OrigenManual)

	c.esperar(t, c.patch(`{"estado":"rechazada"}`), http.StatusUnprocessableEntity, CodigoValidacion)
	c.esperar(t, c.patch(`{"estado":"rechazada","motivo":"La clave no coincide con la NTS."}`), http.StatusOK, "")
	if e, m := c.estado(t); e != EstadoRechazada || m == nil || *m != "La clave no coincide con la NTS." {
		t.Fatalf("rechazada: %s %v", e, m)
	}
	c.esperar(t, c.patch(`{"estado":"borrador"}`), http.StatusOK, "")
	if e, m := c.estado(t); e != EstadoBorrador || m != nil {
		t.Fatalf("reabierta: %s %v", e, m)
	}
	aud := c.auditoria(t, accionEstado)
	if len(aud) != 2 || aud[0]["motivo"] != "La clave no coincide con la NTS." || aud[1]["motivo"] != nil || aud[1]["estado_anterior"] != EstadoRechazada {
		t.Errorf("auditoría: %+v", aud)
	}
}

// Escenarios de guardas, con el código de cada uno y sin cambiar la pregunta.
func TestPatchEstado_Guardas(t *testing.T) {
	tests := []struct {
		name           string
		estado, origen string
		cita           bool
		body           string
		status         int
		code           string
	}{
		{"aprobar sin cita", EstadoBorrador, OrigenManual, false, `{"estado":"aprobada","verificado":true}`, 422, CodigoFaltaCita},
		{"aprobar sin confirmación", EstadoBorrador, OrigenManual, true, `{"estado":"aprobada"}`, 422, CodigoVerificacionRequerida},
		{"origen especializado", EstadoBorrador, OrigenIA, true, `{"estado":"aprobada","verificado":true}`, 409, CodigoRevisionEspecializada},
		{"revalidar ia sin nota", EstadoEnRevision, OrigenIA, true, `{"estado":"aprobada","verificado":true}`, 422, CodigoNotaObligatoria},
		{"orden: ia en revisión sin nota ni casilla", EstadoEnRevision, OrigenIA, true, `{"estado":"aprobada"}`, 422, CodigoNotaObligatoria},
		{"concurrencia: otro admin la rechazó", EstadoRechazada, OrigenManual, true, `{"estado":"aprobada","verificado":true}`, 409, CodigoEstadoInvalido},
		{"estado desconocido", EstadoBorrador, OrigenManual, false, `{"estado":"activo"}`, 422, CodigoValidacion},
		{"campo desconocido", EstadoBorrador, OrigenManual, false, `{"estado":"aprobada","origen":"ia"}`, 400, "cuerpo_invalido"},
		{"verificado con tipo incorrecto", EstadoBorrador, OrigenManual, false, `{"estado":"aprobada","verificado":"si"}`, 400, "cuerpo_invalido"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCicloFx(t)
			c.poner(t, tt.estado, tt.origen)
			if tt.cita {
				c.citar(t, pasajeDePrueba(t, c.f.DB))
			}
			c.esperar(t, c.patch(tt.body), tt.status, tt.code)
			if e, _ := c.estado(t); e != tt.estado {
				t.Errorf("la pregunta cambió a %s", e)
			}
			if aud := c.auditoria(t, accionEstado); len(aud) != 0 {
				t.Errorf("no debe auditar: %+v", aud)
			}
		})
	}
}

func TestPatchEstado_RevalidarIAConNotaYRetiradaDelBanco(t *testing.T) {
	c := newCicloFx(t)
	c.poner(t, EstadoEnRevision, OrigenIA)
	c.citar(t, pasajeDePrueba(t, c.f.DB))
	c.esperar(t, c.patch(`{"estado":"aprobada","verificado":true,"nota":"Contrastada con la fuente actual."}`), http.StatusOK, "")

	// Inherited bank: activo → en_revision by an admin.
	c.poner(t, EstadoActivo, OrigenManual)
	c.esperar(t, c.patch(`{"estado":"en_revision","motivo":"Dudosa"}`), http.StatusOK, "")
	if e, _ := c.estado(t); e != EstadoEnRevision {
		t.Errorf("estado = %s", e)
	}
}

func TestPatchEstado_Parametros(t *testing.T) {
	c := newCicloFx(t)
	do := func(path, body string) *httptest.ResponseRecorder {
		return qt.Do(c.mux, "PATCH", path, c.admin, body)
	}
	c.esperar(t, do("/preguntas/abc/estado", `{"estado":"borrador"}`), 400, "parametro_invalido")
	c.esperar(t, do("/preguntas/0/estado", `{"estado":"borrador"}`), 400, "parametro_invalido")
	c.esperar(t, do("/preguntas/999999999/estado", `{"estado":"borrador"}`), 404, "no_encontrado")
	c.esperar(t, do(fmt.Sprintf("/preguntas/%d/estado", c.p.ID), `{`), 400, "cuerpo_invalido")
	if w := qt.Do(c.mux, "PATCH", fmt.Sprintf("/preguntas/%d/estado", c.p.ID), c.f.Token(t, c.f.UsuarioID, "usuario"), `{"estado":"borrador"}`); w.Code != http.StatusForbidden {
		t.Errorf("rol usuario: %d", w.Code)
	}
}

// P2 of E3-02: DELETE only from aprobada, rechazada and activo.
func TestDelete_SegunEstado(t *testing.T) {
	tests := []struct {
		estado string
		status int
		code   string
	}{
		{EstadoAprobada, http.StatusNoContent, ""},
		{EstadoRechazada, http.StatusNoContent, ""},
		{EstadoActivo, http.StatusNoContent, ""},
		{EstadoBorrador, http.StatusConflict, CodigoEstadoInvalido},
		{EstadoEnRevision, http.StatusConflict, CodigoEstadoInvalido},
		{EstadoEliminada, http.StatusNotFound, ""},
	}
	for _, tt := range tests {
		t.Run(tt.estado, func(t *testing.T) {
			c := newCicloFx(t)
			c.poner(t, tt.estado, OrigenManual)
			w := qt.Do(c.mux, "DELETE", fmt.Sprintf("/preguntas/%d", c.p.ID), c.admin, "")
			c.esperar(t, w, tt.status, tt.code)
			e, _ := c.estado(t)
			aud := c.auditoria(t, accionEstado)
			if tt.status == http.StatusNoContent {
				if e != EstadoEliminada || len(aud) != 1 || aud[0]["estado_anterior"] != tt.estado || aud[0]["estado_nuevo"] != EstadoEliminada {
					t.Errorf("estado=%s auditoría=%+v", e, aud)
				}
				return
			}
			if e != tt.estado || len(aud) != 0 {
				t.Errorf("no debe cambiar ni auditar: estado=%s auditoría=%+v", e, aud)
			}
		})
	}
}
