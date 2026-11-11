package salud

import (
	"net/http"

	"github.com/cbaciliod/learning/internal/auth"
)

// Mount registers GET /admin/salud, admin only (docs/api/plataforma-salud.md S1).
func Mount(mux *http.ServeMux, svc *Service, jwtSecret string, checkSession auth.SessionChecker) {
	h := NewHandler(svc)
	authn := auth.RequireAuth(jwtSecret, checkSession)
	mux.Handle("GET /admin/salud", authn(auth.RequireAdmin(http.HandlerFunc(h.Obtener))))
}
