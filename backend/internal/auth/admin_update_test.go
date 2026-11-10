package auth

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ── H92: admin e-mail/persona edits are audited, unique and revoke claims ──

func (f *fixture) adminDo(token, method, path string, body any) *httptest.ResponseRecorder {
	f.t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		f.t.Fatal(err)
	}
	req := jsonReq(method, path, strings.NewReader(string(b)))
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	f.mux().ServeHTTP(w, req)
	return w
}

// auditRows counts audit rows by admin/accion/persona whose detalle does not
// contain leak.
func (f *fixture) auditRows(adminID int64, accion string, personaID int64, leak string) (total, clean int) {
	f.t.Helper()
	if err := f.db.QueryRow(`SELECT COUNT(*), COUNT(*) FILTER (WHERE position($4 in COALESCE(detalle::text,''))=0)
		FROM audit_log WHERE usuario_id=$1 AND accion=$2 AND entidad='personas' AND entidad_id=$3`,
		adminID, accion, personaID, leak).Scan(&total, &clean); err != nil {
		f.t.Fatal(err)
	}
	return total, clean
}

func (f *fixture) personaPID(userID int64) int64 {
	f.t.Helper()
	var pid int64
	if err := f.db.QueryRow(`SELECT persona_id FROM usuarios WHERE id=$1`, userID).Scan(&pid); err != nil {
		f.t.Fatal(err)
	}
	return pid
}

func (f *fixture) sessionAlive(token string) bool {
	f.t.Helper()
	s, err := f.repo.FindActiveSesion(context.Background(), fmt.Sprintf("%x", sha256.Sum256([]byte(token))))
	if err != nil {
		f.t.Fatal(err)
	}
	return s != nil
}

func TestAdminUpdateEmail_AuditedAndUnique(t *testing.T) {
	f := newFixture(t)
	adminID, adminUser := f.activeUser("ueadm", "admin")
	tok := f.login(adminUser)
	target, _ := f.activeUser("uetgt", "usuario")
	pid := f.personaPID(target)
	f.persona(f.email("other")) // another persona's e-mail

	path := fmt.Sprintf("/admin/usuarios/%d/email", target)
	if w := f.adminDo(tok, http.MethodPatch, path, map[string]string{"email": f.email("Other")}); w.Code != http.StatusConflict {
		t.Fatalf("e-mail of another persona: want 409, got %d %s", w.Code, w.Body.String())
	}
	if f.personaEmailHMAC(pid) != f.repo.rawEmailHMAC(f.email("uetgt")) {
		t.Fatal("e-mail changed despite the conflict")
	}
	newEmail := f.email("nuevo")
	if w := f.adminDo(tok, http.MethodPatch, path, map[string]string{"email": newEmail}); w.Code != http.StatusOK {
		t.Fatalf("update e-mail: want 200, got %d %s", w.Code, w.Body.String())
	}
	if f.personaEmailHMAC(pid) != f.repo.emailHMAC(newEmail) {
		t.Fatal("e-mail not updated")
	}
	// Setting its own current e-mail again is not a conflict.
	if w := f.adminDo(tok, http.MethodPatch, path, map[string]string{"email": newEmail}); w.Code != http.StatusOK {
		t.Fatalf("same e-mail again: want 200, got %d", w.Code)
	}
	if total, clean := f.auditRows(adminID, "update_email", pid, "nuevo"); total != 2 || clean != 2 {
		t.Fatalf("want 2 audit rows without the e-mail, got total=%d clean=%d", total, clean)
	}
}

func TestAdminUpdatePersona_AuditUniqueAndRevokesOnClaimChange(t *testing.T) {
	f := newFixture(t)
	adminID, adminUser := f.activeUser("upadm", "admin")
	tok := f.login(adminUser)
	target, targetUser := f.activeUser("uptgt", "usuario")
	pid := f.personaPID(target)
	f.persona(f.email("taken"))
	var grado2 int64
	if err := f.db.QueryRow(`INSERT INTO grados (institucion_id, nombre, abreviatura, nivel, orden)
		VALUES ($1,$2,'TG2','oficial',2) RETURNING id`, f.instID, "Grado2 "+f.suffix).Scan(&grado2); err != nil {
		t.Fatal(err)
	}
	req := UpdatePersonaRequest{
		Nombres: "Juan", ApellidoPaterno: "Perez", ApellidoMaterno: "Lopez",
		Email: f.email("uptgt"), Telefono: "999999999",
		GradoID: f.gradoID, EspecialidadID: f.espID, InstitucionID: f.instID, Estado: "activo",
	}
	path := fmt.Sprintf("/admin/personas/%d", pid)

	// Duplicate e-mail → 409, nothing changed.
	dup := req
	dup.Email = f.email("taken")
	dup.Telefono = "111111111"
	if w := f.adminDo(tok, http.MethodPut, path, dup); w.Code != http.StatusConflict {
		t.Fatalf("e-mail of another persona: want 409, got %d %s", w.Code, w.Body.String())
	}
	if f.personaEmailHMAC(pid) != f.repo.rawEmailHMAC(f.email("uptgt")) {
		t.Fatal("persona changed despite the conflict")
	}

	// Only teléfono changes → sessions kept.
	userTok := f.login(targetUser)
	tel := req
	tel.Telefono = "988888888"
	if w := f.adminDo(tok, http.MethodPut, path, tel); w.Code != http.StatusOK {
		t.Fatalf("update telefono: want 200, got %d %s", w.Code, w.Body.String())
	}
	if !f.sessionAlive(userTok) {
		t.Fatal("changing only the teléfono revoked the sessions")
	}

	// grado changes → sessions revoked (the JWT carries grado/nivel).
	gr := tel
	gr.GradoID = grado2
	if w := f.adminDo(tok, http.MethodPut, path, gr); w.Code != http.StatusOK {
		t.Fatalf("update grado: want 200, got %d %s", w.Code, w.Body.String())
	}
	if f.sessionAlive(userTok) {
		t.Fatal("changing grado must revoke the usuario's sessions")
	}

	// especialidad too.
	var esp2 int64
	if err := f.db.QueryRow(`INSERT INTO especialidades (institucion_id, nombre, codigo) VALUES ($1,$2,$3) RETURNING id`,
		f.instID, "Esp2 "+f.suffix, "E2"+f.suffix).Scan(&esp2); err != nil {
		t.Fatal(err)
	}
	userTok = f.login(targetUser)
	es := gr
	es.EspecialidadID = esp2
	if w := f.adminDo(tok, http.MethodPut, path, es); w.Code != http.StatusOK || f.sessionAlive(userTok) {
		t.Fatalf("update especialidad: want 200 and sessions revoked, got %d alive=%v", w.Code, f.sessionAlive(userTok))
	}

	total, clean := f.auditRows(adminID, "update_persona", pid, "988888888")
	if total != 3 || clean != 3 {
		t.Fatalf("want 3 audit rows without PII, got total=%d clean=%d", total, clean)
	}
	var detalle string
	if err := f.db.QueryRow(`SELECT detalle::text FROM audit_log WHERE usuario_id=$1 AND accion='update_persona' AND entidad_id=$2
		ORDER BY id DESC LIMIT 1`, adminID, pid).Scan(&detalle); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(detalle, "especialidad_id") || !strings.Contains(detalle, `"sesiones_revocadas": true`) {
		t.Fatalf("detalle should name the changed fields and the revocation: %s", detalle)
	}
}
