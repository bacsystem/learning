package balotario

import (
	"net/http"

	"github.com/cbaciliod/learning/internal/audit"
	"github.com/cbaciliod/learning/internal/auth"
)

// Route patterns of the balotario import (contract E3-01/E3-03 §3, §4).
const (
	RutaVistaPrevia = "GET /admin/fuentes/{id}/balotario"
	RutaConfirmar   = "POST /admin/fuentes/{id}/balotario/importaciones"
)

// Mount registers the admin routes of the balotario import.
func Mount(mux *http.ServeMux, svc *Service, audLog *audit.Logger, jwtSecret string, checkSession auth.SessionChecker) {
	h := NewHandler(svc, audLog)
	authn := auth.RequireAuth(jwtSecret, checkSession)
	admin := auth.RequireAdmin

	mux.Handle(RutaVistaPrevia, authn(admin(http.HandlerFunc(h.VistaPrevia))))
	mux.Handle(RutaConfirmar, authn(admin(http.HandlerFunc(h.Confirmar))))
}
