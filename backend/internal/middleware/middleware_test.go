package middleware

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

var okHandler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

// ── ClientIP ──────────────────────────────────────────────────────────────────

func TestClientIP(t *testing.T) {
	t.Cleanup(func() { SetTrustedProxies("") })
	cases := []struct {
		name, trusted, remote, xff, want string
	}{
		{"no proxy configured ignores XFF", "", "198.51.100.1:1", "1.2.3.4", "198.51.100.1"},
		{"untrusted peer ignores XFF", "10.0.0.0/8", "198.51.100.1:1", "1.2.3.4", "198.51.100.1"},
		{"trusted peer uses XFF", "10.0.0.0/8", "10.1.1.1:1", "203.0.113.5", "203.0.113.5"},
		{"spoofed left-most entry ignored", "10.0.0.0/8", "10.1.1.1:1", "6.6.6.6, 203.0.113.5", "203.0.113.5"},
		{"skips chained trusted proxies", "10.0.0.0/8", "10.1.1.1:1", "203.0.113.5, 10.2.2.2", "203.0.113.5"},
		{"invalid XFF falls back to peer", "10.0.0.0/8", "10.1.1.1:1", "x", "10.1.1.1"},
		{"single IP entry without mask", "10.1.1.1", "10.1.1.1:1", "203.0.113.5", "203.0.113.5"},
		{"invalid remote addr", "", "garbage", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			SetTrustedProxies(c.trusted)
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = c.remote
			if c.xff != "" {
				r.Header.Set("X-Forwarded-For", c.xff)
			}
			if got := ClientIP(r); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

// An invalid list fails closed: no proxy is trusted.
func TestSetTrustedProxies_InvalidListTrustsNoProxy(t *testing.T) {
	t.Cleanup(func() { SetTrustedProxies("") })
	SetTrustedProxies("10.0.0.0/8,nope")
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "10.1.1.1:1"
	r.Header.Set("X-Forwarded-For", "203.0.113.5")
	if got := ClientIP(r); got != "10.1.1.1" {
		t.Errorf("got %q, want the peer 10.1.1.1", got)
	}
}

// ── Rate limiter ──────────────────────────────────────────────────────────────

func TestRateLimiter_BlocksAfterLimit(t *testing.T) {
	rl := NewRateLimiter(2, time.Minute)
	h := rl.Limit(okHandler)
	codes := []int{}
	for i := 0; i < 3; i++ {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = "198.51.100.1:1"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		codes = append(codes, w.Code)
	}
	if codes[0] != 200 || codes[1] != 200 || codes[2] != http.StatusTooManyRequests {
		t.Errorf("got %v, want [200 200 429]", codes)
	}
}

// H19: behind a trusted proxy each client gets its own bucket.
func TestRateLimiter_KeysOnClientIPBehindTrustedProxy(t *testing.T) {
	SetTrustedProxies("10.0.0.0/8")
	t.Cleanup(func() { SetTrustedProxies("") })
	rl := NewRateLimiter(1, time.Minute)
	h := rl.Limit(okHandler)
	for _, client := range []string{"203.0.113.1", "203.0.113.2"} {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = "10.0.0.1:1"
		r.Header.Set("X-Forwarded-For", client)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("client %s: got %d, want 200 (separate bucket per client)", client, w.Code)
		}
	}
}

func TestRateLimiter_SpoofedXFFFromUntrustedPeerSharesBucket(t *testing.T) {
	SetTrustedProxies("")
	rl := NewRateLimiter(1, time.Minute)
	h := rl.Limit(okHandler)
	var last int
	for _, spoof := range []string{"1.1.1.1", "2.2.2.2"} {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = "198.51.100.1:1"
		r.Header.Set("X-Forwarded-For", spoof)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		last = w.Code
	}
	if last != http.StatusTooManyRequests {
		t.Errorf("spoofed XFF bypassed limiter: got %d, want 429", last)
	}
}

func TestRateLimiter_ExceededAndRecord(t *testing.T) {
	rl := NewRateLimiter(2, time.Minute)
	if rl.Exceeded("k") {
		t.Fatal("fresh key exceeded")
	}
	rl.Record("k")
	if rl.Exceeded("k") {
		t.Fatal("1/2 must not be exceeded")
	}
	rl.Record("k")
	if !rl.Exceeded("k") {
		t.Fatal("2/2 must be exceeded")
	}
	if rl.Exceeded("other") {
		t.Fatal("keys must be independent")
	}
	// Exceeded does not count.
	rl2 := NewRateLimiter(1, time.Minute)
	for i := 0; i < 5; i++ {
		rl2.Exceeded("k")
	}
	if rl2.Exceeded("k") {
		t.Fatal("Exceeded must not consume quota")
	}
}

func TestRateLimiter_TryAcquireRelease(t *testing.T) {
	rl := NewRateLimiter(2, time.Minute)
	first, second := rl.TryAcquire("k"), rl.TryAcquire("k")
	if !first || !second {
		t.Fatal("first two acquisitions must succeed")
	}
	if rl.TryAcquire("k") {
		t.Fatal("third acquisition must fail")
	}
	rl.Release("k")
	if !rl.TryAcquire("k") {
		t.Fatal("after Release one more acquisition must succeed")
	}
	rl.Release("missing") // no-op, must not panic
}

func TestRateLimiter_TryAcquireConcurrent(t *testing.T) {
	rl := NewRateLimiter(5, time.Minute)
	var mu sync.Mutex
	ok := 0
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if rl.TryAcquire("k") {
				mu.Lock()
				ok++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if ok != 5 {
		t.Fatalf("want exactly 5 acquisitions, got %d", ok)
	}
}

func TestRateLimiter_WindowExpires(t *testing.T) {
	rl := NewRateLimiter(1, 20*time.Millisecond)
	rl.Record("k")
	if !rl.Exceeded("k") {
		t.Fatal("want exceeded")
	}
	time.Sleep(30 * time.Millisecond)
	if rl.Exceeded("k") {
		t.Fatal("window elapsed: want not exceeded")
	}
}

func TestRateLimiter_ResponseIsJSON(t *testing.T) {
	rl := NewRateLimiter(1, time.Minute)
	h := rl.Limit(okHandler)
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("content-type %q, want application/json", ct)
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body["error"] == "" {
		t.Errorf("body not a JSON error: %q", w.Body.String())
	}
}

// ── CORS ──────────────────────────────────────────────────────────────────────

func TestCORS_AllowedOrigin(t *testing.T) {
	h := CORS("https://app.example.com, https://b.example.com")(okHandler)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Origin", "https://b.example.com")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://b.example.com" {
		t.Errorf("allow-origin %q", got)
	}
	if w.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Error("missing allow-credentials")
	}
}

func TestCORS_DeniedOrigin(t *testing.T) {
	h := CORS("https://app.example.com")(okHandler)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Origin", "https://evil.example.com")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("denied origin got allow-origin %q", got)
	}
}

