package inscripciones

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cbaciliod/learning/internal/audit/audittest"
	"github.com/cbaciliod/learning/internal/db/dbtest"
	"github.com/jmoiron/sqlx"
)

// PLT-12: deciding an inscripción tells the postulante by e-mail and in the app.

// postulante is a seeded user with a pendiente inscripción and the e-mail
// address its persona was seeded with.
type postulante struct {
	dbtest.User
	email       string
	inscripcion int64
	admin       dbtest.User
}

func nuevoPostulante(t *testing.T, db *sqlx.DB, idioma string) postulante {
	t.Helper()
	cat := dbtest.SeedCatalog(t, db)
	p := postulante{
		User:  dbtest.SeedUser(t, db, cat, "usuario", "activo"),
		admin: dbtest.SeedUser(t, db, cat, "admin", "activo"),
	}
	p.email = strings.TrimPrefix(p.Username, "u") + "@test.local" // see dbtest.SeedUser
	if _, err := db.Exec(`UPDATE usuarios SET idioma=$2 WHERE id=$1`, p.ID, idioma); err != nil {
		t.Fatal(err)
	}
	proc := dbtest.SeedProceso(t, db, cat, testAnio, "activo")
	p.inscripcion = seedInscripcion(t, db, p.ID, proc, "pendiente")
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM email_queue WHERE to_address=$1`, p.email)
		_, _ = db.Exec(`DELETE FROM notificaciones WHERE usuario_id=$1`, p.ID)
	})
	return p
}

func decidir(t *testing.T, h *Handler, p postulante, estado, observacion string) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"estado": estado, "observacion": observacion})
	req := httptest.NewRequest(http.MethodPatch, "/admin/inscripciones/x/estado", bytes.NewReader(body))
	req.SetPathValue("id", fmt.Sprint(p.inscripcion))
	return dbtest.Serve(t, h.UpdateInscripcionEstado, req, p.admin.ID, "admin")
}

type correoEncolado struct{ Asunto, HTML, Estado string }

func correos(t *testing.T, db *sqlx.DB, p postulante) []correoEncolado {
	t.Helper()
	var out []correoEncolado
	rows, err := db.Query(`SELECT subject, body_html, status FROM email_queue WHERE to_address=$1 ORDER BY id`, p.email)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var c correoEncolado
		if err := rows.Scan(&c.Asunto, &c.HTML, &c.Estado); err != nil {
			t.Fatal(err)
		}
		out = append(out, c)
	}
	return out
}

type avisoApp struct {
	Destinatario, Tipo, Clave, Titulo, Cuerpo string
	Parametros                                map[string]string
	CreadoPor                                 int64
}

func avisos(t *testing.T, db *sqlx.DB, p postulante) []avisoApp {
	t.Helper()
	rows, err := db.Query(`SELECT destinatario, tipo, clave, titulo, cuerpo, parametros, creado_por
		FROM notificaciones WHERE usuario_id=$1 ORDER BY id`, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []avisoApp
	for rows.Next() {
		var a avisoApp
		var params []byte
		if err := rows.Scan(&a.Destinatario, &a.Tipo, &a.Clave, &a.Titulo, &a.Cuerpo, &params, &a.CreadoPor); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(params, &a.Parametros); err != nil {
			t.Fatal(err)
		}
		out = append(out, a)
	}
	return out
}

func TestAviso_AprobacionEnEspanol(t *testing.T) {
	db, h, _ := setup(t)
	p := nuevoPostulante(t, db, "es")
	if rr := decidir(t, h, p, "aprobado", ""); rr.Code != http.StatusOK {
		t.Fatalf("approve: %d %s", rr.Code, rr.Body)
	}
	cs := correos(t, db, p)
	if len(cs) != 1 || !strings.HasPrefix(cs[0].Asunto, "Tu inscripción fue aprobada: ") || cs[0].Estado != "pending" {
		t.Fatalf("e-mails = %+v", cs)
	}
	if !strings.Contains(cs[0].HTML, testBase+"/procesos") {
		t.Fatal("the e-mail does not link to /procesos")
	}
	as := avisos(t, db, p)
	if len(as) != 1 || as[0].Clave != claveAprobada || as[0].Destinatario != "usuario" || as[0].Tipo != "sistema" ||
		as[0].CreadoPor != p.admin.ID || as[0].Titulo != "Inscripción aprobada" {
		t.Fatalf("notifications = %+v", as)
	}
	if _, ok := as[0].Parametros["motivo"]; ok || as[0].Parametros["proceso"] == "" || as[0].Parametros["institucion"] == "" {
		t.Fatalf("parametros = %v", as[0].Parametros)
	}
}

func TestAviso_RechazoEnInglesConMotivo(t *testing.T) {
	db, h, _ := setup(t)
	p := nuevoPostulante(t, db, "en")
	if rr := decidir(t, h, p, "rechazado", "  Documento ilegible  "); rr.Code != http.StatusOK {
		t.Fatalf("reject: %d %s", rr.Code, rr.Body)
	}
	cs := correos(t, db, p)
	if len(cs) != 1 || !strings.HasPrefix(cs[0].Asunto, "Your enrollment was rejected: ") || !strings.Contains(cs[0].HTML, "Documento ilegible") {
		t.Fatalf("e-mails = %+v", cs)
	}
	as := avisos(t, db, p)
	if len(as) != 1 || as[0].Clave != claveRechazada || as[0].Titulo != "Enrollment rejected" || as[0].Parametros["motivo"] != "Documento ilegible" {
		t.Fatalf("notifications = %+v", as)
	}
	var obs string
	_ = db.QueryRow(`SELECT observacion FROM inscripciones WHERE id=$1`, p.inscripcion).Scan(&obs)
	if obs != "Documento ilegible" {
		t.Fatalf("stored observacion = %q", obs)
	}
}

func TestAviso_AccionRepetidaNoDuplica(t *testing.T) {
	db, h, _ := setup(t)
	p := nuevoPostulante(t, db, "es")
	decidir(t, h, p, "aprobado", "")
	rr := decidir(t, h, p, "aprobado", "")
	var e struct {
		Error   string            `json:"error"`
		Detalle map[string]string `json:"detalle"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &e)
	if rr.Code != http.StatusConflict || e.Error != "transicion_invalida" || e.Detalle["estado_actual"] != "aprobado" {
		t.Fatalf("repeat: %d %s", rr.Code, rr.Body)
	}
	if len(correos(t, db, p)) != 1 || len(avisos(t, db, p)) != 1 {
		t.Fatal("a repeated decision sent another notice")
	}
}

