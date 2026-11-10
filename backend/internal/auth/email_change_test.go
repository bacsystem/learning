package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

// ── H92 (review): changing the recovery e-mail cuts the old channel ─────────

func (f *fixture) queuedTo(addr, subject string) int {
	f.t.Helper()
	var n int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM email_queue WHERE to_address=$1 AND subject=$2`, addr, subject).Scan(&n); err != nil {
		f.t.Fatal(err)
	}
	return n
}

func (f *fixture) bodyTo(addr, subject string) string {
	f.t.Helper()
	var b string
	if err := f.db.QueryRow(`SELECT body_html FROM email_queue WHERE to_address=$1 AND subject=$2 ORDER BY id DESC LIMIT 1`, addr, subject).Scan(&b); err != nil {
		f.t.Fatal(err)
	}
	return b
}

// checkEmailChangeCut asserts the effects of an admin e-mail change: the
// reset token sent to the old address is dead, sessions are revoked, the
// unconsumed pending registration is gone and the old address got a notice
// that does not reveal the new one.
func (f *fixture) checkEmailChangeCut(resetTok, userTok, oldEmail, newEmail string, pendingPID int64) {
	f.t.Helper()
	if err := f.svc.ResetPassword(context.Background(), resetTok, "BrandNewPass1!", "127.0.0.1"); !errors.Is(err, ErrInvalidToken) {
		f.t.Fatalf("reset with a token sent to the old address: want ErrInvalidToken, got %v", err)
	}
	if f.sessionAlive(userTok) {
		f.t.Fatal("sessions must be revoked when the recovery e-mail changes")
	}
	if pendingPID != 0 && f.countPendientes(pendingPID) != 0 {
		f.t.Fatal("unconsumed pending registration survived the e-mail change")
	}
	subj, _ := emailCorreoCambiado()
	if f.queuedTo(oldEmail, subj) != 1 {
		f.t.Fatalf("old address %s did not get the change notice", oldEmail)
	}
	if b := f.bodyTo(oldEmail, subj); strings.Contains(strings.ToLower(b), strings.ToLower(newEmail)) {
		f.t.Fatal("the notice to the old address reveals the new e-mail")
	}
}

func TestAdminUpdateEmail_InvalidatesOldChannel(t *testing.T) {
	f := newFixture(t)
	_, adminUser := f.activeUser("ecadm", "admin")
	adminTok := f.login(adminUser)
	target, targetUser := f.activeUser("ectgt", "usuario")
	oldEmail := f.email("ectgt")
	resetTok := "tok-ec-" + f.suffix
	f.resetToken(target, resetTok, time.Now().Add(time.Hour), false)
	userTok := f.login(targetUser)

	newEmail := f.email("ecnew")
	if w := f.adminDo(adminTok, http.MethodPatch, fmt.Sprintf("/admin/usuarios/%d/email", target),
		map[string]string{"email": newEmail}); w.Code != http.StatusOK {
		t.Fatalf("update e-mail: want 200, got %d %s", w.Code, w.Body.String())
	}
	f.checkEmailChangeCut(resetTok, userTok, oldEmail, newEmail, 0)
}

func TestAdminUpdatePersona_EmailChange_InvalidatesOldChannel(t *testing.T) {
	f := newFixture(t)
	_, adminUser := f.activeUser("epadm", "admin")
	adminTok := f.login(adminUser)
	target, targetUser := f.activeUser("eptgt", "usuario")
	pid := f.personaPID(target)
	oldEmail := f.email("eptgt")
	resetTok := "tok-ep-" + f.suffix
	f.resetToken(target, resetTok, time.Now().Add(time.Hour), false)
	userTok := f.login(targetUser)

	newEmail := f.email("epnew")
	req := UpdatePersonaRequest{
		Nombres: "Juan", ApellidoPaterno: "Perez", ApellidoMaterno: "Lopez",
		Email: newEmail, Telefono: "999999999",
		GradoID: f.gradoID, EspecialidadID: f.espID, InstitucionID: f.instID, Estado: "activo",
	}
	if w := f.adminDo(adminTok, http.MethodPut, fmt.Sprintf("/admin/personas/%d", pid), req); w.Code != http.StatusOK {
		t.Fatalf("update persona: want 200, got %d %s", w.Code, w.Body.String())
	}
	f.checkEmailChangeCut(resetTok, userTok, oldEmail, newEmail, 0)
}

// A persona without account but with a pending registration sent to the old
// address: the pending link dies with the e-mail change.
func TestAdminUpdatePersona_EmailChange_KillsPendingRegistration(t *testing.T) {
	f := newFixture(t)
	_, adminUser := f.activeUser("eqadm", "admin")
	adminTok := f.login(adminUser)
	oldEmail := f.email("eqold")
	pid, dni := f.persona(oldEmail)
	if err := f.register(dni, oldEmail); err != nil {
		t.Fatal(err)
	}
	link := f.lastVerifyToken(oldEmail)
	req := UpdatePersonaRequest{
		Nombres: "Juan", ApellidoPaterno: "Perez", ApellidoMaterno: "Lopez",
		Email: f.email("eqnew"), Telefono: "999999999",
		GradoID: f.gradoID, EspecialidadID: f.espID, InstitucionID: f.instID, Estado: "activo",
	}
	if w := f.adminDo(adminTok, http.MethodPut, fmt.Sprintf("/admin/personas/%d", pid), req); w.Code != http.StatusOK {
		t.Fatalf("update persona: want 200, got %d", w.Code)
	}
	if f.countPendientes(pid) != 0 {
		t.Fatal("pending registration survived the e-mail change")
	}
	if err := f.verify(link, f.username("eq")); !errors.Is(err, ErrInvalidVerifyToken) {
		t.Fatalf("old link after e-mail change: want ErrInvalidVerifyToken, got %v", err)
	}
}