func TestCORS_Preflight(t *testing.T) {
	h := CORS("https://app.example.com")(okHandler)
	r := httptest.NewRequest(http.MethodOptions, "/", nil)
	r.Header.Set("Origin", "https://app.example.com")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusNoContent {
		t.Errorf("preflight got %d, want 204", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Error("missing allow-methods")
	}
}

func TestCORS_EmptyDefaultsToLocalhost3000(t *testing.T) {
	h := CORS("")(okHandler)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Origin", "http://localhost:3000")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Errorf("allow-origin %q", got)
	}
}

// ── Body limit (H17) ──────────────────────────────────────────────────────────

// readAll is a handler that reads the whole body like json.Decoder would and
// reports 413 when the limit trips.
var readAll = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if _, err := io.ReadAll(r.Body); err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
})

func TestMaxBodySize_RejectsLargeJSONByContentLength(t *testing.T) {
	h := MaxBodySize(1024, 4096)(okHandler)
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("a", 2048)))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("got %d, want 413", w.Code)
	}
}

func TestMaxBodySize_LimitsChunkedBodyWhileReading(t *testing.T) {
	h := MaxBodySize(1024, 4096)(readAll)
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("a", 2048)))
	r.ContentLength = -1 // unknown length (chunked)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("got %d, want 413", w.Code)
	}
}