func TestAviso_MotivoInvalidoNoDecideNiAvisa(t *testing.T) {
	db, h, _ := setup(t)
	p := nuevoPostulante(t, db, "es")
	cases := []struct {
		name, estado, obs, code string
	}{
		{"rechazo sin motivo", "rechazado", "", "motivo_requerido"},
		{"rechazo en blanco", "rechazado", " \t\n ", "motivo_requerido"},
		{"rechazo de 501", "rechazado", strings.Repeat("ñ", 501), "validacion"},
		{"aprobacion de 501", "aprobado", strings.Repeat("a", 501), "validacion"},
		{"estado invalido", "pendiente", "", "validacion"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rr := decidir(t, h, p, c.estado, c.obs)
			var e cuerpoError
			_ = json.Unmarshal(rr.Body.Bytes(), &e)
			if rr.Code != http.StatusUnprocessableEntity || e.Error != c.code {
				t.Fatalf("got %d %s, want 422 %s", rr.Code, rr.Body, c.code)
			}
			if got := estadoOf(t, db, p.inscripcion); got != "pendiente" {
				t.Fatalf("estado = %q, want pendiente", got)
			}
			if len(correos(t, db, p)) != 0 || len(avisos(t, db, p)) != 0 {
				t.Fatal("a refused decision sent a notice")
			}
		})
	}
}

func TestAviso_LimiteExactoSeAcepta(t *testing.T) {
	db, h, _ := setup(t)
	p := nuevoPostulante(t, db, "es")
	if rr := decidir(t, h, p, "rechazado", strings.Repeat("ñ", 500)); rr.Code != http.StatusOK {
		t.Fatalf("500 characters (1000 bytes): want 200, got %d %s", rr.Code, rr.Body)
	}
}

func TestAviso_AprobarConMotivoOpcional(t *testing.T) {
	db, h, _ := setup(t)
	p := nuevoPostulante(t, db, "es")
	if rr := decidir(t, h, p, "aprobado", "Bienvenido"); rr.Code != http.StatusOK {
		t.Fatalf("approve with a note: %d %s", rr.Code, rr.Body)
	}
	if as := avisos(t, db, p); len(as) != 1 {
		t.Fatalf("notifications = %+v", as)
	}
}

func TestAviso_SinCorreoSoloNotificacion(t *testing.T) {
	db, h, _ := setup(t)
	p := nuevoPostulante(t, db, "es")
	if _, err := db.Exec(`UPDATE personas SET email='' WHERE id=$1`, p.PersonaID); err != nil {
		t.Fatal(err)
	}
	if rr := decidir(t, h, p, "aprobado", ""); rr.Code != http.StatusOK {
		t.Fatalf("approve: %d %s", rr.Code, rr.Body)
	}
	if len(avisos(t, db, p)) != 1 {
		t.Fatal("the in-app notice must exist even without an e-mail address")
	}
	var n int
	_ = db.QueryRow(`SELECT COUNT(*) FROM email_queue WHERE to_address=''`).Scan(&n)
	if n != 0 {
		t.Fatalf("%d e-mails queued to an empty address", n)
	}
}

