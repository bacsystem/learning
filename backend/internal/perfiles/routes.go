package perfiles

import (
	"net/http"

	"github.com/cbaciliod/learning/internal/auth"
)

// Mount registers F1–F3 for any authenticated user (contract §3). No route
// takes a usuario_id: the user is the session's, so nobody can name another
// user's profile. Unlike the admin modules there is no audit logger: the
// postulante's own choices are not audited (README §7).
func Mount(mux *http.ServeMux, svc *Service, jwtSecret string, checkSession auth.SessionChecker) {
	h := NewHandler(svc)
	authn := auth.RequireAuth(jwtSecret, checkSession)

	mux.Handle("GET /serums-ia/profesion/opciones", authn(http.HandlerFunc(h.Opciones)))
	mux.Handle("GET /serums-ia/profesion", authn(http.HandlerFunc(h.Perfil)))
	mux.Handle("PUT /serums-ia/profesion", authn(http.HandlerFunc(h.Elegir)))
}