func TestMaxBodySize_AllowsSmallBody(t *testing.T) {
	h := MaxBodySize(1024, 4096)(readAll)
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"a":1}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("got %d, want 200", w.Code)
	}
}

func TestMaxBodySize_MultipartUsesLargerLimit(t *testing.T) {
	h := MaxBodySize(1024, 4096, "POST /")(readAll)
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("a", 2048)))
	r.Header.Set("Content-Type", "multipart/form-data; boundary=xyz")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("multipart 2KB got %d, want 200", w.Code)
	}

	r = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("a", 8192)))
	r.Header.Set("Content-Type", "multipart/form-data; boundary=xyz")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("multipart 8KB got %d, want 413", w.Code)
	}
}

// H68: the multipart ceiling applies only to the declared upload routes; a
// multipart Content-Type on any other route (or method) gets the JSON limit.
func TestMaxBodySize_MultipartOnNonUploadRouteUsesJSONLimit(t *testing.T) {
	h := MaxBodySize(1024, 4096, "POST /upload")(readAll)
	r := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(strings.Repeat("a", 2048)))
	r.Header.Set("Content-Type", "multipart/form-data; boundary=xyz")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("multipart 2KB on non-upload route got %d, want 413", w.Code)
	}
	r = httptest.NewRequest(http.MethodPut, "/upload", strings.NewReader(strings.Repeat("a", 2048)))
	r.Header.Set("Content-Type", "multipart/form-data; boundary=xyz")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("multipart 2KB with another method got %d, want 413", w.Code)
	}
}

// ── Limiter IP normalization ─────────────────────────────────────────────────

func TestLimiterIP(t *testing.T) {
	cases := map[string]string{
		"203.0.113.5":                      "203.0.113.5",
		"::ffff:203.0.113.5":               "203.0.113.5",
		"2001:db8:1:2::1":                  "2001:db8:1:2::/64",
		"2001:db8:1:2:ffff:ffff:ffff:ffff": "2001:db8:1:2::/64",
		"2001:db8:1:3::1":                  "2001:db8:1:3::/64",
		"2001:db8:1:2::/64":                "2001:db8:1:2::/64", // idempotent
		"":                                 "",
		"not-an-ip":                        "not-an-ip",
	}
	for in, want := range cases {
		if got := LimiterIP(in); got != want {
			t.Errorf("LimiterIP(%q) = %q, want %q", in, got, want)
		}
	}
}

// Two IPv6 addresses in the same /64 share the in-memory bucket; another /64
// has its own.
func TestRateLimiter_IPv6SameSlash64SharesBucket(t *testing.T) {
	rl := NewRateLimiter(2, time.Minute)
	h := rl.Limit(okHandler)
	do := func(remote string) int {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = remote
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	do("[2001:db8:1:2::1]:1")
	do("[2001:db8:1:2::2]:1")
	if c := do("[2001:db8:1:2::3]:1"); c != http.StatusTooManyRequests {
		t.Fatalf("third request from the same /64: want 429, got %d", c)
	}
	if c := do("[2001:db8:1:3::1]:1"); c != http.StatusOK {
		t.Fatalf("other /64: want 200, got %d", c)
	}
}

// ── Security headers ──────────────────────────────────────────────────────────

func TestSecurityHeaders(t *testing.T) {
	w := httptest.NewRecorder()
	SecurityHeaders(okHandler).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	for _, k := range []string{"X-Content-Type-Options", "X-Frame-Options", "Strict-Transport-Security"} {
		if w.Header().Get(k) == "" {
			t.Errorf("missing %s", k)
		}
	}
}