// The notice is written in the decision's transaction: if it cannot be written
// the decision is reverted (and the other way round).
func TestAviso_FalloDelAuditRevierteTodo(t *testing.T) {
	db, h, _ := setup(t)
	p := nuevoPostulante(t, db, "es")
	audittest.FailInsertsBy(t, db, p.admin.ID)
	if rr := decidir(t, h, p, "aprobado", ""); rr.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d %s", rr.Code, rr.Body)
	}
	if got := estadoOf(t, db, p.inscripcion); got != "pendiente" {
		t.Fatalf("estado = %q, want pendiente", got)
	}
	if len(correos(t, db, p)) != 0 || len(avisos(t, db, p)) != 0 {
		t.Fatal("a reverted decision left a notice")
	}
}

func TestAviso_FalloDelAvisoRevierteLaDecision(t *testing.T) {
	db, h, _ := setup(t)
	p := nuevoPostulante(t, db, "es")
	if _, err := db.Exec(`CREATE OR REPLACE FUNCTION test_falla_aviso() RETURNS trigger AS $$
		BEGIN RAISE EXCEPTION 'aviso bloqueado'; END $$ LANGUAGE plpgsql`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TRIGGER test_falla_aviso BEFORE INSERT ON notificaciones
		FOR EACH ROW WHEN (NEW.usuario_id = ` + fmt.Sprint(p.ID) + `) EXECUTE FUNCTION test_falla_aviso()`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(`DROP TRIGGER IF EXISTS test_falla_aviso ON notificaciones`)
		_, _ = db.Exec(`DROP FUNCTION IF EXISTS test_falla_aviso()`)
	})
	if rr := decidir(t, h, p, "aprobado", ""); rr.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d %s", rr.Code, rr.Body)
	}
	if got := estadoOf(t, db, p.inscripcion); got != "pendiente" {
		t.Fatalf("estado = %q, want pendiente (the decision must be reverted)", got)
	}
	if len(correos(t, db, p)) != 0 {
		t.Fatal("an e-mail was queued for a reverted decision")
	}
}

func TestAviso_ReinscripcionGeneraAvisoNuevo(t *testing.T) {
	db, h, _ := setup(t)
	setConfigPago(t, db, testAnio, "10")
	p := nuevoPostulante(t, db, "es")
	if rr := decidir(t, h, p, "rechazado", "Primer motivo"); rr.Code != http.StatusOK {
		t.Fatalf("first reject: %d %s", rr.Code, rr.Body)
	}
	var proc int64
	_ = db.QueryRow(`SELECT proceso_id FROM inscripciones WHERE id=$1`, p.inscripcion).Scan(&proc)
	if rr := inscribir(t, h, p.ID, proc); rr.Code != http.StatusCreated {
		t.Fatalf("re-apply: %d %s", rr.Code, rr.Body)
	}
	if rr := decidir(t, h, p, "rechazado", "Segundo motivo"); rr.Code != http.StatusOK {
		t.Fatalf("second reject: %d %s", rr.Code, rr.Body)
	}
	as := avisos(t, db, p)
	if len(as) != 2 || as[0].Parametros["motivo"] != "Primer motivo" || as[1].Parametros["motivo"] != "Segundo motivo" || len(correos(t, db, p)) != 2 {
		t.Fatalf("notifications = %+v", as)
	}
}

func TestUpdateEstado_ErroresEnFormato2(t *testing.T) {
	db, h, _ := setup(t)
	p := nuevoPostulante(t, db, "es")
	call := func(id, body string) (int, string) {
		req := httptest.NewRequest(http.MethodPatch, "/admin/inscripciones/x/estado", bytes.NewBufferString(body))
		req.SetPathValue("id", id)
		rr := dbtest.Serve(t, h.UpdateInscripcionEstado, req, p.admin.ID, "admin")
		var e cuerpoError
		_ = json.Unmarshal(rr.Body.Bytes(), &e)
		if e.Message == "" && rr.Code != http.StatusOK {
			t.Errorf("%s %s: error without message: %s", id, body, rr.Body)
		}
		return rr.Code, e.Error
	}
	cases := []struct {
		id, body string
		status   int
		code     string
	}{
		{"abc", `{"estado":"aprobado"}`, 400, "parametro_invalido"},
		{"0", `{"estado":"aprobado"}`, 400, "parametro_invalido"},
		{fmt.Sprint(p.inscripcion), `{`, 400, "cuerpo_invalido"},
		{fmt.Sprint(p.inscripcion), `{"observacion":"x"}`, 400, "cuerpo_invalido"},
		{"9000000001", `{"estado":"aprobado"}`, 404, "no_encontrado"},
	}
	for _, c := range cases {
		if status, code := call(c.id, c.body); status != c.status || code != c.code {
			t.Errorf("%s %s: got %d %s, want %d %s", c.id, c.body, status, code, c.status, c.code)
		}
	}
}

type cuerpoError struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}
