package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	internalmw "github.com/cbaciliod/learning/internal/middleware"
)

// ── H68 / H17: the JSON body limit does not depend on the Content-Type ──────

// A JSON body sent with a multipart Content-Type must not get the multipart
// ceiling: through the real middleware stack, a 2 MB login body is refused
// with 413 by the middleware itself, before the handler runs. A 415 would
// come from decodeJSON, which refuses the media type whatever the
// middleware did, so it would not prove the limit (H106).
func TestHandlerLogin_MultipartContentType_2MB_Refused(t *testing.T) {
	f := newFixture(t)
	stack := internalmw.MaxBodySize(internalmw.DefaultJSONBodyLimit, internalmw.DefaultMultipartBodyLimit)(f.mux())
	big := `{"username":"` + strings.Repeat("a", 2<<20) + `","password":"x"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(big))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=xyz")
	w := httptest.NewRecorder()
	stack.ServeHTTP(w, req)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("want 413 from the body-size middleware, got %d: %.200s", w.Code, w.Body.String())
	}
}

// decodeJSON applies its own 1 MB cap, even without the global middleware.
func TestDecodeJSON_OwnLimit_413(t *testing.T) {
	h, _ := testSetup(t)
	big := `{"username":"` + strings.Repeat("a", 2<<20) + `","password":"x"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(big))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Login(w, req)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("want 413, got %d", w.Code)
	}
}

// decodeJSON rejects bodies that do not declare application/json.
func TestDecodeJSON_NonJSONContentType_415(t *testing.T) {
	h, _ := testSetup(t)
	for _, ct := range []string{"multipart/form-data; boundary=xyz", "text/plain", "application/x-www-form-urlencoded", ""} {
		req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"username":"a","password":"b"}`))
		if ct != "" {
			req.Header.Set("Content-Type", ct)
		}
		w := httptest.NewRecorder()
		h.Login(w, req)
		if w.Code != http.StatusUnsupportedMediaType {
			t.Errorf("Content-Type %q: want 415, got %d", ct, w.Code)
		}
	}
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"username":"nobody-x","password":"b"}`))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	w := httptest.NewRecorder()
	h.Login(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("application/json; charset=utf-8: want 401, got %d", w.Code)
	}
}
