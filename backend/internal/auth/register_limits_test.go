package auth

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// ── Register failure counters persisted in Postgres ─────────────────────────

func (f *fixture) registerVia(h *Handler, dni, email, remote string) int {
	f.t.Helper()
	body := fmt.Sprintf(`{"dni":%q,"email":%q,"username":%q,"password":"Password123!","confirm_password":"Password123!"}`,
		dni, email, f.username("att"))
	req := jsonReq(http.MethodPost, "/auth/register", strings.NewReader(body))
	req.RemoteAddr = remote
	w := httptest.NewRecorder()
	h.Register(w, req)
	return w.Code
}

// A restart (or a second replica) is a new Handler: the failures counted by
// the first one still apply.
func TestHandlerRegister_FailureLimitSurvivesNewHandler(t *testing.T) {
	f := newFixture(t)
	_, dni := f.persona(f.email("owner"))
	const attacker = "203.0.113.90:1"
	h1 := NewHandler(f.svc)
	for i := 0; i < registerFailuresPerDNIAndIP; i++ {
		if c := f.registerVia(h1, dni, f.email(fmt.Sprintf("g%d", i)), attacker); c != http.StatusAccepted {
			t.Fatalf("failure %d: want 202, got %d", i+1, c)
		}
	}
	h2 := NewHandler(f.svc)
	if c := f.registerVia(h2, dni, f.email("gX"), attacker); c != http.StatusTooManyRequests {
		t.Fatalf("new handler instance after %d failures: want 429, got %d", registerFailuresPerDNIAndIP, c)
	}
	// The owner from another IP is not blocked by the per-(dni|ip) counter.
	if c := f.registerVia(h2, dni, f.email("owner"), "198.51.100.90:1"); c != http.StatusAccepted {
		t.Fatalf("owner from another IP: want 202, got %d", c)
	}
	if n := f.countPendientesByDNI(dni); n != 1 {
		t.Fatalf("owner's registration not stored: %d pending", n)
	}
}

// Concurrent failures spread over two handler instances sharing the DB are
// bounded by the limit.
func TestHandlerRegister_ConcurrentFailuresAcrossInstancesBounded(t *testing.T) {
	f := newFixture(t)
	_, dni := f.persona(f.email("owner"))
	hs := []*Handler{NewHandler(f.svc), NewHandler(f.svc)}
	const n = 3 * registerFailuresPerDNIAndIP
	var wg sync.WaitGroup
	var mu sync.Mutex
	passed := 0
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if c := f.registerVia(hs[i%2], dni, f.email(fmt.Sprintf("c%d", i)), "203.0.113.91:1"); c != http.StatusTooManyRequests {
				mu.Lock()
				passed++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	if passed != registerFailuresPerDNIAndIP {
		t.Fatalf("%d attempts got past the shared limiter, want exactly %d", passed, registerFailuresPerDNIAndIP)
	}
}

// The persisted keys never contain the raw DNI or IP.
func TestHandlerRegister_FailureCountersStoreNoRawPII(t *testing.T) {
	f := newFixture(t)
	_, dni := f.persona(f.email("owner"))
	const ip = "203.0.113.92"
	f.registerVia(NewHandler(f.svc), dni, f.email("wrong"), ip+":1")
	var total, leaked int
	if err := f.db.QueryRow(`SELECT COUNT(*) FILTER (WHERE clave LIKE '%'||$1||'%'),
		COUNT(*) FILTER (WHERE clave LIKE '%'||$2||'%' OR clave LIKE '%'||$3||'%')
		FROM auth_intentos_fallidos`, f.repo.dniHMAC(dni), dni, ip).Scan(&total, &leaked); err != nil {
		t.Fatal(err)
	}
	if total != 2 || leaked != 0 {
		t.Fatalf("want 2 hashed counters and no raw PII, got total=%d leaked=%d", total, leaked)
	}
}

func (f *fixture) countPendientesByDNI(dni string) int {
	f.t.Helper()
	var n int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM registros_pendientes rp JOIN personas p ON p.id=rp.persona_id
		WHERE p.dni_hmac=$1 AND rp.consumed_at IS NULL`, f.repo.dniHMAC(dni)).Scan(&n); err != nil {
		f.t.Fatal(err)
	}
	return n
}
