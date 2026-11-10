package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	jwtpkg "github.com/cbaciliod/learning/internal/jwt"
)

// Unit tests for RequireAuth / RequireAdmin with a fake SessionChecker (no DB).

func issue(t *testing.T, userID int64, rol string) string {
	t.Helper()
	tok, _, err := jwtpkg.Issue(testJWTSecret, userID, rol, "suboficial", 1, 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func fakeChecker(s *Session, err error) SessionChecker {
	return func(context.Context, string) (*Session, error) { return s, err }
}

// serve runs handler protected by RequireAuth(+RequireAdmin when admin) and
// returns the status and the claims seen by the handler.
func serve(t *testing.T, checker SessionChecker, admin bool, authz string) (int, *jwtpkg.Claims) {
	t.Helper()
	var seen *jwtpkg.Claims
	var h http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = ClaimsFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	if admin {
		h = RequireAdmin(h)
	}
	h = RequireAuth(testJWTSecret, checker)(h)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if authz != "" {
		req.Header.Set("Authorization", authz)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w.Code, seen
}

func TestRequireAuth(t *testing.T) {
	tok := issue(t, 7, "usuario")
	cases := []struct {
		name    string
		checker SessionChecker
		authz   string
		want    int
	}{
		{"no header", fakeChecker(&Session{UserID: 7, Rol: "usuario", Estado: "activo"}, nil), "", 401},
		{"not bearer", fakeChecker(&Session{UserID: 7, Rol: "usuario", Estado: "activo"}, nil), "Basic abc", 401},
		{"bad signature", fakeChecker(&Session{UserID: 7, Rol: "usuario", Estado: "activo"}, nil), "Bearer " + tok + "x", 401},
		{"session missing", fakeChecker(nil, nil), "Bearer " + tok, 401},
		{"checker error", fakeChecker(nil, errors.New("db down")), "Bearer " + tok, 401},
		{"session of another user", fakeChecker(&Session{UserID: 8, Rol: "usuario", Estado: "activo"}, nil), "Bearer " + tok, 401},
		{"user inactivo", fakeChecker(&Session{UserID: 7, Rol: "usuario", Estado: "inactivo"}, nil), "Bearer " + tok, 401},
		{"user rechazado", fakeChecker(&Session{UserID: 7, Rol: "usuario", Estado: "rechazado"}, nil), "Bearer " + tok, 401},
		{"active session", fakeChecker(&Session{UserID: 7, Rol: "usuario", Estado: "activo"}, nil), "Bearer " + tok, 200},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got, _ := serve(t, c.checker, false, c.authz); got != c.want {
				t.Errorf("got %d, want %d", got, c.want)
			}
		})
	}
}

// H2: the role in the context comes from the DB session, not the JWT.
func TestRequireAdmin_UsesRoleFromSessionNotJWT(t *testing.T) {
	adminTok := issue(t, 7, "admin")
	code, _ := serve(t, fakeChecker(&Session{UserID: 7, Rol: "usuario", Estado: "activo"}, nil), true, "Bearer "+adminTok)
	if code != http.StatusForbidden {
		t.Errorf("JWT says admin but DB says usuario: got %d, want 403", code)
	}

	userTok := issue(t, 7, "usuario")
	code, claims := serve(t, fakeChecker(&Session{UserID: 7, Rol: "admin", Estado: "activo"}, nil), true, "Bearer "+userTok)
	if code != http.StatusOK || claims == nil || claims.Rol != "admin" {
		t.Errorf("DB promoted to admin: got %d claims=%+v, want 200 with rol=admin", code, claims)
	}
}

func TestRequireAuth_ClaimsCarryDBRole(t *testing.T) {
	tok := issue(t, 7, "admin")
	_, claims := serve(t, fakeChecker(&Session{UserID: 7, Rol: "usuario", Estado: "activo"}, nil), false, "Bearer "+tok)
	if claims == nil || claims.Rol != "usuario" || claims.UserID != 7 || claims.GradoID != 1 {
		t.Errorf("unexpected claims %+v", claims)
	}
}

func TestRequireAdmin_WithoutClaims_403(t *testing.T) {
	h := RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) }))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != http.StatusForbidden {
		t.Errorf("got %d, want 403", w.Code)
	}
}
