package auth

import (
	"net/http"
	"time"

	internalmw "github.com/cbaciliod/learning/internal/middleware"
)

// Mount registers all auth routes on the given mux.
// Public routes are mounted at /auth/*, admin routes at /admin/*.
func Mount(mux *http.ServeMux, svc *Service, jwtSecret string, checkSession SessionChecker) {
	h := NewHandler(svc)
	ah := NewAdminHandler(svc)

	auth := RequireAuth(jwtSecret, checkSession)
	admin := RequireAdmin

	// Rate limiters: 20 req/min for check endpoints, 10 req/min for login/register/password
	checkRL := internalmw.NewRateLimiter(20, time.Minute)
	authRL := internalmw.NewRateLimiter(10, time.Minute)

	// Public
	mux.Handle("GET /auth/check-dni", checkRL.Limit(http.HandlerFunc(h.CheckDNI)))
	mux.Handle("GET /auth/check-username", checkRL.Limit(http.HandlerFunc(h.CheckUsername)))
	mux.Handle("POST /auth/register", authRL.Limit(http.HandlerFunc(h.Register)))
	mux.Handle("POST /auth/verify-email", authRL.Limit(http.HandlerFunc(h.VerifyEmail)))
	mux.Handle("POST /auth/login", authRL.Limit(http.HandlerFunc(h.Login)))
	mux.Handle("POST /auth/forgot-password", authRL.Limit(http.HandlerFunc(h.ForgotPassword)))
	mux.Handle("POST /auth/reset-password", authRL.Limit(http.HandlerFunc(h.ResetPassword)))

	// Authenticated
	mux.Handle("POST /auth/logout", auth(http.HandlerFunc(h.Logout)))
	mux.Handle("GET /auth/me", auth(http.HandlerFunc(h.Me)))
	mux.Handle("POST /auth/tour/completar", auth(http.HandlerFunc(h.CompleteTour)))
	mux.Handle("DELETE /auth/tour/completar", auth(http.HandlerFunc(h.ResetTour)))

	// Admin only
	mux.Handle("GET /admin/personas", auth(admin(http.HandlerFunc(ah.ListPersonas))))
	mux.Handle("GET /admin/personas/{id}", auth(admin(http.HandlerFunc(ah.GetPersona))))
	mux.Handle("PUT /admin/personas/{id}", auth(admin(http.HandlerFunc(ah.UpdatePersona))))
	mux.Handle("GET /admin/usuarios", auth(admin(http.HandlerFunc(ah.ListUsuarios))))
	mux.Handle("PATCH /admin/usuarios/{id}/estado", auth(admin(http.HandlerFunc(ah.UpdateEstado))))
	mux.Handle("PATCH /admin/usuarios/{id}/email", auth(admin(http.HandlerFunc(ah.UpdateEmail))))
	mux.Handle("DELETE /admin/usuarios/{id}/tour", auth(admin(http.HandlerFunc(ah.ResetTourUsuario))))
	mux.Handle("DELETE /admin/registro-limites", auth(admin(http.HandlerFunc(ah.ResetRegistroLimites))))
	mux.Handle("POST /admin/personas/batch", auth(admin(http.HandlerFunc(ah.BatchImport))))
	mux.Handle("POST /admin/personas/preview", auth(admin(http.HandlerFunc(ah.PreviewPersonasCSV))))
	mux.Handle("POST /admin/personas/import", auth(admin(http.HandlerFunc(ah.ImportPersonasCSV))))

	// Config sesiones
	mux.Handle("GET /admin/config/sesiones", auth(admin(http.HandlerFunc(ah.GetConfigSesiones))))
	mux.Handle("PUT /admin/config/sesiones", auth(admin(http.HandlerFunc(ah.SetConfigSesiones))))

	// Config recursos (accesos directos del header)
	mux.Handle("GET /admin/config/recursos", auth(http.HandlerFunc(ah.GetConfigRecursos)))
	mux.Handle("PUT /admin/config/recursos", auth(admin(http.HandlerFunc(ah.SetConfigRecursos))))

	// Usuarios online
	mux.Handle("GET /admin/usuarios/online", auth(admin(http.HandlerFunc(ah.ListUsuariosOnline))))
	mux.Handle("GET /admin/usuarios/{id}/sesiones", auth(admin(http.HandlerFunc(ah.ListSesionesUsuario))))
}
