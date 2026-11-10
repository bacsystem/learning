package dbtest

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cbaciliod/learning/internal/auth"
	jwtpkg "github.com/cbaciliod/learning/internal/jwt"
	"github.com/jmoiron/sqlx"
)

// JWTSecret is the signing secret for tokens issued by these helpers; pass it
// to the package's Mount/RegisterRoutes in route tests.
const JWTSecret = testJWTSecret

// SessionChecker is the production session checker (auth.FindActiveSesion)
// over db: it reads the user's CURRENT rol/estado from the database, unlike
// Serve's always-active fake (H88).
func SessionChecker(t *testing.T, db *sqlx.DB) auth.SessionChecker {
	t.Helper()
	return auth.NewRepository(db, GCM(t), nil).FindActiveSesion
}

// RealBearer issues a JWT for userID claiming tokenRol and stores the matching
// session row (token_hash = sha256(token)), so RequireAuth with
// SessionChecker accepts it. tokenRol may differ from the DB rol to check
// that the DB, not the token, decides.
func RealBearer(t *testing.T, db *sqlx.DB, userID int64, tokenRol string) string {
	t.Helper()
	tok, _, err := jwtpkg.Issue(testJWTSecret, userID, tokenRol, "", 0, 0, 0)
	if err != nil {
		t.Fatalf("jwt: %v", err)
	}
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(tok)))
	if _, err := db.Exec(`INSERT INTO sesiones (usuario_id, token_hash, expires_at)
		VALUES ($1, $2, NOW() + INTERVAL '1 hour')`, userID, hash); err != nil {
		t.Fatalf("seed session: %v", err)
	}
	return "Bearer " + tok
}

// Route is "METHOD /path" with concrete path values, e.g. "PATCH /admin/x/1".
type Route string

// Do sends an empty-JSON request for route through mux with the bearer.
func Do(t *testing.T, mux http.Handler, route Route, bearer string) *httptest.ResponseRecorder {
	t.Helper()
	method, path, ok := strings.Cut(string(route), " ")
	if !ok {
		t.Fatalf("bad route %q", route)
	}
	req := httptest.NewRequest(method, path, strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", bearer)
	}
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	return rr
}

// AssertAdminOnly checks, through the real mux and the real session checker,
// that every route answers 401 without a token, 403 to a non-admin (even when
// the token itself claims "admin": the DB rol decides) and is mounted at all:
// an admin with a real session never gets 401/403/404/405.
func AssertAdminOnly(t *testing.T, db *sqlx.DB, mux http.Handler, routes []Route) {
	t.Helper()
	cat := SeedCatalog(t, db)
	user := SeedUser(t, db, cat, "usuario", "activo")
	admin := SeedUser(t, db, cat, "admin", "activo")
	userTok := RealBearer(t, db, user.ID, "usuario")
	forgedTok := RealBearer(t, db, user.ID, "admin")
	adminTok := RealBearer(t, db, admin.ID, "admin")
	for _, rt := range routes {
		if rr := Do(t, mux, rt, ""); rr.Code != http.StatusUnauthorized {
			t.Errorf("%s anonymous: want 401, got %d %s", rt, rr.Code, rr.Body)
		}
		if rr := Do(t, mux, rt, userTok); rr.Code != http.StatusForbidden {
			t.Errorf("%s as usuario: want 403, got %d %s", rt, rr.Code, rr.Body)
		}
		if rr := Do(t, mux, rt, forgedTok); rr.Code != http.StatusForbidden {
			t.Errorf("%s as usuario with a token claiming admin: want 403, got %d %s", rt, rr.Code, rr.Body)
		}
		switch rr := Do(t, mux, rt, adminTok); rr.Code {
		case http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusMethodNotAllowed:
			// 404 from a handler carries a JSON error; a bare mux 404 means
			// the route is not mounted.
			if rr.Code != http.StatusNotFound || !strings.Contains(rr.Body.String(), `"error"`) {
				t.Errorf("%s as admin: got %d %s (route not mounted or not admin-accessible)", rt, rr.Code, rr.Body)
			}
		}
	}
}
