package fuentes_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/cbaciliod/learning/internal/db/dbtest"
	"github.com/cbaciliod/learning/internal/fuentes"
	"github.com/jmoiron/sqlx"
)

func (e *subidaEnv) enviarJSON(t *testing.T, method, path, bearer, body string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", bearer)
	}
	rr := httptest.NewRecorder()
	e.h.ServeHTTP(rr, req)
	var out map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	return rr.Code, out
}

func (e *subidaEnv) estado(t *testing.T, id int64, body string) (int, map[string]any) {
	t.Helper()
	return e.enviarJSON(t, http.MethodPatch, fmt.Sprintf("/admin/fuentes/%d/estado", id), e.admin, body)
}

func (e *subidaEnv) oficial(t *testing.T, id int64, body string) (int, map[string]any) {
	t.Helper()
	return e.enviarJSON(t, http.MethodPatch, fmt.Sprintf("/admin/fuentes/%d/oficial", id), e.admin, body)
}

func (e *subidaEnv) editar(t *testing.T, id int64, body string) (int, map[string]any) {
	t.Helper()
	return e.enviarJSON(t, http.MethodPut, fmt.Sprintf("/admin/fuentes/%d", id), e.admin, body)
}

// ── Fixtures: one fuente per state, created through S1/S2 and the worker ────

func (e *subidaEnv) fActiva(t *testing.T, esp ...int64) int64 {
	t.Helper()
	id := e.crearCon(t, "Gestión "+dbtest.Uniq(), "guia", esp...)
	e.publicarV(t, id, 1)
	return id
}

func (e *subidaEnv) fConProcesando(t *testing.T, esp ...int64) int64 {
	t.Helper()
	id := e.fActiva(t, esp...)
	if status, body := e.version(t, id, contenidoUnico()); status != http.StatusAccepted {
		t.Fatalf("v2: %d %v", status, body)
	}
	return id
}

func (e *subidaEnv) fProcesando(t *testing.T, esp ...int64) int64 {
	t.Helper()
	return e.crearCon(t, "Gestión "+dbtest.Uniq(), "guia", esp...)
}

func (e *subidaEnv) fFallida(t *testing.T, esp ...int64) int64 {
	t.Helper()
	id := e.fProcesando(t, esp...)
	e.fallarV(t, id, 1)
	return id
}

func (e *subidaEnv) fUltimaFallida(t *testing.T, esp ...int64) int64 {
	t.Helper()
	id := e.fConProcesando(t, esp...)
	e.fallarV(t, id, 2)
	return id
}

func (e *subidaEnv) fRetirada(t *testing.T, esp ...int64) int64 {
	t.Helper()
	id := e.fActiva(t, esp...)
	if status, body := e.estado(t, id, `{"estado":"retirada","motivo":"Retirada de prueba"}`); status != http.StatusOK {
		t.Fatalf("retirar: %d %v", status, body)
	}
	return id
}

func (e *subidaEnv) fMarcada(t *testing.T, retirar bool) int64 {
	t.Helper()
	id := e.fActiva(t)
	if status, body := e.oficial(t, id, `{"oficial_verificada":true,"entidad_emisora":"MINSA","nota":"Descargado de la web oficial"}`); status != http.StatusOK {
		t.Fatalf("marcar: %d %v", status, body)
	}
	if retirar {
		if status, body := e.estado(t, id, `{"estado":"retirada","motivo":"Retirada de prueba"}`); status != http.StatusOK {
			t.Fatalf("retirar: %d %v", status, body)
		}
	}
	return id
}

// auditoria returns the actor and detalle of the last row of accion on the fuente.
func (e *subidaEnv) auditoria(t *testing.T, accion string, id int64) (int64, map[string]any) {
	t.Helper()
	var fila struct {
		UsuarioID *int64 `db:"usuario_id"`
		Detalle   []byte `db:"detalle"`
	}
	if err := e.db.Get(&fila, `SELECT usuario_id, detalle FROM audit_log WHERE accion = $1 AND entidad = 'fuente' AND entidad_id = $2
		ORDER BY id DESC LIMIT 1`, accion, id); err != nil {
		t.Fatalf("audit %s of %d: %v", accion, id, err)
	}
	var d map[string]any
	_ = json.Unmarshal(fila.Detalle, &d)
	var actor int64
	if fila.UsuarioID != nil {
		actor = *fila.UsuarioID
	}
	return actor, d
}

