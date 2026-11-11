package auth

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/cbaciliod/learning/internal/jwt"
)

type contextKey string

const claimsKey contextKey = "claims"

// Session is the server-side state of an authenticated session, read from the
// DB on every request. Rol and Estado are the user's CURRENT values, so role
// changes and deactivations take effect immediately, regardless of the JWT.
type Session struct {
	UserID int64
	Rol    string
	Estado string
}

// SessionChecker looks up the active session for a token hash. It returns
// (nil, nil) when the session does not exist, was revoked or expired.
type SessionChecker func(ctx context.Context, tokenHash string) (*Session, error)

// RequireAuth validates the Bearer JWT signature and checks the session exists
// in the DB. The claims placed in the context carry the role from the DB
// (not the one embedded in the token); users that are no longer activo get 401.
// If the session cannot be checked (DB error or timeout) it fails closed with
// 503 instead of 401, so a DB outage does not log every user out
// (docs/api/plataforma-salud.md § Autenticación con la BD caída).
func RequireAuth(secret string, checkSession SessionChecker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") {
				jsonError(w, http.StatusUnauthorized, "No autenticado.")
				return
			}
			tokenStr := strings.TrimPrefix(auth, "Bearer ")
			claims, err := jwt.Validate(secret, tokenStr)
			if err != nil {
				jsonError(w, http.StatusUnauthorized, "No autenticado.")
				return
			}
			// Verify the session is still active in the DB (enforces revocation,
			// max_sesiones, role changes and deactivation).
			tokenHash := fmt.Sprintf("%x", sha256.Sum256([]byte(tokenStr)))
			sess, err := checkSession(r.Context(), tokenHash)
			if err != nil {
				// The driver error may carry connection details: log the op only.
				log.Error().Str("op", "auth.check_session").Msg("session check failed")
				jsonError(w, http.StatusServiceUnavailable, ErrServicioNoDisponible.Error())
				return
			}
			if sess == nil || sess.UserID != claims.UserID {
				jsonError(w, http.StatusUnauthorized, "Sesión revocada.")
				return
			}
			if sess.Estado != "activo" {
				jsonError(w, http.StatusUnauthorized, "Sesión revocada.")
				return
			}
			current := *claims
			current.Rol = sess.Rol
			ctx := context.WithValue(r.Context(), claimsKey, &current)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAdmin checks that the authenticated user has rol=admin. It must run
// after RequireAuth, which loads the current role from the DB into the claims.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := r.Context().Value(claimsKey).(*jwt.Claims)
		if !ok || claims.Rol != "admin" {
			jsonError(w, http.StatusForbidden, "Acceso denegado.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ClaimsFromContext retrieves JWT claims stored by RequireAuth.
func ClaimsFromContext(ctx context.Context) *jwt.Claims {
	c, _ := ctx.Value(claimsKey).(*jwt.Claims)
	return c
}
