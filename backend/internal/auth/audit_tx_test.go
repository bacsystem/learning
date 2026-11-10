package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/cbaciliod/learning/internal/audit/audittest"
)

// ── H127: admin mutations are audited inside their own transaction ──────────
//
// If the audit_log row cannot be written the mutation rolls back (and the
// handler answers 500), so no admin change is ever left unaudited.

type auditTxEnv struct {
	f        *fixture
	adminID  int64
	adminTok string
}

func newAuditTxEnv(t *testing.T) *auditTxEnv {
	f := newFixture(t)
	adminID, adminUser := f.activeUser("atxadm", "admin")
	// Log in BEFORE making the admin's audit inserts fail: login audits too.
	return &auditTxEnv{f: f, adminID: adminID, adminTok: f.login(adminUser)}
}

func (e *auditTxEnv) failAudit() { audittest.FailInsertsBy(e.f.t, e.f.db, e.adminID) }

func (e *auditTxEnv) want500(method, path string, body any) {
	e.f.t.Helper()
	if w := e.f.adminDo(e.adminTok, method, path, body); w.Code != http.StatusInternalServerError {
		e.f.t.Fatalf("%s %s with audit_log failing: want 500, got %d %s", method, path, w.Code, w.Body.String())
	}
}

func TestAdminUpdateEmail_AuditFails_RollsBack(t *testing.T) {
	e := newAuditTxEnv(t)
	f := e.f
	target, _ := f.activeUser("atxtgt", "usuario")
	pid := f.personaPID(target)
	before := f.personaEmailHMAC(pid)
	e.failAudit()
	e.want500(http.MethodPatch, fmt.Sprintf("/admin/usuarios/%d/email", target), map[string]string{"email": f.email("atxnew")})
	if f.personaEmailHMAC(pid) != before {
		t.Fatal("the e-mail change was committed although its audit row failed")
	}
	subj, _ := emailCorreoCambiado()
	if f.queuedTo(f.email("atxtgt"), subj) != 0 {
		t.Fatal("the change notice was queued although the change rolled back")
	}
}

func TestAdminUpdatePersona_AuditFails_RollsBack(t *testing.T) {
	e := newAuditTxEnv(t)
	f := e.f
	target, _ := f.activeUser("atxper", "usuario")
	pid := f.personaPID(target)
	before := f.personaEmailHMAC(pid)
	e.failAudit()
	e.want500(http.MethodPut, fmt.Sprintf("/admin/personas/%d", pid), UpdatePersonaRequest{
		Nombres: "Otro", ApellidoPaterno: "Perez", ApellidoMaterno: "Lopez",
		Email: f.email("atxper2"), Telefono: "999999999", Estado: "activo",
		GradoID: f.gradoID, EspecialidadID: f.espID, InstitucionID: f.instID,
	})
	p, err := f.repo.GetPersona(context.Background(), pid)
	if err != nil {
		t.Fatal(err)
	}
	if p.Nombres != "Juan" || f.personaEmailHMAC(pid) != before {
		t.Fatalf("persona update committed although its audit row failed (nombres=%q)", p.Nombres)
	}
}

func TestAdminUpdateEstado_AuditFails_RollsBack(t *testing.T) {
	e := newAuditTxEnv(t)
	f := e.f
	pid, _ := f.persona(f.email("atxest"))
	target := f.user(pid, f.username("atxest"), "Password123!", "usuario", "pendiente")
	e.failAudit()
	e.want500(http.MethodPatch, fmt.Sprintf("/admin/usuarios/%d/estado", target), map[string]string{"estado": "activo"})
	var estado string
	if err := f.db.QueryRow(`SELECT estado FROM usuarios WHERE id=$1`, target).Scan(&estado); err != nil {
		t.Fatal(err)
	}
	if estado != "pendiente" {
		t.Fatalf("estado changed to %q although its audit row failed", estado)
	}
}

// UpdateEstado audits the admin's IP.
func TestAdminUpdateEstado_AuditsIP(t *testing.T) {
	e := newAuditTxEnv(t)
	f := e.f
	pid, _ := f.persona(f.email("atxip"))
	target := f.user(pid, f.username("atxip"), "Password123!", "usuario", "pendiente")
	if w := f.adminDo(e.adminTok, http.MethodPatch, fmt.Sprintf("/admin/usuarios/%d/estado", target),
		map[string]string{"estado": "activo"}); w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d %s", w.Code, w.Body.String())
	}
	var ip *string
	if err := f.db.QueryRow(`SELECT host(ip) FROM audit_log WHERE usuario_id=$1 AND accion='activo' AND entidad_id=$2`,
		e.adminID, target).Scan(&ip); err != nil {
		t.Fatal(err)
	}
	if ip == nil || *ip == "" {
		t.Fatal("update_estado audit row has no IP")
	}
}

