package ia

import (
	"net/http"

	"github.com/cbaciliod/learning/internal/audit"
	"github.com/cbaciliod/learning/internal/auth"
)

// Mount registers the routes of the cost control (docs/api/serums-ia-e6-02-control-costos.md §3):
// the consumption of the authenticated user, and the admin's configuration
// and spent amount.
func Mount(mux *http.ServeMux, svc administracion, audLog *audit.Logger, jwtSecret string, checkSession auth.SessionChecker) {
	h := NewHandler(svc, audLog)
	authn := auth.RequireAuth(jwtSecret, checkSession)
	admin := auth.RequireAdmin

	mux.Handle("GET /serums-ia/ia/consumo", authn(http.HandlerFunc(h.Consumo)))
	mux.Handle("GET /admin/ia/configuracion", authn(admin(http.HandlerFunc(h.Configuracion))))
	mux.Handle("PUT /admin/ia/configuracion", authn(admin(http.HandlerFunc(h.Configurar))))
	mux.Handle("GET /admin/ia/gasto", authn(admin(http.HandlerFunc(h.Gasto))))
}