func (e *subidaEnv) cuentaAuditoria(t *testing.T, accion string, id int64) int {
	t.Helper()
	var n int
	if err := e.db.Get(&n, `SELECT COUNT(*) FROM audit_log WHERE accion = $1 AND entidad = 'fuente' AND entidad_id = $2`, accion, id); err != nil {
		t.Fatal(err)
	}
	return n
}

// llenar seeds counted fuentes until esp is at the per-profession limit.
func (e *subidaEnv) llenar(t *testing.T, esp int64) {
	t.Helper()
	var contadas int
	if err := e.db.Get(&contadas, `SELECT COUNT(*) FROM fuentes f WHERE estado IN ('activa', 'procesando')
		AND (NOT EXISTS (SELECT 1 FROM fuente_especialidades fe WHERE fe.fuente_id = f.id)
		     OR EXISTS (SELECT 1 FROM fuente_especialidades fe WHERE fe.fuente_id = f.id AND fe.especialidad_id = $1))`, esp); err != nil {
		t.Fatal(err)
	}
	e.sembrar(t, fuentes.MaxFuentesPorProfesion-contadas, fuentes.EstadoProcesando, []int64{esp})
}

func (e *subidaEnv) estadoDe(t *testing.T, id int64) string {
	t.Helper()
	var s string
	if err := e.db.Get(&s, `SELECT estado FROM fuentes WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}
	return s
}

// ── S5 ────────────────────────────────────────────────────────────────────────

// Escenarios «Transición inválida», «Retirar sin motivo», «Reintentar fallida»
// and the restoration: every target from every state, with its audit row.
func TestS5_Transiciones(t *testing.T) {
	e := nuevoSubidaEnv(t, nil)
	retirar := `{"estado":"retirada","motivo":"  Norma derogada  "}`
	cases := []struct {
		name    string
		fuente  func(t *testing.T) int64
		body    string
		status  int
		code    string
		estado  string // fuente state after the call
		accion  string // audit row of a successful change
		detalle string
	}{
		{"retirar activa", func(t *testing.T) int64 { return e.fActiva(t) }, retirar, 200, "", "retirada",
			fuentes.AccionRetirar, "map[contenidos_marcados:0 estado_anterior:activa estado_nuevo:retirada motivo:Norma derogada]"},
		{"retirar activa con versión en proceso", func(t *testing.T) int64 { return e.fConProcesando(t) }, retirar, 409, "estado_invalido", "activa", "", ""},
		{"retirar procesando", func(t *testing.T) int64 { return e.fProcesando(t) }, retirar, 409, "estado_invalido", "procesando", "", ""},
		{"retirar fallida", func(t *testing.T) int64 { return e.fFallida(t) }, retirar, 409, "estado_invalido", "fallida", "", ""},
		{"retirar retirada", func(t *testing.T) int64 { return e.fRetirada(t) }, retirar, 409, "estado_invalido", "retirada", "", ""},
		{"retirar sin motivo", func(t *testing.T) int64 { return e.fActiva(t) }, `{"estado":"retirada"}`, 422, "validacion", "activa", "", ""},
		{"restaurar retirada", func(t *testing.T) int64 { return e.fRetirada(t) }, `{"estado":"activa"}`, 200, "", "activa",
			fuentes.AccionRestaurar, "map[estado_anterior:retirada estado_nuevo:activa]"},
		{"restaurar activa", func(t *testing.T) int64 { return e.fActiva(t) }, `{"estado":"activa"}`, 409, "estado_invalido", "activa", "", ""},
		{"restaurar fallida", func(t *testing.T) int64 { return e.fFallida(t) }, `{"estado":"activa"}`, 409, "estado_invalido", "fallida", "", ""},
		{"reintentar fallida", func(t *testing.T) int64 { return e.fFallida(t) }, `{"estado":"procesando"}`, 200, "", "procesando",
			fuentes.AccionReintentar, "map[estado_anterior:fallida estado_nuevo:procesando version:1]"},
		{"reintentar activa con la última fallida", func(t *testing.T) int64 { return e.fUltimaFallida(t) }, `{"estado":"procesando"}`, 200, "", "activa",
			fuentes.AccionReintentar, "map[estado_anterior:activa estado_nuevo:activa version:2]"},
		{"reintentar activa con la última lista", func(t *testing.T) int64 { return e.fActiva(t) }, `{"estado":"procesando"}`, 409, "estado_invalido", "activa", "", ""},
		{"reintentar procesando", func(t *testing.T) int64 { return e.fProcesando(t) }, `{"estado":"procesando"}`, 409, "estado_invalido", "procesando", "", ""},
		{"reintentar retirada", func(t *testing.T) int64 { return e.fRetirada(t) }, `{"estado":"procesando"}`, 409, "estado_invalido", "retirada", "", ""},
		{"reintentar retirada con la última fallida", func(t *testing.T) int64 {
			id := e.fUltimaFallida(t)
			if status, body := e.estado(t, id, `{"estado":"retirada","motivo":"Retirada de prueba"}`); status != http.StatusOK {
				t.Fatalf("retirar: %d %v", status, body)
			}
			return id
		}, `{"estado":"procesando"}`, 409, "estado_invalido", "retirada", "", ""},
		{"estado desconocido", func(t *testing.T) int64 { return e.fActiva(t) }, `{"estado":"fallida"}`, 422, "validacion", "activa", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			id := c.fuente(t)
			status, body := e.estado(t, id, c.body)
			if status != c.status || codigoDe(body) != c.code {
				t.Fatalf("got %d %v, want %d %s", status, body, c.status, c.code)
			}
			if got := e.estadoDe(t, id); got != c.estado {
				t.Errorf("estado = %s, want %s", got, c.estado)
			}
			if c.accion == "" {
				return
			}
			if body["estado"] != c.estado {
				t.Errorf("answer estado = %v", body["estado"])
			}
			actor, d := e.auditoria(t, c.accion, id)
			if actor != e.adminID || fmt.Sprint(d) != c.detalle {
				t.Errorf("audit by %d: %v, want %s", actor, d, c.detalle)
			}
		})
	}

	t.Run("el motivo se guarda recortado y contenidos_marcados solo al retirar", func(t *testing.T) {
		id := e.fActiva(t)
		_, body := e.estado(t, id, retirar)
		if body["motivo_retiro"] != "Norma derogada" || body["contenidos_marcados"] != 0.0 {
			t.Errorf("retirar = %v", body)
		}
		_, body = e.estado(t, id, `{"estado":"activa"}`)
		if _, ok := body["contenidos_marcados"]; ok || body["motivo_retiro"] != nil {
			t.Errorf("restaurar = %v", body)
		}
	})
	t.Run("reintentar deja la versión como nueva para el worker", func(t *testing.T) {
		id := e.fUltimaFallida(t)
		e.exec(t, `UPDATE fuente_versiones SET intentos = 3, claimed_at = NOW() WHERE fuente_id = $1 AND numero = 2`, id)
		_, body := e.estado(t, id, `{"estado":"procesando"}`)
		var v struct {
			Estado       string  `db:"estado"`
			Motivo       *string `db:"motivo_fallo"`
			Intentos     int     `db:"intentos"`
			Reclamada    bool    `db:"reclamada"`
			Procesada    bool    `db:"procesada"`
			SigueVigente bool    `db:"sigue_vigente"`
		}
		if err := e.db.Get(&v, `SELECT v.estado, v.motivo_fallo, v.intentos, v.claimed_at IS NOT NULL AS reclamada,
			v.procesada_at IS NOT NULL AS procesada, f.version_vigente_id = v1.id AS sigue_vigente
			FROM fuente_versiones v JOIN fuentes f ON f.id = v.fuente_id
			JOIN fuente_versiones v1 ON v1.fuente_id = f.id AND v1.numero = 1
			WHERE v.fuente_id = $1 AND v.numero = 2`, id); err != nil {
			t.Fatal(err)
		}
		if v.Estado != "procesando" || v.Motivo != nil || v.Intentos != 0 || v.Reclamada || v.Procesada || !v.SigueVigente {
			t.Errorf("version 2 = %+v", v)
		}
		if ep := fmt.Sprint(body["version_en_proceso"]); ep != "map[estado:procesando motivo_fallo:<nil> numero:2]" {
			t.Errorf("version_en_proceso = %s", ep)
		}
	})
	t.Run("validación antes que 404, y 404", func(t *testing.T) {
		if status, body := e.estado(t, 999999999, `{"estado":"retirada"}`); status != 422 || codigoDe(body) != "validacion" {
			t.Errorf("got %d %v", status, body)
		}
		if status, body := e.estado(t, 999999999, `{"estado":"activa"}`); status != 404 || codigoDe(body) != "no_encontrado" {
			t.Errorf("got %d %v", status, body)
		}
	})
}

// Escenario «Restaurar por encima del límite» and the fuentes limit of
// reintentar: only a fuente that does not count yet is checked.
func TestS5_Limite(t *testing.T) {
	e := nuevoSubidaEnv(t, nil)
	retirada := e.fRetirada(t, e.espA)
	fallida := e.fFallida(t, e.espA)
	activa := e.fUltimaFallida(t, e.espA)
	e.llenar(t, e.espA)
	cases := []struct {
		name   string
		id     int64
		body   string
		status int
		estado string
	}{
		{"restaurar por encima del límite", retirada, `{"estado":"activa"}`, 422, "retirada"},
		{"reintentar una fallida por encima del límite", fallida, `{"estado":"procesando"}`, 422, "fallida"},
		{"reintentar una activa, que ya cuenta", activa, `{"estado":"procesando"}`, 200, "activa"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, body := e.estado(t, c.id, c.body)
			if status != c.status || (status == 422 && (codigoDe(body) != "limite_fuentes" ||
				fmt.Sprint(body["detalle"]) != fmt.Sprintf("map[especialidad_id:%d limite:100]", e.espA))) {
				t.Fatalf("got %d %v, want %d", status, body, c.status)
			}
			if got := e.estadoDe(t, c.id); got != c.estado {
				t.Errorf("estado = %s, want %s", got, c.estado)
			}
		})
	}
	var v1 string
	if err := e.db.Get(&v1, `SELECT estado FROM fuente_versiones WHERE fuente_id = $1 AND numero = 1`, fallida); err != nil || v1 != "fallida" {
		t.Errorf("the rejected retry changed the version: %s %v", v1, err)
	}
}

// suscriptorPrueba records its calls and marks n contents; with err it fails.
type suscriptorPrueba struct {
	nombre  string
	n       int
	err     error
	mu      sync.Mutex
	llamada []string // "fuente/actor/estado seen inside the transaction"
}

func (s *suscriptorPrueba) Nombre() string { return s.nombre }

func (s *suscriptorPrueba) FuenteRetirada(ctx context.Context, tx sqlx.ExtContext, fuenteID, actorID int64) (int, error) {
	var estado string
	if err := sqlx.GetContext(ctx, tx, &estado, `SELECT estado FROM fuentes WHERE id = $1`, fuenteID); err != nil {
		return 0, err
	}
	s.mu.Lock()
	s.llamada = append(s.llamada, fmt.Sprintf("%d/%d/%s", fuenteID, actorID, estado))
	s.mu.Unlock()
	return s.n, s.err
}

// Escenario «Retirar emite el aviso»: one call per subscriber, inside the
// transaction (it already sees 'retirada'), with the fuente and the actor;
// contenidos_marcados is their sum, also in the audit.
func TestS5_Suscriptores(t *testing.T) {
	a, b := &suscriptorPrueba{nombre: "preguntas", n: 2}, &suscriptorPrueba{nombre: "material", n: 3}
	e := nuevoSubidaEnv(t, nil, a, b)
	id := e.fActiva(t)
	status, body := e.estado(t, id, `{"estado":"retirada","motivo":"Norma derogada"}`)
	if status != 200 || body["contenidos_marcados"] != 5.0 {
		t.Fatalf("got %d %v", status, body)
	}
	want := fmt.Sprintf("[%d/%d/retirada]", id, e.adminID)
	if fmt.Sprint(a.llamada) != want || fmt.Sprint(b.llamada) != want {
		t.Errorf("calls = %v and %v, want %s each", a.llamada, b.llamada, want)
	}
	if _, d := e.auditoria(t, fuentes.AccionRetirar, id); d["contenidos_marcados"] != 5.0 {
		t.Errorf("audit = %v", d)
	}
	// Restoring notifies nobody.
	e.estado(t, id, `{"estado":"activa"}`)
	if len(a.llamada) != 1 {
		t.Errorf("restoring called the subscriber: %v", a.llamada)
	}
}

// Regla 3: a subscriber that fails rolls the retirement back: 500, the fuente
// stays active without motivo and nothing is audited.
func TestS5_SuscriptorQueFalla(t *testing.T) {
	bueno, malo := &suscriptorPrueba{nombre: "preguntas", n: 2}, &suscriptorPrueba{nombre: "material", err: errors.New("caído")}
	e := nuevoSubidaEnv(t, nil, bueno, malo)
	id := e.fActiva(t)
	status, body := e.estado(t, id, `{"estado":"retirada","motivo":"Norma derogada"}`)
	if status != http.StatusInternalServerError || codigoDe(body) != "error_interno" {
		t.Fatalf("got %d %v", status, body)
	}
	var motivo *string
	if err := e.db.Get(&motivo, `SELECT motivo_retiro FROM fuentes WHERE id = $1`, id); err != nil || motivo != nil || e.estadoDe(t, id) != "activa" {
		t.Errorf("fuente changed: motivo %v, estado %s, %v", motivo, e.estadoDe(t, id), err)
	}
	if n := e.cuentaAuditoria(t, fuentes.AccionRetirar, id); n != 0 {
		t.Errorf("%d audit rows", n)
	}
	if len(bueno.llamada) != 1 || len(malo.llamada) != 1 {
		t.Errorf("calls = %v, %v", bueno.llamada, malo.llamada)
	}
}

// ── S6 ────────────────────────────────────────────────────────────────────────

// Escenarios «Marcar oficial verificada», «Marcar sin entidad o nota»,
// «Marcar una fuente no activa» and «Desmarcar».
func TestS6_Oficial(t *testing.T) {
	e := nuevoSubidaEnv(t, nil)
	marcar := `{"oficial_verificada":true,"entidad_emisora":"  MINSA  ","nota":"  Descargado de la web oficial  "}`
	desmarcar := `{"oficial_verificada":false,"motivo":"  Era otra edición  "}`
	cases := []struct {
		name    string
		fuente  func(t *testing.T) int64
		body    string
		status  int
		code    string
		marcada bool // mark of the current version after the call
		accion  string
		detalle string
	}{
		{"marcar activa", func(t *testing.T) int64 { return e.fActiva(t) }, marcar, 200, "", true,
			fuentes.AccionMarcarOficial, "map[entidad_emisora:MINSA nota:Descargado de la web oficial version:1]"},
		{"marcar ya marcada", func(t *testing.T) int64 { return e.fMarcada(t, false) }, marcar, 409, "estado_invalido", true, "", ""},
		{"marcar retirada", func(t *testing.T) int64 { return e.fRetirada(t) }, marcar, 409, "estado_invalido", false, "", ""},
		{"marcar procesando", func(t *testing.T) int64 { return e.fProcesando(t) }, marcar, 409, "estado_invalido", false, "", ""},
		{"marcar fallida", func(t *testing.T) int64 { return e.fFallida(t) }, marcar, 409, "estado_invalido", false, "", ""},
		{"marcar sin nota", func(t *testing.T) int64 { return e.fActiva(t) }, `{"oficial_verificada":true,"entidad_emisora":"MINSA"}`, 422, "validacion", false, "", ""},
		{"marcar con entidad de espacios", func(t *testing.T) int64 { return e.fActiva(t) },
			`{"oficial_verificada":true,"entidad_emisora":"    ","nota":"Descargado de la web oficial"}`, 422, "validacion", false, "", ""},
		{"desmarcar activa", func(t *testing.T) int64 { return e.fMarcada(t, false) }, desmarcar, 200, "", false,
			fuentes.AccionDesmarcarOficial, "map[entidad_emisora:MINSA motivo:Era otra edición version:1]"},
		{"desmarcar retirada", func(t *testing.T) int64 { return e.fMarcada(t, true) }, desmarcar, 200, "", false,
			fuentes.AccionDesmarcarOficial, "map[entidad_emisora:MINSA motivo:Era otra edición version:1]"},
		{"desmarcar sin marca", func(t *testing.T) int64 { return e.fActiva(t) }, desmarcar, 409, "estado_invalido", false, "", ""},
		{"desmarcar procesando", func(t *testing.T) int64 { return e.fProcesando(t) }, desmarcar, 409, "estado_invalido", false, "", ""},
		{"desmarcar con motivo corto", func(t *testing.T) int64 { return e.fMarcada(t, false) }, `{"oficial_verificada":false,"motivo":"no"}`, 422, "validacion", true, "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			id := c.fuente(t)
			status, body := e.oficial(t, id, c.body)
			if status != c.status || codigoDe(body) != c.code {
				t.Fatalf("got %d %v, want %d %s", status, body, c.status, c.code)
			}
			var m struct {
				Marcada bool   `db:"oficial_verificada"`
				Por     *int64 `db:"oficial_por"`
			}
			if err := e.db.Get(&m, `SELECT COALESCE(v.oficial_verificada, false) AS oficial_verificada, v.oficial_por
				FROM fuentes f LEFT JOIN fuente_versiones v ON v.id = f.version_vigente_id WHERE f.id = $1`, id); err != nil {
				t.Fatal(err)
			}
			if m.Marcada != c.marcada || (m.Marcada && (m.Por == nil || *m.Por != e.adminID)) || (!m.Marcada && m.Por != nil) {
				t.Errorf("mark = %v by %v, want %v", m.Marcada, m.Por, c.marcada)
			}
			if c.accion == "" {
				return
			}
			oficial, _ := body["oficial"].(map[string]any)
			if c.marcada != (oficial != nil) || (oficial != nil && (oficial["entidad_emisora"] != "MINSA" || oficial["nota"] != "Descargado de la web oficial")) {
				t.Errorf("oficial = %v", body["oficial"])
			}
			if actor, d := e.auditoria(t, c.accion, id); actor != e.adminID || fmt.Sprint(d) != c.detalle {
				t.Errorf("audit by %d: %v, want %s", actor, d, c.detalle)
			}
		})
	}

	t.Run("versión nueva no hereda la marca", func(t *testing.T) {
		id := e.fMarcada(t, false)
		if status, body := e.version(t, id, contenidoUnico()); status != http.StatusAccepted {
			t.Fatalf("v2: %d %v", status, body)
		}
		e.publicarV(t, id, 2)
		_, d := e.get(t, fmt.Sprintf("/admin/fuentes/%d", id), e.admin)
		if d["oficial"] != nil || fmt.Sprint(d["version_vigente"]) != "map[numero:2 oficial_verificada:false pasajes_count:1]" {
			t.Errorf("detalle = %v", d)
		}
	})
	t.Run("404", func(t *testing.T) {
		if status, body := e.oficial(t, 999999999, marcar); status != 404 || codigoDe(body) != "no_encontrado" {
			t.Errorf("got %d %v", status, body)
		}
	})
}

// ── S7 ────────────────────────────────────────────────────────────────────────

// Escenarios «Editar profesiones», «Editar una fuente con una especialidad
// asociada que se desactivó» and regla 7 (procesando → 409).
func TestS7_Editar(t *testing.T) {
	e := nuevoSubidaEnv(t, nil)
	conInactiva := func(t *testing.T) (int64, int64) {
		esp := e.especialidad(t, "activo")
		id := e.fActiva(t, e.espA, esp)
		e.exec(t, `UPDATE especialidades SET estado = 'inactivo' WHERE id = $1`, esp)
		return id, esp
	}
	inactivaNueva := e.especialidad(t, "inactivo")
	cases := []struct {
		name   string
		fuente func(t *testing.T) (int64, string)
		status int
		code   string
	}{
		{"activa: título y profesiones", func(t *testing.T) (int64, string) {
			return e.fActiva(t, e.espA), fmt.Sprintf(`{"titulo":"  Balotario nuevo  ","especialidad_ids":[%d,%d]}`, e.espB, e.espA)
		}, 200, ""},
		{"activa: pasar a todas", func(t *testing.T) (int64, string) {
			return e.fActiva(t, e.espA), `{"titulo":"Balotario nuevo","especialidad_ids":[]}`
		}, 200, ""},
		{"conserva una inactiva ya asociada", func(t *testing.T) (int64, string) {
			id, esp := conInactiva(t)
			return id, fmt.Sprintf(`{"titulo":"Balotario nuevo","especialidad_ids":[%d,%d]}`, e.espA, esp)
		}, 200, ""},
		{"quita una inactiva ya asociada", func(t *testing.T) (int64, string) {
			id, _ := conInactiva(t)
			return id, fmt.Sprintf(`{"titulo":"Balotario nuevo","especialidad_ids":[%d]}`, e.espA)
		}, 200, ""},
		{"agregar una inactiva nueva", func(t *testing.T) (int64, string) {
			return e.fActiva(t, e.espA), fmt.Sprintf(`{"titulo":"Balotario nuevo","especialidad_ids":[%d,%d]}`, e.espA, inactivaNueva)
		}, 422, "referencia_invalida"},
		{"especialidad inexistente", func(t *testing.T) (int64, string) {
			return e.fActiva(t), `{"titulo":"Balotario nuevo","especialidad_ids":[999999999]}`
		}, 422, "referencia_invalida"},
		{"retirada", func(t *testing.T) (int64, string) {
			return e.fRetirada(t), fmt.Sprintf(`{"titulo":"Balotario nuevo","especialidad_ids":[%d]}`, e.espB)
		}, 200, ""},
		{"fallida", func(t *testing.T) (int64, string) {
			return e.fFallida(t), fmt.Sprintf(`{"titulo":"Balotario nuevo","especialidad_ids":[%d]}`, e.espB)
		}, 200, ""},
		{"procesando", func(t *testing.T) (int64, string) {
			return e.fProcesando(t), `{"titulo":"Balotario nuevo","especialidad_ids":[]}`
		}, 409, "estado_invalido"},
		{"referencia antes que estado", func(t *testing.T) (int64, string) {
			return e.fProcesando(t), `{"titulo":"Balotario nuevo","especialidad_ids":[999999999]}`
		}, 422, "referencia_invalida"},
		{"tipo no se edita", func(t *testing.T) (int64, string) {
			return e.fActiva(t), `{"titulo":"Balotario nuevo","especialidad_ids":[],"tipo":"norma"}`
		}, 400, "cuerpo_invalido"},
		{"sin especialidad_ids", func(t *testing.T) (int64, string) {
			return e.fActiva(t), `{"titulo":"Balotario nuevo"}`
		}, 422, "validacion"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			id, body := c.fuente(t)
			var antes struct {
				Titulo  string `db:"titulo"`
				Esp     string `db:"esp"`
				Vigente int64  `db:"version_vigente_id"`
			}
			leer := func() {
				if err := e.db.Get(&antes, `SELECT titulo, COALESCE(version_vigente_id, 0) AS version_vigente_id,
					COALESCE((SELECT string_agg(especialidad_id::text, ',' ORDER BY especialidad_id) FROM fuente_especialidades WHERE fuente_id = f.id), '') AS esp
					FROM fuentes f WHERE id = $1`, id); err != nil {
					t.Fatal(err)
				}
			}
			leer()
			previo := antes
			status, resp := e.editar(t, id, body)
			if status != c.status || codigoDe(resp) != c.code {
				t.Fatalf("got %d %v, want %d %s", status, resp, c.status, c.code)
			}
			leer()
			if c.status != 200 {
				if antes != previo {
					t.Errorf("a rejected edit changed the fuente: %+v → %+v", previo, antes)
				}
				return
			}
			var req struct {
				Titulo string  `json:"titulo"`
				Esp    []int64 `json:"especialidad_ids"`
			}
			_ = json.Unmarshal([]byte(body), &req)
			if antes.Titulo != strings.TrimSpace(req.Titulo) || resp["titulo"] != antes.Titulo || resp["aplica_a_todas"] != (len(req.Esp) == 0) ||
				antes.Vigente != previo.Vigente {
				t.Errorf("after the edit: %+v, answer %v", antes, resp)
			}
			actor, d := e.auditoria(t, fuentes.AccionEditar, id)
			ids := func(csv string) string {
				if csv == "" {
					return "[]"
				}
				return "[" + strings.ReplaceAll(csv, ",", " ") + "]"
			}
			want := fmt.Sprintf("map[antes:map[especialidad_ids:%s titulo:%s] despues:map[especialidad_ids:%s titulo:%s]]",
				ids(previo.Esp), previo.Titulo, ids(antes.Esp), antes.Titulo)
			if actor != e.adminID || fmt.Sprint(d) != want {
				t.Errorf("audit by %d: %v, want %s", actor, d, want)
			}
		})
	}
	// especialidad_ids is required: absent or null never means «todas» (a
	// precision of the contract, an exception to README §1).
	for _, cuerpo := range []string{`{"titulo":"Balotario nuevo"}`, `{"titulo":"Balotario nuevo","especialidad_ids":null}`} {
		t.Run("especialidad_ids requerido: "+cuerpo, func(t *testing.T) {
			id := e.fActiva(t, e.espA)
			status, body := e.editar(t, id, cuerpo)
			campos, _ := body["detalle"].(map[string]any)["campos"].(map[string]any)
			if status != 422 || codigoDe(body) != "validacion" || campos["especialidad_ids"] != "requerido" {
				t.Errorf("got %d %v", status, body)
			}
		})
	}
	t.Run("la marca y la versión no cambian", func(t *testing.T) {
		id := e.fMarcada(t, false)
		_, d := e.editar(t, id, `{"titulo":"Balotario editado","especialidad_ids":[]}`)
		if d["oficial"] == nil || fmt.Sprint(d["version_vigente"]) != "map[numero:1 oficial_verificada:true pasajes_count:1]" {
			t.Errorf("detalle = %v", d)
		}
	})
	t.Run("404", func(t *testing.T) {
		if status, body := e.editar(t, 999999999, `{"titulo":"Balotario","especialidad_ids":[]}`); status != 404 || codigoDe(body) != "no_encontrado" {
			t.Errorf("got %d %v", status, body)
		}
	})
}

// Escenario «Editar profesiones por encima del límite» (S7 regla 5): an
// active fuente counts once, with its new set; a fuente that does not count
// is not checked.
func TestS7_Limite(t *testing.T) {
	e := nuevoSubidaEnv(t, nil)
	enA := e.fActiva(t, e.espA)
	fallida := e.fFallida(t, e.espA)
	e.llenar(t, e.espA) // enA is one of the 100
	e.llenar(t, e.espB)
	cases := []struct {
		name   string
		id     int64
		esp    []int64
		status int
		llena  int64
	}{
		{"mismo conjunto en el límite, solo el título", enA, []int64{e.espA}, 200, 0},
		{"agregar una profesión llena", enA, []int64{e.espA, e.espB}, 422, e.espB},
		{"pasar a todas con otra profesión llena", enA, []int64{}, 422, e.espA},
		{"una fallida no cuenta", fallida, []int64{e.espA, e.espB}, 200, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ids, _ := json.Marshal(c.esp)
			status, body := e.editar(t, c.id, fmt.Sprintf(`{"titulo":"Límite %s","especialidad_ids":%s}`, dbtest.Uniq(), ids))
			if status != c.status {
				t.Fatalf("got %d %v, want %d", status, body, c.status)
			}
			if c.status == 422 {
				detalle := body["detalle"].(map[string]any)
				if codigoDe(body) != "limite_fuentes" || detalle["limite"] != 100.0 {
					t.Errorf("body = %v", body)
				}
				if c.llena != 0 && detalle["especialidad_id"] == nil {
					t.Errorf("detalle without especialidad_id: %v", detalle)
				}
			}
		})
	}

	// The total of 500 also counts the edited fuente once.
	t.Run("en el total de 500, solo el título", func(t *testing.T) {
		var total int
		if err := e.db.Get(&total, `SELECT COUNT(*) FROM fuentes WHERE estado IN ('activa', 'procesando')`); err != nil {
			t.Fatal(err)
		}
		e.sembrar(t, fuentes.MaxFuentesTotal-total, fuentes.EstadoProcesando, []int64{e.especialidad(t, "activo")})
		status, body := e.editar(t, enA, fmt.Sprintf(`{"titulo":"Total %s","especialidad_ids":[%d]}`, dbtest.Uniq(), e.espA))
		if status != http.StatusOK {
			t.Errorf("got %d %v, want 200", status, body)
		}
	})
}
