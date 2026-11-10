package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// H122: GET /auth/check-email answered whether an address had an account,
// a public enumeration oracle that contradicted the generic 202 of
// POST /auth/register. The route no longer exists.
func TestRoutes_CheckEmailRemoved(t *testing.T) {
	f := newFixture(t)
	f.activeUser("ce", "usuario") // the address has an account
	mux := f.mux()
	req := httptest.NewRequest(http.MethodGet, "/auth/check-email?email="+f.email("ce"), nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("GET /auth/check-email: want 404, got %d %s", w.Code, w.Body.String())
	}
}
