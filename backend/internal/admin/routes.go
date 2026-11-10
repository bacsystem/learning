package admin

import (
	"crypto/cipher"
	"net/http"

	"github.com/cbaciliod/learning/internal/audit"
	"github.com/cbaciliod/learning/internal/auth"
	"github.com/jmoiron/sqlx"
)

// RegisterRoutes mounts all /admin/... routes for the admin package.
func RegisterRoutes(mux *http.ServeMux, db *sqlx.DB, gcm cipher.AEAD, rawKey []byte, jwtSecret string, checkSession auth.SessionChecker, audLog *audit.Logger) {
	repo := NewRepository(db, gcm, rawKey)
	h := NewHandler(repo, audLog)

	requireAuth := auth.RequireAuth(jwtSecret, checkSession)
	requireAdmin := auth.RequireAdmin

	// Dashboard
	mux.Handle("GET /admin/dashboard", requireAuth(requireAdmin(http.HandlerFunc(h.handleDashboard))))
	mux.Handle("GET /admin/dashboard/actividad", requireAuth(requireAdmin(http.HandlerFunc(h.handleDashboardActividad))))

	// Usuario detalle
	mux.Handle("GET /admin/usuarios/{id}", requireAuth(requireAdmin(http.HandlerFunc(h.handleGetUsuarioDetalle))))
	mux.Handle("PATCH /admin/usuarios/{id}/rol", requireAuth(requireAdmin(http.HandlerFunc(h.handlePatchUsuarioRol))))

	// Audit log
	mux.Handle("GET /admin/audit-log", requireAuth(requireAdmin(http.HandlerFunc(h.handleGetAuditLog))))

	// Ingresos
	mux.Handle("GET /admin/ingresos", requireAuth(requireAdmin(http.HandlerFunc(h.handleGetIngresos))))
	mux.Handle("GET /admin/ingresos/config", requireAuth(requireAdmin(http.HandlerFunc(h.handleGetIngresosConfig))))
	mux.Handle("PUT /admin/ingresos/config/{anio}", requireAuth(requireAdmin(http.HandlerFunc(h.handlePutIngresosConfig))))
}
