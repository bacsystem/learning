package auth

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ── H75: check-dni goes through persistent limiters and masks names ─────────

// checkDNIFrom calls GET /auth/check-dni through the mounted routes as a
// client with the given IP. The IP's persisted counters are cleared on
// cleanup so re-runs start fresh.
func (f *fixture) checkDNIFrom(mux http.Handler, ip, dni string) *httptest.ResponseRecorder {
	f.t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/auth/check-dni?dni="+dni, nil)
	req.RemoteAddr = ip + ":1234"
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	return w
}

// testIP returns a random documentation-range IP whose persisted limiter
// counters are deleted on cleanup.
func (f *fixture) testIP() string {
	f.t.Helper()
	ip := fmt.Sprintf("198.18.%d.%d", rand.Intn(256), 1+rand.Intn(254))
	f.t.Cleanup(func() {
		if _, err := f.db.Exec(`DELETE FROM auth_intentos_fallidos WHERE clave LIKE '%'||$1||'%'`, f.repo.ipHMAC(ip)); err != nil {
			f.t.Errorf("cleanup ip counters: %v", err)
		}
	})
	return ip
}

// One IP checking many distinct DNIs is an enumeration attempt: the sixth
// distinct DNI within the window is refused, whether the DNIs exist or not.
func TestHandlerCheckDNI_DistinctDNIsPerIPLimited(t *testing.T) {
	f := newFixture(t)
	mux := f.mux()
	ip := f.testIP()
	for i := 0; i < checkDNIDistinctPerIP; i++ {
		_, dni := f.persona("")
		if w := f.checkDNIFrom(mux, ip, dni); w.Code != http.StatusOK {
			t.Fatalf("check %d: want 200, got %d: %s", i+1, w.Code, w.Body.String())
		}
	}
	_, dni := f.persona("")
	if w := f.checkDNIFrom(mux, ip, dni); w.Code != http.StatusTooManyRequests {
		t.Fatalf("distinct DNI #%d from one IP: want 429, got %d", checkDNIDistinctPerIP+1, w.Code)
	}
	// The limit is persisted: a freshly mounted handler (another replica or
	// a restart) still refuses.
	if w := f.checkDNIFrom(f.mux(), ip, dni); w.Code != http.StatusTooManyRequests {
		t.Fatalf("new handler instance: want 429, got %d", w.Code)
	}
	// Another IP is not affected.
	if w := f.checkDNIFrom(mux, f.testIP(), dni); w.Code != http.StatusOK {
		t.Fatalf("other IP: want 200, got %d", w.Code)
	}
}

// Re-checking the same DNI from one IP does not consume the distinct-DNI
// quota, but is itself bounded per (dni, ip).
func TestHandlerCheckDNI_SameDNIRepeatedBoundedPerPair(t *testing.T) {
	f := newFixture(t)
	mux := f.mux()
	ip := f.testIP()
	_, dni := f.persona("")
	for i := 0; i < checkDNIPerDNIAndIP; i++ {
		if w := f.checkDNIFrom(mux, ip, dni); w.Code != http.StatusOK {
			t.Fatalf("check %d: want 200, got %d", i+1, w.Code)
		}
	}
	if w := f.checkDNIFrom(mux, ip, dni); w.Code != http.StatusTooManyRequests {
		t.Fatalf("same DNI past the pair limit: want 429, got %d", w.Code)
	}
	// Repeats did not burn the distinct quota: other DNIs still pass.
	_, other := f.persona("")
	if w := f.checkDNIFrom(mux, ip, other); w.Code != http.StatusOK {
		t.Fatalf("another DNI after repeats: want 200, got %d", w.Code)
	}
}

// Unknown DNIs count too (the 404 is part of the oracle).
func TestHandlerCheckDNI_UnknownDNIsCount(t *testing.T) {
	f := newFixture(t)
	mux := f.mux()
	ip := f.testIP()
	for i := 0; i < checkDNIDistinctPerIP; i++ {
		dni := fmt.Sprintf("0000%04d", 9000+i)
		f.t.Cleanup(func() { f.clearFailureCounters(dni) })
		if w := f.checkDNIFrom(mux, ip, dni); w.Code != http.StatusNotFound {
			t.Fatalf("unknown %d: want 404, got %d", i, w.Code)
		}
	}
	_, dni := f.persona("")
	if w := f.checkDNIFrom(mux, ip, dni); w.Code != http.StatusTooManyRequests {
		t.Fatalf("after %d unknown DNIs: want 429, got %d", checkDNIDistinctPerIP, w.Code)
	}
}

// The public response shows only the first given name and the initial of the
// first surname, and no has_email flag.
func TestHandlerCheckDNI_MasksNames(t *testing.T) {
	f := newFixture(t)
	_, dni := f.persona(f.email("juan.perez"))
	w := f.checkDNIFrom(f.mux(), f.testIP(), dni)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	body := w.Body.String()
	for _, leak := range []string{"Perez", "Lopez"} {
		if strings.Contains(body, leak) {
			t.Fatalf("check-dni leaks the surname %q: %s", leak, body)
		}
	}
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["nombres"] != "Juan" || got["apellido_paterno"] != "P." {
		t.Fatalf("want nombres=Juan apellido_paterno=P., got %v", got)
	}
	if _, ok := got["has_email"]; ok {
		t.Fatalf("has_email must not be returned: %v", got)
	}
}

func TestMaskNames(t *testing.T) {
	cases := []struct{ nombres, ap, wantN, wantAP string }{
		{"Juan Carlos", "Pérez", "Juan", "P."},
		{"  María  ", "Ñahui Quispe", "María", "Ñ."},
		{"", "", "", ""},
	}
	for _, c := range cases {
		n, ap := maskNames(c.nombres, c.ap)
		if n != c.wantN || ap != c.wantAP {
			t.Errorf("maskNames(%q,%q) = %q,%q want %q,%q", c.nombres, c.ap, n, ap, c.wantN, c.wantAP)
		}
	}
}
