package temas

import (
	"net/http"

	"github.com/cbaciliod/learning/internal/audit"
	"github.com/cbaciliod/learning/internal/auth"
)

// Mount registers the tema routes (docs/api/serums-ia-e1-taxonomia.md §1).
// T1–T6 are admin only; T7 is open to any authenticated user.
func Mount(mux *http.ServeMux, svc *Service, audLog *audit.Logger, jwtSecret string, checkSession auth.SessionChecker) {
	h := NewHandler(svc, audLog)

	authn := auth.RequireAuth(jwtSecret, checkSession)
	admin := auth.RequireAdmin

	mux.Handle("GET /admin/temas", authn(admin(http.HandlerFunc(h.Listar))))
	mux.Handle("GET /admin/temas/{id}", authn(admin(http.HandlerFunc(h.Obtener))))
	mux.Handle("POST /admin/temas", authn(admin(http.HandlerFunc(h.Crear))))
	mux.Handle("PUT /admin/temas/{id}", authn(admin(http.HandlerFunc(h.Editar))))
	mux.Handle("PATCH /admin/temas/{id}/estado", authn(admin(http.HandlerFunc(h.CambiarEstado))))
	mux.Handle("PUT /admin/asignaturas/{id}/temas/orden", authn(admin(http.HandlerFunc(h.Reordenar))))

	mux.Handle("GET /asignaturas/{id}/temas", authn(http.HandlerFunc(h.ListarActivos)))
}
