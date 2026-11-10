package auth

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ── H76: admin reset of a persona's registration limits ─────────────────────

func (f *fixture) resetRegistroLimites(token, dni string) *httptest.ResponseRecorder {
	f.t.Helper()
	req := jsonReq(http.MethodDelete, "/admin/registro-limites", strings.NewReader(fmt.Sprintf(`{"dni":%q}`, dni)))
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	f.mux().ServeHTTP(w, req)
	return w
}

// An attacker spreading failures over several IPs keeps the owner's per-DNI
// window exhausted; an admin clears it and the owner can register again. The
// audit row names the persona id, never the DNI.
func TestAdminResetRegistroLimites_OwnerCanRegisterAgain(t *testing.T) {
	f := newFixture(t)
	email := f.email("owner")
	pid, dni := f.persona(email)
	h := NewHandler(f.svc)
	h.registerMinDuration = 0 // timing padding is not under test here

	for ipN := 0; ipN*registerFailuresPerDNIAndIP < registerFailuresPerDNI; ipN++ {
		ip := f.testIP()
		for i := 0; i < registerFailuresPerDNIAndIP; i++ {
			if c := f.registerVia(h, dni, f.email(fmt.Sprintf("g%d", i)), ip+":1"); c != http.StatusAccepted {
				t.Fatalf("failure from ip %d: want 202, got %d", ipN, c)
			}
		}
	}
	ownerIP := f.testIP() + ":1"
	if c := f.registerVia(h, dni, email, ownerIP); c != http.StatusTooManyRequests {
		t.Fatalf("owner with exhausted per-DNI window: want 429, got %d", c)
	}

	adminID, adminUser := f.activeUser("rladm", "admin")
	if w := f.resetRegistroLimites(f.login(adminUser), dni); w.Code != http.StatusOK {
		t.Fatalf("admin reset: want 200, got %d %s", w.Code, w.Body.String())
	}
	sent := len(f.verifyEmails(email))
	if c := f.registerVia(h, dni, email, ownerIP); c != http.StatusAccepted {
		t.Fatalf("owner after reset: want 202, got %d", c)
	}
	if got := len(f.verifyEmails(email)); got != sent+1 || f.countPendientes(pid) != 1 {
		t.Fatalf("owner after reset: no verification issued (emails %d → %d)", sent, got)
	}

	var n int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE usuario_id=$1 AND accion='reset_limites_registro'
		AND entidad='personas' AND entidad_id=$2 AND position($3 in COALESCE(detalle::text,''))=0`,
		adminID, pid, dni).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("want one audit row with the persona id and no DNI, got %d", n)
	}
}

// The accepted-registrations cap of the persona is cleared too.
func TestAdminResetRegistroLimites_ClearsIssueCap(t *testing.T) {
	f := newFixture(t)
	email := f.email("owner")
	_, dni := f.persona(email)
	for i := 0; i < registerIssuesPerPersona; i++ {
		if err := f.register(dni, email); err != nil {
			t.Fatal(err)
		}
	}
	_, adminUser := f.activeUser("rlcap", "admin")
	if w := f.resetRegistroLimites(f.login(adminUser), dni); w.Code != http.StatusOK {
		t.Fatalf("admin reset: want 200, got %d", w.Code)
	}
	sent := len(f.verifyEmails(email))
	if err := f.register(dni, email); err != nil {
		t.Fatal(err)
	}
	if got := len(f.verifyEmails(email)); got != sent+1 {
		t.Fatalf("issue cap not cleared: emails %d → %d", sent, got)
	}
}

func TestAdminResetRegistroLimites_NonAdmin403_Unknown404(t *testing.T) {
	f := newFixture(t)
	_, dni := f.persona(f.email("owner"))
	_, user := f.activeUser("rlusr", "usuario")
	if w := f.resetRegistroLimites(f.login(user), dni); w.Code != http.StatusForbidden {
		t.Fatalf("non-admin: want 403, got %d", w.Code)
	}
	_, adminUser := f.activeUser("rl404", "admin")
	tok := f.login(adminUser)
	if w := f.resetRegistroLimites(tok, "00000000"); w.Code != http.StatusNotFound {
		t.Fatalf("unknown DNI: want 404, got %d", w.Code)
	}
	if w := f.resetRegistroLimites(tok, "123"); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid DNI: want 422, got %d", w.Code)
	}
}
