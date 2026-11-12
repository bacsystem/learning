package balotario

import (
	"net/http"

	"github.com/cbaciliod/learning/internal/auth"
)

// Route patterns of the balotario import (contract E3-01/E3-03 §3, §4).
const (
	RutaVistaPrevia = "GET /admin/fuentes/{id}/balotario"
)

// Mount registers the admin routes of the balotario import.
func Mount(mux *http.ServeMux, svc *Service, jwtSecret string, checkSession auth.SessionChecker) {
	h := NewHandler(svc)
	authn := auth.RequireAuth(jwtSecret, checkSession)
	admin := auth.RequireAdmin

	mux.Handle(RutaVistaPrevia, authn(admin(http.HandlerFunc(h.VistaPrevia))))
}
