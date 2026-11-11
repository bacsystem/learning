package auth

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

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
		{"checker error", fakeChecker(nil, errors.New("db down")), "Bearer " + tok, 503},
		{"checker timeout", fakeChecker(nil, context.DeadlineExceeded), "Bearer " + tok, 503},
		{"checker error with a session", fakeChecker(&Session{UserID: 7, Rol: "usuario", Estado: "activo"}, errors.New("partial")), "Bearer " + tok, 503},
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

// Contract «Autenticación con la BD caída»: the three cases, with their bodies,
// and the request never reaches the handler unless the session is valid.
func TestRequireAuth_BDCaida(t *testing.T) {
	tok := issue(t, 7, "usuario")
	cases := []struct {
		name     string
		checker  SessionChecker
		wantCode int
		wantBody string
		wantPasa bool
	}{
		{"missing session → 401", fakeChecker(nil, nil), http.StatusUnauthorized, `{"error":"Sesión revocada."}`, false},
		{"DB error → 503", fakeChecker(nil, errors.New("dial tcp: password=secreto")), http.StatusServiceUnavailable, `{"error":"Servicio no disponible."}`, false},
		{"valid session → passes", fakeChecker(&Session{UserID: 7, Rol: "usuario", Estado: "activo"}, nil), http.StatusOK, "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pasa := false
			h := RequireAuth(testJWTSecret, c.checker)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				pasa = true
				w.WriteHeader(http.StatusOK)
			}))
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Authorization", "Bearer "+tok)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if w.Code != c.wantCode || pasa != c.wantPasa {
				t.Fatalf("got %d (handler reached %v), want %d (%v)", w.Code, pasa, c.wantCode, c.wantPasa)
			}
			if body := strings.TrimSpace(w.Body.String()); c.wantBody != "" && body != c.wantBody {
				t.Errorf("body %s, want %s", body, c.wantBody)
			}
		})
	}
}

// The 503 log carries the op only, never the driver error (it may hold the DSN).
func TestRequireAuth_BDCaidaNoRegistraElError(t *testing.T) {
	var buf bytes.Buffer
	anterior := log.Logger
	log.Logger = zerolog.New(&buf)
	t.Cleanup(func() { log.Logger = anterior })

	if code, _ := serve(t, fakeChecker(nil, errors.New("dial tcp db:5432: password=secreto")), false, "Bearer "+issue(t, 7, "usuario")); code != http.StatusServiceUnavailable {
		t.Fatalf("got %d, want 503", code)
	}
	if out := buf.String(); !strings.Contains(out, "auth.check_session") || strings.Contains(out, "secreto") || strings.Contains(out, "dial tcp") {
		t.Errorf("log %q: want the op and no driver error", out)
	}
}
