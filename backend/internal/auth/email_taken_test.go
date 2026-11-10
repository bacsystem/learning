package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ── H88: ErrEmailTaken through the handler ──────────────────────────────────

// Registering with an e-mail that already belongs to another persona or
// account gets the same generic 202 as a success, creates no pending row and
// counts as a failure.
func TestHandlerRegister_EmailTaken_GenericAndNoPending(t *testing.T) {
	f := newFixture(t)
	h := NewHandler(f.svc)
	h.registerMinDuration = 0

	post := func(dni, email, ip string) (int, map[string]string) {
		body := `{"dni":"` + dni + `","email":"` + email + `"}`
		req := jsonReq(http.MethodPost, "/auth/register", strings.NewReader(body))
		req.RemoteAddr = ip + ":1"
		w := httptest.NewRecorder()
		h.Register(w, req)
		var m map[string]string
		_ = json.Unmarshal(w.Body.Bytes(), &m)
		return w.Code, m
	}

	// (a) persona without e-mail, address used by another persona's account.
	f.activeUser("taken", "usuario")
	pid, dni := f.persona("")
	ip := f.testIP()
	if c, m := post(dni, f.email("taken"), ip); c != http.StatusAccepted || m["message"] != msgRegisterAccepted {
		t.Fatalf("e-mail of another account: want generic 202, got %d %v", c, m)
	}
	if f.countPendientes(pid) != 0 {
		t.Fatal("pending registration created with an e-mail already in use")
	}
	pairKey, _ := h.registerFailureKeys(dni, ip)
	if got := f.counter(registerPairPrefix + pairKey); got != 1 {
		t.Fatalf("e-mail taken must count as a failure, counter=%d", got)
	}

	// (b) persona whose e-mail on file is also the e-mail of another
	// persona's account (legacy duplicate): ExistsByEmail refuses it.
	shared := f.email("dup")
	f.activeUser("dup", "usuario")
	pid2, dni2 := f.persona(shared)
	if c, m := post(dni2, shared, f.testIP()); c != http.StatusAccepted || m["message"] != msgRegisterAccepted {
		t.Fatalf("e-mail on file used by another account: want generic 202, got %d %v", c, m)
	}
	if f.countPendientes(pid2) != 0 {
		t.Fatal("pending registration created for an e-mail already linked to an account")
	}
	if n := len(f.verifyEmails(shared)); n != 0 {
		t.Fatalf("verification e-mail sent to an address already linked to an account (%d)", n)
	}
}