func TestAdminResetRegistroLimites_AuditFails_RollsBack(t *testing.T) {
	e := newAuditTxEnv(t)
	f := e.f
	_, dni := f.persona(f.email("atxlim"))
	h := NewHandler(f.svc)
	h.registerMinDuration = 0
	f.registerVia(h, dni, f.email("wrong"), "203.0.113.130:1")
	pairKey, dniKey := h.registerFailureKeys(dni, "203.0.113.130")
	e.failAudit()
	if w := f.resetRegistroLimites(e.adminTok, dni); w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d %s", w.Code, w.Body.String())
	}
	if n, _ := h.registerFailures.Count(context.Background(), pairKey); n != 1 {
		t.Fatalf("counters were cleared although the audit row failed (pair=%d)", n)
	}
	if n, _ := h.registerFailuresPerDNI.Count(context.Background(), dniKey); n != 1 {
		t.Fatalf("counters were cleared although the audit row failed (dni=%d)", n)
	}
}

func TestAdminBatchImport_AuditFails_RollsBack(t *testing.T) {
	e := newAuditTxEnv(t)
	f := e.f
	dni := f.suffix[len(f.suffix)-8:]
	e.failAudit()
	e.want500(http.MethodPost, "/admin/personas/batch", BatchPersonasRequest{Personas: []BatchPersonaItem{{
		DNI: dni, Nombres: "Ana", ApellidoPaterno: "Diaz", ApellidoMaterno: "Ruiz",
		GradoID: f.gradoID, EspecialidadID: f.espID, InstitucionID: f.instID,
		Telefono: "999999999", Email: f.email("atxbatch"),
	}}})
	if _, err := f.repo.FindPersonaByDNI(context.Background(), dni); err == nil {
		t.Fatal("batch insert committed although its audit row failed")
	}
}

func TestAdminResetTour_AuditedAndRollsBack(t *testing.T) {
	e := newAuditTxEnv(t)
	f := e.f
	target, _ := f.activeUser("atxtour", "usuario")
	if _, err := f.db.Exec(`UPDATE usuarios SET tour_completado=true WHERE id=$1`, target); err != nil {
		t.Fatal(err)
	}
	stop := audittest.FailInsertsBy(t, f.db, e.adminID)
	e.want500(http.MethodDelete, fmt.Sprintf("/admin/usuarios/%d/tour", target), nil)
	var done bool
	_ = f.db.QueryRow(`SELECT tour_completado FROM usuarios WHERE id=$1`, target).Scan(&done)
	if !done {
		t.Fatal("tour reset committed although its audit row failed")
	}
	stop()
	if w := f.adminDo(e.adminTok, http.MethodDelete, fmt.Sprintf("/admin/usuarios/%d/tour", target), nil); w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d %s", w.Code, w.Body.String())
	}
	if audittest.Count(t, f.db, "reset_tour", target) != 1 {
		t.Fatal("tour reset not audited")
	}
}

// The config endpoints change global settings: the tests write back the
// value already stored, so other tests never see a different configuration.
func TestAdminConfig_AuditedAndRollsBack(t *testing.T) {
	e := newAuditTxEnv(t)
	f := e.f
	ctx := context.Background()
	maxSes, err := f.repo.GetMaxSesiones(ctx)
	if err != nil {
		t.Fatal(err)
	}
	rawRecursos, err := f.repo.GetRecursos(ctx)
	if err != nil {
		t.Fatal(err)
	}
	recursos := []Recurso{}
	_ = json.Unmarshal([]byte(rawRecursos), &recursos)

	countAudit := func(accion string) int {
		var n int
		if err := f.db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE usuario_id=$1 AND accion=$2`, e.adminID, accion).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	// Safety net: should a rolled-back write leak, put the values back.
	t.Cleanup(func() {
		_, _ = f.db.Exec(`UPDATE configuracion_sistema SET valor=$1 WHERE clave='max_sesiones'`, fmt.Sprint(maxSes))
		_, _ = f.db.Exec(`UPDATE configuracion_sistema SET valor=$1 WHERE clave='recursos'`, rawRecursos)
	})

	// With audit_log failing, a DIFFERENT value is sent and must not stick.
	stop := audittest.FailInsertsBy(t, f.db, e.adminID)
	e.want500(http.MethodPut, "/admin/config/sesiones", map[string]int{"max_sesiones": maxSes%10 + 1})
	e.want500(http.MethodPut, "/admin/config/recursos", map[string]any{"recursos": append(recursos,
		Recurso{ID: "atx", Titulo: "atx", URL: "https://example.test/atx"})})
	stop()
	if got, _ := f.repo.GetMaxSesiones(ctx); got != maxSes {
		t.Fatalf("max_sesiones changed to %d although its audit row failed", got)
	}
	if got, _ := f.repo.GetRecursos(ctx); got != rawRecursos {
		t.Fatalf("recursos changed although the audit row failed: %s", got)
	}

	if w := f.adminDo(e.adminTok, http.MethodPut, "/admin/config/sesiones", map[string]int{"max_sesiones": maxSes}); w.Code != http.StatusOK {
		t.Fatalf("sesiones: want 200, got %d %s", w.Code, w.Body.String())
	}
	if w := f.adminDo(e.adminTok, http.MethodPut, "/admin/config/recursos", map[string]any{"recursos": recursos}); w.Code != http.StatusOK {
		t.Fatalf("recursos: want 200, got %d %s", w.Code, w.Body.String())
	}
	if countAudit("config_sesiones") != 1 || countAudit("config_recursos") != 1 {
		t.Fatalf("config changes not audited (sesiones=%d recursos=%d)", countAudit("config_sesiones"), countAudit("config_recursos"))
	}
}
