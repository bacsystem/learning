package auth

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	jwtpkg "github.com/cbaciliod/learning/internal/jwt"
)

// noSessions is a SessionChecker that finds no session (for tests that only
// exercise unauthenticated paths).
func noSessions(_ context.Context, _ string) (*Session, error) { return nil, nil }

// injectAdminClaims puts admin JWT claims into the request context.
func injectAdminClaims(r *http.Request) *http.Request {
	claims := &jwtpkg.Claims{UserID: 1, Rol: "admin"}
	ctx := context.WithValue(r.Context(), claimsKey, claims)
	return r.WithContext(ctx)
}

// ── BatchImport ───────────────────────────────────────────────────────────────

func TestAdminHandler_BatchImport_ValidationError(t *testing.T) {
	ah, _ := testAdminSetup(t)
	// Personas array present but items are missing required fields
	body := `{"personas":[{"dni":"123"}]}`
	req := jsonReq(http.MethodPost, "/admin/personas/batch", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = injectAdminClaims(req)

	w := httptest.NewRecorder()
	ah.BatchImport(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("want 422, got %d", w.Code)
	}
}

func TestAdminHandler_BatchImport_InvalidJSON(t *testing.T) {
	ah, _ := testAdminSetup(t)
	req := jsonReq(http.MethodPost, "/admin/personas/batch", bytes.NewBufferString("{bad"))
	w := httptest.NewRecorder()
	ah.BatchImport(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestAdminHandler_BatchImport_RequiresAdminRole(t *testing.T) {
	_, svc := testAdminSetup(t)
	mux := http.NewServeMux()
	Mount(mux, svc, "test-secret-32-bytes-exactly-ok!", noSessions)

	// No Authorization header → 401
	req := jsonReq(http.MethodPost, "/admin/personas/batch", bytes.NewBufferString(`{}`))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401 without auth, got %d", w.Code)
	}
}

// ── UpdateEstado ─────────────────────────────────────────────────────────────

func TestAdminHandler_UpdateEstado_InvalidID(t *testing.T) {
	ah, _ := testAdminSetup(t)
	body := `{"estado":"activo","observacion":""}`
	req := jsonReq(http.MethodPatch, "/admin/usuarios/abc/estado", bytes.NewBufferString(body))
	req.SetPathValue("id", "abc") // handler is called directly, not through the ServeMux
	req.Header.Set("Content-Type", "application/json")
	req = injectAdminClaims(req)

	w := httptest.NewRecorder()
	ah.UpdateEstado(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400 for invalid ID, got %d", w.Code)
	}
}

func TestAdminHandler_UpdateEstado_InvalidEstado(t *testing.T) {
	ah, _ := testAdminSetup(t)
	body := `{"estado":"superadmin","observacion":""}`
	req := jsonReq(http.MethodPatch, "/admin/usuarios/1/estado", bytes.NewBufferString(body))
	req.SetPathValue("id", "1") // handler is called directly, not through the ServeMux
	req.Header.Set("Content-Type", "application/json")
	req = injectAdminClaims(req)

	w := httptest.NewRecorder()
	ah.UpdateEstado(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("want 422 for invalid estado, got %d", w.Code)
	}
}

func TestAdminHandler_UpdateEstado_UserNotFound(t *testing.T) {
	ah, _ := testAdminSetup(t)
	body := `{"estado":"activo","observacion":""}`
	req := jsonReq(http.MethodPatch, "/admin/usuarios/99999999/estado", bytes.NewBufferString(body))
	req.SetPathValue("id", "99999999") // handler is called directly, not through the ServeMux
	req.Header.Set("Content-Type", "application/json")
	req = injectAdminClaims(req)

	w := httptest.NewRecorder()
	ah.UpdateEstado(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("want 404 for unknown user, got %d", w.Code)
	}
}

// PATCH /admin/usuarios/{id}/estado moved to internal/inscripciones package.

func testAdminSetup(t *testing.T) (*AdminHandler, *Service) {
	t.Helper()
	_, svc := testSetup(t)
	ah := NewAdminHandler(svc)
	return ah, svc
}

// ── Config Recursos ───────────────────────────────────────────────────────────

func TestAdminHandler_SetConfigRecursos_InvalidJSON(t *testing.T) {
	ah, _ := testAdminSetup(t)
	req := jsonReq(http.MethodPut, "/admin/config/recursos", bytes.NewBufferString("{bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ah.SetConfigRecursos(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestAdminHandler_SetConfigRecursos_InvalidURL(t *testing.T) {
	ah, _ := testAdminSetup(t)
	body := `{"recursos":[{"id":"a","titulo":"Refs","url":"no-es-url","orden":1,"activo":true}]}`
	req := jsonReq(http.MethodPut, "/admin/config/recursos", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ah.SetConfigRecursos(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("want 422, got %d", w.Code)
	}
}

func TestAdminHandler_SetConfigRecursos_MissingTitulo(t *testing.T) {
	ah, _ := testAdminSetup(t)
	body := `{"recursos":[{"id":"a","titulo":"","url":"https://example.com","orden":1,"activo":true}]}`
	req := jsonReq(http.MethodPut, "/admin/config/recursos", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ah.SetConfigRecursos(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("want 422, got %d", w.Code)
	}
}

func TestRoutes_GetConfigRecursos_RequiresAuth(t *testing.T) {
	_, svc := testAdminSetup(t)
	mux := http.NewServeMux()
	Mount(mux, svc, "test-secret-32-bytes-exactly-ok!", noSessions)

	// Sin Authorization → 401
	req := httptest.NewRequest(http.MethodGet, "/admin/config/recursos", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401 without auth, got %d", w.Code)
	}
}
