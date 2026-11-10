package notifications

import (
	"net/http"

	"github.com/cbaciliod/learning/internal/audit"
	"github.com/cbaciliod/learning/internal/auth"
	"github.com/jmoiron/sqlx"
)

// Mount registers notification routes on the given mux.
func Mount(mux *http.ServeMux, db *sqlx.DB, audLog *audit.Logger, jwtSecret string, checkSession auth.SessionChecker) {
	repo := NewRepository(db)
	h := NewHandler(repo, audLog)

	authn := auth.RequireAuth(jwtSecret, checkSession)
	admin := auth.RequireAdmin

	// User routes
	mux.Handle("GET /notificaciones", authn(http.HandlerFunc(h.List)))
	mux.Handle("GET /notificaciones/no-leidas", authn(http.HandlerFunc(h.UnreadCount)))
	mux.Handle("POST /notificaciones/{id}/leer", authn(http.HandlerFunc(h.MarkRead)))
	mux.Handle("POST /notificaciones/leer-todo", authn(http.HandlerFunc(h.MarkAllRead)))

	// Admin routes
	mux.Handle("GET /admin/notificaciones", authn(admin(http.HandlerFunc(h.AdminList))))
	mux.Handle("POST /admin/notificaciones", authn(admin(http.HandlerFunc(h.AdminCreate))))
	mux.Handle("DELETE /admin/notificaciones/{id}", authn(admin(http.HandlerFunc(h.AdminDelete))))
}
